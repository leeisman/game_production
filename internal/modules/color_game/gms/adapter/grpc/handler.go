package grpc

import (
	"context"
	"encoding/json"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	"github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
)

// Handler implements the gRPC server for GMS
type Handler struct {
	pb.UnimplementedGameMachineServiceServer
	roundUC     *usecase.RoundUseCase
	subscribers map[pb.GameMachineService_SubscribeEventsServer]bool
	mu          sync.RWMutex
}

// NewHandler creates a new gRPC handler
func NewHandler(roundUC *usecase.RoundUseCase) *Handler {
	h := &Handler{
		roundUC:     roundUC,
		subscribers: make(map[pb.GameMachineService_SubscribeEventsServer]bool),
	}

	// Register event handler to broadcast to gRPC subscribers
	roundUC.RegisterEventHandler(h.broadcastEvent)

	return h
}

// broadcastEvent broadcasts events to all gRPC subscribers
func (h *Handler) broadcastEvent(event machine.GameEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Convert data to JSON string
	dataJSON, _ := json.Marshal(event.Data)

	pbEvent := &pb.GameEvent{
		Type:    event.Type,
		RoundId: event.RoundID,
		Data:    string(dataJSON),
	}

	// Send to all subscribers
	for stream := range h.subscribers {
		if err := stream.Send(pbEvent); err != nil {
			logger.ErrorGlobal().Err(err).Msg("Failed to send event to subscriber")
		}
	}
}

// RecordBet implements the RecordBet RPC
func (h *Handler) RecordBet(ctx context.Context, req *pb.RecordBetReq) (*pb.RecordBetRsp, error) {
	err := h.roundUC.IncrementBetCount(ctx, req.RoundId, float64(req.Amount))
	if err != nil {
		return &pb.RecordBetRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
			Error:     err.Error(),
		}, nil
	}
	return &pb.RecordBetRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
	}, nil
}

// GetCurrentRound implements the GetCurrentRound RPC
func (h *Handler) GetCurrentRound(ctx context.Context, req *pb.GetCurrentRoundReq) (*pb.GetCurrentRoundRsp, error) {
	round, err := h.roundUC.GetCurrentRound(ctx)
	if err != nil {
		return &pb.GetCurrentRoundRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
		}, nil
	}

	return &pb.GetCurrentRoundRsp{
		ErrorCode:           pbCommon.ErrorCode_SUCCESS,
		RoundId:             round.RoundID,
		State:               string(round.State),
		BettingEndTimestamp: round.BettingEnd.Unix(),
		PlayerBets:          []*pb.PlayerBet{}, // GMS doesn't store player bets, return empty array
	}, nil
}

// SubscribeEvents implements the SubscribeEvents streaming RPC
func (h *Handler) SubscribeEvents(req *pb.SubscribeEventsReq, stream pb.GameMachineService_SubscribeEventsServer) error {
	// Register subscriber
	h.mu.Lock()
	h.subscribers[stream] = true
	h.mu.Unlock()

	logger.InfoGlobal().Int("count", len(h.subscribers)).Msg("New subscriber connected")

	// Wait for context cancellation (client disconnect)
	<-stream.Context().Done()

	// Unregister subscriber
	h.mu.Lock()
	delete(h.subscribers, stream)
	h.mu.Unlock()

	logger.InfoGlobal().Int("count", len(h.subscribers)).Msg("Subscriber disconnected")

	return stream.Context().Err()
}

// StartServer starts the gRPC server
func StartServer(address string, roundUC *usecase.RoundUseCase) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("failed to listen")
	}
	s := grpc.NewServer()
	pb.RegisterGameMachineServiceServer(s, NewHandler(roundUC))

	// Enable reflection for debugging
	reflection.Register(s)

	logger.InfoGlobal().Str("address", address).Msg("GMS gRPC server listening")
	if err := s.Serve(lis); err != nil {
		logger.FatalGlobal().Err(err).Msg("failed to serve gRPC")
	}
}
