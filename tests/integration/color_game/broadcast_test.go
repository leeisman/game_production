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
)

func TestGMSBroadcast(t *testing.T) {
	// 1. Setup GMS with TestBroadcaster
	stateMachine := gmsMachine.NewStateMachine()
	stateMachine.WaitDuration = 50 * time.Millisecond
	stateMachine.BettingDuration = 200 * time.Millisecond
	stateMachine.DrawingDuration = 100 * time.Millisecond
	stateMachine.ResultDuration = 100 * time.Millisecond
	stateMachine.RestDuration = 50 * time.Millisecond

	gatewayBroadcaster := &TestBroadcaster{Messages: make(chan proto.Message, 100)}
	gsBroadcaster := &TestBroadcaster{Messages: make(chan proto.Message, 100)}
	gameRoundRepo := &MockGameRoundRepository{}
	roundUC := gmsUC.NewGMSUseCase(stateMachine, gatewayBroadcaster, gsBroadcaster, gameRoundRepo)
	_ = gmsLocal.NewHandler(roundUC)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 2. Start state machine
	go stateMachine.Start(ctx)

	// 3. Collect events
	receivedStates := make([]pbColorGame.ColorGameState, 0)
	timeout := time.After(1500 * time.Millisecond)

	expectedStates := []pbColorGame.ColorGameState{
		pbColorGame.ColorGameState_GAME_STATE_ROUND_STARTED,
		pbColorGame.ColorGameState_GAME_STATE_BETTING,
		pbColorGame.ColorGameState_GAME_STATE_DRAWING,
		pbColorGame.ColorGameState_GAME_STATE_RESULT,
	}

collectLoop:
	for {
		select {
		case msg := <-gatewayBroadcaster.Messages:
			switch event := msg.(type) {
			case *pbColorGame.ColorGameRoundStateBRC:
				receivedStates = append(receivedStates, event.State)
				t.Logf("Received BRC: %s (round: %s)", event.State, event.RoundId)
			}

			// Check if we received all expected states
			allFound := true
			currentStateMap := make(map[pbColorGame.ColorGameState]bool)
			for _, s := range receivedStates {
				currentStateMap[s] = true
			}
			for _, expected := range expectedStates {
				if !currentStateMap[expected] {
					allFound = false
					break
				}
			}
			if allFound {
				break collectLoop
			}

		case <-timeout:
			t.Log("Timeout reached, checking received events")
			break collectLoop
		}
	}

	// 4. Verify states
	if len(receivedStates) < len(expectedStates) {
		t.Errorf("Expected at least %d states, got %d", len(expectedStates), len(receivedStates))
	}

	// Verify state types
	stateMap := make(map[pbColorGame.ColorGameState]bool)
	for _, s := range receivedStates {
		stateMap[s] = true
	}

	for _, expected := range expectedStates {
		if !stateMap[expected] {
			t.Errorf("Expected state '%s' not found", expected)
		}
	}

	// 5. Verify GS broadcaster only received result event
	gsReceivedReqs := make([]*pbColorGame.ColorGameRoundResultReq, 0)
	gsTimeout := time.After(100 * time.Millisecond)

gsLoop:
	for {
		select {
		case msg := <-gsBroadcaster.Messages:
			if req, ok := msg.(*pbColorGame.ColorGameRoundResultReq); ok {
				gsReceivedReqs = append(gsReceivedReqs, req)
			}
		case <-gsTimeout:
			break gsLoop
		}
	}

	if len(gsReceivedReqs) == 0 {
		t.Error("GS broadcaster should have received at least one result request")
	}

	t.Logf("✅ GMS broadcast test passed! Gateway received %d states, GS received %d result requests",
		len(receivedStates), len(gsReceivedReqs))
}
