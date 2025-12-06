package machine

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	"github.com/frankieli/game_product/pkg/logger"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// GameEvent represents a game event
type GameEvent struct {
	Type                pbColorGame.ColorGameState
	RoundID             string
	Data                interface{}
	LeftTime            int64
	BettingEndTimestamp int64
}

// EventHandler handles game events
type EventHandler func(event GameEvent)

// RoundView is a read-only snapshot of current round
type RoundView struct {
	RoundID    string
	State      pbColorGame.ColorGameState
	Result     domain.Color
	StartTime  time.Time
	BettingEnd time.Time
	TotalBets  int
	LeftTime   int64
}

// StateMachine manages the game state machine (application layer)
type StateMachine struct {
	mu           sync.RWMutex
	currentRound *domain.Round
	roundCounter int

	eventHandlers []EventHandler
	rnd           *rand.Rand

	// Worker Pool
	jobQueue chan func()
	workerWg sync.WaitGroup

	// durations for each phase
	BettingDuration time.Duration
	DrawingDuration time.Duration
	ResultDuration  time.Duration
	WaitDuration    time.Duration
	RestDuration    time.Duration
	phaseEndTime    time.Time

	stopping bool
}

// NewStateMachine creates a new state machine
func NewStateMachine() *StateMachine {
	return &StateMachine{
		eventHandlers:   make([]EventHandler, 0),
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano())),
		jobQueue:        make(chan func(), 100), // Buffered job queue
		BettingDuration: 10 * time.Second,
		DrawingDuration: 2 * time.Second,
		ResultDuration:  5 * time.Second,
		WaitDuration:    2 * time.Second,
		RestDuration:    3 * time.Second,
	}
}

// RegisterEventHandler registers an event handler
func (sm *StateMachine) RegisterEventHandler(handler EventHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.eventHandlers = append(sm.eventHandlers, handler)
}

// emitEvent emits an event to all handlers via worker pool
func (sm *StateMachine) emitEvent(event GameEvent) {
	sm.mu.RLock()
	handlers := make([]EventHandler, len(sm.eventHandlers))
	copy(handlers, sm.eventHandlers)
	sm.mu.RUnlock()

	for _, handler := range handlers {
		// Capture closure variables
		h := handler
		evt := event

		select {
		case sm.jobQueue <- func() { h(evt) }:
			// Job submitted successfully
		default:
			// Queue is full, degrade to spawning a goroutine directly
			logger.Warn(context.Background()).
				Str("round_id", event.RoundID).
				Str("event_type", event.Type.String()).
				Msg("⚠️ [GMS] Job queue full, degrading to goroutine")
			go h(evt)
		}
	}
}

// startWorkers starts the worker pool
func (sm *StateMachine) startWorkers() {
	workerCount := 5
	for i := 0; i < workerCount; i++ {
		sm.workerWg.Add(1)
		go func(workerID int) {
			defer sm.workerWg.Done()
			for job := range sm.jobQueue {
				// Execute job with panic recovery
				func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error(context.Background()).
								Interface("panic", r).
								Int("worker_id", workerID).
								Msg("🔥 [GMS] Worker panic recovered")
						}
					}()
					job()
				}()
			}
		}(i)
	}
}

// shutdownWorkers stops key workers
func (sm *StateMachine) shutdownWorkers() {
	close(sm.jobQueue)
	sm.workerWg.Wait()
}

// Stop signals the state machine to stop after the current round
func (sm *StateMachine) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.stopping = true
}

// Start starts the state machine loop
func (sm *StateMachine) Start(ctx context.Context) {
	logger.Info(ctx).Msg("🚀 [GMS] State Machine Started (Worker Pool Active)")

	// Start workers
	sm.startWorkers()
	defer sm.shutdownWorkers()

	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx).Msg("🛑 [GMS] Context Cancelled, Stopping State Machine")
			return
		default:
		}

		sm.mu.RLock()
		stopping := sm.stopping
		sm.mu.RUnlock()

		if stopping {
			logger.Info(ctx).Msg("🛑 [GMS] State Machine Stopping (Graceful)")
			sm.emitEvent(GameEvent{
				Type:                pbColorGame.ColorGameState_GAME_STATE_STOPPED,
				RoundID:             "",
				LeftTime:            0,
				BettingEndTimestamp: 0,
			})
			return
		}

		sm.runRound(ctx)
	}
}

