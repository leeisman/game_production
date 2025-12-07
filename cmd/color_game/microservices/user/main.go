package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/frankieli/game_product/internal/config"
	userGrpc "github.com/frankieli/game_product/internal/modules/user/adapter/grpc"
	userHttp "github.com/frankieli/game_product/internal/modules/user/adapter/http"
	userRepo "github.com/frankieli/game_product/internal/modules/user/repository"
	"github.com/frankieli/game_product/internal/modules/user/usecase"
	"github.com/frankieli/game_product/pkg/admin"
	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/frankieli/game_product/pkg/netutil"
	pbAdmin "github.com/frankieli/game_product/shared/proto/admin"
	pb "github.com/frankieli/game_product/shared/proto/user"
)

func main() {
	background := flag.Bool("d", false, "Run in background mode (disable console logging)")
	logger.InitWithFile("logs/color_game/user_service.log", "info", "json", !*background)
	defer logger.Flush()

	logger.InfoGlobal().Msg("👤 Starting User Service...")

	// 1. Load Config
	cfg := config.LoadUserConfig()

	// 2. Initialize Database
	dbConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Name)
	db, err := gorm.Open(postgres.Open(dbConnStr), &gorm.Config{})
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to get database instance")
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to ping database")
	}
	logger.InfoGlobal().Msg("✅ Database connected")

	// 3. Initialize Components
	userRepository := userRepo.NewUserRepository(db)
	sessionRepo := userRepo.NewSessionRepository(db)
	userUC := usecase.NewUserUseCase(userRepository, sessionRepo, cfg.JWT.Secret, cfg.JWT.Duration)

	// 4. Start gRPC Server (Use netutil to handle port binding)
	lis, actualPort, err := netutil.ListenWithFallback("0")
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to listen on port")
	}

	logger.InfoGlobal().Int("port", actualPort).Msg("🚀 User gRPC Service listening")

	grpcServer := grpc.NewServer()
	userGrpcHandler := userGrpc.NewHandler(userUC)
	pb.RegisterUserServiceServer(grpcServer, userGrpcHandler)
	pbAdmin.RegisterAdminServiceServer(grpcServer, admin.NewServer())

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.FatalGlobal().Err(err).Msg("Failed to serve gRPC")
		}
	}()

	// 5. Register to Nacos (with Retry)
	ip := netutil.GetOutboundIP()
	nacosClient, err := discovery.NewNacosClient(cfg.Nacos.Host, cfg.Nacos.Port, cfg.Nacos.NamespaceID)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to create Nacos client, skipping registration")
	} else {
		// Retry registration
		go func() {
			maxRetries := 10
			for i := 0; i < maxRetries; i++ {
				// Register with ACTUAL port
				err = nacosClient.RegisterService(cfg.Server.Name, ip, uint64(actualPort), nil)
				if err == nil {
					logger.InfoGlobal().Str("service", cfg.Server.Name).Str("ip", ip).Int("port", actualPort).Msg("✅ Registered to Nacos")
					return
				}
				logger.WarnGlobal().Err(err).Msgf("Failed to register service (attempt %d/%d), retrying...", i+1, maxRetries)
				time.Sleep(2 * time.Second)
			}
			logger.ErrorGlobal().Msg("❌ Failed to register service after retries")
		}()
	}

	// 6. Start HTTP Server
	userHttpHandler := userHttp.NewHandler(userUC)
	httpPort := cfg.Server.HTTPPort
	httpServer := userHttp.NewServer(userHttpHandler, httpPort)

	go func() {
		logger.InfoGlobal().Str("port", httpPort).Msg("🚀 User HTTP Service running")
		if err := httpServer.Run(); err != nil {
			logger.FatalGlobal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.InfoGlobal().Msg("🛑 Shutting down server...")

	nacosClient.DeregisterService(cfg.Server.Name, ip, uint64(actualPort))
	logger.InfoGlobal().Msg("✅ Deregistered from Nacos")

	logger.InfoGlobal().Msg("👋 User shutdown complete")
}
