package color_game

import (
	"context"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// ColorGameService defines the interface for color game operations
type ColorGameService interface {
	// PlaceBet handles placing a bet
	PlaceBet(ctx context.Context, req *pbColorGame.ColorGamePlaceBetReq) (*pbColorGame.ColorGamePlaceBetRsp, error)

	// GetState returns the current game state
	GetState(ctx context.Context, req *pbColorGame.ColorGameGetStateReq) (*pbColorGame.ColorGameGetStateRsp, error)
}
