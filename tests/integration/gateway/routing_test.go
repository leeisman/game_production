package gateway_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gmsLocal "github.com/frankieli/game_product/internal/modules/color_game/gms/adapter/local"
	gmsMachine "github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	gmsUC "github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	gsLocal "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/local"
	gsRepo "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	gsUC "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	gatewayUC "github.com/frankieli/game_product/internal/modules/gateway/usecase"
	"github.com/frankieli/game_product/internal/modules/wallet"
)

func TestGatewayFlow(t *testing.T) {
	// 1. Setup GMS & GS (Needed to test the full flow)
	stateMachine := gmsMachine.NewStateMachine()
	stateMachine.WaitDuration = 50 * time.Millisecond
	stateMachine.BettingDuration = 500 * time.Millisecond
	stateMachine.DrawingDuration = 100 * time.Millisecond
	stateMachine.ResultDuration = 100 * time.Millisecond
	stateMachine.RestDuration = 50 * time.Millisecond

	broadcaster := &MockBroadcaster{}
	gameRoundRepo := &MockGameRoundRepository{}
	roundUC := gmsUC.NewGMSUseCase(stateMachine, broadcaster, broadcaster, gameRoundRepo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go stateMachine.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	gmsHandler := gmsLocal.NewHandler(roundUC)
	// gmsClient := gsLocal.NewGMSClient(gmsHandler)
	betRepo := gsRepo.NewBetRepository()
	betOrderRepo := &MockBetOrderRepository{}
	walletSvc := wallet.NewMockService()

	gsBroadcaster := &MockBroadcaster{}
	playerUC := gsUC.NewGSUseCase(betRepo, betOrderRepo, gmsHandler, walletSvc, gsBroadcaster)
	gsHandler := gsLocal.NewHandler(playerUC)

	// 2. Setup Gateway
	gateway := gatewayUC.NewGatewayUseCase(gsHandler)

	// 3. Test: Place Bet via Gateway (New Protocol)
	userID := int64(2001)

	// Wait for betting state
	time.Sleep(100 * time.Millisecond)

	req := map[string]interface{}{
		"game_code": "color_game",
		"command":   "ColorGamePlaceBetREQ",
		"data": map[string]interface{}{
			"color":  "green",
			"amount": 50,
		},
	}
	reqJSON, _ := json.Marshal(req)

	respJSON, err := gateway.HandleMessage(ctx, userID, reqJSON)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	// 4. Verify Response
	var resp map[string]interface{}
	if err := json.Unmarshal(respJSON, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["game_code"] != "color_game" {
		t.Errorf("Expected game_code color_game, got %v", resp["game_code"])
	}

	if resp["command"] != "ColorGamePlaceBetRSP" {
		t.Errorf("Expected command ColorGamePlaceBetRSP, got %v", resp["command"])
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	// error_code should be 0 (SUCCESS)
	if errorCode, ok := data["error_code"].(float64); !ok || errorCode != 0 {
		t.Errorf("Expected error_code 0 (SUCCESS), got %v (error: %v)", data["error_code"], data["error"])
	}

	t.Logf("Gateway Response: %s", string(respJSON))
}
