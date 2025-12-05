// Package grpc provides gRPC adapters for the color game GS module.
package grpc

import (
	"context"
	"encoding/json"
	"net"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Handler implements the gRPC server for GS
type Handler struct {
	pb.UnimplementedColorGameGSServiceServer
	gsUC *usecase.GSUseCase
}

// NewHandler creates a new gRPC handler
func NewHandler(gsUC *usecase.GSUseCase) *Handler {
	return &Handler{
		gsUC: gsUC,
	}
}

// PlaceBet implements the PlaceBet RPC
func (h *Handler) PlaceBet(ctx context.Context, req *pb.ColorGamePlaceBetReq) (*pb.ColorGamePlaceBetRsp, error) {
	bet, err := h.gsUC.PlaceBet(ctx, req.UserId, domain.Color(req.Color), req.Amount)
	if err != nil {
		return &pb.ColorGamePlaceBetRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR, // TODO: Map specific errors
			Error:     err.Error(),
		}, nil
	}
	return &pb.ColorGamePlaceBetRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		BetId:     bet.BetID,
	}, nil
}

// GetState implements the GetState RPC
func (h *Handler) GetState(ctx context.Context, req *pb.ColorGameGetStateReq) (*pb.ColorGameGetStateRsp, error) {
	state, err := h.gsUC.GetCurrentRound(ctx, req.UserId)
	if err != nil {
		return &pb.ColorGameGetStateRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to marshal state")
		return &pb.ColorGameGetStateRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	return &pb.ColorGameGetStateRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		StateJson: stateJSON,
	}, nil
}

// RoundResult implements the RoundResult RPC
func (h *Handler) RoundResult(ctx context.Context, req *pb.ColorGameRoundResultReq) (*pb.ColorGameRoundResultRsp, error) {
	// For gRPC, we just forward to the use case logic directly or via a service method
	// Since GSUseCase doesn't have a direct RoundResult method that takes proto, we might need to adapt here
	// But wait, the local handler implements it by calling SettleRound asynchronously.
	// We should probably do similar logic here or delegate to a common service implementation.
	// However, the gRPC handler is an adapter for incoming requests.
	// The RoundResult is a notification from GMS.

	// Parse result data
	color := domain.Color(req.Result)

	// Run settlement asynchronously
	// Note: In a real gRPC handler, we might want to return immediately.
	go func() {
		bgCtx := context.Background()
		if err := h.gsUC.SettleRound(bgCtx, req.RoundId, color); err != nil {
			logger.ErrorGlobal().Err(err).Str("round_id", req.RoundId).Msg("Settlement failed")
		}
	}()

	return &pb.ColorGameRoundResultRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
	}, nil
}

// StartServer starts the gRPC server
func StartServer(address string, gsUC *usecase.GSUseCase) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("failed to listen")
	}
	s := grpc.NewServer()
	pb.RegisterColorGameGSServiceServer(s, NewHandler(gsUC))

	// Enable reflection for debugging
	reflection.Register(s)

	logger.InfoGlobal().Str("address", address).Msg("GS gRPC server listening")
	if err := s.Serve(lis); err != nil {
		logger.FatalGlobal().Err(err).Msg("failed to serve gRPC")
	}
}
