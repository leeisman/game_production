package domain

import (
	"context"
)

// GatewayUseCase defines the interface for gateway business logic
type GatewayUseCase interface {
	// HandleMessage handles a message from a user
	HandleMessage(ctx context.Context, userID int64, message []byte) ([]byte, error)
}

// GatewayBroadcaster defines the interface for broadcasting messages via gateway
type GatewayBroadcaster interface {
	// SendToUser sends a message to a specific user
	SendToUser(userID int64, message []byte)

	// Broadcast sends a message to all users
	Broadcast(message []byte)
}
