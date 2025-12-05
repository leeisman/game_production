// Package usecase implements the business logic for the gateway module.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frankieli/game_product/pkg/logger"
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
	case "ColorGamePlaceBetREQ":
		var payload struct {
			Color  string `json:"color"`
			Amount int64  `json:"amount"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			logger.Error(ctx).
				Err(err).
				Int64("user_id", userID).
				Str("command", command).
				Msg("Failed to unmarshal PlaceBet payload")
			return nil, fmt.Errorf("invalid place_bet payload: %w", err)
		}

		rsp, err := uc.colorGameSvc.PlaceBet(ctx, &pbColorGame.ColorGamePlaceBetReq{
			UserId: userID,
			Color:  payload.Color,
			Amount: payload.Amount,
		})

		// Helper to build error response
		buildError := func(errCode pbCommon.ErrorCode, errMsg string) ([]byte, error) {
			logger.Warn(ctx).
				Int64("user_id", userID).
				Str("command", command).
				Str("error_code", errCode.String()).
				Str("error", errMsg).
				Msg("PlaceBet failed")
			return json.Marshal(map[string]interface{}{
				"game_code": "color_game",
				"command":   "ColorGamePlaceBetRSP",
				"data": map[string]interface{}{
					"error_code": int32(errCode),
					"bet_id":     "",
					"error":      errMsg,
				},
			})
		}

		if err != nil {
			return buildError(pbCommon.ErrorCode_INTERNAL_ERROR, err.Error())
		}
		if rsp.ErrorCode != pbCommon.ErrorCode_SUCCESS {
			return buildError(rsp.ErrorCode, rsp.Error)
		}

		return json.Marshal(map[string]interface{}{
			"game_code": "color_game",
			"command":   "ColorGamePlaceBetRSP",
			"data": map[string]interface{}{
				"error_code": int32(pbCommon.ErrorCode_SUCCESS),
				"bet_id":     rsp.BetId,
				"error":      "",
			},
		})

	case "ColorGameGetStateREQ":
		rsp, err := uc.colorGameSvc.GetState(ctx, &pbColorGame.ColorGameGetStateReq{
			UserId: userID,
		})
		if err != nil {
			logger.Error(ctx).
				Err(err).
				Int64("user_id", userID).
				Str("command", command).
				Msg("GetState failed")
			return json.Marshal(map[string]interface{}{
				"game_code": "color_game",
				"command":   "error",
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
			"game_code": "color_game",
			"command":   "ColorGameGetStateRSP",
			"data":      stateData,
		})

	default:
		logger.Error(ctx).
			Int64("user_id", userID).
			Str("command", command).
			Msg("Unknown command for color_game")
		return nil, fmt.Errorf("unknown command for color_game: %s", command)
	}
}
