package domain

import "google.golang.org/protobuf/proto"

// Broadcaster defines the interface for broadcasting messages to players
type Broadcaster interface {
	// Broadcast sends a message to all connected players
	Broadcast(event proto.Message)
}
