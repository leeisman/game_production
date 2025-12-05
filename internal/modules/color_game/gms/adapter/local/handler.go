package local

import (
	"context"

	"github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
)

// Handler is the local adapter handler for GMS
type Handler struct {
	roundUC *usecase.GMSUseCase
}

// NewHandler creates a new local handler
func NewHandler(roundUC *usecase.GMSUseCase) *Handler {
	return &Handler{
		roundUC: roundUC,
	}
}

// RecordBet records a bet
func (h *Handler) RecordBet(ctx context.Context, req *pb.ColorGameRecordBetReq) (*pb.ColorGameRecordBetRsp, error) {
	err := h.roundUC.IncrementBetCount(ctx, req.RoundId, req.UserId, float64(req.Amount))
	if err != nil {
		return nil, err
	}
	return &pb.ColorGameRecordBetRsp{}, nil
}

// GetCurrentRound gets the current round
func (h *Handler) GetCurrentRound(ctx context.Context, req *pb.ColorGameGetCurrentRoundReq) (*pb.ColorGameGetCurrentRoundRsp, error) {
	round, err := h.roundUC.GetCurrentRound(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ColorGameGetCurrentRoundRsp{
		RoundId:             round.RoundID,
		State:               string(round.State),
		BettingEndTimestamp: round.BettingEnd.Unix(),
		PlayerBets:          []*pb.ColorGamePlayerBet{}, // GMS doesn't store player bets, return empty array
	}, nil
}
