# Color Game Platform

A scalable, real-time multiplayer color betting game platform built with Go.
This project demonstrates a **Modular Monolith** architecture that can seamlessly evolve into **Microservices**.

## 🌟 Key Features

*   **Modular Architecture**: Designed with Clean Architecture principles, allowing modules (GMS, User, Gateway) to be deployed as a Monolith or distributed Microservices.
*   **High Performance**: Optimized for high concurrency, supporting **10,000+ concurrent users** on a single node.
    *   **Smart Buffered Logging**: Async I/O with immediate error flush.
    *   **Efficient WebSocket**: Non-blocking broadcast with fail-fast strategy.
*   **Real-time Gaming**:
    *   **GMS (Game Manager Service)**: Manages game state (Round, Betting, Drawing, Result) using a robust State Machine.
    *   **Gateway**: Manages WebSocket connections and broadcasts game events to clients.
*   **Type Safety**: All game events and interactions are defined using Protocol Buffers (gRPC).

---

## 📚 Documentation

Detailed documentation is available in the `docs/` directory:

*   **Architecture & Design**:
    *   [Project Evolution](docs/ai/project_evolution.md): History of architectural decisions and design philosophy.
    *   [Gateway Module](docs/module/gateway/README.md): WebSocket design and protocol.
    *   [User Module](docs/module/user/README.md): User authentication and management.
*   **Performance**:
    *   [Benchmark Report](docs/performance/color_game_benchmark.md): How we achieved 10k+ CCU.
*   **Shared Components**:
    *   [Logger System](docs/pkg/logger.md): SmartWriter and logging strategy.
    *   [Protobuf Definitions](docs/shared/protobuf.md): Service interfaces and message standards.
*   **Operations**:
    *   [Service Guide](docs/cmd/color_game.md): Startup and troubleshooting guide.

---

## 🚀 Performance Highlights

We have successfully benchmarked the system with **12,500+ Concurrent Users (CCU)** on a local development machine.

*   **Optimization Journey**:
    1.  **Login**: Solved bcrypt CPU spikes with ramp-up strategies.
    2.  **Connection**: Tuned OS limits (`ulimit`, ephemeral ports).
    3.  **Logging**: Implemented `SmartWriter` to eliminate console I/O blocking, which was the primary bottleneck at 4500 users.

See [Benchmark Report](docs/performance/color_game_benchmark.md) for details.

---

## 🏗 Project Structure

```
game_product/
├── cmd/
│   └── color_game/
│       ├── monolith/           # Single process deployment (Recommended)
│       └── microservices/      # Distributed deployment
├── internal/
│   └── modules/
│       ├── color_game/         # Game logic
│       ├── gateway/            # WebSocket Gateway
│       └── user/               # User & Auth logic
├── pkg/                        # Public libraries (Logger, etc.)
├── shared/
│   └── proto/                  # gRPC protocol definitions
└── docs/                       # Comprehensive documentation
```

---

## 📡 API & Protocol

The platform uses WebSocket for client communication with a standardized **Header + Body** JSON format.

**Client Request (Place Bet):**
```json
{
  "game": "color_game",
  "command": "place_bet",
  "data": {
    "color": "red",
    "amount": 100
  }
}
```

**Server Broadcast (Game State):**
```json
{
  "game": "color_game",
  "command": "game_state",
  "data": {
    "round_id": "20231204120000",
    "state": "BETTING",
    "countdown": 10
  }
}
```

---

## 🚀 Getting Started

### Prerequisites
*   Go 1.24+
*   Redis (Optional)
*   PostgreSQL

### Running Monolith (Recommended)

```bash
# Start the service
go run cmd/color_game/monolith/main.go
```

The service will start on port `8080`.

---

## 📝 License

Proprietary
