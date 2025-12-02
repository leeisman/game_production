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
	pb.UnimplementedColorGameServiceServer
	playerUC *usecase.PlayerUseCase
}

// NewHandler creates a new gRPC handler
func NewHandler(playerUC *usecase.PlayerUseCase) *Handler {
	return &Handler{
		playerUC: playerUC,
	}
}

// PlaceBet implements the PlaceBet RPC
func (h *Handler) PlaceBet(ctx context.Context, req *pb.PlaceBetReq) (*pb.PlaceBetRsp, error) {
	bet, err := h.playerUC.PlaceBet(ctx, req.UserId, domain.Color(req.Color), req.Amount)
	if err != nil {
		return &pb.PlaceBetRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR, // TODO: Map specific errors
			Error:     err.Error(),
		}, nil
	}
	return &pb.PlaceBetRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		BetId:     bet.BetID,
	}, nil
}

// GetState implements the GetState RPC
func (h *Handler) GetState(ctx context.Context, req *pb.GetStateReq) (*pb.GetStateRsp, error) {
	state, err := h.playerUC.GetCurrentRound(ctx, req.UserId)
	if err != nil {
		return &pb.GetStateRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to marshal state")
		return &pb.GetStateRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	return &pb.GetStateRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		StateJson: stateJSON,
	}, nil
}

// StartServer starts the gRPC server
func StartServer(address string, playerUC *usecase.PlayerUseCase) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("failed to listen")
	}
	s := grpc.NewServer()
	pb.RegisterColorGameServiceServer(s, NewHandler(playerUC))

	// Enable reflection for debugging
	reflection.Register(s)

	logger.InfoGlobal().Str("address", address).Msg("GS gRPC server listening")
	if err := s.Serve(lis); err != nil {
		logger.FatalGlobal().Err(err).Msg("failed to serve gRPC")
	}
}
