package grpc

import (
	"context"
	"encoding/json"

	"github.com/frankieli/game_product/internal/modules/gateway/ws"
	"github.com/frankieli/game_product/pkg/logger"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
	pb "github.com/frankieli/game_product/shared/proto/gateway"
	"google.golang.org/protobuf/types/known/anypb"
)

// Handler implements the gRPC server for Gateway service
type Handler struct {
	pb.UnimplementedGatewayServiceServer
	wsManager *ws.Manager
}

// NewHandler creates a new gRPC gateway handler
func NewHandler(wsManager *ws.Manager) *Handler {
	return &Handler{
		wsManager: wsManager,
	}
}

// convertEvent converts a protobuf message (wrapped in Any) to the JSON format expected by clients
func (h *Handler) convertEvent(gameCode string, anyEvent *anypb.Any) []byte {
	// Try to unmarshal into known types
	// 1. ColorGameRoundStateBRC
	brcState := &pbColorGame.ColorGameRoundStateBRC{}
	if err := anyEvent.UnmarshalTo(brcState); err == nil {
		finalData := map[string]interface{}{
			"round_id":              brcState.RoundId,
			"state":                 brcState.State.String(),
			"betting_end_timestamp": brcState.BettingEndTimestamp,
			"left_time":             brcState.LeftTime,
		}
		jsonMsg, _ := json.Marshal(map[string]interface{}{
			"game_code": gameCode,
			"command":   "ColorGameRoundStateBRC",
			"data":      finalData,
		})
		return jsonMsg
	}

	// 2. ColorGameSettlementBRC
	brcSettlement := &pbColorGame.ColorGameSettlementBRC{}
	if err := anyEvent.UnmarshalTo(brcSettlement); err == nil {
		finalData := map[string]interface{}{
			"round_id":      brcSettlement.RoundId,
			"winning_color": brcSettlement.WinningColor.String(),
			"bet_id":        brcSettlement.BetId,
			"bet_color":     brcSettlement.BetColor.String(),
			"bet_amount":    brcSettlement.BetAmount,
			"win_amount":    brcSettlement.WinAmount,
			"is_winner":     brcSettlement.IsWinner,
		}
		jsonMsg, _ := json.Marshal(map[string]interface{}{
			"game_code": gameCode,
			"command":   "ColorGameSettlementBRC",
			"data":      finalData,
		})
		return jsonMsg
	}

	// Fallback: If unknown type, try generic conversion (less ideal but safe)
	// Or log warning and return nil
	logger.WarnGlobal().Str("type_url", anyEvent.TypeUrl).Msg("Unknown event type in convertEvent")
	return nil
}

// Broadcast implements the Broadcast RPC
// Broadcasts a message to all users in a specific game
func (h *Handler) Broadcast(ctx context.Context, req *pb.BroadcastReq) (*pb.BroadcastRsp, error) {
	logger.Debug(ctx).
		Str("game_code", req.GameCode).
		Msg("gRPC Broadcast request")

	if req.Event == nil {
		return &pb.BroadcastRsp{Success: true}, nil
	}

	msgBytes := h.convertEvent(req.GameCode, req.Event)
	if msgBytes != nil {
		h.wsManager.Broadcast(msgBytes)
		logger.Info(ctx).Str("game_code", req.GameCode).Msg("Broadcast successful")
	} else {
		logger.Warn(ctx).Str("game_code", req.GameCode).Msg("Failed to convert event for broadcast")
	}

	return &pb.BroadcastRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		Success:   true,
	}, nil
}

// SendToUser implements the SendToUser RPC
// Sends a message to a specific user
func (h *Handler) SendToUser(ctx context.Context, req *pb.SendToUserReq) (*pb.SendToUserRsp, error) {
	logger.Debug(ctx).
		Int64("user_id", req.UserId).
		Str("game_code", req.GameCode).
		Msg("gRPC SendToUser request")

	if req.Event == nil {
		return &pb.SendToUserRsp{Success: true}, nil
	}

	msgBytes := h.convertEvent(req.GameCode, req.Event)
	if msgBytes != nil {
		h.wsManager.SendToUser(req.UserId, msgBytes)
		logger.Debug(ctx).Int64("user_id", req.UserId).Msg("SendToUser successful")
	}

	return &pb.SendToUserRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		Success:   true,
	}, nil
}
