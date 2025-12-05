package service

import "google.golang.org/protobuf/proto"

// GSBroadcaster defines the interface for broadcasting messages to the Game Service (GS)
type GSBroadcaster interface {
	GSBroadcast(event proto.Message)
}
