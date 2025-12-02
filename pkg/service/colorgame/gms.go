package colorgame

import (
	"context"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// GMSService defines the interface for Game Machine Service
type GMSService interface {
	// RecordBet records a bet in GMS
	RecordBet(ctx context.Context, req *pbColorGame.RecordBetReq) (*pbColorGame.RecordBetRsp, error)

	// GetCurrentRound gets the current round from GMS
	GetCurrentRound(ctx context.Context, req *pbColorGame.GetCurrentRoundReq) (*pbColorGame.GetCurrentRoundRsp, error)
}
