# Color Game Platform

A scalable, real-time multiplayer color betting game platform built with Go.
This project demonstrates a **Modular Monolith** architecture that can seamlessly evolve into **Microservices**.

## 🌟 Key Features

*   **Modular Architecture**: Designed with Clean Architecture principles, allowing modules (GMS, GS, Gateway) to be deployed as a Monolith or distributed Microservices.
*   **Real-time Gaming**:
    *   **GMS (Game Manager Service)**: Manages game state (Round, Betting, Drawing, Result) using a robust State Machine.
    *   **GS (Game Service)**: Handles user bets, wallet transactions, and settlement.
    *   **Gateway**: Manages WebSocket connections and broadcasts game events to clients.
*   **Distributed ID Generation**: Uses Snowflake algorithm for unique Bet IDs.
*   **Type Safety**: All game events and interactions are defined using Protocol Buffers (gRPC) with strict Enum types.
*   **Flexible Deployment**: Support for both Monolith (single process) and Microservices (gRPC communication) deployments.

## 🏗 Project Structure

```
game_product/
├── cmd/
│   └── color_game/
│       ├── monolith/           # Single process deployment (All-in-One)
│       └── microservices/      # Distributed deployment
│           ├── gateway/        # WebSocket Gateway
│           ├── gms/            # Game Manager Service
│           └── gs/             # Game Service
├── internal/
│   └── modules/
│       ├── color_game/         # Game logic (GMS & GS)
│       └── gateway/            # Gateway logic
├── pkg/                        # Public libraries (Service interfaces, Logger, etc.)
├── shared/
│   └── proto/                  # gRPC protocol definitions
└── tests/                      # Integration tests
```

## 🚀 Getting Started

### Prerequisites

*   Go 1.24+
*   Redis (Optional, for distributed storage)
*   Protobuf Compiler (protoc)

### Running Locally (Monolith)

The easiest way to run the game for development.

```bash
go run cmd/color_game/monolith/main.go
```

### Running Locally (Microservices)

Run each service in a separate terminal:

1.  **GMS (Game Manager Service)**
    ```bash
    go run cmd/color_game/microservices/gms/main.go
    ```

2.  **GS (Game Service)**
    ```bash
    go run cmd/color_game/microservices/gs/main.go
    ```

3.  **Gateway**
    ```bash
    go run cmd/color_game/microservices/gateway/main.go
    ```

## 📡 API & Protocol

The platform uses WebSocket for client communication.

### WebSocket Events

All events are JSON formatted. The `type` field corresponds to the `EventType` enum defined in Proto, converted to lowercase string.

**Client -> Server:**

*   **Place Bet**:
    ```json
    {
      "game": "color_game",
      "command": "place_bet",
      "color": "red",
      "amount": 100
    }
    ```

**Server -> Client (Broadcasts):**

*   **Round Started**: `{"type": "round_started", "round_id": "...", "timestamp": 1234567890}`
*   **Betting Started**: `{"type": "betting_started", "round_id": "...", "timestamp": 1234567890}`
*   **Drawing**: `{"type": "drawing", "round_id": "...", "timestamp": 1234567890}`
*   **Result**: `{"type": "result", "round_id": "...", "data": "{\"color\": \"red\"}", "timestamp": 1234567890}`
*   **Settlement**: `{"type": "settlement", "round_id": "...", "data": "{\"win_amount\": 200}", "timestamp": 1234567890}`

## 🛠 Development

### Generating Proto Files

If you modify `.proto` files, regenerate the Go code:

```bash
protoc -I=. --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    shared/proto/colorgame/colorgame.proto
```

### Running Tests

```bash
go test -v ./tests/integration/gateway/...
go test -v ./tests/integration/color_game/...
```

## 📝 License

Proprietary
