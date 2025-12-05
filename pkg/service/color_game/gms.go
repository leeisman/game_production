package color_game

import (
	"context"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// GMSService defines the interface for Game Machine Service
type GMSService interface {
	// RecordBet records a bet in GMS
	RecordBet(ctx context.Context, req *pbColorGame.ColorGameRecordBetReq) (*pbColorGame.ColorGameRecordBetRsp, error)

	// GetCurrentRound gets the current round from GMS
	GetCurrentRound(ctx context.Context, req *pbColorGame.ColorGameGetCurrentRoundReq) (*pbColorGame.ColorGameGetCurrentRoundRsp, error)
}
