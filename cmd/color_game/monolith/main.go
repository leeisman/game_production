package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/frankieli/game_product/internal/config"
	colorgameGMSLocal "github.com/frankieli/game_product/internal/modules/color_game/gms/adapter/local"
	colorgameGMSDomain "github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	colorgameGMSMachine "github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	colorgameGMSRepo "github.com/frankieli/game_product/internal/modules/color_game/gms/repository/db"
	colorgameGMSUseCase "github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	colorgameGSLocal "github.com/frankieli/game_product/internal/modules/color_game/gs/adapter/local"
	colorgameGSDomain "github.com/frankieli/game_product/internal/modules/color_game/gs/domain"
	colorgameGSDB "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/db"
	colorgameGSMemory "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/memory"
	colorgameGSRedis "github.com/frankieli/game_product/internal/modules/color_game/gs/repository/redis"
	colorgameGSUseCase "github.com/frankieli/game_product/internal/modules/color_game/gs/usecase"
	gatewayHttp "github.com/frankieli/game_product/internal/modules/gateway/adapter/http"
	gatewayAdapter "github.com/frankieli/game_product/internal/modules/gateway/adapter/local" // Updated to local
	gatewayUseCase "github.com/frankieli/game_product/internal/modules/gateway/usecase"
	"github.com/frankieli/game_product/internal/modules/gateway/ws"
	userHttp "github.com/frankieli/game_product/internal/modules/user/adapter/http"
	userLocal "github.com/frankieli/game_product/internal/modules/user/adapter/local"
	userRepo "github.com/frankieli/game_product/internal/modules/user/repository"
	userUseCase "github.com/frankieli/game_product/internal/modules/user/usecase"
	walletModule "github.com/frankieli/game_product/internal/modules/wallet"
	"github.com/frankieli/game_product/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Create logs directory if not exists
	if err := os.MkdirAll("logs/color_game", 0o755); err != nil {
		panic(err)
	}

	// Use lumberjack for log rotation
	logFile := &lumberjack.Logger{
		Filename:   "logs/color_game/monolith.log",
		MaxSize:    100,  // megabytes
		MaxBackups: 3,    // keep 3 old files
		MaxAge:     28,   // days
		Compress:   true, // compress old files
	}

	// Initialize logger
	logger.Init(logger.Config{
		Level:  "debug", // Ignore Debug logs, only log Info and above
		Format: "json",  // Use JSON for file output
		Output: logFile, // Write to rotating file
		Async:  false,   // Enable smart async logging (Info+ are sync now)
	})

	// Also print to console that we started
	fmt.Println("🚀 Starting Color Game Monolith... Logs are being written to logs/color_game/monolith.log (rotating)")
	logger.InfoGlobal().Msg("🎮 Starting Color Game Monolith...")

	// 1. Load Config
	cfg := config.LoadMonolithConfig()

	// 2. Initialize Infrastructure
	dbConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.User.Database.Host, cfg.User.Database.Port, cfg.User.Database.User, cfg.User.Database.Password, cfg.User.Database.Name)
	db, err := gorm.Open(postgres.Open(dbConnStr), &gorm.Config{})
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to connect to database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to get database instance")
	}

	// Connection Pool Configuration
	// Postgres default max_connections is usually 100.
	// We limit our application to use at most 50 connections to leave room for other tools/services.
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		logger.FatalGlobal().Err(err).Msg("Failed to ping database")
	}
	logger.InfoGlobal().Msg("✅ Database connected")

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.User.Redis.Host, cfg.User.Redis.Port),
	})
	defer rdb.Close()
	logger.InfoGlobal().Msg("✅ Redis connected")

	// 3. Initialize Modules

	// User Module (formerly Auth)
	userRepository := userRepo.NewUserRepository(db)
	sessionRepo := userRepo.NewSessionRepository(db)
	userUC := userUseCase.NewUserUseCase(userRepository, sessionRepo, cfg.User.JWT.Secret, cfg.User.JWT.Duration)
	userSvc := userLocal.NewHandler(userUC) // Use local adapter
	userHandler := userHttp.NewHandler(userSvc)
	logger.InfoGlobal().Msg("✅ User module initialized")

	// Wallet Module (Mock)
	walletSvc := walletModule.NewMockService()
	logger.InfoGlobal().Msg("✅ Wallet module initialized (Mock)")

	// Gateway Module (initialize early to get broadcast channel)
	wsManager := ws.NewManager()
	go wsManager.Run()

	// ColorGame Module (GMS + GS)
	logger.InfoGlobal().Msg("🎲 Initializing Color Game...")

	// 1. Initialize GMS (Game Machine Service) with broadcaster
	stateMachine := colorgameGMSMachine.NewStateMachine()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		stateMachine.Start(context.Background())
	}()
	logger.InfoGlobal().Msg("  ✅ State machine started")

	// Auto Migrate GameRound and BetOrder
	db.AutoMigrate(&colorgameGMSDomain.GameRound{}, &colorgameGSDomain.BetOrder{})
	gameRoundRepo := colorgameGMSRepo.NewGameRoundRepository(db)

	// Create Broadcasters
	// Create Broadcasters
	// Gateway listening to GMS events (and forwarding to WS)
	broadcaster := gatewayAdapter.NewBroadcaster(wsManager)

	// Initialize GMS with multiple broadcasters (Gateway initially, GS later)
	roundUC := colorgameGMSUseCase.NewRoundUseCase(stateMachine, broadcaster, nil, gameRoundRepo)
	gmsHandler := colorgameGMSLocal.NewHandler(roundUC)
	logger.InfoGlobal().Msg("  ✅ GMS initialized")

	// 2. Initialize GS (Game Service)
	var betRepo colorgameGSDomain.BetRepository
	if cfg.ColorGame.RepoType == "redis" {
		betRepo = colorgameGSRedis.NewBetRepository(rdb)
		logger.InfoGlobal().Msg("  ✅ GS Repository: Redis")
	} else {
		betRepo = colorgameGSMemory.NewBetRepository()
		logger.InfoGlobal().Msg("  ✅ GS Repository: Memory")
	}

	// Initialize BetOrderRepository for bet history
	betOrderRepo := colorgameGSDB.NewBetOrderRepository(db)

	// gmsHandler implements service.GMSService directly now
	playerUC := colorgameGSUseCase.NewPlayerUseCase(betRepo, betOrderRepo, gmsHandler, walletSvc, broadcaster)
	gsHandler := colorgameGSLocal.NewHandler(playerUC)

	// Initialize GS Broadcaster (listens to GMS events and triggers settlement)
	gsBroadcaster := colorgameGSLocal.NewGSBroadcaster(playerUC)

	// Set GS Broadcaster to GMS RoundUseCase
	roundUC.SetGSBroadcaster(gsBroadcaster)

	logger.InfoGlobal().Msg("  ✅ GS initialized")

	// 3. Initialize Gateway UseCase
	gatewayUC := gatewayUseCase.NewGatewayUseCase(gsHandler)
	logger.InfoGlobal().Msg("✅ Color Game ready")

	// Initialize HTTP Handler
	httpHandler := gatewayHttp.NewHandler(gatewayUC, wsManager, userSvc)

	// 4. Setup HTTP Server
	r := gin.Default()

	// Add logger middleware (must be first to capture all requests)
	r.Use(logger.GinMiddleware())

	// API Routes
	api := r.Group("/api")
	{
		// Let User module register its own routes
		userHandler.RegisterRoutes(api.Group("/users"))
	}

	// WebSocket Route
	r.GET("/ws", func(c *gin.Context) {
		httpHandler.HandleWebSocket(c.Writer, c.Request)
	})

	// 5. Start Server
	port := cfg.Gateway.Server.Port
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	logger.InfoGlobal().
		Str("port", port).
		Str("ws_url", fmt.Sprintf("ws://localhost:%s/ws?token=YOUR_TOKEN", port)).
		Str("api_url", fmt.Sprintf("http://localhost:%s/api", port)).
		Msg("🚀 Color Game Monolith running")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.FatalGlobal().Err(err).Msg("Failed to start server")
		}
	}()

	// 6. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.InfoGlobal().Msg("🛑 Shutting down server...")

	// 6.1 Stop HTTP Server first (stop accepting new requests)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.FatalGlobal().Err(err).Msg("Server forced to shutdown")
	}

	// 6.2 Stop State Machine (wait for current round to finish)
	logger.InfoGlobal().Msg("⏳ Waiting for current round to finish...")
	stateMachine.Stop()
	wg.Wait()

	// 6.3 Shutdown Gateway (close all WebSocket connections)
	logger.InfoGlobal().Msg("🔌 Closing all WebSocket connections...")
	wsManager.Shutdown()

	logger.InfoGlobal().Msg("👋 Server exited properly")
}
