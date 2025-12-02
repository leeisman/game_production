// Package grpc provides gRPC adapters for the color game GS module.
package grpc

import (
	"context"

	"github.com/frankieli/game_product/pkg/service/colorgame"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	"google.golang.org/grpc"
)

// ClientAdapter adapts gRPC client to colorgame.ColorGameService interface
type ClientAdapter struct {
	client pb.ColorGameServiceClient
}

// NewColorGameClient creates a new color game service client adapter
func NewColorGameClient(conn *grpc.ClientConn) colorgame.ColorGameService {
	return &ClientAdapter{
		client: pb.NewColorGameServiceClient(conn),
	}
}

// PlaceBet handles placing a bet via gRPC
func (c *ClientAdapter) PlaceBet(ctx context.Context, req *pb.PlaceBetReq) (*pb.PlaceBetRsp, error) {
	return c.client.PlaceBet(ctx, req)
}

// GetState returns the current game state via gRPC
func (c *ClientAdapter) GetState(ctx context.Context, req *pb.GetStateReq) (*pb.GetStateRsp, error) {
	return c.client.GetState(ctx, req)
}
