package service

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// GatewayService defines the interface for broadcasting messages via the Gateway
type GatewayService interface {
	// Broadcast sends a message to all connected players for a specific game
	Broadcast(ctx context.Context, gameCode string, event proto.Message)
	// SendToUser sends a message to a specific user
	SendToUser(ctx context.Context, userID int64, gameCode string, event proto.Message)
}
