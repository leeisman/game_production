package domain

import "context"

// BetOrderRepository defines the interface for bet order persistence
type BetOrderRepository interface {
	// BatchCreate creates multiple bet orders in a single transaction
	BatchCreate(ctx context.Context, orders []*BetOrder) error
}
