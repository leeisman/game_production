// Package usecase implements the business logic for the gateway module.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frankieli/game_product/pkg/service/colorgame"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
)

// GatewayUseCase handles gateway logic
type GatewayUseCase struct {
	colorGameSvc colorgame.ColorGameService
}

// NewGatewayUseCase creates a new gateway use case
func NewGatewayUseCase(colorGameSvc colorgame.ColorGameService) *GatewayUseCase {
	return &GatewayUseCase{
		colorGameSvc: colorGameSvc,
	}
}

// MessageHeader defines the structure for message routing
type MessageHeader struct {
	Game    string `json:"game"`    // e.g. "color_game"
	Command string `json:"command"` // e.g. "place_bet"
}

// HandleMessage forwards message to game service
func (uc *GatewayUseCase) HandleMessage(ctx context.Context, userID int64, message []byte) ([]byte, error) {
	var header MessageHeader
	if err := json.Unmarshal(message, &header); err != nil {
		return nil, fmt.Errorf("invalid message format: %w", err)
	}

	// Determine Game and Command
	game := header.Game
	command := header.Command

	if game == "" || command == "" {
		return nil, fmt.Errorf("missing game or command")
	}

	switch game {
	case "color_game":
		return uc.handleColorGame(ctx, userID, command, message)
	default:
		return nil, fmt.Errorf("unknown game: %s", game)
	}
}

func (uc *GatewayUseCase) handleColorGame(ctx context.Context, userID int64, command string, message []byte) ([]byte, error) {
	switch command {
	case "place_bet":
		// Extract payload for PlaceBet
		var req struct {
			Color  string `json:"color"`
			Amount int64  `json:"amount"`
		}
		if err := json.Unmarshal(message, &req); err != nil {
			return nil, fmt.Errorf("invalid place_bet payload: %w", err)
		}

		rsp, err := uc.colorGameSvc.PlaceBet(ctx, &pbColorGame.PlaceBetReq{
			UserId: userID,
			Color:  req.Color,
			Amount: req.Amount,
		})
		if err != nil {
			return json.Marshal(map[string]interface{}{
				"type":    "bet_error",
				"success": false,
				"error":   err.Error(),
			})
		}
		// Also check application level error if needed, though usually err covers it or Success flag
		if rsp.ErrorCode != pbCommon.ErrorCode_SUCCESS {
			return json.Marshal(map[string]interface{}{
				"type":    "bet_error",
				"success": false,
				"error":   rsp.Error,
			})
		}

		return json.Marshal(map[string]interface{}{
			"type":    "bet_placed",
			"success": true,
			"bet_id":  rsp.BetId,
			"color":   req.Color,
			"amount":  req.Amount,
		})

	case "get_state":
		rsp, err := uc.colorGameSvc.GetState(ctx, &pbColorGame.GetStateReq{
			UserId: userID,
		})
		if err != nil {
			return json.Marshal(map[string]interface{}{
				"type":  "state_error",
				"error": err.Error(),
			})
		}

		// state_json is bytes in proto, we might need to unmarshal it to embed it properly in JSON
		// or just return it as raw json message if the client expects that.
		// Assuming we want to embed it:
		var stateData interface{}
		if len(rsp.StateJson) > 0 {
			_ = json.Unmarshal(rsp.StateJson, &stateData)
		}

		return json.Marshal(map[string]interface{}{
			"type": "game_state",
			"data": stateData,
		})

	default:
		return nil, fmt.Errorf("unknown command for color_game: %s", command)
	}
}
