package domain

import "time"

// BetOrderStatus defines the status of a bet order
type BetOrderStatus int

const (
	BetOrderStatusPending BetOrderStatus = 0 // 待結算
	BetOrderStatusSettled BetOrderStatus = 1 // 已結算
)

// BetOrder represents a player's bet order record
type BetOrder struct {
	OrderID   string         `gorm:"primaryKey;type:varchar(64)" json:"order_id"`
	UserID    int64          `gorm:"not null;index:idx_bet_orders_user_id" json:"user_id"`
	RoundID   string         `gorm:"type:varchar(64);not null;index:idx_bet_orders_round_id" json:"round_id"`
	GameCode  string         `gorm:"type:varchar(32);not null;index:idx_bet_orders_game_code" json:"game_code"`
	BetArea   string         `gorm:"type:varchar(512);not null" json:"bet_area"` // Bet area/zone (e.g., "red" for color game, "player" for baccarat)
	Amount    float64        `gorm:"type:decimal(18,2);not null" json:"amount"`
	Payout    float64        `gorm:"type:decimal(18,2);not null;default:0" json:"payout"`
	Status    BetOrderStatus `gorm:"type:int;not null;default:0;index:idx_bet_orders_status" json:"status"`
	CreatedAt time.Time      `gorm:"not null;index:idx_bet_orders_created_at" json:"created_at"`
	SettledAt *time.Time     `json:"settled_at"`
}

// TableName overrides the table name
func (BetOrder) TableName() string {
	return "bet_orders"
}
