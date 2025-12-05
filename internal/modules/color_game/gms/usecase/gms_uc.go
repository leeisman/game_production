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
	"github.com/frankieli/game_product/pkg/service"
	"github.com/frankieli/game_product/pkg/service/color_game"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// GMSUseCase handles game round logic
type GMSUseCase struct {
	stateMachine       *machine.StateMachine
	betCount           map[string]int                // roundID -> bet count
	betPlayers         map[string]map[int64]struct{} // roundID -> set of userIDs
	betAmount          map[string]float64            // roundID -> total bet amount
	gatewayBroadcaster service.GatewayService
	gsBroadcaster      color_game.ColorGameGSService
	gameRoundRepo      domain.GameRoundRepository
	mu                 sync.RWMutex
}

// NewGMSUseCase creates a new round use case
func NewGMSUseCase(stateMachine *machine.StateMachine, gatewayBroadcaster service.GatewayService, gsBroadcaster color_game.ColorGameGSService, gameRoundRepo domain.GameRoundRepository) *GMSUseCase {
	uc := &GMSUseCase{
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

// SetGSService sets the GS service
func (uc *GMSUseCase) SetGSService(gsService color_game.ColorGameGSService) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.gsBroadcaster = gsService
}

// handleGameEvent handles game events from the state machine
func (uc *GMSUseCase) handleGameEvent(event machine.GameEvent) {
	// Convert event to Proto
	// Map state machine events to ColorGameRoundStateBRC
	switch event.Type {
	case pbColorGame.ColorGameState_GAME_STATE_ROUND_STARTED,
		pbColorGame.ColorGameState_GAME_STATE_BETTING,
		pbColorGame.ColorGameState_GAME_STATE_DRAWING,
		pbColorGame.ColorGameState_GAME_STATE_RESULT,
		pbColorGame.ColorGameState_GAME_STATE_ROUND_ENDED,
		pbColorGame.ColorGameState_GAME_STATE_STOPPED:

		// Convert event type enum to string
		brc := &pbColorGame.ColorGameRoundStateBRC{
			RoundId:             event.RoundID,
			State:               event.Type,
			BettingEndTimestamp: event.BettingEndTimestamp,
			LeftTime:            event.LeftTime,
		}

		// Broadcast BRC to gateway
		if uc.gatewayBroadcaster != nil {
			uc.gatewayBroadcaster.Broadcast("color_game", brc)
		}

	default:
		// For other events - ignore or log
		// We are moving away from generic ColorGameEvent
	}

	// Update DB
	if uc.gameRoundRepo != nil {
		ctx := context.Background()
		switch event.Type {
		case pbColorGame.ColorGameState_GAME_STATE_ROUND_STARTED:
			uc.gameRoundRepo.Create(ctx, &domain.GameRound{
				RoundID:   event.RoundID,
				GameCode:  "color_game",
				Status:    domain.RoundStatusInProgress,
				StartTime: time.Now(),
			})

		case pbColorGame.ColorGameState_GAME_STATE_RESULT:
			endTime := time.Now()

			// Get stats from memory
			uc.mu.RLock()
			totalBets := uc.betCount[event.RoundID]
			totalPlayers := len(uc.betPlayers[event.RoundID])
			totalAmount := uc.betAmount[event.RoundID]
			uc.mu.RUnlock()

			uc.gameRoundRepo.UpdateResult(ctx, event.RoundID, event.Data.(pbColorGame.ColorGameReward).String(), &endTime, totalBets, totalPlayers, totalAmount)
		}
	}

	uc.mu.RLock()
	defer uc.mu.RUnlock()

	// Broadcast to GS - result event (filtering done in adapter)
	if event.Type == pbColorGame.ColorGameState_GAME_STATE_RESULT && uc.gsBroadcaster != nil {
		req := &pbColorGame.ColorGameRoundResultReq{
			RoundId: event.RoundID,
			Result:  event.Data.(pbColorGame.ColorGameReward),
		}
		// Create a background context for the call
		ctx := context.Background()
		_, _ = uc.gsBroadcaster.RoundResult(ctx, req) // Ignore response for now as it's fire-and-forget
	}
}

// IncrementBetCount increments the bet count for the current round
func (uc *GMSUseCase) IncrementBetCount(ctx context.Context, roundID string, userID int64, amount float64) error {
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

func (uc *GMSUseCase) GetCurrentRound(ctx context.Context) (*domain.Round, error) {
	roundView := uc.stateMachine.GetCurrentRound()
	if roundView.RoundID == "" {
		return nil, fmt.Errorf("no active round")
	}

	return &domain.Round{
		RoundID:    roundView.RoundID,
		State:      roundView.State,
		BettingEnd: roundView.BettingEnd,
		LeftTime:   roundView.LeftTime,
	}, nil
}

// RegisterEventHandler registers an additional event handler
func (uc *GMSUseCase) RegisterEventHandler(handler machine.EventHandler) {
	uc.stateMachine.RegisterEventHandler(handler)
}
