package domain

import (
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
)

// Color represents a game color (same as GMS)
type Color string

const (
	ColorRed    Color = "red"
	ColorGreen  Color = "green"
	ColorBlue   Color = "blue"
	ColorYellow Color = "yellow"
)

// Bet represents a player's bet
type Bet struct {
	BetID   string
	RoundID string
	UserID  int64
	Color   Color
	Amount  int64
	Time    time.Time
}

var (
	node *snowflake.Node
	once sync.Once
)

func initSnowflake() {
	var err error
	// TODO: Get NodeID from config or environment variable
	// For now, we use a default NodeID of 1.
	// In a real distributed system, each instance MUST have a unique NodeID.
	node, err = snowflake.NewNode(1)
	if err != nil {
		panic(err)
	}
}

// NewBet creates a new bet
func NewBet(roundID string, userID int64, color Color, amount int64) *Bet {
	return &Bet{
		BetID:   generateBetID(),
		RoundID: roundID,
		UserID:  userID,
		Color:   color,
		Amount:  amount,
		Time:    time.Now(),
	}
}

func generateBetID() string {
	once.Do(initSnowflake)
	return node.Generate().String()
}
