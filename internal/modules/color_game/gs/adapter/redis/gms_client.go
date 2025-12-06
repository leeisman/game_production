package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	pbCommon "github.com/frankieli/game_product/shared/proto/common"
)

const (
	StateKey = "color_game:state"
)

// GMSClient implements service.GMSService using Redis
type GMSClient struct {
	client *redis.Client
}

func NewGMSClient(client *redis.Client) *GMSClient {
	return &GMSClient{
		client: client,
	}
}

// RecordBet records a bet (No-op in Redis implementation, or publish metric)
// Validation should be done by checking state before calling this.
func (c *GMSClient) RecordBet(ctx context.Context, req *pb.ColorGameRecordBetReq) (*pb.ColorGameRecordBetRsp, error) {
	// In a fully decoupled architecture, GS manages bets. GMS just drives state.
	// We could publish an event if GMS needs to know, but for now we assume GS handles it.
	return &pb.ColorGameRecordBetRsp{
		ErrorCode: pbCommon.ErrorCode_SUCCESS,
	}, nil
}

// GetCurrentRound gets the current round from Redis
func (c *GMSClient) GetCurrentRound(ctx context.Context, req *pb.ColorGameGetCurrentRoundReq) (*pb.ColorGameGetCurrentRoundRsp, error) {
	val, err := c.client.Get(ctx, StateKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("game state not found")
		}
		return nil, err
	}

	var stateData struct {
		RoundID             string      `json:"round_id"`
		State               string      `json:"state"`
		Timestamp           int64       `json:"timestamp"`
		LeftTime            int64       `json:"left_time"`
		BettingEndTimestamp int64       `json:"betting_end_timestamp"`
		Data                interface{} `json:"data"`
	}

	if err := json.Unmarshal(val, &stateData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %v", err)
	}

	// Calculate remaining time based on BettingEndTimestamp if available
	// Otherwise fallback to LeftTime (which is static from event time)
	// But actually, LeftTime in event is duration.
	// If we have BettingEndTimestamp, we can calculate dynamic LeftTime.

	var leftTime int64
	if stateData.BettingEndTimestamp > 0 {
		leftTime = stateData.BettingEndTimestamp - time.Now().Unix()
		if leftTime < 0 {
			leftTime = 0
		}
	} else {
		// Fallback logic if needed, or just use what was stored (which is likely stale duration)
		// But since we updated Broadcaster to store BettingEndTimestamp, we should be good.
		leftTime = stateData.LeftTime
	}

	return &pb.ColorGameGetCurrentRoundRsp{
		ErrorCode:           pbCommon.ErrorCode_SUCCESS,
		RoundId:             stateData.RoundID,
		State:               pb.ColorGameState(pb.ColorGameState_value[stateData.State]),
		BettingEndTimestamp: stateData.BettingEndTimestamp,
		LeftTime:            leftTime,
		PlayerBets:          []*pb.ColorGamePlayerBet{}, // Not stored in GMS state
	}, nil
}
