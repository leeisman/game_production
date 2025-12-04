// Package local provides local adapters for the gateway module.
package local

import (
	"encoding/json"
	"strings"

	"github.com/frankieli/game_product/internal/modules/gateway/ws"

	"google.golang.org/protobuf/proto"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// Broadcaster receives events and broadcasts them to WebSocket clients
type Broadcaster struct {
	gatewayBroadcaster *ws.Manager
}

func NewBroadcaster(gatewayBroadcaster *ws.Manager) *Broadcaster {
	return &Broadcaster{
		gatewayBroadcaster: gatewayBroadcaster,
	}
}

func (b *Broadcaster) convertEvent(event proto.Message) []byte {
	switch e := event.(type) {
	case *pbColorGame.GameEvent:
		// Convert Enum to string (e.g. EVENT_TYPE_ROUND_STARTED -> round_started)
		eventType := strings.ToLower(strings.TrimPrefix(e.Type.String(), "EVENT_TYPE_"))

		// Try to parse inner data if it's JSON
		var innerData interface{} = e.Data
		if len(e.Data) > 0 {
			var parsed interface{}
			if err := json.Unmarshal([]byte(e.Data), &parsed); err == nil {
				innerData = parsed
			}
		}

		// Convert to JSON for WebSocket clients (Standard Header + Body)
		jsonMsg, err := json.Marshal(map[string]interface{}{
			"game":    "color_game",
			"command": eventType,
			"data": map[string]interface{}{
				"round_id":  e.RoundId,
				"data":      innerData, // This might be redundant if data is already flattened, but let's keep it generic
				"timestamp": e.Timestamp,
			},
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
