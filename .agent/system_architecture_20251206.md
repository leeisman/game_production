# System Architecture & Design Principles (2025-12-06)

This document is the **Single Source of Truth (SSOT)** for AI agents to understand the Game Product's architecture, design philosophy, and operational context. It consolidates previous evolution notes, design principles, and vision into one cohesive guide.

---

## 1. Project Vision: Staff Engineer Portfolio

This project is built to demonstrate **Staff Engineer** capabilities, focusing on:
*   **Cost Efficiency**: Optimizing resource usage (e.g., Worker Pools, Batch Processing) to minimize infrastructure costs.
*   **Performance**: Handling C10K+ concurrency with low latency using non-blocking I/O and optimized protocols.
*   **Scalability**: A **Modular Monolith** architecture that allows for easy transition to Microservices without code rewrite.
*   **Observability**: Structured logging (Zerolog), metrics hooks, and traceable request flows.

---

## 2. Core Architectural Patterns

### 2.1 Modular Monolith First
*   **Concept**: Use a single binary for simplicity (deployment, no RPC overhead) but maintain strict module boundaries.
*   **Implementation**:
    *   Modules (Gateway, User, GMS, GS) reside in `internal/modules`.
    *   Inter-module communication uses **Interfaces** defined in `pkg/service`.
    *   **Adapter Pattern**: Dependency Injection determines whether to use a `LocalAdapter` (direct call) or `GRPCAdapter` (remote call).

### 2.2 Clean Architecture (DDD)
We strictly follow dependencies: `Adapter -> UseCase -> Domain`.
*   **Domain**: Pure business entities and interface definitions. No external deps.
*   **UseCase**: Business orchestration. Depends only on Domain interfaces.
*   **Adapter (Handler/Repo)**: Implementation of interfaces. Connects to the outside world (HTTP, DB, Redis).
    *   *Naming Convention*: All adapters must be named `Handler` (e.g., `local/handler.go`).

### 2.3 Concurrency Model
*   **Worker Pools**: Used for high-frequency async tasks (e.g., GMS broadcasting) to bound resource usage and prevent OOM.
    *   *Observation*: Always include a **Fallback Mechanism** (spawn goroutine) if the pool is full to ensure critical events aren't lost.
*   **Batch Processing**: High-volume database writes (e.g., Settlement) are batched (chunk size: 500) to reduce I/O pressure.
*   **Non-Blocking I/O**: Critical paths (WebSocket pumps) use non-blocking select and buffered channels.

---

## 3. Module Communication & Contracts

### 3.1 Interface Location
*   **Internal**: `domain/` (e.g., `BetRepository` used by `GSUseCase`).
*   **External**: `pkg/service/` (e.g., `GMSService` used by `GSUseCase` to call `GMS`).

### 3.2 GMS & GS Interaction
*   **Bidirectional Dependency**:
    *   **GS -> GMS**: `GMSService.GetCurrentRound` (On-Demand Query for state validation).
    *   **GMS -> GS**: `ColorGameGSService.RoundResult` (RPC Push for settlement trigger).
*   **State Machine**:
    *   GMS drives the game loop.
    *   Broadcasts events via `pkg/service.GatewayService`.

---

## 4. Development Standards

### 4.1 Logging
*   **Library**: `pkg/logger` (Zerolog wrapper).
*   **Rule**: Always pass `ctx` to simple logging calls to propagate `request_id`.
*   **Performance**: Use `SmartWriter` for async buffered writing in production.

### 4.2 Protobuf Strategy
*   **SSOT**: All APIs and Models defined in `shared/proto`.
*   **Naming**: `*Req` (Request), `*Rsp` (Response), `*BRC` (Broadcast).
*   **Migration**: When changing proto, run `./scripts/gen_proto.sh` (or equivalent) and update all adapters.

### 4.3 Testing
*   **Location**: `tests/` for integration tests.
*   **Tool**: `cmd/color_game/test_robot` for end-to-end load testing.

---

## 5. Project Structure & Navigation

How to navigate the codebase:

### 5.1 Root Directories
*   `cmd/`: **Entry Points**.
    *   `monolith/`: The main executable for the Modular Monolith (wires everything together).
    *   `microservices/`: Individual entry points for distributed deployment.
*   `internal/modules/`: **Business Logic**. Contains `color_game` (GMS/GS), `gateway`, `user`.
*   `pkg/`: **Public Shared**.
    *   `service/`: **External Contracts** (Interfaces) for module communication.
    *   `logger/`, `utils/`: Common utilities.
*   `shared/proto/`: **API Definitions**. The Single Source of Truth for all data structures and RPC methods.

### 5.2 Module Layout (Standardized)
Every module inside `internal/modules/` follows this exact structure:
```text
internal/modules/<module_name>/
├── domain/       # [Pure] Structs, Repository Interfaces, Errors
├── usecase/      # [Logic] Business Flow, State Management (Depends on Domain)
└── adapter/      # [Glue]  Implementation of Interfaces
    ├── http/     # Gin Handlers
    ├── local/    # In-Memory Service Implementation (for Monolith)
    └── repository/ # DB Implementations (Postgres/Redis)
```

---

## 6. Current Roadmap (Status: 2025-12-06)

*   [x] **Refactoring**: Documentation modularization (GMS, GS, Monolith READMEs).
*   [x] **Resilience**: GMS Worker Pool with Fallback.
*   [x] **Protocol**: Unified WebSocket Protocol in Gateway.
*   [ ] **Optimization**: Memory profiling for struct alignment.
*   [ ] **Feature**: Adaptive Throttling for Gateway.
*   [ ] **Docs**: Architecture Decision Records (ADRs).

---

**Note to AI Agents**:
When modifying this codebase, **always verify** if your changes align with the "Modular Monolith" boundary. Do not import `internal/modules/A` directly into `internal/modules/B`. Use `pkg/service` interfaces.
