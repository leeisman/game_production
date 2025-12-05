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
	receivedEvents := make([]*pbColorGame.ColorGameEvent, 0)
	timeout := time.After(1500 * time.Millisecond)

	expectedEventTypes := []pbColorGame.ColorGameEventType{
		pbColorGame.ColorGameEventType_EVENT_TYPE_ROUND_STARTED,
		pbColorGame.ColorGameEventType_EVENT_TYPE_BETTING_STARTED,
		pbColorGame.ColorGameEventType_EVENT_TYPE_DRAWING,
		pbColorGame.ColorGameEventType_EVENT_TYPE_RESULT,
	}

collectLoop:
	for {
		select {
		case msg := <-gatewayBroadcaster.Messages:
			switch event := msg.(type) {
			case *pbColorGame.ColorGameEvent:
				receivedEvents = append(receivedEvents, event)
				t.Logf("Received GameEvent: %s (round: %s)", event.Type, event.RoundId)
			case *pbColorGame.ColorGameRoundStateBRC:
				// Map BRC back to GameEvent for verification simplicity, or verify BRC directly
				// Here we map it to a dummy GameEvent with the type string
				eventType := pbColorGame.ColorGameEventType(pbColorGame.ColorGameEventType_value[event.State])
				dummyEvent := &pbColorGame.ColorGameEvent{
					Type:    eventType,
					RoundId: event.RoundId,
				}
				receivedEvents = append(receivedEvents, dummyEvent)
				t.Logf("Received BRC: %s (round: %s)", event.State, event.RoundId)
			}

			// Check if we received all expected events
			// Note: We might receive duplicates or extra events, so we check if we have covered all expected types
			allFound := true
			currentTypeMap := make(map[pbColorGame.ColorGameEventType]bool)
			for _, e := range receivedEvents {
				currentTypeMap[e.Type] = true
			}
			for _, expected := range expectedEventTypes {
				if !currentTypeMap[expected] {
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

	// 4. Verify events
	if len(receivedEvents) < len(expectedEventTypes) {
		t.Errorf("Expected at least %d events, got %d", len(expectedEventTypes), len(receivedEvents))
	}

	// Verify event types
	eventTypeMap := make(map[pbColorGame.ColorGameEventType]bool)
	for _, event := range receivedEvents {
		eventTypeMap[event.Type] = true
	}

	for _, expectedType := range expectedEventTypes {
		if !eventTypeMap[expectedType] {
			t.Errorf("Expected event type '%s' not found", expectedType)
		}
	}

	// 5. Verify GS broadcaster only received result event
	gsReceivedEvents := make([]*pbColorGame.ColorGameEvent, 0)
	gsTimeout := time.After(100 * time.Millisecond)

gsLoop:
	for {
		select {
		case msg := <-gsBroadcaster.Messages:
			if gameEvent, ok := msg.(*pbColorGame.ColorGameEvent); ok {
				gsReceivedEvents = append(gsReceivedEvents, gameEvent)
			}
		case <-gsTimeout:
			break gsLoop
		}
	}

	// GS should only receive result events
	for _, event := range gsReceivedEvents {
		if event.Type != pbColorGame.ColorGameEventType_EVENT_TYPE_RESULT {
			t.Errorf("GS broadcaster should only receive 'result' events, got '%s'", event.Type)
		}
	}

	if len(gsReceivedEvents) == 0 {
		t.Error("GS broadcaster should have received at least one result event")
	}

	t.Logf("✅ GMS broadcast test passed! Gateway received %d events, GS received %d result events",
		len(receivedEvents), len(gsReceivedEvents))
}
