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
	stateMachine.BettingDuration = 200 * time.Millisecond
	stateMachine.DrawingDuration = 100 * time.Millisecond
	stateMachine.ResultDuration = 100 * time.Millisecond

	gatewayBroadcaster := &TestBroadcaster{Messages: make(chan proto.Message, 100)}
	gsBroadcaster := &TestBroadcaster{Messages: make(chan proto.Message, 100)}
	gameRoundRepo := &MockGameRoundRepository{}
	roundUC := gmsUC.NewRoundUseCase(stateMachine, gatewayBroadcaster, gsBroadcaster, gameRoundRepo)
	_ = gmsLocal.NewHandler(roundUC)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 2. Start state machine
	go stateMachine.Start(ctx)

	// 3. Collect events
	receivedEvents := make([]*pbColorGame.GameEvent, 0)
	timeout := time.After(1500 * time.Millisecond)

	expectedEventTypes := []pbColorGame.EventType{
		pbColorGame.EventType_EVENT_TYPE_ROUND_STARTED,
		pbColorGame.EventType_EVENT_TYPE_BETTING_STARTED,
		pbColorGame.EventType_EVENT_TYPE_DRAWING,
		pbColorGame.EventType_EVENT_TYPE_RESULT,
	}

collectLoop:
	for {
		select {
		case msg := <-gatewayBroadcaster.Messages:
			if gameEvent, ok := msg.(*pbColorGame.GameEvent); ok {
				receivedEvents = append(receivedEvents, gameEvent)
				t.Logf("Received event: %s (round: %s)", gameEvent.Type, gameEvent.RoundId)

				// Check if we received all expected events
				if len(receivedEvents) >= len(expectedEventTypes) {
					break collectLoop
				}
			}
		case <-timeout:
			t.Log("Timeout reached, checking received events")
			break collectLoop
		}
	}

	// 4. Verify events
	if len(receivedEvents) < len(expectedEventTypes) {
		t.Errorf("Expected at least %d events, got %d", len(expectedEventTypes), len(receivedEvents))
	}

	// Verify event types
	eventTypeMap := make(map[pbColorGame.EventType]bool)
	for _, event := range receivedEvents {
		eventTypeMap[event.Type] = true
	}

	for _, expectedType := range expectedEventTypes {
		if !eventTypeMap[expectedType] {
			t.Errorf("Expected event type '%s' not found", expectedType)
		}
	}

	// 5. Verify GS broadcaster only received result event
	gsReceivedEvents := make([]*pbColorGame.GameEvent, 0)
	gsTimeout := time.After(100 * time.Millisecond)

gsLoop:
	for {
		select {
		case msg := <-gsBroadcaster.Messages:
			if gameEvent, ok := msg.(*pbColorGame.GameEvent); ok {
				gsReceivedEvents = append(gsReceivedEvents, gameEvent)
			}
		case <-gsTimeout:
			break gsLoop
		}
	}

	// GS should only receive result events
	for _, event := range gsReceivedEvents {
		if event.Type != pbColorGame.EventType_EVENT_TYPE_RESULT {
			t.Errorf("GS broadcaster should only receive 'result' events, got '%s'", event.Type)
		}
	}

	if len(gsReceivedEvents) == 0 {
		t.Error("GS broadcaster should have received at least one result event")
	}

	t.Logf("✅ GMS broadcast test passed! Gateway received %d events, GS received %d result events",
		len(receivedEvents), len(gsReceivedEvents))
}
