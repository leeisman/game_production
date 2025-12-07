# 架構原則

## 模組通訊與介面

### 內部介面 vs 外部介面
*   **Domain 介面 (`internal/modules/<module>/domain`)**:
    *   這些介面定義了**特定模組內部**的契約。
    *   用於解耦模組內部的各層（例如：UseCase 依賴 Repository 或內部 Broadcaster）。
    *   代表模組的內部需求。

*   **Service 介面 (`pkg/service`)**:
    *   這些介面定義了**外部**依賴或跨模組通訊的契約。
    *   當一個模組需要與另一個模組互動時（例如：GMS 呼叫 Gateway，或 GS 呼叫 Wallet），應該依賴定義在 `pkg/service` 中的介面。
    *   這確保了模組之間保持鬆散耦合，可以輕鬆替換或模擬。

### 範例：Gateway 通訊
*   **GatewayService**：定義在 `pkg/service/gateway.go`。
    *   方法：`Broadcast(gameCode, event)`、`SendToUser(userID, gameCode, event)`。
    *   用途：GMS 和 GS 都使用此介面向 Gateway 發送訊息。
    *   實作：
        *   **單體架構**：包裝 WebSocket manager 的本地適配器。
        *   **微服務架構**：gRPC 客戶端或 Redis Pub/Sub 發布者。

## Clean Architecture（整潔架構）
*   **分層**：
    *   **Domain**：實體和業務規則。純 Go 結構體，無外部依賴。
    *   **UseCase**：應用程式業務邏輯。依賴 Domain 和介面。
    *   **Adapter**：介面的實作（Repositories、Services、Handlers）。依賴外部函式庫（GORM、Redis、Gin、gRPC）。
    *   **Infrastructure**：框架和驅動程式。本專案中主要體現在 `cmd/` 下的 `main.go` (Composition Root)，負責依賴注入、配置加載、DB 連接與服務啟動。

## 單體與微服務混合架構
*   程式碼庫設計為同時支援單體和微服務部署。
*   **適配器**：我們使用適配器在本地方法呼叫（單體）和遠端呼叫（gRPC/Redis）之間切換，而無需更改業務邏輯。

## Adapter 層設計原則

### Adapter 的職責
*   **介面實作**：Adapter 層負責實作 Domain 和 Service 介面。
*   **技術細節隔離**：將外部技術（資料庫、訊息佇列、HTTP、gRPC）的細節封裝在 Adapter 中。
*   **依賴反轉**：UseCase 依賴介面，Adapter 實作介面，實現依賴反轉原則（DIP）。

### Adapter 的類型

#### 1. Repository Adapter
*   **位置**：`internal/modules/<module>/repository/`
*   **職責**：實作 Domain 中定義的 Repository 介面，處理資料持久化。
*   **範例**：
    *   `db/bet_repository.go`：使用 GORM 實作 `BetRepository`
    *   `redis/bet_repository.go`：使用 Redis 實作 `BetRepository`
    *   `memory/bet_repository.go`：使用記憶體實作 `BetRepository`（用於測試）

#### 2. Service Adapter (Local)
*   **位置**：`internal/modules/<module>/adapter/local/`
*   **職責**：實作 `pkg/service` 中定義的介面，用於單體架構中的本地方法呼叫。
*   **範例**：
    *   `gs/adapter/local/handler.go`：實作 `ColorGameService` 和 `GSBroadcaster`
    *   `gms/adapter/local/handler.go`：實作 `GMSService`
*   **特點**：直接呼叫 UseCase 方法，無網路開銷。

#### 3. Service Adapter (gRPC)
*   **位置**：`internal/modules/<module>/adapter/grpc/`
*   **職責**：
    *   **Server 端**：實作 gRPC 服務介面，接收遠端請求並呼叫 UseCase。
    *   **Client 端**：實作 `pkg/service` 介面，透過 gRPC 呼叫遠端服務。
*   **範例**：
    *   `gs/adapter/grpc/handler.go`：gRPC Server，實作 `ColorGameServiceServer`
    *   `gs/adapter/grpc/client.go`：gRPC Client，實作 `service.ColorGameService`

#### 4. Gateway Adapter
*   **位置**：`internal/modules/gateway/adapter/`
*   **職責**：處理外部通訊協定（HTTP、WebSocket）。
*   **範例**：
    *   `http/handler.go`：處理 HTTP 請求
    *   `local/broadcaster.go`：實作 `service.GatewayService`，透過 WebSocket 廣播訊息

### Adapter 設計準則

1.  **單一職責**：每個 Adapter 只負責一種技術實作（例如：GORM、Redis、gRPC）。
2.  **可替換性**：同一個介面可以有多個 Adapter 實作，可以根據部署模式（單體/微服務）或環境（開發/生產）選擇不同的實作。
3.  **錯誤轉換**：Adapter 應該將外部錯誤（如資料庫錯誤、網路錯誤）轉換為 Domain 錯誤或標準錯誤。
4.  **資料轉換**：Adapter 負責在外部資料格式（如 Protobuf、JSON、資料庫模型）和 Domain 實體之間進行轉換。
5.  **無業務邏輯**：Adapter 不應包含業務邏輯，業務邏輯應該在 UseCase 層。
6.  **命名規範**：所有 Adapter 實作統一命名為 `Handler`（例如：`local/handler.go`、`grpc/handler.go`），保持一致性。

### Adapter 選擇策略

