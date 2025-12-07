package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/frankieli/game_product/internal/config"
	colorgameGMSGrpc "github.com/frankieli/game_product/internal/modules/color_game/gms/adapter/grpc"
	colorgameGMSMachine "github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	colorgameGMSRepo "github.com/frankieli/game_product/internal/modules/color_game/gms/repository/db"
	colorgameGMSUseCase "github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/grpc_client/base"
	"github.com/frankieli/game_product/pkg/grpc_client/color_game"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger.Init(logger.Config{
		Level:  "info",
		Format: "json",
	})

	logger.InfoGlobal().Msg("🎰 Starting Color Game GMS (Game Management Service)...")

	// 1. Load Config
	cfg := config.LoadColorGameConfig()

	// 2. Initialize Database
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Database.Host, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.Port)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to database")
	}
	logger.InfoGlobal().Msg("✅ Database connected")

	// 3. Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to Redis")
	}
	logger.InfoGlobal().Msg("✅ Redis connected")

	// 4. Initialize Nacos for Service Discovery
	nacosClient, err := discovery.NewNacosClient(cfg.Nacos.Host, cfg.Nacos.Port, cfg.Nacos.NamespaceID)
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to create Nacos client")
	}

	// 5. Initialize Base gRPC Client
	baseClient := base.NewBaseClient(nacosClient)

	// 6. Initialize ColorGame Client (for calling Gateway)
	cgClient, err := color_game.NewClient(baseClient)
	if err != nil {
		logger.WarnGlobal().Err(err).Msg("Failed to create ColorGame client")
	}

	// 7. Initialize State Machine
	stateMachine := colorgameGMSMachine.NewStateMachine()
	go stateMachine.Start(context.Background())
	logger.InfoGlobal().Msg("✅ State machine started")

	// 8. Initialize Repository
	gameRoundRepo := colorgameGMSRepo.NewGameRoundRepository(db)

	// 9. Initialize UseCase with Gateway broadcaster
	// cgClient embeds BaseClient which implements GatewayService interface for broadcasting
	gmsUC := colorgameGMSUseCase.NewGMSUseCase(stateMachine, cgClient, cgClient, gameRoundRepo)
	logger.InfoGlobal().Msg("✅ GMS UseCase initialized")

	// 9. Start gRPC Server (Random Port)
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to listen on random port")
	}

	addr := lis.Addr().(*net.TCPAddr)
	actualPort := addr.Port
	logger.InfoGlobal().Int("port", actualPort).Msg("🚀 GMS gRPC Service listening (Random Port)")

	grpcServer := grpc.NewServer()
	gmsGrpcHandler := colorgameGMSGrpc.NewHandler(gmsUC)
	pb.RegisterColorGameGMSServiceServer(grpcServer, gmsGrpcHandler)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.FatalGlobal().Err(err).Msg("Failed to serve gRPC")
		}
	}()

	// 10. Register to Nacos with retry
	ip := discovery.GetOutboundIP()
	serviceName := "gms-service"

	var registered bool
	for i := 0; i < 10; i++ {
		err = nacosClient.RegisterService(serviceName, ip, uint64(actualPort), nil)
		if err == nil {
			logger.InfoGlobal().Str("service", serviceName).Str("ip", ip).Int("port", actualPort).Msg("✅ Registered to Nacos")
			registered = true
			break
		}
		logger.WarnGlobal().Err(err).Int("attempt", i+1).Msg("Failed to register, retrying...")
		time.Sleep(2 * time.Second)
	}

	if !registered {
		logger.ErrorGlobal().Msg("Failed to register to Nacos after retries")
	}

	// 11. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.InfoGlobal().Msg("🛑 Shutting down GMS (Waiting for current round to finish)...")

	// 1. Deregister from Nacos first (Stop traffic)
	nacosClient.DeregisterService(serviceName, ip, uint64(actualPort))
	logger.InfoGlobal().Msg("✅ Deregistered from Nacos")

	// 2. Stop gRPC Server (Wait for pending requests)
	logger.InfoGlobal().Msg("🛑 Stopping gRPC Server...")
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

	// 3. Stop State Machine (Wait for current round to finish)
	// Now safe to stop because no new requests are coming in
	if err := stateMachine.GracefulShutdown(30 * time.Second); err != nil {
		logger.WarnGlobal().Err(err).Msg("⚠️ State Machine shutdown timed out (forced exit)")
	} else {
		logger.InfoGlobal().Msg("✅ State Machine stopped gracefully")
	}

	logger.InfoGlobal().Msg("✅ GMS shutdown complete")
}
