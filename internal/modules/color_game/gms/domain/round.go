package domain

import (
	"time"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// Color represents a game color
type Color = pbColorGame.ColorGameReward

// Round represents a game round
type Round struct {
	RoundID    string
	State      pbColorGame.ColorGameState
	Result     pbColorGame.ColorGameReward
	StartTime  time.Time
	BettingEnd time.Time
	TotalBets  int
	LeftTime   int64
}

// NewRound creates a new round
func NewRound(roundID string) *Round {
	return &Round{
		RoundID:   roundID,
		State:     pbColorGame.ColorGameState_GAME_STATE_ROUND_STARTED,
		StartTime: time.Now(),
	}
}

// StartBetting transitions to betting state
func (r *Round) StartBetting(duration time.Duration) {
	r.State = pbColorGame.ColorGameState_GAME_STATE_BETTING
	r.BettingEnd = time.Now().Add(duration)
}

// CanAcceptBet checks if bets can be accepted
func (r *Round) CanAcceptBet() bool {
	return r.State == pbColorGame.ColorGameState_GAME_STATE_BETTING && time.Now().Before(r.BettingEnd)
}

// Draw transitions to drawing state and selects result
func (r *Round) Draw(result Color) {
	r.State = pbColorGame.ColorGameState_GAME_STATE_DRAWING
	r.Result = result
}

// ShowResult transitions to result state
func (r *Round) ShowResult() {
	r.State = pbColorGame.ColorGameState_GAME_STATE_RESULT
}

// IsFinished checks if round is finished
func (r *Round) IsFinished() bool {
	return r.State == pbColorGame.ColorGameState_GAME_STATE_RESULT
}
