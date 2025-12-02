package domain

import (
	"time"
)

// RoundStatus defines the status of a game round
type RoundStatus int

const (
	RoundStatusInProgress RoundStatus = 0 // 進行中
	RoundStatusEnded      RoundStatus = 1 // 已結束
)

// GameRound represents a game round history record
type GameRound struct {
	RoundID        string      `gorm:"primaryKey;type:varchar(64)" json:"round_id"`
	GameCode       string      `gorm:"index;type:varchar(32);not null" json:"game_code"`
	Status         RoundStatus `gorm:"type:int;not null;default:0" json:"status"`
	StartTime      time.Time   `gorm:"not null" json:"start_time"`
	EndTime        *time.Time  `json:"end_time"`
	Result         string      `gorm:"type:varchar(512)" json:"result"` // Game result (simple string for color game, or JSON for complex games)
	TotalBets      int         `gorm:"default:0" json:"total_bets"`
	TotalPlayers   int         `gorm:"default:0" json:"total_players"`
	TotalBetAmount float64     `gorm:"type:decimal(18,2);default:0" json:"total_bet_amount"`
	CreatedAt      time.Time   `gorm:"autoCreateTime:false" json:"created_at"`
	UpdatedAt      time.Time   `gorm:"autoUpdateTime:false" json:"updated_at"`
}

// TableName overrides the table name
func (GameRound) TableName() string {
	return "game_rounds"
}
