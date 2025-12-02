package grpc

import (
	"context"

	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	"google.golang.org/grpc"
)

type ColorGameClient struct {
	client pb.ColorGameServiceClient
}

func NewColorGameClient(conn *grpc.ClientConn) *ColorGameClient {
	return &ColorGameClient{
		client: pb.NewColorGameServiceClient(conn),
	}
}

func (c *ColorGameClient) PlaceBet(ctx context.Context, req *pb.PlaceBetReq) (*pb.PlaceBetRsp, error) {
	return c.client.PlaceBet(ctx, req)
}

func (c *ColorGameClient) GetState(ctx context.Context, req *pb.GetStateReq) (*pb.GetStateRsp, error) {
	return c.client.GetState(ctx, req)
}
