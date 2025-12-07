# System Architecture & Design Principles (2025-12-07 Update)

This document is the **Single Source of Truth (SSOT)** for AI agents to understand the Game Product's architecture, design philosophy, and operational context.

---

## 1. Core Architecture Philosophy

The system implements a **Hybrid Monolith/Microservices** architecture, leveraging **Clean Architecture** and **Interface-Based Dependency Injection**.

*   **Logic Isolation**: Business logic (UseCase) is decoupled from infrastructure via `interfaces`.
*   **Flexible Deployment**: The same codebase runs as:
    *   **Monolith (High Performance)**: Single process, zero-copy local calls (`LocalAdapter`).
    *   **Microservices (High Scalability)**: Distributed processes, gRPC remote calls (`GrpcAdapter`).
*   **Adapter Pattern**: The critical bridge.
    *   **Inbound**: HTTP/gRPC Handlers (Drive the UseCase).
    *   **Outbound**: `pkg/service` Implementations & Repositories (Driven by UseCase).

---

## 2. Microservices Architecture & Communication

### 2.1 Service Decomposition
1.  **Gateway Service**: **Access Layer**. Maintains WebSocket connections & routes messages. **No business logic**.
2.  **Game Management Service (GMS)**: **Stateful Core**. Runs the game loop State Machine.
3.  **Game Service (GS)**: **Transactional Core**. Handles bets, settlements (batch), and wallet ops.
4.  **User Service**: Managing user identity.

### 2.2 Communication Protocol
*   **Internal**: **gRPC (Protobuf)** via `pkg/service` interfaces. P2P Direct Connection (No Nginx).
*   **External**: **WebSocket (JSON)**. Gateway terminates WS and proxies to gRPC.
*   **Event Broadcast**: **gRPC Fan-out**.
    *   GMS/GS discovery all healthy Gateways.
    *   Parallel Unary gRPC calls to push events.
    *   **NO Redis Pub/Sub**.

### 2.3 Service Discovery & Routing (`pkg/grpc_client`)
*   **Registry**: Alibaba Nacos.
*   **Mechanism**: **Subscription (Push Model)** with **Jitter Updates**.
*   **Flow**:
    1.  **Subscribe**: Register Nacos listener on first access.
    2.  **Push**: Receive updates immediately on service changes.
    3.  **Jitter**: Delay local cache update by **0-3s** (random) to prevent thundering herd.
*   **Advantage**: Prevents "Thundering Herd" on Nacos and "Cache Avalanche".

### 2.4 Network Utilities (`pkg/netutil`)
*   Centralized logic for:
    *   **Outbound IP**: Auto-detect machine IP via UDP dial.
    *   **Port Selection**: Preferred port with random fallback (`ListenWithFallback`).

---

## 3. Service Lifecycle Management

### 3.1 Graceful Shutdown
**Strategy**: "Stop Ingress -> Wait for Completion -> Stop Core".

*   **Gateway**: Deregister -> Stop HTTP/gRPC (Stop new reqs) -> Stop WS Manager (Close conns).
*   **GMS**: Deregister -> Stop gRPC -> **Wait for State Machine Round End**.
*   **GS**: Deregister -> Stop gRPC -> **Wait for Settlement Batch Completion**.

### 3.2 Task-Specific Timeout
*   **Critical Tasks** (e.g., Round End, Settlement) have extended timeouts (e.g., 30s) to limit process zombies.
*   **Standard Tasks** (e.g., API requests) have short timeouts (e.g., 5s).

---

## 4. Development Standards

### 4.1 Development Workflow
*   **SSOT**: `shared/proto` defines the contract. All changes start here.
*   **Environment**: `cmd/ops` contains local dev/test tools.

### 4.2 Logging
*   **Library**: `pkg/logger` (Zerolog).
*   **Rule**: Always pass `ctx` to propagate `request_id`.

---

## 5. Directory Structure Navigation

*   `internal/modules/gateway/adapter/grpc/`: **Gateway gRPC Handler** (Protobuf -> JSON transformation).
*   `internal/modules/<module>/adapter/local/`: **Local Adapters** (Monolith mode glue code).
*   `pkg/grpc_client/base/`: **BaseClient** (Discovery, Jitter TTL, Connection Pool).
*   `pkg/netutil/`: **Network Utilities** (IP/Port logic).
*   `cmd/color_game/microservices/`: **Main Entry Points** (Dependency Injection Root).

---

## 6. Observability & Operations

### 6.1 Ops Service (`cmd/ops`)
*   **Role**: Centralized Operations Dashboard & Control Plane.
*   **Functions**:
    *   **Service & Instance Explorer**: Discovers services via Nacos.
    *   **Remote RPC Invocation**: Can invoke generic debug methods on services.
    *   **Performance Profiling**: Orchestrates the collection of Pprof data.

### 6.2 Performance Profiling (Pprof)
*   **Philosophy**: **On-Demand** & **Secure**. No open pprof ports on production services.
*   **Workflow**:
    1.  User requests profile via Ops UI.
    2.  Ops Service sends gRPC `CollectPerformance` command to target instance.
    3.  Target instance runs `runtime/pprof` locally for N seconds.
    4.  Data returned to Ops Service and stored.
    5.  Ops Service launches `go tool pprof` Web UI on demand and proxies it to the user.
*   **Resource Management**:
    *   **Idle Cleanup**: Pprof sessions killed after 15m inactivity.
    *   **LRU Eviction**: Max 2 concurrent sessions to save resources.
