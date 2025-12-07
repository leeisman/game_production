# System Architecture & Design Principles (2025-12-06 Update)

This document is the **Single Source of Truth (SSOT)** for AI agents to understand the Game Product's architecture, design philosophy, and operational context.

---

## 1. Project Vision: Staff Engineer Portfolio

This project is built to demonstrate **Staff Engineer** capabilities, focusing on:
*   **Cost Efficiency**: Optimizing resource usage (e.g., Worker Pools, Batch Processing).
*   **Performance**: Handling C10K+ concurrency with low latency.
*   **Scalability**: A **Modular Monolith** architecture transitioning to **Microservices**.
*   **Observability**: Structured logging (Zerolog), metrics hooks, and traceable request flows.

---

## 2. Microservices Architecture (Current Focus)

### 2.1 Service Decomposition
The system is split into distinct microservices:
1.  **Gateway Service**: Pure proxy handling WebSocket connections and request forwarding. **State-less**.
2.  **Game Management Service (GMS)**: Runs the game loop state machine. **State-ful**.
3.  **Game Service (GS)**: Handles betting logic, settlement, and wallet interactions.
4.  **User Service**: Handles user identity and wallet balance (currently Shared/Monolith).

### 2.2 Event Broadcasting Architecture (Revised)

**CRITICAL: NO REDIS PUB/SUB IS USED.**

We use a **gRPC Fan-out** pattern for delivering game state changes to clients:

1.  **Source**: GMS State Machine transitions.
2.  **Discovery**: GMS uses `BaseClient` to discover **ALL** healthy `gateway-service` instances from Nacos.
3.  **Transport**: GMS calls `Broadcast` gRPC method on each Gateway instance.
4.  **Transformation (Gateway)**:
    *   Gateway receives `Any` protobuf message.
    *   **Crucial Step**: Gateway `convertEvent` method unmarshals `Any` into specific types (`ColorGameRoundStateBRC`, etc.).
    *   Transformed into JSON format expected by frontend: `{"command": "...", "data": {...}}`.
5.  **Delivery**: Gateway pushes JSON to connected WebSocket clients.

### 2.3 Service Discovery & Routing

*   **Registry**: Alibaba Nacos.
*   **Mechanism**: Client-side load balancing with **TTL Caching**.
    *   Clients cache service addresses for **10 seconds**.
    *   If cache expires, the next call fetches fresh addresses from Nacos.
    *   This ensures new instances are discovered within 10s without overloading Nacos.
*   **Ports**:
    *   Gateway WS: `8081`
    *   User API: `8082`
    *   gRPC Services: Random ports (registered to Nacos).

### 2.4 Graceful Shutdown Strategy
To ensure zero downtime and data integrity (especially for game rounds and settlements):

**Principles**:
1.  **Stop Ingress First**: Cut off new traffic before stopping core logic.
2.  **Wait-For-Completion**: Core logic (State Machine, Settlements) must finish the current unit of work.

**Shutdown Sequences**:

*   **GMS (Game Management Service)**:
    1.  **Deregister**: Stop Nacos discovery.
    2.  **Stop gRPC**: `GracefulStop` (stop accepting new requests).
    3.  **Stop StateMachine**: Set flag and **Wait** for current round to finish (`GAME_STATE_ROUND_ENDED`).
    4.  **Timeout**: Force kill if stuck > 30s.

*   **GS (Game Service)**:
    1.  **Deregister**: Stop Nacos discovery.
    2.  **Stop gRPC**: `GracefulStop` waits for all active RPCs (including `SettleRound` batch processing) to complete.
    3.  **Timeout**: Force kill if stuck > 30s.

---

## 3. Core Architectural Patterns

### 3.1 Modular Monolith vs. Microservices
*   The codebase supports **both** modes.
*   **Monolith**: Uses `LocalAdapter` for direct method calls.
*   **Microservices**: Uses `GRPCAdapter` (via `BaseClient`) for remote calls.
*   **Gateway**: In microservices mode, it is stripped of all business logic (GS/GMS), acting purely as a router.

### 3.2 Testing & Mocks
*   **Mock Wallet**: Currently, `GS` uses a Mock Wallet Service for testing to isolate dependencies.
*   **OPS Tool**: Located in `cmd/ops`, used for manual gRPC testing and system monitoring.

---

## 4. Development Standards

### 4.1 Logging
*   **Library**: `pkg/logger` (Zerolog).
*   **Rule**: Always pass `ctx` to propagate `request_id`.

### 4.2 Protobuf Strategy
*   **SSOT**: `shared/proto`.
*   **Naming**: `*Req`, `*Rsp`, `*BRC`.
*   **Any Type**: Used for generic Broadcast/SendToUser payloads. Must be unmarshaled by the receiver (Gateway) before sending to WS.

---

## 5. Directory Structure Navigation

*   `cmd/color_game/microservices/`: Entry points for Gateway, GMS, GS.
*   `internal/modules/gateway/adapter/grpc/`: **Gateway gRPC Handler** (Critical for Broadcast transformation).
*   `pkg/grpc_client/base/`: **BaseClient** (Service Discovery, Connection Pool, TTL Cache).
*   `internal/modules/color_game/gms/machine/`: **State Machine** logic.
