package colorgame_test

import (
	"context"
	"testing"
	"time"

	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
	"google.golang.org/protobuf/proto"

	gmsLocal "github.com/frankieli/game_product/internal/modules/color_game/gms/adapter/local"
	gmsMachine "github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	gmsUC "github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	gsLocal "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/local"
	gsDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"

	gsRepo "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	gsUC "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"

	"github.com/frankieli/game_product/internal/modules/wallet"
)

func TestSettlement(t *testing.T) {
	// 1. Setup GMS
	stateMachine := gmsMachine.NewStateMachine()
	stateMachine.WaitDuration = 50 * time.Millisecond
	stateMachine.BettingDuration = 500 * time.Millisecond
	stateMachine.DrawingDuration = 100 * time.Millisecond
	stateMachine.ResultDuration = 100 * time.Millisecond
	stateMachine.RestDuration = 50 * time.Millisecond

	broadcaster := &TestBroadcaster{Messages: make(chan proto.Message, 100)}
	gameRoundRepo := &MockGameRoundRepository{}
	roundUC := gmsUC.NewGMSUseCase(stateMachine, broadcaster, broadcaster, gameRoundRepo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go stateMachine.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	// 2. Setup GS
	gmsHandler := gmsLocal.NewHandler(roundUC)
	betRepo := gsRepo.NewBetRepository()
	betOrderRepo := &MockBetOrderRepository{}
	walletSvc := wallet.NewMockService()

	playerUC := gsUC.NewGSUseCase(betRepo, betOrderRepo, gmsHandler, walletSvc, broadcaster)

	gsHandler := gsLocal.NewHandler(playerUC)
	roundUC.SetGSService(gsHandler)

	// 3. Wait for betting state
	var currentRoundID string
	for i := 0; i < 20; i++ {
		round, err := playerUC.GetCurrentRound(ctx, 1001)
		if err != nil {
			t.Logf("GetCurrentRound error: %v", err)
		} else {
			state := round["state"].(string)
			targetState := pbColorGame.ColorGameState_GAME_STATE_BETTING.String()
			t.Logf("Current state: %s, Target: %s, Match: %v, RoundID: %v", state, targetState, state == targetState, round["round_id"])
		}
		if err == nil && round["state"] == pbColorGame.ColorGameState_GAME_STATE_BETTING.String() {
			currentRoundID = round["round_id"].(string)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if currentRoundID == "" {
		t.Fatal("Failed to wait for Betting state")
	}

	t.Logf("Current round: %s", currentRoundID)

	// 4. Setup test data
	testCases := []struct {
		userID int64
		color  gsDomain.Color
		amount int64
	}{
		{1001, pbColorGame.ColorGameReward_REWARD_RED, 100},
		{1002, pbColorGame.ColorGameReward_REWARD_GREEN, 200},
		{1003, pbColorGame.ColorGameReward_REWARD_RED, 150},
		{1004, pbColorGame.ColorGameReward_REWARD_BLUE, 50},
	}

	// Set initial balance and place bets
	for _, tc := range testCases {
		walletSvc.SetBalance(tc.userID, 1000)
		_, err := playerUC.PlaceBet(ctx, tc.userID, tc.color, tc.amount)
		if err != nil {
			t.Fatalf("PlaceBet failed for user %d: %v", tc.userID, err)
		}
	}

	// 5. Trigger settlement with Red as winning color
	winningColor := pbColorGame.ColorGameReward_REWARD_RED
	err := playerUC.SettleRound(ctx, currentRoundID, winningColor)
	if err != nil {
		t.Fatalf("SettleRound failed: %v", err)
	}

	// Give settlement time to complete
	time.Sleep(100 * time.Millisecond)

	// 6. Verify settlement results
	t.Logf("Verifying settlement with winning color: %s", winningColor)

	// Winners (Red): User 1001 (100), User 1003 (150)
	// Losers: User 1002 (Green), User 1004 (Blue)

	expectedBalances := map[int64]int64{
		1001: 1000 - 100 + 200, // Lost 100, won 200 (2x)
		1002: 1000 - 200,       // Lost 200, no win
		1003: 1000 - 150 + 300, // Lost 150, won 300 (2x)
		1004: 1000 - 50,        // Lost 50, no win
	}

	for userID, expectedBalance := range expectedBalances {
		balance, err := walletSvc.GetBalance(ctx, userID)
		if err != nil {
			t.Fatalf("GetBalance failed for user %d: %v", userID, err)
		}
		if balance != expectedBalance {
			t.Errorf("User %d: Expected final balance %d, got %d", userID, expectedBalance, balance)
		} else {
			t.Logf("User %d: Final balance %d ✓", userID, balance)
		}
	}

	// 7. Verify bets are cleared after settlement
	for _, tc := range testCases {
		bets, err := betRepo.GetUserBets(ctx, currentRoundID, tc.userID)
		if err != nil {
			t.Fatalf("GetUserBets failed for user %d: %v", tc.userID, err)
		}
		if len(bets) != 0 {
			t.Errorf("User %d: Expected bets to be cleared, but found %d bets", tc.userID, len(bets))
		}
	}

	// 7. Verify settlement broadcast
	timeout := time.After(2 * time.Second)
	foundSettlement := false

	for {
		select {
		case msg := <-broadcaster.Messages:
			if _, ok := msg.(*pbColorGame.ColorGameSettlementBRC); ok {
				foundSettlement = true
				break
			}
		case <-timeout:
			t.Error("Timeout waiting for settlement broadcast")
			return
		}
		if foundSettlement {
			break
		}
	}

	if !foundSettlement {
		t.Error("Did not receive settlement event")
	}

	t.Log("✅ Settlement test passed!")
}
