package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/frankieli/game_product/internal/config"
	colorgameGMSGRPC "github.com/frankieli/game_product/internal/modules/color_game/gms/adapter/grpc"
	colorgameGMSDomain "github.com/frankieli/game_product/internal/modules/color_game/gms/domain"
	colorgameGMSMachine "github.com/frankieli/game_product/internal/modules/color_game/gms/machine"
	colorgameGMSRepo "github.com/frankieli/game_product/internal/modules/color_game/gms/repository/db"
	colorgameGMSUseCase "github.com/frankieli/game_product/internal/modules/color_game/gms/usecase"
	"github.com/frankieli/game_product/pkg/discovery"
)

func main() {
	log.Println("🎰 Starting Color Game GMS (Game Machine Service)...")

	// Load Config
	cfg := config.LoadColorGameConfig()

	// Initialize State Machine
	stateMachine := colorgameGMSMachine.NewStateMachine()
	go stateMachine.Start(context.Background())
	log.Println("✅ State machine started")

	// Initialize Postgres
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Database.Host, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.Port)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("❌ Failed to connect to database: %v", err)
	} else {
		log.Println("✅ Database connected")
		// Auto Migrate
		db.AutoMigrate(&colorgameGMSDomain.GameRound{})
	}

	var gameRoundRepo colorgameGMSDomain.GameRoundRepository
	if db != nil {
		gameRoundRepo = colorgameGMSRepo.NewGameRoundRepository(db)
	}

	// Initialize RoundUseCase (no broadcaster in microservices mode)
	// Events will be streamed via gRPC
	roundUC := colorgameGMSUseCase.NewRoundUseCase(stateMachine, nil, nil, gameRoundRepo)
	log.Println("✅ Round UseCase initialized")

	// Start gRPC Server
	address := ":" + cfg.Server.Port
	log.Printf("🚀 GMS gRPC server starting on %s", address)
	log.Println("   Endpoints:")
	log.Println("   - RecordBet")
	log.Println("   - GetCurrentRound")
	log.Println("   - SubscribeEvents (streaming)")

	go colorgameGMSGRPC.StartServer(address, roundUC)

	// Register to Nacos
	ip := discovery.GetOutboundIP()
	portInt, _ := strconv.Atoi(cfg.Server.Port)
	nacosClient, err := discovery.NewNacosClient(cfg.Nacos.Host, cfg.Nacos.Port, cfg.Nacos.NamespaceID)
	if err != nil {
		log.Printf("❌ Failed to create Nacos client: %v", err)
	} else {
		err = nacosClient.RegisterService(cfg.Server.Name, ip, uint64(portInt), nil)
		if err != nil {
			log.Printf("❌ Failed to register service: %v", err)
		} else {
			log.Printf("✅ Registered to Nacos: %s at %s:%d", cfg.Server.Name, ip, portInt)
		}
	}

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down server...")

	if nacosClient != nil {
		nacosClient.DeregisterService(cfg.Server.Name, ip, uint64(portInt))
		log.Println("✅ Deregistered from Nacos")
	}
}
