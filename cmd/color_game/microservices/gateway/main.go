package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/frankieli/game_product/internal/config"
	gsGrpc "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/grpc"
	colorgameGSLocal "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/local"
	colorgameGSDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	colorgameGSMemory "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	colorgameGSUseCase "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	gatewayHttp "github.com/frankieli/game_product/internal/modules/gateway/adapter/http"
	gatewayAdapter "github.com/frankieli/game_product/internal/modules/gateway/adapter/local"
	gatewayUseCase "github.com/frankieli/game_product/internal/modules/gateway/usecase"
	"github.com/frankieli/game_product/internal/modules/gateway/ws"
	userGrpc "github.com/frankieli/game_product/internal/modules/user/adapter/grpc"
	walletModule "github.com/frankieli/game_product/internal/modules/wallet"

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
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to Redis")
	}
	logger.InfoGlobal().Msg("✅ Redis connected")

	// Initialize Nacos Client for Discovery
	nacosClient, err := discovery.NewNacosClient(cfg.Nacos.Host, cfg.Nacos.Port, cfg.Nacos.NamespaceID)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to create Nacos client, using default addresses")
	}

	// 3. Connect to User Service via gRPC
	userAddr := "localhost:50051" // Default fallback
	if nacosClient != nil {
		addr, err := nacosClient.GetService("auth-service")
		if err == nil {
			userAddr = addr
			logger.InfoGlobal().Str("address", userAddr).Msg("Discovered User Service via Nacos")
		} else {
			logger.ErrorGlobal().Err(err).Msg("Failed to discover User Service")
		}
	}

	userConn, err := grpc.Dial(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to User Service")
	}
	defer userConn.Close()
	logger.InfoGlobal().Str("address", userAddr).Msg("Connected to User Service")

	userClient := userGrpc.NewUserClient(userConn)

	// 4. Connect to GMS via gRPC
	gmsAddr := "localhost:50052" // Default fallback
	if nacosClient != nil {
		addr, err := nacosClient.GetService("game-service")
		if err == nil {
			gmsAddr = addr
			logger.InfoGlobal().Str("address", gmsAddr).Msg("Discovered GMS via Nacos")
		} else {
			logger.ErrorGlobal().Err(err).Msg("Failed to discover GMS")
		}
	}

	gmsConn, err := grpc.Dial(gmsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to GMS")
	}
	defer gmsConn.Close()
	logger.InfoGlobal().Str("address", gmsAddr).Msg("Connected to GMS")
	// Raw client for event subscription
	rawGMSClient := pb.NewGameMachineServiceClient(gmsConn)
	logger.InfoGlobal().Msg("✅ Connected to GMS")

	// 5. Initialize WebSocket Manager
	// 5. Initialize WebSocket Manager
	wsManager := ws.NewManager()
	go wsManager.Run()

	// 6. Initialize Gateway UseCase
	// For now, passing nil gameService initially, then updating it
	walletSvc := walletModule.NewMockService()

	var betRepo colorgameGSDomain.BetRepository
	// Note: Gateway config doesn't have RepoType yet, assuming memory for now or adding it.
	betRepo = colorgameGSMemory.NewBetRepository()

	// Use the proper gRPC adapter for GS -> GMS communication
	gmsClientAdapter := gsGrpc.NewGMSClient(gmsConn)

	// Gateway listening to GMS events (and forwarding to WS)
	broadcaster := gatewayAdapter.NewBroadcaster(wsManager)

	playerUC := colorgameGSUseCase.NewPlayerUseCase(betRepo, nil, gmsClientAdapter, walletSvc, broadcaster)
	gsHandler := colorgameGSLocal.NewHandler(playerUC)

	// 7. Initialize Gateway UseCase
	gatewayUC := gatewayUseCase.NewGatewayUseCase(gsHandler)
	logger.InfoGlobal().Msg("✅ Gateway module initialized")

	// 8. Subscribe to GMS events (Moved here to have gsBroadcaster ready)
	gsBroadcaster := colorgameGSLocal.NewGSBroadcaster(playerUC)
	go subscribeToGMSEvents(rawGMSClient, broadcaster, gsBroadcaster)

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

// subscribeToGMSEvents subscribes to GMS event stream and broadcasts to WebSocket clients AND GS
func subscribeToGMSEvents(client pb.GameMachineServiceClient, broadcaster *gatewayAdapter.Broadcaster, gsBroadcaster *colorgameGSLocal.GSBroadcaster) {
	logger.InfoGlobal().Msg("📡 Subscribing to GMS events...")

	for {
		stream, err := client.SubscribeEvents(context.Background(), &pb.SubscribeEventsReq{})
		if err != nil {
			logger.ErrorGlobal().Err(err).Msg("Failed to subscribe to GMS events")
			logger.InfoGlobal().Msg("Retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}

		logger.InfoGlobal().Msg("✅ Subscribed to GMS events")

		for {
			event, err := stream.Recv()
			if err != nil {
				logger.ErrorGlobal().Err(err).Msg("Stream error")
				logger.InfoGlobal().Msg("Reconnecting...")
				break
			}

			// Broadcast event to all WebSocket clients
			// Note: Gateway expects []byte (JSON) for WebSocket
			// But GSBroadcaster expects proto.Message

			// 1. To WebSocket Clients
			broadcaster.Broadcast(event)

			// 2. To GS (for Settlement)
			gsBroadcaster.Broadcast(event)

			logger.InfoGlobal().Str("type", event.Type.String()).Msg("Broadcasting GMS event")
		}
	}
}
