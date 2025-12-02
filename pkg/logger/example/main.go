package main

import (
	"context"
	"time"

	"github.com/frankieli/game_product/pkg/logger"
)

// 示例：在业务代码中使用日志系统

func main() {
	// 1. 初始化日志系统
	logger.Init(logger.Config{
		Level:  "debug",
		Format: "console", // 开发环境用 console，生产用 json
	})

	// 2. 创建带 Request ID 的 context
	requestID := logger.GenerateRequestID()
	ctx := logger.WithRequestID(context.Background(), requestID)

	// 3. 使用 logger
	simulateUserRequest(ctx)
}

func simulateUserRequest(ctx context.Context) {
	// 开始处理请求
	logger.Info(ctx).Msg("收到用户请求")

	// 模拟查询数据库
	user := queryUser(ctx, 12345)
	if user == nil {
		logger.Warn(ctx).
			Int64("user_id", 12345).
			Msg("用户不存在")
		return
	}

	// 模拟处理业务逻辑
	err := processBet(ctx, user)
	if err != nil {
		logger.Error(ctx).
			Err(err).
			Int64("user_id", user.ID).
			Msg("下注失败")
		return
	}

	logger.Info(ctx).
		Int64("user_id", user.ID).
		Msg("请求处理成功")
}

type User struct {
	ID     int64
	Name   string
	Wallet int64
}

func queryUser(ctx context.Context, userID int64) *User {
	start := time.Now()

	// 模拟数据库查询
	time.Sleep(50 * time.Millisecond)

	logger.Debug(ctx).
		Int64("user_id", userID).
		Dur("duration", time.Since(start)).
		Msg("查询用户完成")

	return &User{
		ID:     userID,
		Name:   "测试用户",
		Wallet: 10000,
	}
}

func processBet(ctx context.Context, user *User) error {
	// 添加额外字段到 context
	ctx = logger.WithFields(ctx, map[string]interface{}{
		"user_id":   user.ID,
		"user_name": user.Name,
	})

	logger.Info(ctx).
		Int64("amount", 100).
		Msg("开始处理下注")

	// 模拟业务逻辑
	time.Sleep(100 * time.Millisecond)

	// 检查余额
	if user.Wallet < 100 {
		logger.Warn(ctx).
			Int64("wallet", user.Wallet).
			Int64("required", 100).
			Msg("余额不足")
		return nil
	}

	// 扣除余额
	user.Wallet -= 100

	logger.Info(ctx).
		Int64("new_wallet", user.Wallet).
		Msg("下注成功")

	return nil
}
