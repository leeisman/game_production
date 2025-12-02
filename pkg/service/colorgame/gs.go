package colorgame

import (
	"context"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// ColorGameService defines the interface for color game operations
type ColorGameService interface {
	// PlaceBet handles placing a bet
	PlaceBet(ctx context.Context, req *pbColorGame.PlaceBetReq) (*pbColorGame.PlaceBetRsp, error)

	// GetState returns the current game state
	GetState(ctx context.Context, req *pbColorGame.GetStateReq) (*pbColorGame.GetStateRsp, error)
}
