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

// Register implements the Register RPC
func (h *Handler) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterRsp, error) {
	logger.Info(ctx).
		Str("username", req.Username).
		Str("email", req.Email).
		Msg("gRPC 用户注册请求")

	userID, err := h.userUC.Register(ctx, req.Username, req.Password, req.Email)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Str("username", req.Username).
			Msg("用户注册失败")
		// TODO: Map specific errors (e.g. duplicate user)
		return &pb.RegisterRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
			Success:   false,
			Message:   err.Error(),
		}, nil
	}

	logger.Info(ctx).
		Int64("user_id", userID).
		Str("username", req.Username).
		Msg("用户注册成功")

	return &pb.RegisterRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		UserId:    userID,
		Success:   true,
		Message:   "Registration successful",
	}, nil
}

// Login implements the Login RPC
func (h *Handler) Login(ctx context.Context, req *pb.LoginReq) (*pb.LoginRsp, error) {
	logger.Info(ctx).
		Str("username", req.Username).
		Msg("gRPC 用户登录请求")

	userID, token, refreshToken, expiresAt, err := h.userUC.Login(ctx, req.Username, req.Password)
	if err != nil {
		logger.Warn(ctx).
			Err(err).
			Str("username", req.Username).
			Msg("用户登录失败")
		return &pb.LoginRsp{
			ErrorCode: pbCommon.ErrorCode_INVALID_CREDENTIALS, // Assuming login fail is mostly credentials
			Success:   false,
			Message:   err.Error(),
		}, nil
	}

	logger.Info(ctx).
		Int64("user_id", userID).
		Str("username", req.Username).
		Msg("用户登录成功")

	return &pb.LoginRsp{
		ErrorCode:    pbCommon.ErrorCode_SUCCESS,
		UserId:       userID,
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    timestamppb.New(expiresAt),
		Success:      true,
		Message:      "Login successful",
	}, nil
}

// Logout implements the Logout RPC
func (h *Handler) Logout(ctx context.Context, req *pb.LogoutReq) (*pb.LogoutRsp, error) {
	logger.Info(ctx).Msg("gRPC 用户登出请求")

	err := h.userUC.Logout(ctx, req.Token)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Msg("用户登出失败")
		return &pb.LogoutRsp{
			ErrorCode: pbCommon.ErrorCode_INTERNAL_ERROR,
			Success:   false,
			Message:   err.Error(),
		}, nil
	}

	logger.Info(ctx).Msg("用户登出成功")

	return &pb.LogoutRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
		Success:   true,
		Message:   "Logout successful",
	}, nil
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

// RefreshToken implements the RefreshToken RPC
func (h *Handler) RefreshToken(ctx context.Context, req *pb.RefreshTokenReq) (*pb.RefreshTokenRsp, error) {
	logger.Info(ctx).Msg("gRPC 刷新 Token 请求")

	token, newRefreshToken, expiresAt, err := h.userUC.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Msg("刷新 Token 失败")
		return &pb.RefreshTokenRsp{
			ErrorCode: pbCommon.ErrorCode_UNAUTHORIZED,
			Success:   false,
			Message:   err.Error(),
		}, nil
	}

	logger.Info(ctx).Msg("刷新 Token 成功")

	return &pb.RefreshTokenRsp{
		ErrorCode:    pbCommon.ErrorCode_SUCCESS,
		Token:        token,
		RefreshToken: newRefreshToken,
		ExpiresAt:    timestamppb.New(expiresAt),
		Success:      true,
		Message:      "Token refreshed successfully",
	}, nil
}
