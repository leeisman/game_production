package grpc

import (
	"context"
	"time"

	pb "github.com/frankieli/game_product/shared/proto/user"
	"google.golang.org/grpc"
)

// UserClient implements service.UserService over gRPC.
type UserClient struct {
	client pb.UserServiceClient
}

func NewUserClient(conn *grpc.ClientConn) *UserClient {
	return &UserClient{
		client: pb.NewUserServiceClient(conn),
	}
}

func (c *UserClient) ValidateToken(ctx context.Context, token string) (int64, string, time.Time, error) {
	pbReq := &pb.ValidateTokenReq{
		Token: token,
	}

	resp, err := c.client.ValidateToken(ctx, pbReq)
	if err != nil {
		return 0, "", time.Time{}, err
	}

	return resp.UserId, resp.Username, resp.ExpiresAt.AsTime(), nil
}
