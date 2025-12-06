// Package grpc provides gRPC adapters for the color game GMS module.
package grpc

import (
	"context"

	"github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	"github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
)

// Handler implements the gRPC server for GMS
type Handler struct {
	pb.UnimplementedColorGameGMSServiceServer
	gmsUC *usecase.GMSUseCase
}

// NewHandler creates a new gRPC handler
func NewHandler(gmsUC *usecase.GMSUseCase) *Handler {
	return &Handler{
		gmsUC: gmsUC,
	}
}

// RecordBet implements the RecordBet RPC
func (h *Handler) RecordBet(ctx context.Context, req *pb.ColorGameRecordBetReq) (*pb.ColorGameRecordBetRsp, error) {
	logger.Debug(ctx).
		Str("round_id", req.RoundId).
		Int64("user_id", req.UserId).
		Int32("color", int32(req.Color)).
		Int64("amount", req.Amount).
		Msg("RecordBet RPC called")

	err := h.gmsUC.RecordBet(ctx, req.RoundId, req.UserId, domain.Color(req.Color), req.Amount)
	if err != nil {
		logger.Error(ctx).Err(err).Msg("Failed to record bet")
		return &pb.ColorGameRecordBetRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
			Error:     err.Error(),
		}, nil
	}

	return &pb.ColorGameRecordBetRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
	}, nil
}

// GetCurrentRound implements the GetCurrentRound RPC
func (h *Handler) GetCurrentRound(ctx context.Context, req *pb.ColorGameGetCurrentRoundReq) (*pb.ColorGameGetCurrentRoundRsp, error) {
	logger.Debug(ctx).Int64("user_id", req.UserId).Msg("GetCurrentRound RPC called")

	round, err := h.gmsUC.GetCurrentRound(ctx)
	if err != nil {
		logger.Error(ctx).Err(err).Msg("Failed to get current round")
		return &pb.ColorGameGetCurrentRoundRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	// Get player bets for this user if needed
	var playerBets []*pb.ColorGamePlayerBet
	if req.UserId > 0 {
		bets, err := h.gmsUC.GetPlayerBets(ctx, round.RoundID, req.UserId)
		if err != nil {
			logger.Warn(ctx).Err(err).Msg("Failed to get player bets")
		} else {
			for _, bet := range bets {
				playerBets = append(playerBets, &pb.ColorGamePlayerBet{
					Color:  pb.ColorGameReward(bet.Color),
					Amount: bet.Amount,
				})
			}
		}
	}

	return &pb.ColorGameGetCurrentRoundRsp{
		ErrorCode:           pbCommon.ErrorCode_SUCCESS,
		RoundId:             round.RoundID,
		State:               pb.ColorGameState(round.State),
		BettingEndTimestamp: round.BettingEnd.Unix(),
		PlayerBets:          playerBets,
		LeftTime:            round.LeftTime,
	}, nil
}