// sleepWithContext sleeps for duration d, or until ctx is cancelled
func sleepWithContext(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// runRound executes a single round
func (sm *StateMachine) runRound(ctx context.Context) {
	// ... (rest of the function using sleepWithContext)
	sm.mu.Lock()
	sm.roundCounter++
	roundID := sm.generateRoundID()
	sm.currentRound = domain.NewRound(roundID)
	round := sm.currentRound
	sm.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Int("round_counter", sm.roundCounter).
		Msg("🔄 [GMS] 回合開始 (Round Started)")

	sm.emitEvent(GameEvent{
		Type:                pbColorGame.ColorGameState_GAME_STATE_ROUND_STARTED,
		RoundID:             roundID,
		LeftTime:            int64(sm.WaitDuration.Seconds()),
		BettingEndTimestamp: 0,
	})

	sleepWithContext(ctx, sm.WaitDuration)
	if ctx.Err() != nil {
		return
	}

	//--------------------------------------------
	// Betting phase
	//--------------------------------------------
	sm.mu.Lock()
	round.StartBetting(sm.BettingDuration)
	bettingEnd := round.BettingEnd
	sm.phaseEndTime = bettingEnd
	sm.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Time("betting_end", bettingEnd).
		Dur("duration", sm.BettingDuration).
		Msg("🟢 [GMS] 開始下注 (Betting Started)")

	sm.emitEvent(GameEvent{
		Type:                pbColorGame.ColorGameState_GAME_STATE_BETTING,
		RoundID:             roundID,
		Data:                bettingEnd,
		LeftTime:            int64(sm.BettingDuration.Seconds()),
		BettingEndTimestamp: bettingEnd.Unix(),
	})

	sleepWithContext(ctx, sm.BettingDuration)
	if ctx.Err() != nil {
		return
	}

	//--------------------------------------------
	// Drawing phase
	//--------------------------------------------
	result := sm.drawResult()

	sm.mu.Lock()
	round.Draw(result)
	sm.phaseEndTime = time.Now().Add(sm.DrawingDuration)
	sm.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Str("result_color", result.String()).
		Dur("duration", sm.DrawingDuration).
		Msg("🎲 [GMS] 停止下注，正在開獎 (Drawing)")

	sm.emitEvent(GameEvent{
		Type:                pbColorGame.ColorGameState_GAME_STATE_DRAWING,
		RoundID:             roundID,
		Data:                result,
		LeftTime:            int64(sm.DrawingDuration.Seconds()),
		BettingEndTimestamp: round.BettingEnd.Unix(),
	})

	sleepWithContext(ctx, sm.DrawingDuration)
	if ctx.Err() != nil {
		return
	}

	//--------------------------------------------
	// Result phase
	//--------------------------------------------
	sm.mu.Lock()
	round.ShowResult()
	sm.phaseEndTime = time.Now().Add(sm.ResultDuration)
	sm.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Str("final_result", result.String()).
		Dur("duration", sm.ResultDuration).
		Msg("📊 [GMS] 公布結果 (Show Result)")

	sm.emitEvent(GameEvent{
		Type:                pbColorGame.ColorGameState_GAME_STATE_RESULT,
		RoundID:             roundID,
		Data:                result,
		LeftTime:            int64(sm.ResultDuration.Seconds()),
		BettingEndTimestamp: round.BettingEnd.Unix(),
	})

	sleepWithContext(ctx, sm.ResultDuration)
	if ctx.Err() != nil {
		return
	}

	//--------------------------------------------
	// Round Ended phase (Rest)
	//--------------------------------------------
	logger.Info(ctx).
		Str("round_id", roundID).
		Msg("🏁 [GMS] 回合結束 (Round Ended)")

	sm.emitEvent(GameEvent{
		Type:                pbColorGame.ColorGameState_GAME_STATE_ROUND_ENDED,
		RoundID:             roundID,
		Data:                nil,
		LeftTime:            int64(sm.RestDuration.Seconds()),
		BettingEndTimestamp: round.BettingEnd.Unix(),
	})

	sm.mu.Lock()
	sm.phaseEndTime = time.Now().Add(sm.RestDuration)
	sm.mu.Unlock()

	sleepWithContext(ctx, sm.RestDuration)
}

// drawResult simulates drawing a result
func (sm *StateMachine) drawResult() domain.Color {
	colors := []domain.Color{
		pbColorGame.ColorGameReward_REWARD_RED,
		pbColorGame.ColorGameReward_REWARD_GREEN,
		pbColorGame.ColorGameReward_REWARD_BLUE,
		pbColorGame.ColorGameReward_REWARD_YELLOW,
	}
	return colors[sm.rnd.Intn(len(colors))]
}

func (sm *StateMachine) generateRoundID() string {
	return time.Now().Format("20060102150405")
}

// GetCurrentRound returns a snapshot of the current round (thread-safe)
func (sm *StateMachine) GetCurrentRound() RoundView {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.currentRound == nil {
		return RoundView{}
	}

	r := sm.currentRound
	leftTime := int64(time.Until(sm.phaseEndTime).Seconds())
	if leftTime < 0 {
		leftTime = 0
	}

	return RoundView{
		RoundID:    r.RoundID,
		State:      r.State,
		Result:     r.Result,
		StartTime:  r.StartTime,
		BettingEnd: r.BettingEnd,
		TotalBets:  r.TotalBets,
		LeftTime:   leftTime,
	}
}

// CanAcceptBet checks if current round can accept bets
func (sm *StateMachine) CanAcceptBet() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.currentRound == nil {
		return false
	}
	return sm.currentRound.CanAcceptBet()
}
