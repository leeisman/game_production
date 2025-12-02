package domain

import "context"

// BetRepository defines the interface for bet storage
type BetRepository interface {
	// SaveBet saves a bet
	SaveBet(ctx context.Context, bet *Bet) error

	// GetBets retrieves all bets for a round
	GetBets(ctx context.Context, roundID string) ([]*Bet, error)

	// GetUserBets retrieves all bets for a user in a round
	GetUserBets(ctx context.Context, roundID string, userID int64) ([]*Bet, error)

	// ClearBets clears all bets for a round
	ClearBets(ctx context.Context, roundID string) error

	// GetBetsForSettlement retrieves bets for settlement.
	// For Memory repo: returns all bets.
	// For Redis repo: pops a batch of bets for concurrent processing.
	GetBetsForSettlement(ctx context.Context, roundID string) ([]*Bet, error)

	// GetUserBet retrieves a specific bet for a user in a round for a specific color
	GetUserBet(ctx context.Context, roundID string, userID int64, color Color) (*Bet, error)

	// UpdateBetAmount updates the amount of an existing bet
	UpdateBetAmount(ctx context.Context, bet *Bet, additionalAmount int64) error
}
