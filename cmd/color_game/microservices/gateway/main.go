package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	"github.com/frankieli/game_product/internal/config"
	colorgameGSLocal "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/local"
	gsRedis "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/redis"
	colorgameGSDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	colorgameGSMemory "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	colorgameGSUseCase "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	gatewayHttp "github.com/frankieli/game_product/internal/modules/gateway/adapter/http"
	gatewayAdapter "github.com/frankieli/game_product/internal/modules/gateway/adapter/local"
	gatewayUseCase "github.com/frankieli/game_product/internal/modules/gateway/usecase"
	"github.com/frankieli/game_product/internal/modules/gateway/ws"
	walletModule "github.com/frankieli/game_product/internal/modules/wallet"
	grpcClient "github.com/frankieli/game_product/pkg/grpc_client"

	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/logger"

	pb "github.com/frankieli/game_product/shared/proto/colorgame"
)

func main() {
	logger.InfoGlobal().Msg("🌐 Starting Color Game Gateway (Microservices Mode)...")

	// 1. Load Config
	cfg := config.LoadGatewayConfig()

	// 2. Initialize Redis (for Broadcast)
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// 3. Initialize Service Discovery (Nacos)
	registry, err := discovery.NewNacosClient(cfg.Nacos.Host, cfg.Nacos.Port, cfg.Nacos.NamespaceID)
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to create Nacos client")
	}
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to Redis")
	}
	logger.InfoGlobal().Msg("✅ Redis connected")

	// 4. Initialize Unified gRPC Client (acting as User Service Client)
	userClient := grpcClient.NewClient(registry)
	logger.InfoGlobal().Msg("✅ Unified gRPC Client initialized")

	// 4. Initialize GMS Client (Redis)
	// No gRPC connection needed for GMS anymore
	gmsClientAdapter := gsRedis.NewGMSClient(rdb)
	logger.InfoGlobal().Msg("✅ GMS Client (Redis) initialized")

	// 5. Initialize WebSocket Manager
	wsManager := ws.NewManager()
	go wsManager.Run()

	// 6. Initialize Gateway UseCase
	walletSvc := walletModule.NewMockService()

	var betRepo colorgameGSDomain.BetRepository
	// Note: Gateway config doesn't have RepoType yet, assuming memory for now or adding it.
	betRepo = colorgameGSMemory.NewBetRepository()

	// Gateway listening to GMS events (and forwarding to WS)
	broadcaster := gatewayAdapter.NewHandler(wsManager)

	// Placeholder for betOrderRepo and gmsService, as they are not defined in the original code
	// and their types are not provided in the instruction.
	// Assuming they would be defined elsewhere in a complete refactor.
	var betOrderRepo colorgameGSDomain.BetOrderRepository // Assuming a type for betOrderRepo
	gmsService := gmsClientAdapter                        // Assuming gmsService is the existing gmsClientAdapter
	gsUseCase := colorgameGSUseCase.NewGSUseCase(betRepo, betOrderRepo, gmsService, walletSvc, broadcaster)
	gsHandler := colorgameGSLocal.NewHandler(gsUseCase)

	// 7. Initialize Gateway UseCase
	gatewayUC := gatewayUseCase.NewGatewayUseCase(gsHandler)
	logger.InfoGlobal().Msg("✅ Gateway module initialized")

	// 8. Subscribe to GMS events (Redis Pub/Sub)
	go subscribeToGMSEvents(rdb, broadcaster, gsHandler)

	logger.InfoGlobal().Msg("✅ Color Game GS initialized")

	// 9. Initialize HTTP Handler
	httpHandler := gatewayHttp.NewHandler(gatewayUC, wsManager, userClient)

	// 10. Setup HTTP Server
	r := gin.Default()
	r.Use(logger.GinMiddleware())

	r.GET("/ws", func(c *gin.Context) {
		httpHandler.HandleWebSocket(c.Writer, c.Request)
	})

	// 11. Start Server
	port := cfg.Server.Port
	logger.InfoGlobal().
		Str("port", port).
		Str("ws_url", fmt.Sprintf("ws://localhost:%s/ws?token=YOUR_TOKEN", port)).
		Msg("🚀 Gateway running")

	if err := r.Run(":" + port); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to start server")
	}
}

// subscribeToGMSEvents subscribes to GMS event stream (Redis) and broadcasts to WebSocket clients AND GS
func subscribeToGMSEvents(rdb *redis.Client, broadcaster *gatewayAdapter.Handler, gsHandler *colorgameGSLocal.Handler) {
	logger.InfoGlobal().Msg("📡 Subscribing to GMS events (Redis)...")

	pubsub := rdb.Subscribe(context.Background(), "color_game:events")
	defer pubsub.Close()

	// Wait for subscription to be created
	_, err := pubsub.Receive(context.Background())
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to subscribe to Redis channel")
	}

	ch := pubsub.Channel()
	for msg := range ch {
		// Try ColorGameRoundStateBRC
		var brc pb.ColorGameRoundStateBRC
		if err := proto.Unmarshal([]byte(msg.Payload), &brc); err == nil {
			broadcaster.Broadcast("color_game", &brc)
			logger.InfoGlobal().Str("state", brc.State.String()).Msg("Broadcasting GMS event (BRC)")
			continue
		}

		// Try ColorGameRoundResultReq
		var req pb.ColorGameRoundResultReq
		if err := proto.Unmarshal([]byte(msg.Payload), &req); err == nil {
			// To GS (for Settlement)
			ctx := context.Background()
			_, _ = gsHandler.RoundResult(ctx, &req)
			logger.InfoGlobal().Str("round_id", req.RoundId).Msg("Forwarding RoundResult to GS")
			continue
		}

		logger.ErrorGlobal().Msg("Failed to unmarshal event from Redis (unknown type)")
	}
}
