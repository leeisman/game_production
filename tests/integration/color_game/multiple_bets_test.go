package colorgame_test

import (
	"context"
	"testing"
	"time"

	gmsLocal "github.com/frankieli/game_product/internal/modules/color_game/gms/adapter/local"
	gmsDomain "github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	gmsMachine "github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	gmsUC "github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	gsLocal "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/local"
	"google.golang.org/protobuf/proto"

	gsDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	gsRepo "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	gsUC "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"

	"github.com/frankieli/game_product/internal/modules/wallet"
)

func TestMultipleBetsPerUser(t *testing.T) {
	// 1. Setup
	stateMachine := gmsMachine.NewStateMachine()
	stateMachine.WaitDuration = 50 * time.Millisecond
	stateMachine.BettingDuration = 500 * time.Millisecond
	stateMachine.DrawingDuration = 100 * time.Millisecond
	stateMachine.ResultDuration = 100 * time.Millisecond
	stateMachine.RestDuration = 50 * time.Millisecond

	broadcaster := &TestBroadcaster{Messages: make(chan proto.Message, 100)}
	gameRoundRepo := &MockGameRoundRepository{}
	gmsUseCase := gmsUC.NewGMSUseCase(stateMachine, broadcaster, nil, gameRoundRepo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go stateMachine.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	gmsHandler := gmsLocal.NewHandler(gmsUseCase)
	betRepo := gsRepo.NewBetRepository()
	betOrderRepo := &MockBetOrderRepository{}
	walletSvc := wallet.NewMockService()

	playerUC := gsUC.NewGSUseCase(betRepo, betOrderRepo, gmsHandler, walletSvc, broadcaster)

	gsHandler := gsLocal.NewHandler(playerUC)
	gmsUseCase.SetGSBroadcaster(gsHandler)

	// 2. Wait for betting state
	var currentRoundID string
	for i := 0; i < 20; i++ {
		round, err := playerUC.GetCurrentRound(ctx, 1001)
		if err == nil && round["state"] == string(gmsDomain.StateBetting) {
			currentRoundID = round["round_id"].(string)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if currentRoundID == "" {
		t.Fatal("Failed to wait for Betting state")
	}

	// 3. Test: Same user places multiple bets
	userID := int64(1001)
	walletSvc.SetBalance(userID, 1000)

	bets := []struct {
		color  gsDomain.Color
		amount int64
	}{
		{gsDomain.ColorRed, 100},
		{gsDomain.ColorGreen, 50},
		{gsDomain.ColorRed, 150},
	}

	for _, bet := range bets {
		_, err := playerUC.PlaceBet(ctx, userID, bet.color, bet.amount)
		if err != nil {
			t.Fatalf("PlaceBet failed: %v", err)
		}
	}

	// 4. Verify all bets are stored
	userBets, err := betRepo.GetUserBets(ctx, currentRoundID, userID)
	if err != nil {
		t.Fatalf("GetUserBets failed: %v", err)
	}
	// With accumulation, we expect 2 bets (Red and Green), not 3
	if len(userBets) != 2 {
		t.Fatalf("Expected 2 bets, got %d", len(userBets))
	}

	// 5. Verify GetCurrentRound returns all player bets
	round, err := playerUC.GetCurrentRound(ctx, userID)
	if err != nil {
		t.Fatalf("GetCurrentRound failed: %v", err)
	}
	playerBets, ok := round["player_bets"].([]map[string]interface{})
	if !ok {
		t.Fatal("player_bets not found in round response")
	}
	if len(playerBets) != 2 {
		t.Errorf("Expected 2 bets in response, got %d", len(playerBets))
	}

	// 6. Verify wallet deduction
	expectedBalance := int64(1000 - 100 - 50 - 150)
	balance, err := walletSvc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}
	if balance != expectedBalance {
		t.Errorf("Expected balance %d, got %d", expectedBalance, balance)
	}

	// 7. Settle with Red as winning color
	winningColor := gsDomain.ColorRed
	err = playerUC.SettleRound(ctx, currentRoundID, winningColor)
	if err != nil {
		t.Fatalf("SettleRound failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 8. Verify payout (Red bets: 100 + 150 = 250, payout = 500)
	expectedFinalBalance := expectedBalance + 200 + 300 // 2x for each Red bet
	finalBalance, err := walletSvc.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}
	if finalBalance != expectedFinalBalance {
		t.Errorf("Expected final balance %d, got %d", expectedFinalBalance, finalBalance)
	}

	t.Log("✅ Multiple bets per user test passed!")
}
