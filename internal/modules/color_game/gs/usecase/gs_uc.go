// Package usecase implements the business logic for the color game GS module.
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/frankieli/game_product/pkg/service"
	colorgame "github.com/frankieli/game_product/pkg/service/color_game"
	pbColorGame "github.com/frankieli/game_product/shared/proto/colorgame"
)

// GSUseCase handles player betting logic
type GSUseCase struct {
	betRepo            domain.BetRepository
	betOrderRepo       domain.BetOrderRepository
	gmsService         colorgame.GMSService
	walletSvc          service.WalletService
	gatewayBroadcaster service.GatewayService
}

// NewGSUseCase creates a new player use case
func NewGSUseCase(
	betRepo domain.BetRepository,
	betOrderRepo domain.BetOrderRepository,
	gmsService colorgame.GMSService,
	walletSvc service.WalletService,
	gatewayBroadcaster service.GatewayService,
) *GSUseCase {
	return &GSUseCase{
		betRepo:            betRepo,
		betOrderRepo:       betOrderRepo,
		gmsService:         gmsService,
		walletSvc:          walletSvc,
		gatewayBroadcaster: gatewayBroadcaster,
	}
}

// PlaceBet handles a player placing a bet
func (uc *GSUseCase) PlaceBet(ctx context.Context, userID int64, color domain.Color, amount int64) (*domain.Bet, error) {
	// Inject UserID into context logger
	ctx = logger.WithFields(ctx, map[string]interface{}{
		"user_id": userID,
	})

	logger.Info(ctx).
		Str("color", color.String()).
		Int64("amount", amount).
		Msg("下注请求开始")

	// 1. Get current round from GMS
	roundRsp, err := uc.gmsService.GetCurrentRound(ctx, &pbColorGame.ColorGameGetCurrentRoundReq{})
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Msg("获取当前回合失败")
		return nil, fmt.Errorf("failed to get current round: %w", err)
	}

	// Inject RoundID into context logger
	ctx = logger.WithFields(ctx, map[string]interface{}{
		"round_id": roundRsp.RoundId,
	})

	logger.Debug(ctx).
		Str("round_state", roundRsp.State.String()).
		Msg("当前回合信息")

	// 2. Validate color
	if !isValidColor(color) {
		logger.Warn(ctx).
			Str("color", color.String()).
			Msg("无效的颜色")
		return nil, fmt.Errorf("invalid color: %s", color)
	}

	// 3. Deduct from wallet
	_, err = uc.walletSvc.PlaceBet(ctx, userID, amount, roundRsp.RoundId)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Int64("amount", amount).
			Msg("钱包扣款失败")
		return nil, fmt.Errorf("failed to deduct from wallet: %w", err)
	}

	logger.Debug(ctx).
		Int64("amount", amount).
		Msg("钱包扣款成功")

	// 4. Record bet in GMS
	_, err = uc.gmsService.RecordBet(ctx, &pbColorGame.ColorGameRecordBetReq{
		RoundId: roundRsp.RoundId,
		UserId:  userID,
		Color:   color,
		Amount:  amount,
	})
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Msg("GMS 记录下注失败")
		// TODO: Rollback wallet deduction
		return nil, fmt.Errorf("failed to record bet in GMS: %w", err)
	}

	// 5. Check if user already placed a bet on this color
	existingBet, err := uc.betRepo.GetUserBet(ctx, roundRsp.RoundId, userID, color)
	if err != nil {
		logger.Error(ctx).Err(err).Msg("检查现有下注失败")
		return nil, fmt.Errorf("failed to check existing bet: %w", err)
	}

	var bet *domain.Bet

	if existingBet != nil {
		// Update existing bet
		logger.Info(ctx).
			Str("bet_id", existingBet.BetID).
			Int64("existing_amount", existingBet.Amount).
			Int64("additional_amount", amount).
			Msg("更新现有下注金额")

		err = uc.betRepo.UpdateBetAmount(ctx, existingBet, amount)
		if err != nil {
			logger.Error(ctx).Err(err).Msg("更新下注金额失败")
			return nil, fmt.Errorf("failed to update bet amount: %w", err)
		}
		bet = existingBet
	} else {
		// Create new bet
		bet = domain.NewBet(roundRsp.RoundId, userID, color, amount)
		err = uc.betRepo.SaveBet(ctx, bet)
		if err != nil {
			logger.Error(ctx).
				Err(err).
				Str("bet_id", bet.BetID).
				Msg("保存下注记录失败")
			return nil, fmt.Errorf("failed to save bet: %w", err)
		}
	}

	logger.Info(ctx).
		Str("color", color.String()).
		Int64("total_amount", bet.Amount).
		Str("bet_id", bet.BetID).
		Msg("下注成功")

	return bet, nil
}

