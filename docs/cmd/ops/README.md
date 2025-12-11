# OPS Center Service

OPS Center 是遊戲平台的中央管理與監控系統。除了基本的服務發現與 gRPC 控制外，最核心的功能是提供**深度的效能分析 (Deep Profiling)** 能力。

當線上遊戲服務出現卡頓、延遲或崩潰時，請參考以下場景選擇對應的分析工具。

## 🎯 實戰場景與分析指南 (Color Game Troubleshooting)

### 場景 1: 「玩家登入轉圈圈，但 DB 看起來很閒」
- **症狀**: 登入 API 回應極慢 (>2s)，User Service Pod CPU 使用率很高 (>80%)。
- **可能原因**: **密碼雜湊運算 (Bcrypt)** 耗盡了 CPU 資源。
- **推薦工具**: **CPU Profile (CPU 火焰圖)**
- **如何判讀**: 
  - 下載 `cpu.pprof`。
  - 看到 `bcrypt.CompareHashAndPassword` 佔據了火焰圖絕大部分寬度。
  - **解法**: 增加 Pod 數量 (HPA) 或降低 Bcrypt Cost (需評估安全性)。

### 場景 2: 「遊戲結算寫入很慢，但 CPU 根本沒在跑」
- **症狀**: `game-record-service` 處理速度變慢，CPU 使用率極低 (<5%)，但 Goroutine 數量飆升。
- **可能原因**: **I/O Wait (虛假空閒)**。資料庫寫入 (Postgres/Mongo) 變慢，導致大量請求卡在等待回應，甚至引發連線池 (Connection Pool) 耗盡。
- **推薦工具**: **Trace (執行路徑追蹤)** 或 **Block Profile**
- **如何判讀**:
  - **Trace (`trace.out`)**: 看到 Goroutine 大部分時間是藍色 (Network Wait) 或紅色 (Sync Block)，綠色 (執行) 時間極短。
  - **Block Profile (`block.pb.gz`)**: 看到 `database/sql` 或 `pgx` 的等待時間佔據紅色方塊最大面積。
  - **解法**: 啟用 Batch Write (批量寫入)，減少 DB RTT。

### 場景 3: 「WebSocket 推送延遲，大廳廣播卡住」
- **症狀**: Gateway Service 廣播全服公告時，部分玩家很久才收到，且 Gateway 記憶體緩慢上升。
- **可能原因**: **Channel 阻塞 (Backpressure)**。因為部分玩家網路差，導致 Gateway 內部負責推送的 `Go Channel` 滿了，發送端 (Broadcaster) 被迫阻塞等待。
- **推薦工具**: **Block Profile (阻塞分析)**
- **如何判讀**:
  - 下載 `block.pprof`。
  - 尋找 `runtime.chansend` (Channel 發送等待)。
  - 如果發現都在等某個 `ws.Client.sendCh`，代表下游消費太慢。
  - **解法**: 增加 Channel Buffer，或實作「主動斷線 (Drop Slow Consumer)」機制。

### 場景 4: 「高併發下遊戲邏輯偶發卡頓」
- **症狀**: Game Service (GS) 在萬人同時下注時，偶爾會卡住幾十毫秒。
- **可能原因**: **鎖競爭 (Lock Contention)**。多個 Goroutine 同時搶奪同一個 `Room` 或 `Wallet` 的 `Mutex` 鎖。
- **推薦工具**: **Mutex Profile (鎖競爭分析)**
- **如何判讀**:
  - 下載 `mutex.pprof`。
  - 看到 `sync.(*Mutex).Lock` 耗時很長。
  - 檢查是哪一行程式碼持有了鎖太久 (例如在鎖內做了 DB 查詢)。
  - **解法**: 縮小鎖的範圍 (Critical Section)，或改用 Sharding (分片鎖)。

### 場景 5: 「服務跑幾天後 OOM (Out Of Memory) 被殺掉」
- **症狀**: Pod 經常重啟，監控顯示記憶體呈現階梯式上漲。
- **可能原因**: **Goroutine 洩漏** 或 **物件殘留**。例如 WebSocket 斷線後 `Client` 物件沒被釋放。
- **推薦工具**: **Heap Snapshot** & **Goroutine Dump**
- **如何判讀**:
  - **Heap**: 檢查 `inuse_objects`，看哪種 Struct 數量異常多 (例如有 10 萬個 `Session` 物件)。
  - **Goroutine**: 檢查是否有大量 Goroutine 卡在 `select` 或 `nil channel` 上永遠不退出。

---

## Architecture details

### Performance Profiling (gRPC Streaming)

To support analyzing large heap dumps (which can exceed standard gRPC 4MB limits), the OPS Service uses **Server-Side Streaming**.

1. **Request**: OPS Center sends a `CollectPerformanceData` request with a duration (e.g., 30s).
2. **Collection**: The target service (e.g., `user-service`) collects profiles locally in memory buffers.
   - **Note**: `runtime.SetBlockProfileRate(1)` and `runtime.SetMutexProfileFraction(1)` are automatically enabled during this window for full-fidelity IO/Lock debugging.
3. **Streaming**: 
   - The target service splits the binary data into **32KB chunks**.
   - These chunks are streamed back to OPS Center via gRPC.
4. **Analysis**:### Pprof UI Proxy
OPS Center hosts a reverse proxy for **go tool pprof** and **go tool trace**, allowing you to view FlameGraphs and Execution Traces directly in the browser.cally

---

## Running Locally

```bash
# Run Backend
go run cmd/ops/main.go
```
