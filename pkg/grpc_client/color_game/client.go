// Package color_game provides gRPC client for ColorGame GMS and GS services.
package color_game

import (
	"context"

	baseClient "github.com/frankieli/game_product/pkg/grpc_client/base"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
)

// Client implements ColorGame GS & GMS operations
// It embeds the Base Client to inherit User Service capabilities and connection management.
// It does NOT cache gRPC client stubs to ensure load balancing works correctly.
type Client struct {
	*baseClient.BaseClient // Embedding Base Client
}

// NewClient creates a new color game client that extends the base client
func NewClient(base *baseClient.BaseClient) (*Client, error) {
	return &Client{
		BaseClient: base,
	}, nil
}

// --- GMS Service Implementation ---

// RecordBet records a bet in GMS
// This implements pkg/service/color_game.GMSService
func (c *Client) RecordBet(ctx context.Context, req *pb.ColorGameRecordBetReq) (*pb.ColorGameRecordBetRsp, error) {
	// Get connection with load balancing
	conn, err := c.BaseClient.GetConn("gms-service")
	if err != nil {
		return nil, err
	}

	// Create client stub for this call
	gmsClient := pb.NewColorGameGMSServiceClient(conn)

	logger.Debug(ctx).Msg("Calling RPC RecordBet")
	return gmsClient.RecordBet(ctx, req)
}

// GetCurrentRound gets the current round from GMS
func (c *Client) GetCurrentRound(ctx context.Context, req *pb.ColorGameGetCurrentRoundReq) (*pb.ColorGameGetCurrentRoundRsp, error) {
	conn, err := c.BaseClient.GetConn("gms-service")
	if err != nil {
		return nil, err
	}

	gmsClient := pb.NewColorGameGMSServiceClient(conn)
	return gmsClient.GetCurrentRound(ctx, req)
}

// --- GS Service Implementation ---

// PlaceBet handles placing a bet through GS
// This implements pkg/service/color_game.ColorGameGSService
func (c *Client) PlaceBet(ctx context.Context, req *pb.ColorGamePlaceBetReq) (*pb.ColorGamePlaceBetRsp, error) {
	// Get connection with load balancing (important for multiple GS instances)
	conn, err := c.BaseClient.GetConn("gs-service")
	if err != nil {
		return nil, err
	}

	gsClient := pb.NewColorGameGSServiceClient(conn)
	return gsClient.PlaceBet(ctx, req)
}

// GetState returns the current game state from GS
func (c *Client) GetState(ctx context.Context, req *pb.ColorGameGetStateReq) (*pb.ColorGameGetStateRsp, error) {
	conn, err := c.BaseClient.GetConn("gs-service")
	if err != nil {
		return nil, err
	}

	gsClient := pb.NewColorGameGSServiceClient(conn)
	return gsClient.GetState(ctx, req)
}

// RoundResult handles round result notification (GS endpoint)
func (c *Client) RoundResult(ctx context.Context, req *pb.ColorGameRoundResultReq) (*pb.ColorGameRoundResultRsp, error) {
	conn, err := c.BaseClient.GetConn("gs-service")
	if err != nil {
		return nil, err
	}

	gsClient := pb.NewColorGameGSServiceClient(conn)
	return gsClient.RoundResult(ctx, req)
}
