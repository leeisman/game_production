package domain

import "google.golang.org/protobuf/proto"

// Broadcaster defines the interface for broadcasting GS events
type Broadcaster interface {
	Broadcast(event proto.Message)
	SendToUser(userID int64, event proto.Message)
}
