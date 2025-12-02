package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/frankieli/game_product/internal/config"
	userGrpc "github.com/frankieli/game_product/internal/modules/user/adapter/grpc"
	userHttp "github.com/frankieli/game_product/internal/modules/user/adapter/http"
	userRepo "github.com/frankieli/game_product/internal/modules/user/repository"
	"github.com/frankieli/game_product/internal/modules/user/usecase"
	"github.com/frankieli/game_product/pkg/discovery"
	"github.com/frankieli/game_product/pkg/logger"
	pb "github.com/frankieli/game_product/shared/proto/user"
)

func main() {
	logger.Init(logger.Config{
		Level:  "debug",
		Format: "console",
	})

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

	// 4. Start gRPC Server
	grpcPort := cfg.Server.Port
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.FatalGlobal().Str("port", grpcPort).Err(err).Msg("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	userGrpcHandler := userGrpc.NewHandler(userUC)
	pb.RegisterUserServiceServer(grpcServer, userGrpcHandler)

	go func() {
		logger.InfoGlobal().Str("port", grpcPort).Msg("🚀 User gRPC Service running")
		if err := grpcServer.Serve(lis); err != nil {
			logger.FatalGlobal().Err(err).Msg("Failed to serve gRPC")
		}
	}()

	// 5. Register to Nacos
	ip := discovery.GetOutboundIP()
	portInt, _ := strconv.Atoi(cfg.Server.Port)
	nacosClient, err := discovery.NewNacosClient(cfg.Nacos.Host, cfg.Nacos.Port, cfg.Nacos.NamespaceID)
	if err != nil {
		logger.ErrorGlobal().Err(err).Msg("Failed to create Nacos client, skipping registration")
	} else {
		err = nacosClient.RegisterService(cfg.Server.Name, ip, uint64(portInt), nil)
		if err != nil {
			logger.ErrorGlobal().Err(err).Msg("Failed to register service")
		} else {
			logger.InfoGlobal().Str("service", cfg.Server.Name).Str("ip", ip).Msg("✅ Registered to Nacos")
		}
	}

	// 6. Start HTTP Server with Graceful Shutdown
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

	// Deregister from Nacos
	if nacosClient != nil {
		nacosClient.DeregisterService(cfg.Server.Name, ip, uint64(portInt))
		logger.InfoGlobal().Msg("✅ Deregistered from Nacos")
	}
}
