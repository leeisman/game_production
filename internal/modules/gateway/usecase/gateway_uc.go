// Package usecase implements the business logic for the gateway module.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	colorgame "github.com/frankieli/game_product/pkg/service/color_game"
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
