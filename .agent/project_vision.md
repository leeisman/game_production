# Staff Engineer Project Vision & Goals

This document outlines the strategic vision for the "Game Product" project, positioning it as a Staff Engineer level portfolio piece. It focuses on demonstrating high-level architectural decision-making, cost-efficiency, performance optimization, and AI-friendliness.

## 1. Project Objectives (Staff Engineer Level)

The goal is to go beyond "functional code" and demonstrate:
*   **Architectural Scalability**: Designing systems that grow with business needs (e.g., Modular Monolith -> Microservices).
*   **Cost Efficiency**: Optimizing resource usage (CPU/Memory) to reduce infrastructure costs.
*   **Performance Engineering**: Handling high concurrency (C10K problem) and latency-sensitive operations (Gaming).
*   **Documentation Excellence**: Creating clear, maintainable documentation that scales the team's understanding.
*   **AI-Native Development**: Structuring the codebase and context to be easily understood and maintained by AI agents.

## 2. Key Technical Highlights to Demonstrate

### 2.1 Cost Efficiency & Resource Management
*   **Worker Pools**: Avoiding unlimited goroutine creation (e.g., GMS Event Broadcasting) to prevent OOM and reduce GC pressure.
*   **Batch Processing**: Optimizing database writes via batched inserts (e.g., GS Settlement) to reduce I/O cost and transactional overhead.
*   **Connection Pooling**: Proper configuration of DB and Redis pools to maximize throughput per node.

### 2.2 Performance Optimization
*   **Non-blocking Design**: Utilizing Go's concurrency model (Channels, Select) effectively without deadlocks.
*   **Stateless Services**: Ensuring GS (Game Service) is stateless for infinite horizontal scaling.
*   **Optimized Protocol**: Using efficient Protobuf/JSON payloads over WebSocket for low-latency updates.

### 2.3 Design Patterns & Maintainability
*   **Modular Monolith**: The ability to deploy as a simple binary (for small scale/dev) or split into microservices (for scale) using the **Adapter Pattern** (Local vs gRPC).
*   **Observer Pattern**: Decoupling the State Machine (GMS) from business logic (GS) and notification (Gateway) via asynchronous events.
*   **Clean Architecture**: Strict adherence to Domain/UseCase/Repository layers.

## 3. Pending Tasks (Roadmap)

### Performance & Cost
- [ ] **Memory Profiling**: Audit struct alignments and object allocations to reduce memory footprint.
- [ ] **Adaptive Throttling**: Implement rate limiting or simple backpressure mechanisms if Gateway is overwhelmed.

### Documentation & AI
- [ ] **Architecture Decision Records (ADRs)**: Document *why* certain choices were made (e.g., why Postgres over MongoDB, why Modular Monolith).
- [ ] **Agent Context Enhancement**: Expand `.agent/*.md` files to give AI agents a holistic view of the system constraints and conventions.

### Reliability
- [ ] **Graceful Degradation**: Ensure the system can run with degraded features (e.g., disable betting but keep broadcasting if DB is slow).
- [ ] **Chaos Engineering**: (Optional) Test system resilience by simulating network partitions or service failures.
