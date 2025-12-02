// Package usecase implements the business logic for the color game GMS module.
package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	"github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	"github.com/frankieli/game_product/pkg/logger"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// RoundUseCase handles round management logic
type RoundUseCase struct {
	stateMachine       *machine.StateMachine
	betCount           map[string]int                // roundID -> bet count
	betPlayers         map[string]map[int64]struct{} // roundID -> set of userIDs
	betAmount          map[string]float64            // roundID -> total bet amount
	gatewayBroadcaster domain.Broadcaster
	gsBroadcaster      domain.Broadcaster
	gameRoundRepo      domain.GameRoundRepository
	mu                 sync.RWMutex
}

// NewRoundUseCase creates a new round use case
func NewRoundUseCase(stateMachine *machine.StateMachine, gatewayBroadcaster domain.Broadcaster, gsBroadcaster domain.Broadcaster, gameRoundRepo domain.GameRoundRepository) *RoundUseCase {
	uc := &RoundUseCase{
		stateMachine:       stateMachine,
		betCount:           make(map[string]int),
		betPlayers:         make(map[string]map[int64]struct{}),
		betAmount:          make(map[string]float64),
		gatewayBroadcaster: gatewayBroadcaster,
		gsBroadcaster:      gsBroadcaster,
		gameRoundRepo:      gameRoundRepo,
	}

	// Register event handler to broadcast game events
	stateMachine.RegisterEventHandler(uc.handleGameEvent)

	return uc
}

// SetGSBroadcaster sets the GS broadcaster (to resolve circular dependency)
func (uc *RoundUseCase) SetGSBroadcaster(broadcaster domain.Broadcaster) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.gsBroadcaster = broadcaster
}

// handleGameEvent handles game events from state machine and broadcasts to players
func (uc *RoundUseCase) handleGameEvent(event machine.GameEvent) {
	// Convert event to Proto
	pbEvent := &pbColorGame.GameEvent{
		Type:      event.Type,
		RoundId:   event.RoundID,
		Data:      fmt.Sprintf("%v", event.Data), // Convert data to string
		Timestamp: time.Now().Unix(),
	}

	// Update DB
	if uc.gameRoundRepo != nil {
		ctx := context.Background()
		switch event.Type {
		case pbColorGame.EventType_EVENT_TYPE_ROUND_STARTED:
			uc.gameRoundRepo.Create(ctx, &domain.GameRound{
				RoundID:   event.RoundID,
				GameCode:  "color_game",
				Status:    domain.RoundStatusInProgress,
				StartTime: time.Now(),
			})

		case pbColorGame.EventType_EVENT_TYPE_RESULT:
			endTime := time.Now()

			// Get stats from memory
			uc.mu.RLock()
			totalBets := uc.betCount[event.RoundID]
			totalPlayers := len(uc.betPlayers[event.RoundID])
			totalAmount := uc.betAmount[event.RoundID]
			uc.mu.RUnlock()

			uc.gameRoundRepo.UpdateResult(ctx, event.RoundID, fmt.Sprintf("%v", event.Data), &endTime, totalBets, totalPlayers, totalAmount)
		}
	}

	uc.mu.RLock()
	defer uc.mu.RUnlock()

	// Broadcast to gateway (WebSocket clients) - all events
	if uc.gatewayBroadcaster != nil {
		uc.gatewayBroadcaster.Broadcast(pbEvent)
	}

	// Broadcast to GS - result event (filtering done in adapter)
	if pbEvent.Type == pbColorGame.EventType_EVENT_TYPE_RESULT && uc.gsBroadcaster != nil {
		uc.gsBroadcaster.Broadcast(pbEvent)
	}
}

// IncrementBetCount increments the bet count for a round (for coordination only)
// GMS doesn't need to know bet details (color) - that's GS's responsibility
// But it needs userID to count unique players
func (uc *RoundUseCase) IncrementBetCount(ctx context.Context, roundID string, userID int64, amount float64) error {
	logger.Debug(ctx).
		Str("round_id", roundID).
		Int64("user_id", userID).
		Msg("GMS 接收下注计数请求")

	// Check if can accept bet
	if !uc.stateMachine.CanAcceptBet() {
		logger.Warn(ctx).
			Str("round_id", roundID).
			Msg("当前状态不接受下注")
		return fmt.Errorf("betting not allowed in current state")
	}

	currentRound := uc.stateMachine.GetCurrentRound()
	if currentRound.RoundID == "" || currentRound.RoundID != roundID {
		logger.Warn(ctx).
			Str("round_id", roundID).
			Str("current_round_id", currentRound.RoundID).
			Msg("回合 ID 不匹配")
		return fmt.Errorf("invalid round ID")
	}

	// Increment bet count (thread-safe, handles concurrency across multiple GS instances)
	uc.mu.Lock()
	uc.betCount[roundID]++
	if uc.betPlayers[roundID] == nil {
		uc.betPlayers[roundID] = make(map[int64]struct{})
	}
	uc.betPlayers[roundID][userID] = struct{}{}
	uc.betAmount[roundID] += amount
	count := uc.betCount[roundID]
	uc.mu.Unlock()

	logger.Info(ctx).
		Str("round_id", roundID).
		Int("total_bets", count).
		Msg("GMS 下注计数成功")

	return nil
}

// GetCurrentRound returns current round info
func (uc *RoundUseCase) GetCurrentRound(ctx context.Context) (*domain.Round, error) {
	roundView := uc.stateMachine.GetCurrentRound()
	if roundView.RoundID == "" {
		return nil, fmt.Errorf("no active round")
	}

	return &domain.Round{
		RoundID:    roundView.RoundID,
		State:      roundView.State,
		BettingEnd: roundView.BettingEnd,
	}, nil
}

// RegisterEventHandler registers an additional event handler
func (uc *RoundUseCase) RegisterEventHandler(handler machine.EventHandler) {
	uc.stateMachine.RegisterEventHandler(handler)
}
