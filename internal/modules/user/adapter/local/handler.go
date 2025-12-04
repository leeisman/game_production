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