// GetCurrentRound gets the current round info with player's bets
func (uc *GSUseCase) GetCurrentRound(ctx context.Context, userID int64) (map[string]interface{}, error) {
	roundRsp, err := uc.gmsService.GetCurrentRound(ctx, &pbColorGame.ColorGameGetCurrentRoundReq{
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}

	// Get player's bets for this round
	playerBets, err := uc.betRepo.GetUserBets(ctx, roundRsp.RoundId, userID)
	if err != nil {
		logger.Warn(ctx).
			Err(err).
			Str("round_id", roundRsp.RoundId).
			Int64("user_id", userID).
			Msg("Failed to get user bets")
		// Don't fail the request, just return empty bets
		playerBets = []*domain.Bet{}
	}

	// Convert bets to response format
	bets := make([]map[string]interface{}, 0, len(playerBets))
	for _, bet := range playerBets {
		bets = append(bets, map[string]interface{}{
			"color":  bet.Color.String(),
			"amount": bet.Amount,
		})
	}

	return map[string]interface{}{
		"round_id":    roundRsp.RoundId,
		"state":       roundRsp.State.String(),
		"betting_end": time.Unix(roundRsp.BettingEndTimestamp, 0),
		"player_bets": bets,
	}, nil
}

// SettleRound processes settlement for a round
func (uc *GSUseCase) SettleRound(ctx context.Context, roundID string, winningColor domain.Color) error {
	startTime := time.Now()
	logger.Info(ctx).Str("round_id", roundID).Str("winning_color", winningColor.String()).Msg("Starting settlement")

	// Batch processing configuration
	const batchSize = 500

	var allBetOrders []*domain.BetOrder
	totalProcessed := 0
	batchNumber := 0

	// 1. Process bets in batches
	for {
		bets, err := uc.betRepo.GetBetsForSettlement(ctx, roundID)
		if err != nil {
			return fmt.Errorf("failed to get bets for settlement: %w", err)
		}
		if len(bets) == 0 {
			break // No more bets to process
		}

		// Collect bets into batches
		var currentBatch []*domain.BetOrder
		var currentBets []*domain.Bet
		now := time.Now()

		for _, bet := range bets {
			winAmount := calculateWin(bet, winningColor)

			// Create bet order record
			betOrder := &domain.BetOrder{
				OrderID:   bet.BetID,
				UserID:    bet.UserID,
				RoundID:   roundID,
				GameCode:  "color_game",
				BetArea:   bet.Color.String(),
				Amount:    float64(bet.Amount),
				Payout:    float64(winAmount),
				Status:    domain.BetOrderStatusSettled,
				CreatedAt: bet.Time,
				SettledAt: &now,
			}

			currentBatch = append(currentBatch, betOrder)
			currentBets = append(currentBets, bet)
			allBetOrders = append(allBetOrders, betOrder)

			// When batch is full, process it
			if len(currentBatch) >= batchSize {
				batchNumber++
				if err := uc.processBatch(ctx, roundID, winningColor, currentBatch, currentBets, batchNumber); err != nil {
					return err
				}
				totalProcessed += len(currentBatch)
				currentBatch = nil
				currentBets = nil
			}
		}

		// Process remaining items in the last batch
		if len(currentBatch) > 0 {
			batchNumber++
			if err := uc.processBatch(ctx, roundID, winningColor, currentBatch, currentBets, batchNumber); err != nil {
				return err
			}
			totalProcessed += len(currentBatch)
		}
	}

	// Explicitly clear bets for Memory repo safety (idempotent for Redis)
	_ = uc.betRepo.ClearBets(ctx, roundID)

	// Log settlement summary
	totalBets := len(allBetOrders)
	totalPayout := int64(0)
	winCount := 0
	for _, order := range allBetOrders {
		totalPayout += int64(order.Payout)
		if order.Payout > 0 {
			winCount++
		}
	}

	logger.Info(ctx).
		Str("round_id", roundID).
		Str("winning_color", winningColor.String()).
		Int("total_bets", totalBets).
		Int("total_processed", totalProcessed).
		Int("batches", batchNumber).
		Int("win_count", winCount).
		Int("lose_count", totalBets-winCount).
		Int64("total_payout", totalPayout).
		Dur("duration_ms", time.Since(startTime)).
		Msg("Settlement completed successfully")

	// Broadcast settlement result to all online players
	// Note: Players who bet will receive TWO notifications:
	//   1. Personal notification with their bet details (sent in processBatch)
	//   2. This broadcast notification (for consistency with non-bettors)
	// Frontend should handle deduplication by checking if bet_id is present
	if uc.gatewayBroadcaster != nil {
		uc.gatewayBroadcaster.Broadcast(ctx, "color_game", &pbColorGame.ColorGameSettlementBRC{
			RoundId:      roundID,
			WinningColor: winningColor,
			BetId:        "",
			BetColor:     pbColorGame.ColorGameReward_REWARD_UNSPECIFIED,
			BetAmount:    0,
			WinAmount:    0,
			IsWinner:     false,
		})
		logger.Debug(ctx).
			Str("round_id", roundID).
			Str("winning_color", winningColor.String()).
			Msg("Broadcasted settlement result to all players")
	}

	return nil
}

// processBatch processes a batch of bet orders: write to DB, then handle wallet and notifications
func (uc *GSUseCase) processBatch(ctx context.Context, roundID string, winningColor domain.Color, betOrders []*domain.BetOrder, bets []*domain.Bet, batchNum int) error {
	// 1. Write batch to database
	if uc.betOrderRepo != nil && len(betOrders) > 0 {
		startTime := time.Now()
		if err := uc.betOrderRepo.BatchCreate(ctx, betOrders); err != nil {
			logger.Error(ctx).
				Err(err).
				Int("batch_num", batchNum).
				Int("count", len(betOrders)).
				Str("round_id", roundID).
				Msg("Failed to persist bet orders batch to database")
			return fmt.Errorf("failed to persist bet orders batch %d: %w", batchNum, err)
		}
		duration := time.Since(startTime)
		logger.Info(ctx).
			Int("batch_num", batchNum).
			Int("count", len(betOrders)).
			Str("round_id", roundID).
			Dur("duration_ms", duration).
			Msg("Bet orders batch persisted to database")
	}

	// 2. Process wallet and notifications for this batch
	for i, bet := range bets {
		betOrder := betOrders[i]
		winAmount := int64(betOrder.Payout)

		// Track if we should notify the player
		shouldNotify := true

		if winAmount > 0 {
			// Call Wallet Service to add win amount
			_, err := uc.walletSvc.AddBalance(ctx, bet.UserID, winAmount, "win:"+roundID)
			if err != nil {
				logger.Error(ctx).
					Err(err).
					Int64("user_id", bet.UserID).
					Int64("win_amount", winAmount).
					Str("bet_id", bet.BetID).
					Msg("Failed to deposit winnings - player will NOT be notified")
				// Don't notify player if wallet operation failed
				shouldNotify = false
				// TODO: Retry mechanism or compensation queue
			} else {
				logger.Debug(ctx).
					Int64("user_id", bet.UserID).
					Int64("win_amount", winAmount).
					Msg("Winnings deposited successfully")
			}
		}

		// Only notify player if wallet operation succeeded (or if they lost)
		if shouldNotify && uc.gatewayBroadcaster != nil {
			uc.gatewayBroadcaster.SendToUser(ctx, bet.UserID, "color_game", &pbColorGame.ColorGameSettlementBRC{
				RoundId:      roundID,
				WinningColor: winningColor,
				BetId:        bet.BetID,
				BetColor:     bet.Color,
				BetAmount:    bet.Amount,
				WinAmount:    winAmount,
				IsWinner:     winAmount > 0,
			})
		}
	}

	return nil
}

func isValidColor(color domain.Color) bool {
	return color == pbColorGame.ColorGameReward_REWARD_RED ||
		color == pbColorGame.ColorGameReward_REWARD_GREEN ||
		color == pbColorGame.ColorGameReward_REWARD_BLUE ||
		color == pbColorGame.ColorGameReward_REWARD_YELLOW
}

func calculateWin(bet *domain.Bet, winningColor domain.Color) int64 {
	if bet.Color == winningColor {
		// Simple 2x multiplier for now
		return bet.Amount * 2
	}
	return 0
}
