package color_game

import (
	"context"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// ColorGameGSService defines the interface for color game operations
type ColorGameGSService interface {
	// PlaceBet handles placing a bet
	PlaceBet(ctx context.Context, req *pbColorGame.ColorGamePlaceBetReq) (*pbColorGame.ColorGamePlaceBetRsp, error)

	// GetState returns the current game state
	GetState(ctx context.Context, req *pbColorGame.ColorGameGetStateReq) (*pbColorGame.ColorGameGetStateRsp, error)

	// RoundResult handles round result notification from GMS
	RoundResult(ctx context.Context, req *pbColorGame.ColorGameRoundResultReq) (*pbColorGame.ColorGameRoundResultRsp, error)
}
