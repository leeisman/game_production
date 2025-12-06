package base

import (
	"context"
	"fmt"
	"time"

	"github.com/frankieli/game_product/pkg/service"
	pbWallet "github.com/frankieli/game_product/shared/proto/wallet"
)

// Ensure BaseClient implements WalletService
var _ service.WalletService = (*BaseClient)(nil)

func (c *BaseClient) getWalletClient() (pbWallet.WalletServiceClient, error) {
	conn, err := c.GetLBConn("wallet-service") // Load Balanced
	if err != nil {
		return nil, err
	}
	return pbWallet.NewWalletServiceClient(conn), nil
}

func (c *BaseClient) GetBalance(ctx context.Context, userID int64) (int64, error) {
	ctx = c.withRequestID(ctx)
	client, err := c.getWalletClient()
	if err != nil {
		return 0, err
	}

	rsp, err := client.GetBalance(ctx, &pbWallet.GetBalanceReq{PlayerId: userID})
	if err != nil {
		return 0, fmt.Errorf("rpc GetBalance failed: %w", err)
	}
	if !rsp.Success {
		return 0, fmt.Errorf("wallet error: %s", rsp.Message)
	}
	return rsp.Balance, nil
}

func (c *BaseClient) DeductBalance(ctx context.Context, userID int64, amount int64, reason string) (int64, error) {
	ctx = c.withRequestID(ctx)
	client, err := c.getWalletClient()
	if err != nil {
		return 0, err
	}

	// Using Withdraw as generic deduction
	rsp, err := client.Withdraw(ctx, &pbWallet.WithdrawReq{
		PlayerId: userID,
		Amount:   amount,
		TxId:     fmt.Sprintf("deduct-%d-%d", userID, time.Now().UnixNano()), // TODO: Better ID gen
		Metadata: fmt.Sprintf(`{"reason": "%s"}`, reason),
	})
	if err != nil {
		return 0, fmt.Errorf("rpc Withdraw failed: %w", err)
	}
	if !rsp.Success {
		return 0, fmt.Errorf("wallet error: %s", rsp.Message)
	}
	return rsp.NewBalance, nil
}

func (c *BaseClient) AddBalance(ctx context.Context, userID int64, amount int64, reason string) (int64, error) {
	ctx = c.withRequestID(ctx)
	client, err := c.getWalletClient()
	if err != nil {
		return 0, err
	}

	rsp, err := client.Deposit(ctx, &pbWallet.DepositReq{
		PlayerId: userID,
		Amount:   amount,
		TxId:     fmt.Sprintf("add-%d-%d", userID, time.Now().UnixNano()),
		Metadata: fmt.Sprintf(`{"reason": "%s"}`, reason),
	})
	if err != nil {
		return 0, fmt.Errorf("rpc Deposit failed: %w", err)
	}
	if !rsp.Success {
		return 0, fmt.Errorf("wallet error: %s", rsp.Message)
	}
	return rsp.NewBalance, nil
}

func (c *BaseClient) PlaceBet(ctx context.Context, userID int64, amount int64, roundID string) (int64, error) {
	ctx = c.withRequestID(ctx)
	client, err := c.getWalletClient()
	if err != nil {
		return 0, err
	}

	rsp, err := client.PlaceBet(ctx, &pbWallet.PlaceBetReq{
		PlayerId:    userID,
		Amount:      amount,
		GameRoundId: roundID,
		TxId:        fmt.Sprintf("bet-%d-%s", userID, roundID), // Idempotency key from round?
	})
	if err != nil {
		return 0, fmt.Errorf("rpc PlaceBet failed: %w", err)
	}
	if !rsp.Success {
		return 0, fmt.Errorf("wallet error: %s", rsp.Message)
	}
	return rsp.NewBalance, nil
}
