// Package usecase implements the business logic for the gateway module.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	colorgame "github.com/frankieli/game_product/pkg/service/color_game"
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

// RequestEnvelope defines the standard request structure
type RequestEnvelope struct {
	Game    string          `json:"game"`
	Command string          `json:"command"`
	Data    json.RawMessage `json:"data"`
}

// HandleMessage forwards message to game service
func (uc *GatewayUseCase) HandleMessage(ctx context.Context, userID int64, message []byte) ([]byte, error) {
	var req RequestEnvelope
	if err := json.Unmarshal(message, &req); err != nil {
		return nil, fmt.Errorf("invalid message format: %w", err)
	}

	if req.Game == "" || req.Command == "" {
		return nil, fmt.Errorf("missing game or command")
	}

	switch req.Game {
	case "color_game":
		return uc.handleColorGame(ctx, userID, req.Command, req.Data)
	default:
		return nil, fmt.Errorf("unknown game: %s", req.Game)
	}
}

func (uc *GatewayUseCase) handleColorGame(ctx context.Context, userID int64, command string, data []byte) ([]byte, error) {
	switch command {
	case "place_bet":
		var payload struct {
			Color  string `json:"color"`
			Amount int64  `json:"amount"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, fmt.Errorf("invalid place_bet payload: %w", err)
		}

		rsp, err := uc.colorGameSvc.PlaceBet(ctx, &pbColorGame.PlaceBetReq{
			UserId: userID,
			Color:  payload.Color,
			Amount: payload.Amount,
		})

		// Helper to build error response
		buildError := func(errMsg string) ([]byte, error) {
			return json.Marshal(map[string]interface{}{
				"game":    "color_game",
				"command": "place_bet_response", // Or just "error"
				"data": map[string]interface{}{
					"success": false,
					"error":   errMsg,
				},
			})
		}

		if err != nil {
			return buildError(err.Error())
		}
		if rsp.ErrorCode != pbCommon.ErrorCode_SUCCESS {
			return buildError(rsp.Error)
		}

		return json.Marshal(map[string]interface{}{
			"game":    "color_game",
			"command": "place_bet_response",
			"data": map[string]interface{}{
				"success": true,
				"bet_id":  rsp.BetId,
				"color":   payload.Color,
				"amount":  payload.Amount,
			},
		})

	case "get_state":
		rsp, err := uc.colorGameSvc.GetState(ctx, &pbColorGame.GetStateReq{
			UserId: userID,
		})
		if err != nil {
			return json.Marshal(map[string]interface{}{
				"game":    "color_game",
				"command": "error",
				"data": map[string]interface{}{
					"error": err.Error(),
				},
			})
		}

		var stateData interface{}
		if len(rsp.StateJson) > 0 {
			_ = json.Unmarshal(rsp.StateJson, &stateData)
		}

		return json.Marshal(map[string]interface{}{
			"game":    "color_game",
			"command": "game_state",
			"data":    stateData,
		})

	default:
		return nil, fmt.Errorf("unknown command for color_game: %s", command)
	}
}
