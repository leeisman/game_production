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
	stateMachine.BettingDuration = 500 * time.Millisecond
	stateMachine.DrawingDuration = 100 * time.Millisecond
	stateMachine.ResultDuration = 100 * time.Millisecond

	broadcaster := &MockBroadcaster{}
	gameRoundRepo := &MockGameRoundRepository{}
	roundUC := gmsUC.NewRoundUseCase(stateMachine, broadcaster, broadcaster, gameRoundRepo)

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
	playerUC := gsUC.NewPlayerUseCase(betRepo, betOrderRepo, gmsHandler, walletSvc, gsBroadcaster)
	gsHandler := gsLocal.NewHandler(playerUC)

	// 2. Setup Gateway
	gateway := gatewayUC.NewGatewayUseCase(gsHandler)

	// 3. Test: Place Bet via Gateway (New Protocol)
	userID := int64(2001)

	// Wait for betting state
	time.Sleep(100 * time.Millisecond)

	req := map[string]interface{}{
		"game":    "color_game",
		"command": "place_bet",
		"color":   "green",
		"amount":  50,
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

	if resp["type"] != "bet_placed" {
		t.Errorf("Expected type bet_placed, got %v", resp["type"])
	}
	if resp["success"] != true {
		t.Errorf("Expected success true, got %v", resp["success"])
	}

	t.Logf("Gateway Response: %s", string(respJSON))
}
