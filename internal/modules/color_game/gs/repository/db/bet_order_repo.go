package db

import (
	"context"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"gorm.io/gorm"
)

type BetOrderRepository struct {
	db *gorm.DB
}

func NewBetOrderRepository(db *gorm.DB) *BetOrderRepository {
	return &BetOrderRepository{db: db}
}

func (r *BetOrderRepository) BatchCreate(ctx context.Context, orders []*domain.BetOrder) error {
	if len(orders) == 0 {
		return nil
	}
	// UseCase layer handles batching, so we just insert all records here
	return r.db.WithContext(ctx).Create(&orders).Error
}
