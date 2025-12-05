// Package local provides local adapters for the gateway module.
package local

import (
	"encoding/json"

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
			"command":   "ColorGameRoundStateBRC",
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
