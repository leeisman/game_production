package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/frankieli/game_product/internal/config"
	colorgameGSGrpc "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/grpc"
	colorgameGSDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	colorgameGSRepo "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/db"
	colorgameGSMemory "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	colorgameGSUseCase "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	walletModule "github.com/frankieli/game_product/internal/modules/wallet"
	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/grpc_client/base"
	"github.com/frankieli/game_product/pkg/grpc_client/color_game"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/frankieli/game_product/pkg/netutil"
	pb "github.com/frankieli/game_product/shared/proto/colorgame"
)

func main() {
	background := flag.Bool("d", false, "Run in background mode (disable console logging)")
	logger.InitWithFile("logs/color_game/gs_service.log", "info", "json", !*background)
	defer logger.Flush()

	logger.InfoGlobal().Msg("🎮 Starting Color Game GS (Game Service)...")

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

	// Auto Migrate
	db.AutoMigrate(&colorgameGSDomain.BetOrder{})

	// 3. Initialize Nacos for Service Discovery
	nacosClient, err := discovery.NewNacosClient(cfg.Nacos.Host, cfg.Nacos.Port, cfg.Nacos.NamespaceID)
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to create Nacos client")
	}

	// 4. Initialize Base gRPC Client
	baseClient := base.NewBaseClient(nacosClient)

	// 5. Initialize Color Game Client (for GMS calls)
	cgClient, err := color_game.NewClient(baseClient)
	if err != nil {
		logger.WarnGlobal().Err(err).Msg("Failed to create ColorGame client")
	}

	// 6. Initialize Repositories
	// Use memory repository for bets (in-memory cache)
	betRepo := colorgameGSMemory.NewBetRepository()

	// Use DB repository for bet orders (persistent)
	betOrderRepo := colorgameGSRepo.NewBetOrderRepository(db)

	// 7. Initialize UseCase
	// Parameters: betRepo, betOrderRepo, gmsService, walletSvc, gatewayBroadcaster

	// Temporary: Use Mock Wallet Service as requested
	mockWalletSvc := walletModule.NewMockService()
	logger.WarnGlobal().Msg("⚠️ Using Mock Wallet Service (Not Production Ready)")

	gsUC := colorgameGSUseCase.NewGSUseCase(
		betRepo,
		betOrderRepo,
		cgClient,      // GMSService
		mockWalletSvc, // WalletService (Mock)
		baseClient,    // GatewayService
	)
	logger.InfoGlobal().Msg("✅ GS UseCase initialized")

	// 8. Start gRPC Server (Random Port)
	lis, actualPort, err := netutil.ListenWithFallback("0")
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to listen on port")
	}

	logger.InfoGlobal().Int("port", actualPort).Msg("🚀 GS gRPC Service listening")

	grpcServer := grpc.NewServer()
	gsGrpcHandler := colorgameGSGrpc.NewHandler(gsUC)
	pb.RegisterColorGameGSServiceServer(grpcServer, gsGrpcHandler)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.FatalGlobal().Err(err).Msg("Failed to serve gRPC")
		}
	}()

	// 9. Register to Nacos with retry
	ip := netutil.GetOutboundIP()
	serviceName := "gs-service"

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

	// 10. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.InfoGlobal().Msg("🛑 Shutting down GS...")

	// 1. Deregister first to stop new traffic (Client Side LB might still have cache)
	nacosClient.DeregisterService(serviceName, ip, uint64(actualPort))
	logger.InfoGlobal().Msg("✅ Deregistered from Nacos")

	// 2. Stop gRPC Server (Wait for ongoing requests including Session/SettleRound)
	logger.InfoGlobal().Msg("⏳ Stopping gRPC Server (Waiting for active RPCs)...")
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		logger.InfoGlobal().Msg("✅ gRPC Server stopped gracefully")
	case <-time.After(30 * time.Second): // Allow 30s for long-running batch settlements
		logger.WarnGlobal().Msg("⚠️ gRPC Server stop timed out (30s), forcing Stop")
		grpcServer.Stop()
	}

	logger.InfoGlobal().Msg("👋 GS shutdown complete")
}
