// Package memory provides memory-based repositories for the color game GS module.
package memory

import (
	"context"
	"sync"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
)

// BetRepository implements domain.BetRepository using memory
type BetRepository struct {
	bets            map[string][]*domain.Bet // roundID -> bets (History for GetUserBets)
	settlementQueue map[string][]*domain.Bet // roundID -> bets (Queue for GetBetsForSettlement)
	mu              sync.RWMutex
}

// NewBetRepository creates a new memory bet repository
func NewBetRepository() *BetRepository {
	return &BetRepository{
		bets:            make(map[string][]*domain.Bet),
		settlementQueue: make(map[string][]*domain.Bet),
	}
}

func (r *BetRepository) SaveBet(ctx context.Context, bet *domain.Bet) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Add to history
	if r.bets[bet.RoundID] == nil {
		r.bets[bet.RoundID] = make([]*domain.Bet, 0)
	}
	r.bets[bet.RoundID] = append(r.bets[bet.RoundID], bet)

	// 2. Add to settlement queue
	if r.settlementQueue[bet.RoundID] == nil {
		r.settlementQueue[bet.RoundID] = make([]*domain.Bet, 0)
	}
	r.settlementQueue[bet.RoundID] = append(r.settlementQueue[bet.RoundID], bet)

	return nil
}

func (r *BetRepository) GetBets(ctx context.Context, roundID string) ([]*domain.Bet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bets := r.bets[roundID]
	if bets == nil {
		return make([]*domain.Bet, 0), nil
	}
	return bets, nil
}

func (r *BetRepository) GetUserBets(ctx context.Context, roundID string, userID int64) ([]*domain.Bet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allBets := r.bets[roundID]
	userBets := make([]*domain.Bet, 0)

	for _, bet := range allBets {
		if bet.UserID == userID {
			userBets = append(userBets, bet)
		}
	}

	return userBets, nil
}

func (r *BetRepository) ClearBets(ctx context.Context, roundID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.bets, roundID)
	delete(r.settlementQueue, roundID)
	return nil
}

func (r *BetRepository) GetBetsForSettlement(ctx context.Context, roundID string) ([]*domain.Bet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	bets := r.settlementQueue[roundID]
	if len(bets) == 0 {
		return nil, nil
	}

	// Return all bets in the queue and clear the queue
	// This simulates "popping" all items
	delete(r.settlementQueue, roundID)

	return bets, nil
}

func (r *BetRepository) GetUserBet(ctx context.Context, roundID string, userID int64, color domain.Color) (*domain.Bet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allBets := r.bets[roundID]
	for _, bet := range allBets {
		if bet.UserID == userID && bet.Color == color {
			return bet, nil
		}
	}

	return nil, nil // Not found
}

func (r *BetRepository) UpdateBetAmount(ctx context.Context, bet *domain.Bet, additionalAmount int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update the bet amount (bet is a pointer, so it updates in place)
	bet.Amount += additionalAmount

	return nil
}
