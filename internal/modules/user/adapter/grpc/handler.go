package grpc

import (
	"context"

	"github.com/frankieli/game_product/internal/modules/user/usecase"
	"github.com/frankieli/game_product/pkg/logger"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
	pb "github.com/frankieli/game_product/shared/proto/user"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler implements the gRPC server for User service
type Handler struct {
	pb.UnimplementedUserServiceServer
	userUC *usecase.UserUseCase
}

// NewHandler creates a new gRPC user handler
func NewHandler(userUC *usecase.UserUseCase) *Handler {
	return &Handler{
		userUC: userUC,
	}
}

// ValidateToken implements the ValidateToken RPC
func (h *Handler) ValidateToken(ctx context.Context, req *pb.ValidateTokenReq) (*pb.ValidateTokenRsp, error) {
	logger.Debug(ctx).Msg("gRPC 验证 Token 请求")

	userID, username, expiresAt, err := h.userUC.ValidateToken(ctx, req.Token)
	if err != nil {
		logger.Debug(ctx).
			Err(err).
			Msg("Token 验证失败")
		return &pb.ValidateTokenRsp{
			ErrorCode: pbCommon.ErrorCode_UNAUTHORIZED,
			Valid:     false,
		}, nil
	}

	logger.Debug(ctx).
		Int64("user_id", userID).
		Str("username", username).
		Msg("Token 验证成功")

	return &pb.ValidateTokenRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		Valid:     true,
		UserId:    userID,
		Username:  username,
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}
