// Package local provides local adapters for the color game GS module.
package local

import (
	"context"
	"encoding/json"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
)

// Handler is the local adapter handler for GS
// It implements service.ColorGameService directly
type Handler struct {
	playerUC *usecase.PlayerUseCase
}

// NewHandler creates a new local handler
func NewHandler(playerUC *usecase.PlayerUseCase) *Handler {
	return &Handler{
		playerUC: playerUC,
	}
}

// PlaceBet handles placing a bet
func (h *Handler) PlaceBet(ctx context.Context, req *pb.PlaceBetReq) (*pb.PlaceBetRsp, error) {
	bet, err := h.playerUC.PlaceBet(ctx, req.UserId, domain.Color(req.Color), req.Amount)
	if err != nil {
		return &pb.PlaceBetRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
			Error:     err.Error(),
		}, nil
	}

	return &pb.PlaceBetRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		BetId:     bet.BetID,
	}, nil
}

// GetState returns current game state
func (h *Handler) GetState(ctx context.Context, req *pb.GetStateReq) (*pb.GetStateRsp, error) {
	state, err := h.playerUC.GetCurrentRound(ctx, req.UserId)
	if err != nil {
		return &pb.GetStateRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	state["type"] = "state"
	stateBytes, _ := json.Marshal(state)

	return &pb.GetStateRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		StateJson: stateBytes,
	}, nil
}
