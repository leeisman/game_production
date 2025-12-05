package gateway_test

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	gmsDomain "github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	gsDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/frankieli/game_product/pkg/logger"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

func init() {
	logger.Init(logger.Config{Level: "debug", Format: "console"})
}

type MockBroadcaster struct{}

func (m *MockBroadcaster) Broadcast(gameCode string, message proto.Message)                {}
func (m *MockBroadcaster) SendToUser(userID int64, gameCode string, message proto.Message) {}
func (m *MockBroadcaster) RoundResult(ctx context.Context, req *pbColorGame.ColorGameRoundResultReq) (*pbColorGame.ColorGameRoundResultRsp, error) {
	return &pbColorGame.ColorGameRoundResultRsp{}, nil
}

func (m *MockBroadcaster) PlaceBet(ctx context.Context, req *pbColorGame.ColorGamePlaceBetReq) (*pbColorGame.ColorGamePlaceBetRsp, error) {
	return &pbColorGame.ColorGamePlaceBetRsp{}, nil
}

func (m *MockBroadcaster) GetState(ctx context.Context, req *pbColorGame.ColorGameGetStateReq) (*pbColorGame.ColorGameGetStateRsp, error) {
	return &pbColorGame.ColorGameGetStateRsp{}, nil
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
