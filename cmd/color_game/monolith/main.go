package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/frankieli/game_product/internal/config"
	colorgameGMSLocal "github.com/frankieli/game_product/internal/modules/color_game/gms/adapter/local"
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
	gatewayAdapter "github.com/frankieli/game_product/internal/modules/gateway/adapter/local"
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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	// Parse command line flags
	pprofPort := flag.String("pprof-port", "", "Port to run pprof server on (e.g., 6060)")
	background := flag.Bool("d", false, "Run in background mode (disable console logging)")
	flag.Parse()

	// Initialize logger
	// If background is true, disable console logging (enableConsole = false)
	logger.InitWithFile("logs/color_game/monolith.log", "info", "json", !*background)
	defer logger.Flush()

	// Start pprof server if requested
	if *pprofPort != "" {
		go func() {
			addr := "localhost:" + *pprofPort
			logger.InfoGlobal().Str("addr", addr).Msg("📈 Starting pprof server")
			if err := http.ListenAndServe(addr, nil); err != nil {
				logger.ErrorGlobal().Err(err).Msg("Failed to start pprof server")
			}
		}()
	}

	// Also print to console that we started
	fmt.Println("🚀 Starting Color Game Monolith... Logs are being written to logs/color_game/monolith.log (rotating)")
	logger.InfoGlobal().Msg("🎮 Starting Color Game Monolith...")

	// 1. Load Config
	cfg := config.LoadMonolithConfig()

	// 2. Initialize Infrastructure
	dbConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.User.Database.Host, cfg.User.Database.Port, cfg.User.Database.User, cfg.User.Database.Password, cfg.User.Database.Name)

	// Configure GORM Logger
	gormLog := logger.NewGormLogger()
	gormLog.LogLevel = gormlogger.Info // Log all queries as requested

	db, err := gorm.Open(postgres.Open(dbConnStr), &gorm.Config{
		Logger: gormLog,
	})
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
	userHttpHandler := userHttp.NewHandler(userUC)
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

	gameRoundRepo := colorgameGMSRepo.NewGameRoundRepository(db)

	// Create Broadcasters
	// Gateway listening to GMS events (and forwarding to WS)
	gatewayHandler := gatewayAdapter.NewHandler(wsManager)

	// Initialize GMS with multiple broadcasters (Gateway initially, GS later)
	gmsUC := colorgameGMSUseCase.NewGMSUseCase(stateMachine, gatewayHandler, nil, gameRoundRepo)
	gmsHandler := colorgameGMSLocal.NewHandler(gmsUC)
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
	// Initialize BetOrderRepository for bet history
	betOrderRepo := colorgameGSDB.NewBetOrderRepository(db)

	// gmsHandler implements service.GMSService directly now
	gsUseCase := colorgameGSUseCase.NewGSUseCase(betRepo, betOrderRepo, gmsHandler, walletSvc, gatewayHandler)
	gsHandler := colorgameGSLocal.NewHandler(gsUseCase)

	// Set GS Handler as broadcaster (it implements both ColorGameService and GSBroadcaster)
	gmsUC.SetGSService(gsHandler)

	logger.InfoGlobal().Msg("  ✅ GS initialized")

	// 3. Initialize Gateway UseCase
	gatewayUC := gatewayUseCase.NewGatewayUseCase(gsHandler)
	logger.InfoGlobal().Msg("✅ Color Game ready")

	// Initialize HTTP Handler
	gatewayHttpHandler := gatewayHttp.NewHandler(gatewayUC, wsManager, userSvc)

	// 4. Setup HTTP Servers

	// Gateway Server (WebSocket) on 8081
	gatewayRouter := gin.New()
	gatewayRouter.Use(gin.Recovery())
	gatewayRouter.Use(logger.GinMiddleware())

	gatewayRouter.GET("/ws", func(c *gin.Context) {
		gatewayHttpHandler.HandleWebSocket(c.Writer, c.Request)
	})

	// User HTTP Server (REST API) on 8082
	userRouter := gin.New()
	userRouter.Use(gin.Recovery())
	userRouter.Use(logger.GinMiddleware())

	api := userRouter.Group("/api")
	{
		userHttpHandler.RegisterRoutes(api.Group("/users"))
	}

	// 5. Start Servers
	gatewayPort := cfg.Gateway.Server.Port // 8081
	userPort := cfg.User.Server.HTTPPort   // 8082

	gatewaySrv := &http.Server{
		Addr:    ":" + gatewayPort,
		Handler: gatewayRouter,
	}

	userSrv := &http.Server{
		Addr:    ":" + userPort,
		Handler: userRouter,
	}

	logger.InfoGlobal().
		Str("gateway_port", gatewayPort).
		Str("user_http_port", userPort).
		Str("ws_url", fmt.Sprintf("ws://localhost:%s/ws?token=YOUR_TOKEN", gatewayPort)).
		Str("user_api_url", fmt.Sprintf("http://localhost:%s/api/users", userPort)).
		Msg("🚀 Color Game Monolith running")

	// Start both servers concurrently
	go func() {
		if err := gatewaySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.FatalGlobal().Err(err).Msg("Gateway server failed")
		}
	}()

	go func() {
		if err := userSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.FatalGlobal().Err(err).Msg("User HTTP server failed")
		}
	}()

	// 6. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.InfoGlobal().Msg("🛑 Shutting down servers...")

	// 6.1 Stop HTTP Servers first (stop accepting new requests)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := gatewaySrv.Shutdown(ctx); err != nil {
		logger.ErrorGlobal().Err(err).Msg("Gateway server forced to shutdown")
	}

	if err := userSrv.Shutdown(ctx); err != nil {
		logger.ErrorGlobal().Err(err).Msg("User HTTP server forced to shutdown")
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
