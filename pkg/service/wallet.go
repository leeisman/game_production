package service

import "context"

// WalletService defines the interface for wallet-related operations
type WalletService interface {
	GetBalance(ctx context.Context, userID int64) (int64, error)
	DeductBalance(ctx context.Context, userID int64, amount int64, reason string) (int64, error)
	AddBalance(ctx context.Context, userID int64, amount int64, reason string) (int64, error)
	PlaceBet(ctx context.Context, userID int64, amount int64, roundID string) (int64, error)
}