*   **單體模式**：使用 Local Adapter，所有模組在同一個進程中，透過方法呼叫通訊。
*   **微服務模式**：使用 gRPC Adapter，模組分佈在不同的服務中，透過網路通訊。
*   **測試模式**：使用 Memory Adapter 或 Mock，快速測試業務邏輯，無需外部依賴。

---

## 關鍵架構決策 (Key Architectural Decisions)

### 1. 事件廣播策略 (Event Broadcasting)
**原則**：事件廣播機制必須根據部署模式進行適配，且**不依賴 Redis Pub/Sub** 作為核心廣播手段。

*   **單體模式**:
    *   **機制**: 直接方法調用 (Direct Method Call)。
    *   **路徑**: `GMS -> LocalHandler -> WebSocketManager -> Clients`。
    *   **優點**: 極低延遲，無序列化開銷。
*   **微服務模式**:
    *   **機制**: **gRPC Fan-out**。
    *   **路徑**: `GMS -> BaseClient (Discovery) -> gRPC Broadcast (All Gateways) -> WebSocketManager -> Clients`。
    *   **數據轉換**: GMS 發送 Protobuf `Any` 類型，Gateway 的 gRPC Handler 負責將其轉換為前端需要的 JSON 格式 (`{"command": "...", "data": ...}`)。

### 2. 服務發現與路由 (Service Discovery & Routing)
**原則**：客戶端負載均衡 (Client-side Load Balancing) 配合 TTL 緩存，以減少對註冊中心的壓力。

*   **註冊中心**: Nacos。
*   **客戶端實現**: `pkg/grpc_client/base/client.go`。
*   **TTL 機制**: 客戶端緩存服務實例地址 **10秒**。過期後自動異步更新。
*   **負載均衡**: 隨機選擇 (Random)。

### 3. Gateway 的雙重角色 (Dual Role of Gateway)
Gateway 模組的行為取決於部署模式：

*   **單體模式**: 作為一個 **Adapter**，它包含並運行其他模組 (GS, GMS) 的各個部分（透過 Local Adapters）。
*   **微服務模式**: 作為一個 **Pure Proxy (純代理)**。
    *   **無業務邏輯**: 不初始化與 Color Game 相關的 UseCase 或 Repository。
    *   **職責**: 僅負責維護 WebSocket 連接和轉發 gRPC 請求/廣播。

### 4. 服務生命週期管理 (Service Lifecycle Management)

#### 優雅關機（Graceful Shutdown）原則

為了保證數據完整性和用戶體驗，所有有狀態服務（Stateful Services）必須遵循嚴格的關機順序。

**核心法則：先關門，再熄火 (Stop Ingress, then Stop Core)**

錯誤的順序會導致 "上車後沒司機" 的情況：即請求進入了服務，但內部的核心邏輯（如狀態機）已經停止，導致請求失敗或狀態不一致。

**推薦的關機序列**:

1.  **Deregister (下線服務發現)**:
    *   從 Nacos/Consul 註銷，停止負載均衡器將新流量導向本節點。
2.  **Stop Ingress (關閉入口)**:
    *   調用 `grpcServer.GracefulStop()` 或 `httpServer.Shutdown()`。
    *   這一步會拒絕新的連接，但會**等待**已建立連接上的請求（如結算、下注）處理完畢。
3.  **Stop Core Logic (關閉核心邏輯)**:
    *   調用 `StateMachine.GracefulShutdown()`。
    *   對於週期性任務（如遊戲回合），必須採用 **Wait-For-Completion** 策略：等待當前週期（Round）完整結束後再退出。
    *   嚴禁在業務邏輯執行到一半時（如 Betting 階段）強制中斷。
4.  **Force Kill Protection (超時強制殺)**:
    *   如果上述過程超過預設時間（如 30s），必須有強制退出的機制，防止進程僵死。

### 5. 通訊協議與高性能設計 (Communication & Performance)

#### gRPC 模式與效能優勢

本專案在微服務間通訊全面採用 **gRPC**，而非 HTTP/REST，主要考量如下：

1.  **Protobuf 二進制序列化**:
    *   **效能**: 相比 JSON，Protobuf 序列化後體積更小（減少 30-50% 頻寬），編解碼速度快一個數量級。這對於**高頻下注**和**毫秒級狀態同步**至關重要。
    *   **類型安全**: 強類型的 `.proto` 定義，在編譯期就能發現接口不兼容問題。

2.  **通訊模式**:
    *   **Unary Call (一對一)**: 用於大多數業務請求（如 `PlaceBet`, `Login`）。簡單可靠，易於負載均衡。
    *   **Application-Level Fan-out (一對多廣播)**:
        *   在狀態廣播場景（GMS -> Gateways），我們沒有使用 gRPC Streaming，而是採用了**應用層扇出 (Fan-out)**。
        *   **機制**: GMS 從 Nacos 獲取所有健康的 Gateway IP，並行發送 Unary `Broadcast` 請求。
        *   **優勢**: 比起 Redis Pub/Sub，這種方式**可控性更强**（明確知道發給了誰，失敗可以重試）且**延遲更低**（少了一跳 Redis）。

3.  **去中心化負載均衡 (Client-Side Load Balancing)**:
    *   我們不使用中心化的 L7 LB（如 Nginx/Istio）來轉發內部 gRPC 流量。
    *   **機制**: 每個服務（Client）定期從 Nacos 拉取目標服務實例列表，並在本地進行隨機負載均衡。
    *   **優勢**:
        *   消除單點瓶頸。
        *   減少一跳網絡延遲 (Direct Pod-to-Pod communication)。
        *   配合 Nacos 實現快速的實例上下線感知。
