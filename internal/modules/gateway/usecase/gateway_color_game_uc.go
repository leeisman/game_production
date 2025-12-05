// Package usecase implements the business logic for the gateway module.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/frankieli/game_product/pkg/logger"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
)

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

		colorEnumVal, ok := pbColorGame.ColorGameReward_value["REWARD_"+strings.ToUpper(payload.Color)]
		if !ok {
			return buildError(pbCommon.ErrorCode_INVALID_BET_OPTION, fmt.Sprintf("invalid color: %s", payload.Color))
		}

		rsp, err := uc.colorGameSvc.PlaceBet(ctx, &pbColorGame.ColorGamePlaceBetReq{
			UserId: userID,
			Color:  pbColorGame.ColorGameReward(colorEnumVal),
			Amount: payload.Amount,
		})
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
