// Package grpc provides gRPC adapters for the color game GS module.
package grpc

import (
	"context"

	colorgame "github.com/frankieli/game_product/pkg/service/color_game"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	"google.golang.org/grpc"
)

// ClientAdapter adapts gRPC client to colorgame.ColorGameGSService interface
type ClientAdapter struct {
	client pb.ColorGameGSServiceClient
}

// NewColorGameClient creates a new color game service client adapter
func NewColorGameClient(conn *grpc.ClientConn) colorgame.ColorGameGSService {
	return &ClientAdapter{
		client: pb.NewColorGameGSServiceClient(conn),
	}
}

// PlaceBet handles placing a bet via gRPC
func (c *ClientAdapter) PlaceBet(ctx context.Context, req *pb.ColorGamePlaceBetReq) (*pb.ColorGamePlaceBetRsp, error) {
	return c.client.PlaceBet(ctx, req)
}

// GetState returns the current game state via gRPC
func (c *ClientAdapter) GetState(ctx context.Context, req *pb.ColorGameGetStateReq) (*pb.ColorGameGetStateRsp, error) {
	return c.client.GetState(ctx, req)
}

// RoundResult handles round result notification via gRPC
func (c *ClientAdapter) RoundResult(ctx context.Context, req *pb.ColorGameRoundResultReq) (*pb.ColorGameRoundResultRsp, error) {
	return c.client.RoundResult(ctx, req)
}
