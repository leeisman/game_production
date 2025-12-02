package domain

import (
	"context"
	"time"
)

// GameRoundRepository defines the interface for game round persistence
type GameRoundRepository interface {
	Create(ctx context.Context, round *GameRound) error
	UpdateResult(ctx context.Context, roundID string, result string, endTime *time.Time, totalBets int, totalPlayers int, totalAmount float64) error
}
