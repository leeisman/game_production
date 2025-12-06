package user

import (
	"context"
	"fmt"
	"time"

	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client implements the UserService interface using gRPC
type Client struct {
	registry    discovery.Registry
	serviceName string
	conn        *grpc.ClientConn
	client      pb.UserServiceClient
}

// NewClient creates a new gRPC user client
func NewClient(registry discovery.Registry, serviceName string) (*Client, error) {
	c := &Client{
		registry:    registry,
		serviceName: serviceName,
	}
	return c, nil
}

// getClient ensures a valid gRPC client connection
func (c *Client) getClient() (pb.UserServiceClient, error) {
	if c.client != nil {
		return c.client, nil
	}

	// Discovery service
	addr, err := c.registry.GetService(c.serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover service %s: %w", c.serviceName, err)
	}

	// Connect gRPC
	// In production, you might want to use a load balancer or gRPC's built-in resolution
	// For now, we connect directly to the discovered instance
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	c.conn = conn
	c.client = pb.NewUserServiceClient(conn)

	logger.InfoGlobal().
		Str("service", c.serviceName).
		Str("addr", addr).
		Msg("Established gRPC connection to User Service")

	return c.client, nil
}

// ValidateToken implements service.UserService
func (c *Client) ValidateToken(ctx context.Context, token string) (int64, string, time.Time, error) {
	client, err := c.getClient()
	if err != nil {
		return 0, "", time.Time{}, err
	}

	req := &pb.ValidateTokenReq{
		Token: token,
	}

	// Call RPC
	rsp, err := client.ValidateToken(ctx, req)
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("rpc ValidateToken failed: %w", err)
	}

	if !rsp.Valid {
		return 0, "", time.Time{}, fmt.Errorf("token invalid")
	}

	return rsp.UserId, rsp.Username, rsp.ExpiresAt.AsTime(), nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
