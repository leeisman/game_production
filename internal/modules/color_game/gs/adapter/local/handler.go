// Package local provides local adapters for the color game GS module.
package local

import (
	"context"
	"encoding/json"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
	"google.golang.org/protobuf/proto"
)

// Handler is the local adapter handler for GS
// It implements service.ColorGameService directly and service.GSBroadcaster
type Handler struct {
	gsUC *usecase.GSUseCase
}

// NewHandler creates a new local handler
func NewHandler(gsUC *usecase.GSUseCase) *Handler {
	return &Handler{
		gsUC: gsUC,
	}
}

// PlaceBet handles placing a bet
func (h *Handler) PlaceBet(ctx context.Context, req *pb.ColorGamePlaceBetReq) (*pb.ColorGamePlaceBetRsp, error) {
	bet, err := h.gsUC.PlaceBet(ctx, req.UserId, domain.Color(req.Color), req.Amount)
	if err != nil {
		return &pb.ColorGamePlaceBetRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
			Error:     err.Error(),
		}, nil
	}

	return &pb.ColorGamePlaceBetRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		BetId:     bet.BetID,
	}, nil
}

// GetState returns current game state
func (h *Handler) GetState(ctx context.Context, req *pb.ColorGameGetStateReq) (*pb.ColorGameGetStateRsp, error) {
	state, err := h.gsUC.GetCurrentRound(ctx, req.UserId)
	if err != nil {
		return &pb.ColorGameGetStateRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	state["type"] = "state"
	stateBytes, _ := json.Marshal(state)

	return &pb.ColorGameGetStateRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		StateJson: stateBytes,
	}, nil
}

// GSBroadcast handles events from GMS (implements service.GSBroadcaster)
func (h *Handler) GSBroadcast(event proto.Message) {
	switch gameEvent := event.(type) {
	case *pb.ColorGameEvent:
		// Filter events: only process 'result' events to trigger settlement
		if gameEvent.Type == pb.ColorGameEventType_EVENT_TYPE_RESULT {
			// Parse result data to get winning color (e.g., "green")
			color := domain.Color(gameEvent.Data)

			// Run settlement asynchronously
			go func() {
				ctx := context.Background()
				if err := h.gsUC.SettleRound(ctx, gameEvent.RoundId, color); err != nil {
					logger.ErrorGlobal().Err(err).Str("round_id", gameEvent.RoundId).Msg("Settlement failed")
				}
			}()
		}
	default:
		// Ignore other event types
	}
}
