// Package grpc provides gRPC adapters for the color game GS module.
package grpc

import (
	"context"

	"google.golang.org/grpc"

	colorgame "github.com/frankieli/game_product/pkg/service/color_game"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// GMSClientAdapter adapts gRPC client to colorgame.GMSService interface
type GMSClientAdapter struct {
	client pbColorGame.GameMachineServiceClient
}

// NewGMSClient creates a new GMS client adapter
func NewGMSClient(conn *grpc.ClientConn) colorgame.GMSService {
	return &GMSClientAdapter{
		client: pbColorGame.NewGameMachineServiceClient(conn),
	}
}

func (a *GMSClientAdapter) RecordBet(ctx context.Context, req *pbColorGame.RecordBetReq) (*pbColorGame.RecordBetRsp, error) {
	return a.client.RecordBet(ctx, req)
}

func (a *GMSClientAdapter) GetCurrentRound(ctx context.Context, req *pbColorGame.GetCurrentRoundReq) (*pbColorGame.GetCurrentRoundRsp, error) {
	return a.client.GetCurrentRound(ctx, req)
}
