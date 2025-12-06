package base

import (
	"context"
	"fmt"
	"time"

	"github.com/frankieli/game_product/pkg/service"
	pbUser "github.com/frankieli/game_product/shared/proto/user"
)

// Ensure BaseClient implements UserService
var _ service.UserService = (*BaseClient)(nil)

// getUserClient returns a new client wrapper using a Load Balanced connection
func (c *BaseClient) getUserClient() (pbUser.UserServiceClient, error) {
	conn, err := c.GetLBConn("user-service") // Load Balanced
	if err != nil {
		return nil, err
	}
	return pbUser.NewUserServiceClient(conn), nil
}

// ValidateToken implements service.UserService
func (c *BaseClient) ValidateToken(ctx context.Context, token string) (int64, string, time.Time, error) {
	// Inject Request ID
	ctx = c.withRequestID(ctx)

	// LB Per Request
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
