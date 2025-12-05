// Package local provides local adapters for the gateway module.
package local

import (
	"encoding/json"
	"strings"

	"github.com/frankieli/game_product/internal/modules/gateway/ws"

	"google.golang.org/protobuf/proto"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// Handler receives events and broadcasts them to WebSocket clients
// It implements service.GatewayService
type Handler struct {
	wsManager *ws.Manager
}

// NewHandler creates a new gateway handler
func NewHandler(wsManager *ws.Manager) *Handler {
	return &Handler{
		wsManager: wsManager,
	}
}

func (h *Handler) convertEvent(gameCode string, event proto.Message) []byte {
	switch e := event.(type) {
	case *pbColorGame.ColorGameRoundStateBRC:
		finalData := map[string]interface{}{
			"round_id":              e.RoundId,
			"state":                 e.State,
			"betting_end_timestamp": e.BettingEndTimestamp,
			"left_time":             e.LeftTime,
		}

		jsonMsg, err := json.Marshal(map[string]interface{}{
			"game_code": gameCode,
			"command":   "ColorGameStateBRC",
			"data":      finalData,
		})
		if err == nil {
			return jsonMsg
		}

	case *pbColorGame.ColorGameSettlementBRC:
		finalData := map[string]interface{}{
			"round_id":      e.RoundId,
			"winning_color": e.WinningColor,
			"bet_id":        e.BetId,
			"bet_color":     e.BetColor,
			"bet_amount":    e.BetAmount,
			"win_amount":    e.WinAmount,
			"is_winner":     e.IsWinner,
		}

		jsonMsg, err := json.Marshal(map[string]interface{}{
			"game_code": gameCode,
			"command":   "ColorGameSettlementBRC",
			"data":      finalData,
		})
		if err == nil {
			return jsonMsg
		}

	case *pbColorGame.ColorGameEvent:
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
		finalData := map[string]interface{}{
			"round_id":  e.RoundId,
			"timestamp": e.Timestamp,
		}

		// Flatten innerData if it's a map, otherwise put it in "data" field
		if innerData != nil {
			if innerMap, ok := innerData.(map[string]interface{}); ok {
				for k, v := range innerMap {
					finalData[k] = v
				}
			} else {
				finalData["data"] = innerData
			}
		}

		jsonMsg, err := json.Marshal(map[string]interface{}{
			"game_code": gameCode,
			"command":   eventType,
			"data":      finalData,
		})
		if err == nil {
			return jsonMsg
		}
	default:
		// Ignore unknown types
	}
	return nil
}

func (h *Handler) Broadcast(gameCode string, event proto.Message) {
	msgBytes := h.convertEvent(gameCode, event)
	if msgBytes != nil {
		h.wsManager.Broadcast(msgBytes)
	}
}

func (h *Handler) SendToUser(userID int64, gameCode string, event proto.Message) {
	msgBytes := h.convertEvent(gameCode, event)
	if msgBytes != nil {
		h.wsManager.SendToUser(userID, msgBytes)
	}
}
