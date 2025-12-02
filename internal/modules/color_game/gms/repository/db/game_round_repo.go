package db

import (
	"context"
	"time"

	"github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	"gorm.io/gorm"
)

type GameRoundRepository struct {
	db *gorm.DB
}

func NewGameRoundRepository(db *gorm.DB) *GameRoundRepository {
	return &GameRoundRepository{db: db}
}

func (r *GameRoundRepository) Create(ctx context.Context, round *domain.GameRound) error {
	now := time.Now()
	round.CreatedAt = now
	round.UpdatedAt = now
	return r.db.WithContext(ctx).Create(round).Error
}

func (r *GameRoundRepository) UpdateResult(ctx context.Context, roundID string, result string, endTime *time.Time, totalBets int, totalPlayers int, totalAmount float64) error {
	now := time.Now()
	updates := map[string]interface{}{
		"result":           result,
		"status":           domain.RoundStatusEnded,
		"total_bets":       totalBets,
		"total_players":    totalPlayers,
		"total_bet_amount": totalAmount,
		"updated_at":       now,
	}
	if endTime != nil {
		updates["end_time"] = endTime
	}
	return r.db.WithContext(ctx).Model(&domain.GameRound{}).
		Where("round_id = ?", roundID).
		Updates(updates).Error
}
