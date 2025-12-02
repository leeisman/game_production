// Package local provides local adapters for the gateway module.
package local

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/frankieli/game_product/internal/modules/gateway/domain"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// Broadcaster receives events and broadcasts them to WebSocket clients
type Broadcaster struct {
	gatewayBroadcaster domain.GatewayBroadcaster
}

func NewBroadcaster(gatewayBroadcaster domain.GatewayBroadcaster) *Broadcaster {
	return &Broadcaster{
		gatewayBroadcaster: gatewayBroadcaster,
	}
}

func (b *Broadcaster) convertEvent(event proto.Message) []byte {
	switch e := event.(type) {
	case *pbColorGame.GameEvent:
		// Convert Enum to string (e.g. EVENT_TYPE_ROUND_STARTED -> round_started)
		eventType := strings.ToLower(strings.TrimPrefix(e.Type.String(), "EVENT_TYPE_"))

		// Convert to JSON for WebSocket clients
		jsonMsg, err := json.Marshal(map[string]interface{}{
			"type":      eventType,
			"round_id":  e.RoundId,
			"data":      e.Data,
			"timestamp": e.Timestamp,
		})
		if err == nil {
			return jsonMsg
		}
	default:
		// Ignore unknown types
	}
	return nil
}

func (b *Broadcaster) Broadcast(event proto.Message) {
	msgBytes := b.convertEvent(event)
	if msgBytes != nil {
		// Use Broadcast to avoid duplicate messages via Redis
		// since every Gateway instance runs GS logic and connects to GMS.
		b.gatewayBroadcaster.Broadcast(msgBytes)
	}
}

func (b *Broadcaster) SendToUser(userID int64, event proto.Message) {
	msgBytes := b.convertEvent(event)
	if msgBytes != nil {
		b.gatewayBroadcaster.SendToUser(userID, msgBytes)
	}
}
