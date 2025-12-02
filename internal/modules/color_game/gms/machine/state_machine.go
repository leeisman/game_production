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
	Type    pbColorGame.EventType
	RoundID string
	Data    interface{}
}

// EventHandler handles game events
type EventHandler func(event GameEvent)

// RoundView is a read-only snapshot of current round
type RoundView struct {
	RoundID    string
	State      domain.GameState
	Result     domain.Color
	StartTime  time.Time
	BettingEnd time.Time
	TotalBets  int
}

// StateMachine manages the game state machine (application layer)
type StateMachine struct {
	mu           sync.RWMutex
	currentRound *domain.Round
	roundCounter int

	eventHandlers []EventHandler
	rnd           *rand.Rand

	// durations for each phase
	BettingDuration time.Duration
	DrawingDuration time.Duration
	ResultDuration  time.Duration

	stopping bool
}

// NewStateMachine creates a new state machine
func NewStateMachine() *StateMachine {
	return &StateMachine{
		eventHandlers:   make([]EventHandler, 0),
		rnd:             rand.New(rand.NewSource(time.Now().UnixNano())),
		BettingDuration: 10 * time.Second,
		DrawingDuration: 2 * time.Second,
		ResultDuration:  5 * time.Second,
	}
}

// RegisterEventHandler registers an event handler
func (sm *StateMachine) RegisterEventHandler(handler EventHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.eventHandlers = append(sm.eventHandlers, handler)
}

// emitEvent emits an event to all handlers
func (sm *StateMachine) emitEvent(event GameEvent) {
	sm.mu.RLock()
	handlers := make([]EventHandler, len(sm.eventHandlers))
	copy(handlers, sm.eventHandlers)
	sm.mu.RUnlock()

	for _, handler := range handlers {
		go handler(event)
	}
}

// Stop signals the state machine to stop after the current round
func (sm *StateMachine) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.stopping = true
}

// Start starts the state machine loop
func (sm *StateMachine) Start(ctx context.Context) {
	logger.Info(ctx).Msg("🚀 [GMS] State Machine Started")
	for {
		sm.mu.RLock()
		stopping := sm.stopping
		sm.mu.RUnlock()

		if stopping {
			logger.Info(ctx).Msg("🛑 [GMS] State Machine Stopping (Graceful)")
			sm.emitEvent(GameEvent{
				Type:    pbColorGame.EventType_EVENT_TYPE_MACHINE_STOPPED,
				RoundID: "",
			})
			return
		}

		sm.runRound(ctx)
	}
}

// runRound executes a single round
func (sm *StateMachine) runRound(ctx context.Context) {
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
		Type:    pbColorGame.EventType_EVENT_TYPE_ROUND_STARTED,
		RoundID: roundID,
	})

	//--------------------------------------------
	// Betting phase
	//--------------------------------------------
	sm.mu.Lock()
	round.StartBetting(sm.BettingDuration)
	bettingEnd := round.BettingEnd
	sm.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Time("betting_end", bettingEnd).
		Dur("duration", sm.BettingDuration).
		Msg("🟢 [GMS] 開始下注 (Betting Started)")

	sm.emitEvent(GameEvent{
		Type:    pbColorGame.EventType_EVENT_TYPE_BETTING_STARTED,
		RoundID: roundID,
		Data:    bettingEnd,
	})

	time.Sleep(sm.BettingDuration)

	//--------------------------------------------
	// Drawing phase
	//--------------------------------------------
	result := sm.drawResult()

	sm.mu.Lock()
	round.Draw(result)
	sm.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Str("result_color", string(result)).
		Dur("duration", sm.DrawingDuration).
		Msg("🎲 [GMS] 停止下注，正在開獎 (Drawing)")

	sm.emitEvent(GameEvent{
		Type:    pbColorGame.EventType_EVENT_TYPE_DRAWING,
		RoundID: roundID,
		Data:    result,
	})

	time.Sleep(sm.DrawingDuration)

	//--------------------------------------------
	// Result phase
	//--------------------------------------------
	sm.mu.Lock()
	round.ShowResult()
	sm.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Str("final_result", string(result)).
		Dur("duration", sm.ResultDuration).
		Msg("📊 [GMS] 公布結果 (Show Result)")

	sm.emitEvent(GameEvent{
		Type:    pbColorGame.EventType_EVENT_TYPE_RESULT,
		RoundID: roundID,
		Data:    result,
	})

	time.Sleep(sm.ResultDuration)

	logger.Info(ctx).
		Str("round_id", roundID).
		Msg("🏁 [GMS] 回合結束 (Round Ended)")

	sm.emitEvent(GameEvent{
		Type:    pbColorGame.EventType_EVENT_TYPE_ROUND_ENDED,
		RoundID: roundID,
	})
}

func (sm *StateMachine) drawResult() domain.Color {
	colors := []domain.Color{
		domain.ColorRed,
		domain.ColorGreen,
		domain.ColorBlue,
		domain.ColorYellow,
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
	return RoundView{
		RoundID:    r.RoundID,
		State:      r.State,
		Result:     r.Result,
		StartTime:  r.StartTime,
		BettingEnd: r.BettingEnd,
		TotalBets:  r.TotalBets,
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
