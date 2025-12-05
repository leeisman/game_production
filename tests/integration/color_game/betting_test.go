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

	gsRepo "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	gsUC "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"

	"github.com/frankieli/game_product/internal/modules/wallet"
)

// MockBroadcaster for testing
type MockBroadcaster struct {
	events []proto.Message
}

func (m *MockBroadcaster) Broadcast(gameCode string, event proto.Message) {
	if m.events == nil {
		m.events = make([]proto.Message, 0)
	}
	m.events = append(m.events, event)
}

func (m *MockBroadcaster) SendToUser(userID int64, gameCode string, message proto.Message) {
	if m.events == nil {
		m.events = make([]proto.Message, 0)
	}
	m.events = append(m.events, message)
}

func (m *MockBroadcaster) RoundResult(ctx context.Context, req *pbColorGame.ColorGameRoundResultReq) (*pbColorGame.ColorGameRoundResultRsp, error) {
	if m.events == nil {
		m.events = make([]proto.Message, 0)
	}
	m.events = append(m.events, req)
	return &pbColorGame.ColorGameRoundResultRsp{}, nil
}

func (m *MockBroadcaster) PlaceBet(ctx context.Context, req *pbColorGame.ColorGamePlaceBetReq) (*pbColorGame.ColorGamePlaceBetRsp, error) {
	return &pbColorGame.ColorGamePlaceBetRsp{}, nil
}

func (m *MockBroadcaster) GetState(ctx context.Context, req *pbColorGame.ColorGameGetStateReq) (*pbColorGame.ColorGameGetStateRsp, error) {
	return &pbColorGame.ColorGameGetStateRsp{}, nil
}

func TestBetting(t *testing.T) {
	// 1. Setup GMS
	stateMachine := gmsMachine.NewStateMachine()
	stateMachine.WaitDuration = 50 * time.Millisecond
	stateMachine.BettingDuration = 500 * time.Millisecond
	stateMachine.DrawingDuration = 100 * time.Millisecond
	stateMachine.ResultDuration = 100 * time.Millisecond
	stateMachine.RestDuration = 50 * time.Millisecond

	gatewayBroadcaster := &MockBroadcaster{}
	gsBroadcaster := &MockBroadcaster{}

	// Mock GameRoundRepository for GMS
	gameRoundRepo := &MockGameRoundRepository{}

	roundUC := gmsUC.NewGMSUseCase(stateMachine, gatewayBroadcaster, gsBroadcaster, gameRoundRepo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go stateMachine.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	// 2. Setup GS
	gmsHandler := gmsLocal.NewHandler(roundUC)
	betRepo := gsRepo.NewBetRepository()
	betOrderRepo := &MockBetOrderRepository{}
	walletSvc := wallet.NewMockService()

	gsEventBroadcaster := &MockBroadcaster{}
	gsUseCase := gsUC.NewGSUseCase(betRepo, betOrderRepo, gmsHandler, walletSvc, gsEventBroadcaster)
	gsHandler := gsLocal.NewHandler(gsUseCase)

	// GS Broadcaster (listens to GMS events and triggers settlement)
	// gsHandler implements both ColorGameService and GSBroadcaster
	roundUC.SetGSService(gsHandler)

	// 3. Wait for betting state
	var currentRoundID string
	for i := 0; i < 20; i++ {
		round, err := gsUseCase.GetCurrentRound(ctx, 1001)
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

	// 4. Test: Multiple players place bets
	testCases := []struct {
		userID int64
		color  pbColorGame.ColorGameReward
		amount int64
	}{
		{1001, pbColorGame.ColorGameReward_REWARD_RED, 100},
		{1002, pbColorGame.ColorGameReward_REWARD_GREEN, 200},
		{1003, pbColorGame.ColorGameReward_REWARD_RED, 150},
		{1004, pbColorGame.ColorGameReward_REWARD_BLUE, 50},
	}

	// Set initial balance for all users
	for _, tc := range testCases {
		walletSvc.SetBalance(tc.userID, 2000) // Give enough balance
	}

	// Place initial bets
	for _, tc := range testCases {
		_, err := gsUseCase.PlaceBet(ctx, tc.userID, tc.color, tc.amount)
		if err != nil {
			t.Fatalf("PlaceBet failed for user %d: %v", tc.userID, err)
		}
		t.Logf("User %d placed bet: %s, %d", tc.userID, tc.color, tc.amount)
	}

	// 4.1 Test Accumulate Bet: User 1001 places another bet on Red
	t.Log("Testing bet accumulation...")
	originalBet, _ := betRepo.GetUserBet(ctx, currentRoundID, 1001, pbColorGame.ColorGameReward_REWARD_RED)
	originalBetID := originalBet.BetID

	_, err := gsUseCase.PlaceBet(ctx, 1001, pbColorGame.ColorGameReward_REWARD_RED, 50)
	if err != nil {
		t.Fatalf("Second PlaceBet failed for user 1001: %v", err)
	}

	// Verify accumulation
	updatedBet, _ := betRepo.GetUserBet(ctx, currentRoundID, 1001, pbColorGame.ColorGameReward_REWARD_RED)
	if updatedBet.Amount != 150 { // 100 + 50
		t.Errorf("Bet accumulation failed. Expected 150, got %d", updatedBet.Amount)
	}
	if updatedBet.BetID != originalBetID {
		t.Errorf("Bet ID changed after accumulation. Expected %s, got %s", originalBetID, updatedBet.BetID)
	}
	t.Log("✅ Bet accumulation verified")

	// 5. Verify bets are stored (checking updated amounts)
	// User 1001 should have 150 total
	bets, err := betRepo.GetUserBets(ctx, currentRoundID, 1001)
	if err != nil {
		t.Fatalf("GetUserBets failed: %v", err)
	}
	if len(bets) != 1 {
		t.Errorf("Expected 1 bet record for user 1001, got %d", len(bets))
	} else if bets[0].Amount != 150 {
		t.Errorf("User 1001: Expected total amount 150, got %d", bets[0].Amount)
	}

	// 6. Verify wallet deduction
	// User 1001: 100 + 50 = 150 deducted
	balance, err := walletSvc.GetBalance(ctx, 1001)
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}
	if balance != 1850 { // 2000 - 150
		t.Errorf("User 1001: Expected balance 1850, got %d", balance)
	}

	// 7. Test GetCurrentRound returns player bets
	round, err := gsUseCase.GetCurrentRound(ctx, 1001)
	if err != nil {
		t.Fatalf("GetCurrentRound failed: %v", err)
	}
	playerBets, ok := round["player_bets"].([]map[string]interface{})
	if !ok {
		t.Fatal("player_bets not found in round response")
	}
	if len(playerBets) != 1 {
		t.Errorf("Expected 1 bet for user 1001, got %d", len(playerBets))
	}
	if len(playerBets) > 0 {
		if playerBets[0]["amount"] != int64(150) {
			t.Errorf("Expected amount 150, got %v", playerBets[0]["amount"])
		}
	}

	t.Log("✅ Betting test passed!")
}
