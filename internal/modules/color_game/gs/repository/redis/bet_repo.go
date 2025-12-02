package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	"github.com/redis/go-redis/v9"
)

const ShardCount = 16

// BetRepository implements domain.BetRepository using Redis
type BetRepository struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewBetRepository creates a new Redis bet repository
func NewBetRepository(rdb *redis.Client) *BetRepository {
	return &BetRepository{
		rdb: rdb,
		ttl: 24 * time.Hour, // Keep bets for 24 hours
	}
}

// SaveBet saves a bet
func (r *BetRepository) SaveBet(ctx context.Context, bet *domain.Bet) error {
	data, err := json.Marshal(bet)
	if err != nil {
		return err
	}

	pipe := r.rdb.Pipeline()

	// 1. Save bet data to Hash
	dataKey := fmt.Sprintf("bet_data:%s", bet.RoundID)
	pipe.HSet(ctx, dataKey, bet.BetID, data)
	pipe.Expire(ctx, dataKey, r.ttl)

	// 2. Add to settlement queue (List of BetIDs)
	shardID := bet.UserID % ShardCount
	queueKey := fmt.Sprintf("settlement_queue:%s:%d", bet.RoundID, shardID)
	pipe.RPush(ctx, queueKey, bet.BetID)
	pipe.Expire(ctx, queueKey, r.ttl)

	// 3. Update user index (Hash: color -> bet_id)
	indexKey := fmt.Sprintf("user_index:%s:%d", bet.RoundID, bet.UserID)
	pipe.HSet(ctx, indexKey, string(bet.Color), bet.BetID)
	pipe.Expire(ctx, indexKey, r.ttl)

	_, err = pipe.Exec(ctx)
	return err
}

// GetBets retrieves all bets for a round
func (r *BetRepository) GetBets(ctx context.Context, roundID string) ([]*domain.Bet, error) {
	dataKey := fmt.Sprintf("bet_data:%s", roundID)
	dataMap, err := r.rdb.HGetAll(ctx, dataKey).Result()
	if err != nil {
		return nil, err
	}

	allBets := make([]*domain.Bet, 0, len(dataMap))
	for _, data := range dataMap {
		var bet domain.Bet
		if err := json.Unmarshal([]byte(data), &bet); err != nil {
			continue
		}
		allBets = append(allBets, &bet)
	}
	return allBets, nil
}

// GetUserBets retrieves all bets for a user in a round
func (r *BetRepository) GetUserBets(ctx context.Context, roundID string, userID int64) ([]*domain.Bet, error) {
	// 1. Get bet IDs from user index
	indexKey := fmt.Sprintf("user_index:%s:%d", roundID, userID)
	betIDs, err := r.rdb.HVals(ctx, indexKey).Result()
	if err != nil {
		return nil, err
	}

	if len(betIDs) == 0 {
		return []*domain.Bet{}, nil
	}

	// 2. Get bet data
	dataKey := fmt.Sprintf("bet_data:%s", roundID)
	// HMGet allows getting multiple fields
	dataList, err := r.rdb.HMGet(ctx, dataKey, betIDs...).Result()
	if err != nil {
		return nil, err
	}

	bets := make([]*domain.Bet, 0, len(dataList))
	for _, data := range dataList {
		if data == nil {
			continue
		}
		var bet domain.Bet
		// HMGet returns []interface{}, need to assert to string
		if strData, ok := data.(string); ok {
			if err := json.Unmarshal([]byte(strData), &bet); err != nil {
				continue
			}
			bets = append(bets, &bet)
		}
	}
	return bets, nil
}

// GetUserBet retrieves a specific bet for a user
func (r *BetRepository) GetUserBet(ctx context.Context, roundID string, userID int64, color domain.Color) (*domain.Bet, error) {
	// 1. Get bet ID from user index
	indexKey := fmt.Sprintf("user_index:%s:%d", roundID, userID)
	betID, err := r.rdb.HGet(ctx, indexKey, string(color)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Not found
		}
		return nil, err
	}

	// 2. Get bet data
	dataKey := fmt.Sprintf("bet_data:%s", roundID)
	data, err := r.rdb.HGet(ctx, dataKey, betID).Result()
	if err != nil {
		return nil, err
	}

	var bet domain.Bet
	if err := json.Unmarshal([]byte(data), &bet); err != nil {
		return nil, err
	}
	return &bet, nil
}

// UpdateBetAmount updates the amount of an existing bet
func (r *BetRepository) UpdateBetAmount(ctx context.Context, bet *domain.Bet, additionalAmount int64) error {
	// Update amount in memory object
	bet.Amount += additionalAmount

	// Serialize updated bet
	data, err := json.Marshal(bet)
	if err != nil {
		return err
	}

	// Update in Redis Hash
	dataKey := fmt.Sprintf("bet_data:%s", bet.RoundID)
	return r.rdb.HSet(ctx, dataKey, bet.BetID, data).Err()
}

// ClearBets clears all bets for a round
func (r *BetRepository) ClearBets(ctx context.Context, roundID string) error {
	pipe := r.rdb.Pipeline()

	// Delete bet data
	pipe.Del(ctx, fmt.Sprintf("bet_data:%s", roundID))

	// Delete queues
	for i := 0; i < ShardCount; i++ {
		pipe.Del(ctx, fmt.Sprintf("settlement_queue:%s:%d", roundID, i))
	}

	// Note: user_index keys expire automatically, hard to delete all without scan
	// We rely on TTL for user_index cleanup

	_, err := pipe.Exec(ctx)
	return err
}

// GetBetsForSettlement retrieves bets for settlement
func (r *BetRepository) GetBetsForSettlement(ctx context.Context, roundID string) ([]*domain.Bet, error) {
	startShard := rand.Intn(ShardCount)

	for i := 0; i < ShardCount; i++ {
		shardID := (startShard + i) % ShardCount
		queueKey := fmt.Sprintf("settlement_queue:%s:%d", roundID, shardID)

		// Pop bet IDs
		betIDs, err := r.rdb.LPopCount(ctx, queueKey, 100).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, err
		}

		if len(betIDs) > 0 {
			// Get bet data
			dataKey := fmt.Sprintf("bet_data:%s", roundID)
			dataList, err := r.rdb.HMGet(ctx, dataKey, betIDs...).Result()
			if err != nil {
				return nil, err
			}

			bets := make([]*domain.Bet, 0, len(dataList))
			for _, data := range dataList {
				if data == nil {
					continue
				}
				var bet domain.Bet
				if strData, ok := data.(string); ok {
					if err := json.Unmarshal([]byte(strData), &bet); err != nil {
						continue
					}
					bets = append(bets, &bet)
				}
			}
			return bets, nil
		}
	}

	return make([]*domain.Bet, 0), nil
}
