package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"github.com/frankieli/game_product/internal/config"
	gatewayGrpc "github.com/frankieli/game_product/internal/modules/gateway/adapter/grpc"
	gatewayHttp "github.com/frankieli/game_product/internal/modules/gateway/adapter/http"
	gatewayUseCase "github.com/frankieli/game_product/internal/modules/gateway/usecase"
	"github.com/frankieli/game_product/internal/modules/gateway/ws"
	"github.com/frankieli/game_product/pkg/discovery"
	grpcClient "github.com/frankieli/game_product/pkg/grpc_client/base"
	colorGameClient "github.com/frankieli/game_product/pkg/grpc_client/color_game"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/frankieli/game_product/pkg/netutil"

	pbGateway "github.com/frankieli/game_product/shared/proto/gateway"
)

func main() {
	logger.Init(logger.Config{
		Level:  "info",
		Format: "json",
	})

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
	baseClient := grpcClient.NewBaseClient(registry)
	logger.InfoGlobal().Msg("✅ Unified gRPC Client initialized")

	// 4. Initialize ColorGame Client (for calling GS and GMS services)
	cgClient, err := colorGameClient.NewClient(baseClient)
	if err != nil {
		logger.WarnGlobal().Err(err).Msg("Failed to create ColorGame client")
	}
	logger.InfoGlobal().Msg("✅ ColorGame Client initialized")

	// 5. Initialize WebSocket Manager
	wsManager := ws.NewManager()
	go wsManager.Run()

	// 6. Initialize Gateway UseCase (forwards requests to GS/GMS via gRPC)
	gatewayUC := gatewayUseCase.NewGatewayUseCase(cgClient)
	logger.InfoGlobal().Msg("✅ Gateway module initialized")

	// Note: In microservices mode, Gateway is a pure proxy:
	// - Receives WebSocket connections from clients
	// - Forwards game requests to GS service via gRPC
	// - Receives broadcasts from GMS via gRPC and forwards to WebSocket clients

	// 7. Initialize HTTP Handler
	httpHandler := gatewayHttp.NewHandler(gatewayUC, wsManager, cgClient)

	// 10. Start gRPC Server (for Broadcast and SendToUser from GMS/GS)
	gatewayGrpcHandler := gatewayGrpc.NewHandler(wsManager)

	// Use random port for gRPC (handled by ListenWithFallback "0")
	grpcLis, grpcPort, err := netutil.ListenWithFallback("0")
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to listen on gRPC port")
	}

	logger.InfoGlobal().Int("grpc_port", grpcPort).Msg("🚀 Gateway gRPC Service listening")

	grpcServer := grpc.NewServer()
	pbGateway.RegisterGatewayServiceServer(grpcServer, gatewayGrpcHandler)

	go func() {
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.FatalGlobal().Err(err).Msg("Failed to serve gRPC")
		}
	}()

	// Register Gateway gRPC service to Nacos
	ip := netutil.GetOutboundIP()
	gatewayGrpcServiceName := "gateway-service"

	var grpcRegistered bool
	for i := 0; i < 10; i++ {
		err = registry.RegisterService(gatewayGrpcServiceName, ip, uint64(grpcPort), nil)
		if err == nil {
			logger.InfoGlobal().Str("service", gatewayGrpcServiceName).Str("ip", ip).Int("port", grpcPort).Msg("✅ Gateway gRPC registered to Nacos")
			grpcRegistered = true
			break
		}
		logger.WarnGlobal().Err(err).Int("attempt", i+1).Msg("Failed to register Gateway gRPC, retrying...")
		time.Sleep(2 * time.Second)
	}

	if !grpcRegistered {
		logger.ErrorGlobal().Msg("Failed to register Gateway gRPC to Nacos after retries")
	}

	// 11. Setup HTTP Server
	r := gin.Default()
	r.Use(logger.GinMiddleware())

	r.GET("/ws", func(c *gin.Context) {
		httpHandler.HandleWebSocket(c.Writer, c.Request)
	})

	// 12. Start HTTP/WebSocket Server
	port := cfg.Server.Port
	logger.InfoGlobal().
		Str("port", port).
		Str("ws_url", fmt.Sprintf("ws://localhost:%s/ws?token=YOUR_TOKEN", port)).
		Msg("🚀 Gateway HTTP/WebSocket running")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.FatalGlobal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	// 13. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.InfoGlobal().Msg("🛑 Shutting down Gateway...")

	// 1. Deregister from Nacos (Stop gRPC traffic)
	registry.DeregisterService(gatewayGrpcServiceName, ip, uint64(grpcPort))
	logger.InfoGlobal().Msg("✅ Gateway gRPC deregistered")

	// 2. Stop HTTP Server (Stop new WebSocket connections)
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.ErrorGlobal().Err(err).Msg("HTTP Server forced to shutdown")
	}
	logger.InfoGlobal().Msg("✅ HTTP Server stopped")

	// 3. Stop gRPC Server (Finish pending broadcasts)
	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
		logger.InfoGlobal().Msg("✅ gRPC Server stopped gracefully")
	case <-time.After(5 * time.Second):
		logger.WarnGlobal().Msg("⚠️ gRPC Server stop timed out, forcing Stop")
		grpcServer.Stop()
	}

	// 4. Shutdown WebSocket Manager (Close active connections)
	wsManager.Shutdown()
	logger.InfoGlobal().Msg("✅ WebSocket Manager stopped")

	logger.InfoGlobal().Msg("👋 Gateway shutdown complete")
}

// Note: subscribeToGMSEvents removed in microservices mode
// GMS broadcasts directly via gRPC to Gateway's Broadcast RPC handler
