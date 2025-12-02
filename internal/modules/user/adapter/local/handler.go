package local

import (
	"context"
	"time"

	"github.com/frankieli/game_product/internal/modules/user/usecase"
	"github.com/frankieli/game_product/pkg/logger"
)

// Handler is the local adapter for User service (Monolith / Gateway mode)
// It implements contract.UserService.
type Handler struct {
	userUC *usecase.UserUseCase
}

// NewHandler creates a new local user handler
func NewHandler(userUC *usecase.UserUseCase) *Handler {
	return &Handler{
		userUC: userUC,
	}
}

// Register registers a new user
func (h *Handler) Register(ctx context.Context, username, password, email string) (int64, error) {
	logger.Info(ctx).
		Str("username", username).
		Str("email", email).
		Msg("用户注册请求")

	userID, err := h.userUC.Register(ctx, username, password, email)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Str("username", username).
			Msg("用户注册失败")
		return 0, err
	}

	logger.Info(ctx).
		Int64("user_id", userID).
		Str("username", username).
		Msg("用户注册成功")

	return userID, nil
}

// Login authenticates a user
func (h *Handler) Login(ctx context.Context, username, password string) (int64, string, string, time.Time, error) {
	logger.Info(ctx).
		Str("username", username).
		Msg("用户登录请求")

	userID, token, refreshToken, expiresAt, err := h.userUC.Login(ctx, username, password)
	if err != nil {
		logger.Warn(ctx).
			Err(err).
			Str("username", username).
			Msg("用户登录失败")
		return 0, "", "", time.Time{}, err
	}

	logger.Info(ctx).
		Int64("user_id", userID).
		Str("username", username).
		Msg("用户登录成功")

	return userID, token, refreshToken, expiresAt, nil
}

// Logout logs out a user
func (h *Handler) Logout(ctx context.Context, token string) error {
	logger.Info(ctx).Msg("用户登出请求")

	err := h.userUC.Logout(ctx, token)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Msg("用户登出失败")
		return err
	}

	logger.Info(ctx).Msg("用户登出成功")
	return nil
}

// ValidateToken validates a token
func (h *Handler) ValidateToken(ctx context.Context, token string) (int64, string, time.Time, error) {
	logger.Debug(ctx).Msg("验证 Token 请求")

	userID, username, expiresAt, err := h.userUC.ValidateToken(ctx, token)
	if err != nil {
		logger.Debug(ctx).
			Err(err).
			Msg("Token 验证失败")
		return 0, "", time.Time{}, err
	}

	logger.Debug(ctx).
		Int64("user_id", userID).
		Str("username", username).
		Msg("Token 验证成功")

	return userID, username, expiresAt, nil
}

// RefreshToken refreshes a token
func (h *Handler) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	logger.Info(ctx).Msg("刷新 Token 请求")

	token, newRefreshToken, expiresAt, err := h.userUC.RefreshToken(ctx, refreshToken)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Msg("刷新 Token 失败")
		return "", "", time.Time{}, err
	}

	logger.Info(ctx).Msg("刷新 Token 成功")

	return token, newRefreshToken, expiresAt, nil
}
