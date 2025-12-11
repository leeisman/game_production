# System Performance Tuning & Architecture Lessons (2025-12-11)

## 1. Architecture Patterns
- **Modular Monolith**: The current `internal/modules` structure is excellent. Don't rush to microservices unless necessary (e.g., team scaling, independent resource scaling).
- **Async Decoupling**: For high-throughput features like "Game Records", use an async architecture:
  - `TableServer` -> `GameRecordServer` (Fire & Forget via gRPC/MQ).
  - `GameRecordServer` uses **Buffer + Batch Write** to persist to MongoDB/Postgres.

## 2. Database Performance (Postgres/MongoDB)
- **The "No-Batch" Penalty**:
  - Single Insert/Update is bound by **IOPS** and **RTT**.
  - A typical Docker/SSD setup might cap at ~900 TPS for single writes.
  - **Batching** (e.g., `InsertMany`, `COPY`, `ON CONFLICT DO UPDATE`) can increase throughput by **50x-100x** (easily reaching 40k+ TPS).
- **Update vs Insert**:
  - `UPDATE` is "Find + Write".
  - Postgres `UPDATE` creates "Dead Tuples" (Write Amplification).
  - **Upsert (`ON CONFLICT`)** is significantly faster than "Transaction + Loop Update" but still heavier than pure Insert.
- **Latency vs Throughput**:
  - Without batching, Throughput = 1 / Latency.
  - With batching, Throughput is decoupled from Latency.

## 3. Performance Debugging Guide
We have identified distinct signatures for different bottlenecks:

### A. The "False Idle" (IO Bound)
- **Symptoms**:
  - Low CPU usage (< 5%).
  - High Latency (Slow response).
  - High Goroutine count (all in `Waiting` state).
- **Diagnosis Tools**:
  - **`go tool trace`**: Look for long **"Network Wait"** or **"Sync Block"** bars (Blue/Red), with very little execution (Green).
  - **Block Profile**: use `go tool pprof block.pb.gz`.
- **Solution**: Async Batching, Increase Connection Pool, Optimize Index.

### B. The "Login Storm" (CPU Bound)
- **Symptoms**:
  - High CPU usage (> 80%).
  - Latency correlates with CPU saturation.
- **Common Culprits**:
  - **Bcrypt**: Password hashing is computationally expensive.
  - **TLS Handshake**: RSA/ECC operations.
  - **Serialization**: Heavy JSON/Protobuf reflection.
- **Diagnosis Tools**:
  - **CPU Flame Graph**: `go tool pprof cpu.pb.gz`. You will see `bcrypt.CompareHashAndPassword` or `runtime.mallocgc` taking up width.
- **Solution**: Scale Out (HPA), Tune Bcrypt Cost (carefully), Offload Authentication.

### C. Retry Storm
- **Symptoms**: DB QPS > App QPS. Error logs show `DeadlineExceeded`.
- **Solution**: Circuit Breaker, Exponential Backoff.

## 4. OPS / Admin Service Capabilities
We have enhanced the `AdminService` (`pkg/admin/server.go`) and OPS Frontend (`cmd/ops/frontend`) to support full observability.

### Supported Tools:
1. **CPU Profile** (`cpu.prof`): For computation bottlenecks.
2. **Trace** (`trace.out`): For Latency/IO bottlenecks. **Proxy Supported via `go tool trace`**.
3. **Heap Snapshot** (`heap.prof`): For Memory Leaks.
4. **Goroutine Dump** (`goroutine.prof`): For Deadlocks.
5. **Block Profile** (`block.prof`): For Channel/IO blocking - *Newly Added*.
6. **Mutex Profile** (`mutex.prof`): For Lock Contention - *Newly Added*.

### Features:
- **Server-Side Streaming**: Handles large profiles (>4MB).
- **Dynamic Sampling**: `runtime.SetBlockProfileRate(1)` is auto-enabled during collection.
- **Web Proxy**: OPS Center now proxies both `pprof` and `trace` UIs, allowing analysis directly in the browser.
