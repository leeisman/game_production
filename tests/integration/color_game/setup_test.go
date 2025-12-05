package colorgame_test

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	gmsDomain "github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	gsDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/frankieli/game_product/pkg/logger"
)

func init() {
	// Init logger for all tests in this package
	logger.Init(logger.Config{Level: "debug", Format: "console"})
}

type TestBroadcaster struct {
	Messages chan proto.Message
}

func NewTestBroadcaster() *TestBroadcaster {
	return &TestBroadcaster{
		Messages: make(chan proto.Message, 100),
	}
}

func (m *TestBroadcaster) Broadcast(gameCode string, message proto.Message) {
	m.Messages <- message
}

func (m *TestBroadcaster) GSBroadcast(message proto.Message) {
	m.Messages <- message
}

func (m *TestBroadcaster) SendToUser(userID int64, gameCode string, message proto.Message) {
	m.Messages <- message
}

// MockGameRoundRepository for testing
type MockGameRoundRepository struct {
	rounds []*gmsDomain.GameRound
}

func (m *MockGameRoundRepository) Create(ctx context.Context, round *gmsDomain.GameRound) error {
	m.rounds = append(m.rounds, round)
	return nil
}

func (m *MockGameRoundRepository) UpdateResult(ctx context.Context, roundID string, result string, endTime *time.Time, totalBets int, totalPlayers int, totalAmount float64) error {
	return nil
}

// MockBetOrderRepository for testing
type MockBetOrderRepository struct {
	orders []*gsDomain.BetOrder
}

func (m *MockBetOrderRepository) BatchCreate(ctx context.Context, orders []*gsDomain.BetOrder) error {
	if m.orders == nil {
		m.orders = make([]*gsDomain.BetOrder, 0)
	}
	m.orders = append(m.orders, orders...)
	return nil
}
