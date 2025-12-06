package base

import (
	"context"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/frankieli/game_product/pkg/logger"
	"github.com/frankieli/game_product/pkg/service"
	pbGateway "github.com/frankieli/game_product/shared/proto/gateway"
)

// Ensure BaseClient implements GatewayService
var _ service.GatewayService = (*BaseClient)(nil)

// Broadcast sends a message to all connected players for a specific game
// Performs client-side fan-out to all Gateway instances using SubmitTask.
func (c *BaseClient) Broadcast(ctx context.Context, gameCode string, event proto.Message) {
	logger.InfoGlobal().Str("game_code", gameCode).Msg("BaseClient.Broadcast called")

	// Optimization: Use cached service addresses (Watcher + Singleflight)
	addrs, err := c.GetServiceAddrs("gateway-service")
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to discover gateway services for Broadcast")
		return
	}

	logger.InfoGlobal().Int("gateway_count", len(addrs)).Strs("addrs", addrs).Msg("Discovered gateway instances")

	anyEvent, err := anypb.New(event)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to marshal event to Any for Broadcast")
		return
	}

	req := &pbGateway.BroadcastReq{
		GameCode: gameCode,
		Event:    anyEvent,
	}

	// Capture Request ID for simple propagation to async tasks
	reqID := logger.GetRequestID(ctx)

	// Fan-out to ALL addresses
	for _, addr := range addrs {
		addr := addr
		c.SubmitTask(func() {
			conn, err := c.getConnDirect(addr)
			if err != nil {
				logger.ErrorGlobal().Str("addr", addr).Err(err).Msg("Failed to dial gateway for Broadcast")
				return
			}
			client := pbGateway.NewGatewayServiceClient(conn)

			// Create detached context with propagated Request ID
			outCtx := context.Background()
			if reqID != "" {
				outCtx = metadata.AppendToOutgoingContext(outCtx, "request_id", reqID)
			}

			_, err = client.Broadcast(outCtx, req)
			if err != nil {
				logger.ErrorGlobal().Str("addr", addr).Err(err).Msg("Broadcast RPC failed")
			} else {
				logger.InfoGlobal().Str("addr", addr).Msg("Broadcast RPC succeeded")
			}
		})
	}
}

// SendToUser sends a message to a specific user
// Performs fan-out to all Gateways because we don't know which one the user is connected to.
func (c *BaseClient) SendToUser(ctx context.Context, userID int64, gameCode string, event proto.Message) {
	// Optimization: Use cached service addresses
	addrs, err := c.GetServiceAddrs("gateway-service")
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to discover gateway services for SendToUser")
		return
	}

	anyEvent, err := anypb.New(event)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to marshal event to Any for SendToUser")
		return
	}

	req := &pbGateway.SendToUserReq{
		UserId:   userID,
		GameCode: gameCode,
		Event:    anyEvent,
	}

	// Capture Request ID
	reqID := logger.GetRequestID(ctx)

	// Fan-out to ALL addresses
	for _, addr := range addrs {
		addr := addr
		c.SubmitTask(func() {
			conn, err := c.getConnDirect(addr)
			if err != nil {
				logger.ErrorGlobal().Str("addr", addr).Err(err).Msg("Failed to dial gateway for SendToUser")
				return
			}
			client := pbGateway.NewGatewayServiceClient(conn)

			// Create detached context with propagated Request ID
			outCtx := context.Background()
			if reqID != "" {
				outCtx = metadata.AppendToOutgoingContext(outCtx, "request_id", reqID)
			}

			_, err = client.SendToUser(outCtx, req)
			if err != nil {
				logger.ErrorGlobal().Str("addr", addr).Err(err).Msg("SendToUser RPC failed")
			}
		})
	}
}
