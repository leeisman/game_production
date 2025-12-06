# Game Production Platform

[![中文](https://img.shields.io/badge/Language-中文-blue.svg)](./README_CN.md)

> A production-grade framework for building scalable real-time multiplayer games in Go.

This project serves as a **Reference Architecture** for building high-concurrency game backends. It uses **"Color Game"** (a fast-paced multiplayer betting game) as a concrete implementation to demonstrate how to handle real-world challenges like state synchronization, atomic settlements, and broadcast storms.

## 🌟 Vision & Goal

Our goal is to provide a "Battle-Tested" foundation for game developers, solving common infrastructure problems out-of-the-box so you can focus on Gameplay.

*   **Production Ready**: Not just a toy project. Includes graceful shutdown, structured logging, metrics hooks, and docker builds.
*   **Architecture First**: Balances simplicity (Monolith) with scalability (Microservices).
*   **Live Demo Case**: The "Color Game" module showcases a complete lifecycle of a betting game (Round -> Bet -> Settlement).

### 1. Modular Monolith Architecture
*   **Clean Architecture**: Strictly follows Domain-Driven Design (DDD) with clear separation of layers (Domain, Usecase, Adapter, Repository).
*   **Flexible Deployment**: Supports both **Monolith** and **Microservices** deployment modes with the same codebase.
    *   **Monolith**: Ideal for development and small-to-medium deployments (zero RPC overhead, simple ops).
    *   **Microservices**: Suitable for large-scale scaling with gRPC communication between modules.

### 2. Proto-Driven Development
*   **Single Source of Truth**: All APIs, events, and data structures are defined in `shared/proto`.
*   **Type Safety**: Auto-generated Go code ensures type-safe communication between frontend, backend, and services.
*   **Standardized Protocol**: Unified WebSocket envelope format (`game_code`, `command`, `data`) and error code standards.

### 3. High Performance Gateway
*   **10k+ Concurrent Users**: Benchmarked to support 10,000+ simultaneous online players on a single node.
*   **Fail-Fast Broadcast**: Broadcast mechanism uses a fail-fast strategy to prevent slow connections from blocking the system.
*   **Optimized I/O**: Utilizes `epoll` (via net/http) and optimized WebSocket read/write pumps.

### 4. Smart Centralized Logging
*   **SmartWriter**: Custom log buffering mechanism.
    *   **Normal**: Asynchronous writing to reduce I/O blocking.
    *   **Panic/Error**: Synchronous immediate flush to ensure critical logs are preserved.
*   **Zero Allocation**: High-performance logging based on Zerolog wrapper.

### 5. Robust Game Core
*   **State Machine**: Manages game flow (Start -> Betting -> Drawing -> Result) with a rigorous state machine.
*   **Atomic Settlement**: Supports batch settlement and transactions to ensure atomicity of wallet deductions and payouts.
*   **Reconnection Safe**: Players can immediately retrieve the current complete state upon reconnection.

---

## 📚 Documentation

Detailed documentation is available in the `docs/` directory:

*   **Architecture**: [Design Principles](docs/shared/design_principles.md) | [Project Evolution](docs/ai/project_evolution.md)
*   **Modules**: [GMS](docs/module/color_game/gms/README.md) | [GS](docs/module/color_game/gs/README.md) | [Gateway](docs/module/gateway/README.md) | [User](docs/module/user/README.md)
*   **Guides**: [Monolith Startup](docs/cmd/color_game/monolith/README.md)
*   **Performance**: [Benchmark Report](docs/performance/color_game_benchmark.md)

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
│       ├── monolith/           # Monolith entry point (Recommended)
│       └── microservices/      # Microservices entry point
├── internal/
│   └── modules/
│       ├── color_game/         # Game Logic (GMS, GS)
│       ├── gateway/            # WebSocket Gateway
│       └── user/               # User & Auth Logic
├── pkg/                        # Public Libraries (Logger, Service interfaces)
├── shared/
│   └── proto/                  # Protobuf Definitions (API Contracts)
└── docs/                       # Project Documentation
```

---

## 📡 Protocol Snapshot

 The platform uses a standardized WebSocket JSON protocol:

**Client Request (Place Bet):**
```json
{
  "game_code": "color_game",
  "command": "ColorGamePlaceBetREQ",
  "data": {
    "color": "red",
    "amount": 100
  }
}
```

**Server Broadcast (Game State):**
```json
{
  "game_code": "color_game",
  "command": "ColorGameRoundStateBRC",
  "data": {
    "round_id": "20231204120000",
    "state": "GAME_STATE_BETTING",
    "left_time": 10
  }
}
```

---

## 🚀 Getting Started

### Prerequisites
*   Go 1.24+
*   PostgreSQL
*   Redis (Optional but recommended)

### Running Monolith

```bash
# Start the service
go run cmd/color_game/monolith/main.go
```

The service will start on port `8081`.

---

## 📝 License

Proprietary
