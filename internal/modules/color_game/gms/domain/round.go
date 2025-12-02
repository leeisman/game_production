package domain

import "time"

// Color represents a game color
type Color string

const (
	ColorRed    Color = "red"
	ColorGreen  Color = "green"
	ColorBlue   Color = "blue"
	ColorYellow Color = "yellow"
)

// GameState represents the current state of the game
type GameState string

const (
	StateWaiting GameState = "waiting" // Waiting for next round
	StateBetting GameState = "betting" // Accepting bets
	StateDrawing GameState = "drawing" // Drawing result
	StateResult  GameState = "result"  // Showing results
)

// Round represents a game round
type Round struct {
	RoundID    string
	State      GameState
	Result     Color
	StartTime  time.Time
	BettingEnd time.Time
	TotalBets  int
}

// NewRound creates a new round
func NewRound(roundID string) *Round {
	return &Round{
		RoundID:   roundID,
		State:     StateWaiting,
		StartTime: time.Now(),
	}
}

// StartBetting transitions to betting state
func (r *Round) StartBetting(duration time.Duration) {
	r.State = StateBetting
	r.BettingEnd = time.Now().Add(duration)
}

// CanAcceptBet checks if bets can be accepted
func (r *Round) CanAcceptBet() bool {
	return r.State == StateBetting && time.Now().Before(r.BettingEnd)
}

// Draw transitions to drawing state and selects result
func (r *Round) Draw(result Color) {
	r.State = StateDrawing
	r.Result = result
}

// ShowResult transitions to result state
func (r *Round) ShowResult() {
	r.State = StateResult
}

// IsFinished checks if round is finished
func (r *Round) IsFinished() bool {
	return r.State == StateResult
}
