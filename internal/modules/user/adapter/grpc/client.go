package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/frankieli/game_product/pkg/service"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
	pb "github.com/frankieli/game_product/shared/proto/user"
	"google.golang.org/grpc"
)

// ClientAdapter adapts gRPC client to service.UserService interface
type ClientAdapter struct {
	client pb.UserServiceClient
}

// NewUserClient creates a new user service client adapter
func NewUserClient(conn *grpc.ClientConn) service.UserService {
	return &ClientAdapter{
		client: pb.NewUserServiceClient(conn),
	}
}

// ValidateToken validates the token via gRPC
func (c *ClientAdapter) ValidateToken(ctx context.Context, token string) (int64, string, time.Time, error) {
	req := &pb.ValidateTokenReq{
		Token: token,
	}

	resp, err := c.client.ValidateToken(ctx, req)
	if err != nil {
		return 0, "", time.Time{}, err
	}

	if resp.ErrorCode != pbCommon.ErrorCode_SUCCESS {
		// TODO: Map ErrorCode back to specific Go errors if needed
		return 0, "", time.Time{}, fmt.Errorf("validate token failed: %s", resp.ErrorCode)
	}

	// Handle potential nil ExpiresAt
	var expiresAt time.Time
	if resp.ExpiresAt != nil {
		expiresAt = resp.ExpiresAt.AsTime()
	}

	return resp.UserId, resp.Username, expiresAt, nil
}
