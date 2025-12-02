package wallet

import (
	"context"
	"sync"
)

// MockService implements contract.WalletService with mock logic
type MockService struct {
	balances map[int64]int64
	mu       sync.RWMutex
}

// NewMockService creates a new mock wallet service
func NewMockService() *MockService {
	return &MockService{
		balances: make(map[int64]int64),
	}
}

// SetBalance sets the balance for a user (for testing)
func (s *MockService) SetBalance(userID int64, balance int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.balances[userID] = balance
}

// GetBalance returns the user's balance
func (s *MockService) GetBalance(ctx context.Context, userID int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	balance, exists := s.balances[userID]
	if !exists {
		return 1000000, nil // Default balance for testing
	}
	return balance, nil
}

// DeductBalance deducts balance (PlaceBet equivalent)
func (s *MockService) DeductBalance(ctx context.Context, userID int64, amount int64, reason string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	balance, exists := s.balances[userID]
	if !exists {
		balance = 1000000 // Default balance
	}

	newBalance := balance - amount
	s.balances[userID] = newBalance
	return newBalance, nil
}

// AddBalance adds balance (SettleWin equivalent)
func (s *MockService) AddBalance(ctx context.Context, userID int64, amount int64, reason string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	balance, exists := s.balances[userID]
	if !exists {
		balance = 0
	}

	newBalance := balance + amount
	s.balances[userID] = newBalance
	return newBalance, nil
}

// PlaceBet places a bet (wrapper for compatibility)
func (s *MockService) PlaceBet(ctx context.Context, userID int64, amount int64, roundID string) (int64, error) {
	return s.DeductBalance(ctx, userID, amount, "bet:"+roundID)
}
