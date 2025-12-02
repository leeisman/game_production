package service

import (
	"context"
	"time"
)

// UserService defines the interface for user-related operations exposed to other modules
type UserService interface {
	ValidateToken(ctx context.Context, token string) (int64, string, time.Time, error)
}
