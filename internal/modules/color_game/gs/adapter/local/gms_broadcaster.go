// Package local provides local adapters for the color game GS module.
package local

import (
	"context"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	"github.com/frankieli/game_product/pkg/logger"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
	"google.golang.org/protobuf/proto"
)

// GSBroadcaster listens to GMS events and triggers GS logic (like settlement)
// It implements the Broadcaster interface from GMS perspective.
type GSBroadcaster struct {
	playerUC *usecase.PlayerUseCase
}

func NewGSBroadcaster(playerUC *usecase.PlayerUseCase) *GSBroadcaster {
	return &GSBroadcaster{
		playerUC: playerUC,
	}
}

func (b *GSBroadcaster) Broadcast(event proto.Message) {
	switch gameEvent := event.(type) {
	case *pbColorGame.GameEvent:
		// Filter events: only process 'result' events to trigger settlement
		if gameEvent.Type == pbColorGame.EventType_EVENT_TYPE_RESULT {
			// Parse result data to get winning color (e.g., "green")
			color := domain.Color(gameEvent.Data)

			// Run settlement asynchronously
			go func() {
				ctx := context.Background()
				if err := b.playerUC.SettleRound(ctx, gameEvent.RoundId, color); err != nil {
					logger.ErrorGlobal().Err(err).Str("round_id", gameEvent.RoundId).Msg("Settlement failed")
				}
			}()
		}
	default:
		// Ignore other event types
	}
}
