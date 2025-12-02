# Casino Game Platform - Project Structure

## Created Files

### Documentation & Configuration
- `/README.md` - Project overview and getting started guide
- `/docker-compose.yml` - Docker Compose configuration for local development
- `/scripts/init_db.sql` - PostgreSQL database initialization script

### Shared Proto Definitions
- `/shared/proto/auth/auth.proto` - Auth Service gRPC definitions
- `/shared/proto/player/player.proto` - Player Service gRPC definitions
- `/shared/proto/wallet/wallet.proto` - Wallet Service gRPC definitions

### Auth Service (Partially Implemented)
- `/services/auth/go.mod` - Go module definition
- `/services/auth/internal/domain/user.go` - User domain models
- `/services/auth/internal/repository/user_repository.go` - User data access layer
- `/services/auth/internal/usecase/auth_usecase.go` - Authentication business logic

## Next Steps to Complete Auth Service

1. **Session Repository** - Create `/services/auth/internal/repository/session_repository.go`
2. **gRPC Delivery Layer** - Create `/services/auth/internal/delivery/grpc/auth_handler.go`
3. **Configuration** - Create `/services/auth/internal/config/config.go`
4. **Main Entry Point** - Create `/services/auth/cmd/main.go`
5. **Dockerfile** - Create `/deployments/docker/Dockerfile.auth`

## Next Steps to Complete Player Service

1. Create similar structure as Auth Service:
   - Domain models
   - Repository layer
   - Use case layer
   - gRPC delivery layer
   - Configuration
   - Main entry point

## Next Steps to Complete Wallet Service

1. Create similar structure with additional complexity:
   - Transaction management with ACID guarantees
   - Optimistic locking for concurrent updates
   - Idempotency handling
   - Event publishing to NATS

## Proto Compilation

To generate Go code from proto files:

```bash
# Install protoc compiler and Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Compile proto files
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    shared/proto/auth/auth.proto

protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    shared/proto/player/player.proto

protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    shared/proto/wallet/wallet.proto
```

## Architecture Highlights

### Clean Architecture Layers

Each service follows Clean Architecture:

```
service/
├── cmd/                    # Application entry point
│   └── main.go
├── internal/
│   ├── domain/            # Business entities (User, Player, Wallet)
│   ├── usecase/           # Business logic (AuthUseCase, PlayerUseCase)
│   ├── repository/        # Data access (UserRepository, PlayerRepository)
│   ├── delivery/          # Handlers (gRPC, HTTP)
│   │   └── grpc/
│   └── config/            # Configuration management
├── go.mod
└── go.sum
```

### Key Design Patterns

1. **Repository Pattern** - Abstracts data access
2. **Use Case Pattern** - Encapsulates business logic
3. **Dependency Injection** - Services receive dependencies via constructors
4. **Clean Architecture** - Separation of concerns with clear boundaries

### Database Schema

The `init_db.sql` script creates:
- **users** - User authentication data
- **sessions** - Active user sessions
- **players** - Player profiles and stats
- **wallets** - Player balances with version for optimistic locking
- **transactions** - Complete transaction history with idempotency
- **game_configs** - Game configurations
- **table_rooms** - Multiplayer game rooms
- **game_rounds** - Game round data
- **bets** - Player bets
- **slot_spins** - Slot game spin results
- **rng_requests** - RNG audit trail
- **game_history** - Historical game data

## Current Implementation Status

### ✅ Completed
- Project structure design
- Database schema design
- Docker Compose configuration
- Proto definitions for Auth, Player, Wallet services
- Auth Service domain models
- Auth Service repository layer
- Auth Service business logic (use case)

### 🚧 In Progress
- Auth Service gRPC handler
- Auth Service configuration and main entry point

### ⏳ Pending
- Player Service implementation
- Wallet Service implementation
- RNG Service implementation
- Slot Game Service implementation
- Table Game Service implementation
- API Gateway implementation
- WebSocket Gateway implementation
- Integration testing
- Load testing

## Development Workflow

1. **Start Infrastructure**:
   ```bash
   docker-compose up -d postgres redis nats
   ```

2. **Compile Proto Files** (see above)

3. **Run Services**:
   ```bash
   cd services/auth && go run cmd/main.go
   cd services/player && go run cmd/main.go
   cd services/wallet && go run cmd/main.go
   ```

4. **Test Services**:
   ```bash
   go test ./...
   ```

## Technology Decisions

- **Go 1.21+**: Modern, performant, excellent for microservices
- **gRPC**: Efficient inter-service communication
- **PostgreSQL**: ACID compliance for financial transactions
- **Redis**: Fast caching and session storage
- **NATS**: Lightweight message queue for events
- **JWT**: Stateless authentication tokens
- **bcrypt**: Secure password hashing
- **Docker**: Containerization for easy deployment
