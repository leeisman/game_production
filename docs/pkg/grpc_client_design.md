# 統一 gRPC 客戶端設計 (Unified gRPC Client Design)

## 概述 (Overview)
`pkg/grpc_client` 套件為 Game Product 系統提供了一個健壯、高效能且可擴展的 gRPC 客戶端架構。它集中解決了分散式系統中常見的挑戰，如服務發現 (Service Discovery)、負載均衡 (Load Balancing)、連線池 (Connection Pooling) 以及廣播扇出 (Broadcast Fan-out)。

此設計確保各個微服務（如 Color Game）只需專注於自身的業務邏輯，而由 Base Client 負責處理與基礎設施通訊的複雜性。

## 架構 (Architecture)

### 套件結構 (Package Structure)
核心邏輯位於 `pkg/grpc_client/base/` 目錄下：

- **`client.go`**: 定義了 `BaseClient` 結構體、連線池 (Connection Pool)、工作池 (Worker Pool) 以及服務監控 (Service Watcher) 邏輯。
- **`user.go`**: 實作 `UserService` 介面 (認證相關)。
- **`wallet.go`**: 實作 `WalletService` 介面 (錢包相關)。
- **`gateway.go`**: 實作 `GatewayService` (廣播) 介面。

### 關鍵組件 (Key Components)

1.  **Registry**: 連接 Nacos（或其他發現後端）以獲取服務地址的介面。
2.  **Connection Pool (`conns` map)**: 緩存活躍的 `grpc.ClientConn` 物件，以 IP 地址為 Key。確保 TCP 連線能被高效復用。
3.  **Service Cache (`serviceAddrs` map)**: 本地記憶體中的健康服務實例快取 (IP:Port 列表)，避免頻繁對 Registry 發起網路請求。
4.  **Worker Pool (`broadcastQueue`)**: 用於非同步廣播任務的有界併發機制。

---

## 核心流程 (Core Workflows)

### 1. 動態服務發現與快取 (零延遲)
**問題**: 每次 RPC 調用都查詢 Nacos 會很慢並造成瓶頸。
**解決方案**:
1.  **惰性註冊 (Lazy Registration)**: 當第一次呼叫 `GetLBConn("svc-name")` 時，"svc-name" 會被加入到本地快取 (`serviceAddrs`) 中。
2.  **背景監控 (Background Watcher)**: 一個背景 Goroutine (`StartServiceWatcher`) 每 3 秒喚醒一次。
3.  **輪詢 (Polling)**: 它遍歷快取中所有已知的服務，向 Nacos 查詢最新的實例列表，並更新本地快取。
4.  **結果**: RPC 調用總是直接讀取本地記憶體 (RWMutex)，實現了近乎零延遲的服務發現開銷。
5.  **防擊穿機制 (Singleflight)**: 萬一快取失效或服務剛啟動，為了防止數千個併發請求同時查詢 Nacos (Thundering Herd)，我們使用了 `singleflight` 機制。這確保了同一時間對同一服務的查詢 **只有一個** 會發送到 Registry，其他請求會等待並共享結果。

### 2. 客戶端負載均衡 (隨機策略)
**問題**: 如果快取單個 `UserServiceClient` 包裝器，會導致所有請求都綁定到同一個後端實例 (Sticky Connection)，造成負載不均。
**解決方案**:
1.  **無狀態客戶端**: `BaseClient` **不** 永久存儲 `userClient` 或 `walletClient`。
2.  **逐次請求負載均衡 (Per-Request LB)**: 每次呼叫 `ValidateToken` 或 `GetBalance` 等方法時：
    - 呼叫 `GetLBConn`。
    - 從 Service Cache 獲取 IP 列表。
    - **隨機挑選 (Randomly Pick)** 其中一個 IP。
    - 返回該 IP 對應的持久化連線 (從 Connection Pool)。
3.  **效率**: 即使我們隨機挑選，底層 TCP 連線是復用的。如果選中了 IP A，就會使用與 IP A 已經建立好的連線。

### 3. 廣播扇出 (Gateway Fan-out)
**問題**: 我們不知道用戶連接在哪個 Gateway 實例上。廣播必須觸達 **所有** Gateway 實例。
**限制**: Nacos 命名解析器通常只提供負載均衡（選一個實例），而不是廣播（所有實例）。
**解決方案**:
1.  **手動扇出 (Manual Fan-out)**: `Broadcast` 方法查詢 Registry 獲取 `gateway-service` 的 **所有** 實例。
2.  **並行執行**: 遍歷每一個實例地址。
3.  **併發控制**: 將任務提交給 **Worker Pool**，以便並行地向每個實例發送 gRPC 請求。

### 4. 工作池與降級機制 (Worker Pool & Fallback)
**問題**: 如果要向 50 個 Gateway 並行廣播給 10,000 個用戶，可能會瞬間產生數百萬個 Goroutines，導致程式崩潰。
**解決方案**:
1.  **有界隊列 (Bounded Queue)**: 一個緩衝 Channel `broadcastQueue` (大小 1024) 限制待處理任務。
2.  **固定 Workers**: 固定數量的 Goroutines (例如 20 個) 從隊列中處理任務。
3.  **降級機制 (Fallback Mechanism)**: 如果隊列已滿 (背壓 Backpressure)，客戶端 **不會** 阻塞。相反，它會記錄警告並啟動一個臨時的 Goroutine (`go func()`) 來確保訊息能立即發送。這確保了在流量尖峰時的可靠性，同時在平時保持資源安全。

---

## 使用指南 (Usage Guide)

### 初始化客戶端
```go
// 在 main.go 中
registry, _ := discovery.NewNacosClient(...)
baseClient := base.NewBaseClient(registry)
```

### 呼叫服務 (單播 Unicast)
直接呼叫介面方法即可。負載均衡會自動處理。
```go
uid, name, _, err := baseClient.ValidateToken(ctx, token)
balance, err := baseClient.GetBalance(ctx, uid)
```

### 廣播 (多播 Multicast)
```go
// 這將傳達給所有的 Gateway 實例
baseClient.Broadcast("color_game", eventMsg)
```

### 擴展新服務
1.  建立新檔案 `pkg/grpc_client/base/new_service.go`。
2.  實作服務方法。
3.  在方法內部使用 `c.GetLBConn("new-service-name")`。
4.  Service Watcher 會在第一次呼叫後自動開始監控該服務。
