package grpc_client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/frankieli/game_product/pkg/service"
	pbUser "github.com/frankieli/game_product/shared/proto/user"
)

// Client handles gRPC connections to various services
type Client struct {
	registry discovery.Registry

	// Connections cache
	conns   map[string]*grpc.ClientConn
	connsMu sync.RWMutex

	// Clients
	userClient pbUser.UserServiceClient
}

// NewClient creates a new unified gRPC client manager
func NewClient(registry discovery.Registry) *Client {
	return &Client{
		registry: registry,
		conns:    make(map[string]*grpc.ClientConn),
	}
}

// getConn implements lazy connection logic for a service
func (c *Client) getConn(serviceName string) (*grpc.ClientConn, error) {
	c.connsMu.RLock()
	conn, ok := c.conns[serviceName]
	c.connsMu.RUnlock()
	if ok {
		return conn, nil
	}

	c.connsMu.Lock()
	defer c.connsMu.Unlock()

	// Double check
	if conn, ok := c.conns[serviceName]; ok {
		return conn, nil
	}

	// Discovery
	addr, err := c.registry.GetService(serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover service %s: %w", serviceName, err)
	}

	// Dial
	conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial service %s at %s: %w", serviceName, addr, err)
	}

	c.conns[serviceName] = conn
	logger.InfoGlobal().Str("service", serviceName).Str("addr", addr).Msg("Established gRPC connection")

	return conn, nil
}

// --- User Service Implementation ---

// Ensure Client implements UserService
var _ service.UserService = (*Client)(nil)

// getUserClient lazily initializes the UserServiceClient
func (c *Client) getUserClient() (pbUser.UserServiceClient, error) {
	if c.userClient != nil {
		return c.userClient, nil
	}

	conn, err := c.getConn("auth-service") // TODO: Make service name configurable
	if err != nil {
		return nil, err
	}

	c.userClient = pbUser.NewUserServiceClient(conn)
	return c.userClient, nil
}

// ValidateToken implements service.UserService
func (c *Client) ValidateToken(ctx context.Context, token string) (int64, string, time.Time, error) {
	client, err := c.getUserClient()
	if err != nil {
		return 0, "", time.Time{}, err
	}

	req := &pbUser.ValidateTokenReq{Token: token}
	rsp, err := client.ValidateToken(ctx, req)
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("rpc ValidateToken failed: %w", err)
	}

	if !rsp.Valid {
		return 0, "", time.Time{}, fmt.Errorf("token invalid")
	}

	return rsp.UserId, rsp.Username, rsp.ExpiresAt.AsTime(), nil
}

// Close closes all connections
func (c *Client) Close() error {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}
	return nil
}
