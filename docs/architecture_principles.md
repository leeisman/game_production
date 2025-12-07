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
    *   **實作**:
    *   **單體架構**: `LocalAdapter` - 直接包裝 WebSocket manager 進行本地調用。
    *   **微服務架構**: `GRPCAdapter` - 使用 **gRPC Fan-out** 機制，直接向所有發現的 Gateway 實例發送消息（不使用 Redis Pub/Sub）。

## Clean Architecture（整潔架構）
*   **分層**：
    *   **Domain**: 實體和業務規則。純 Go 結構體，**無基礎設施依賴**（但可依賴 `shared/proto` 作為核心契約）。
    *   **UseCase**：應用程式業務邏輯。依賴 Domain 和介面。
    *   **Adapter (適配器層)**: 負責協議轉換與外部交互，主要對應專案中的 `adapter/` 目錄。其核心價值在於**隔離業務邏輯與部署架構**：
        *   **HTTP/gRPC Servers**: 將外部請求解析後，調用 UseCase。
        *   **Service Implementation**: 實作 `pkg/service` 定義的介面。透過不同的 Adapter 實作，我們能在不修改業務邏輯的情況下切換架構：
            *   **Monolith**: 實作本地調用（Local Adapter），直接在內存中交互。
            *   **Microservices**: 實作遠端調用（gRPC Adapter），透過網絡交互。
    *   **Infrastructure**：框架和驅動程式。本專案中主要體現在 `cmd/` 下的 `main.go` (Composition Root)，負責依賴注入、配置加載、DB 連接與服務啟動。

## 單體與微服務混合架構
本專案的代碼庫設計為同時支援單體和微服務兩種部署模式，其核心在於**依賴注入 (DI)** 與 **介面抽象**。

*   **統一介面 (pkg/service)**: 所有跨模組的交互（如 GMS -> Gateway, GMS -> User）都定義在 `pkg/service` 介面中。業務邏輯（UseCase）只依賴這些介面。
*   **靈活切換 (Composition Root)**: 在 `main.go` 中根據部署模式注入不同的實作：
    *   **單體模式**: 直接注入目標模組的 UseCase 或 Service 實例。調用是**進程內 (In-Process)** 的函數調用，零網絡開銷。
    *   **微服務模式**: 注入 `pkg/grpc_client` (gRPC Client)。調用會被序列化並通過網絡發送到遠端服務。

## Adapter 代碼結構對照 (Code Map)

以下是各類型 Adapter 在專案中的具體位置：

1.  **HTTP/gRPC Handlers (Inbound)**
    *   **位置**: `internal/modules/<module>/adapter/http/` 或 `grpc/`
    *   **職責**: 處理外部請求，調用 UseCase。

2.  **Repositories (Outbound)**
    *   **位置**: `internal/modules/<module>/repository/`
    *   **職責**: 實作 Domain Repository 介面 (CRUD)。

3.  **Service Clients (Outbound/Service Impl)**
    *   **位置 (Local)**: `internal/modules/<module>/adapter/local/` (實作本地直接調用)
    *   **位置 (Remote)**: `pkg/grpc_client/`
    *   **職責**: 實作 `pkg/service` 介面，用於模組間通訊。


### Adapter 選擇策略

1.  **單體模式 (Monolith)**：
    *   **Adapter**: `LocalAdapter` (直接調用 Service/UseCase)。
    *   **場景**: 單機高併發部署、開發調試。
    *   **優勢**: **最高效能**。完全沒有網絡序列化/反序列化開銷 (Zero-Copy)，延遲最低。

2.  **微服務模式 (Microservices)**：
    *   **Adapter**: `gRPCAdapter` (Client)。
    *   **場景**: 大規模集群部署、跨團隊開發。
    *   **優勢**: **可擴展性**。支援水平擴展，故障隔離。

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

*   **機制**: 客戶端負載均衡 (Client-side Load Balancing) 配合 **主動訂閱 (Subscription)**。
*   **流程**:
    1.  **Subscribe**: 客戶端首次訪問服務時，向 Nacos 註冊監聽 (Watcher)。
    2.  **Push Update**: 當服務實例變更（上下線）時，Nacos 主動推送最新列表。
    3.  **Jittered Update**: 客戶端收到推送後，隨機延遲 **0-3秒** 更新本地緩存。這防止了大規模集群在同一時刻並發更新導致的潛在鎖競爭或 CPU 峰值。
*   **優勢**:
    *   **實時性**: 秒級感知服務變更。
    *   **低開銷**: 平時無需輪詢，僅在變更時觸發。
*   **負載均衡策略**:
    *   **現狀**: **隨機 (Random)**。
    *   **擴展性**: 由於我們獲取了完整實例列表，未來可在客戶端輕鬆實作 **加權隨機 (Weighted Random)** 或 **輪詢 (Round Robin)** 算法。
*   **註冊中心**: Nacos。
*   **客戶端實現**: `pkg/grpc_client/base/client.go`。

### 3. Gateway 的定位 (Gateway Role)
Gateway 是系統的**接入層 (Access Layer)**，也是唯一的 WebSocket 入口。它**不包含**核心遊戲業務邏輯。

*   **核心職責**:
    1.  **連接管理**: 維護大量 WebSocket 長連接 (Stateful)。
    2.  **消息路由**:
        *   **Inbound**: 接收客戶端指令，透過 `pkg/service` 介面轉發給對應的業務模組 (GS, GMS, User)。
        *   **Outbound**: 接收業務模組的廣播請求，精準推送給前端客戶端。
*   **架構行為**:
    *   **單體模式**: 系統啟動時注入 **Local Adapter**。Gateway 直接調用同進程內的 GS/GMS 方法 (In-Process)。
    *   **微服務模式**: 系統啟動時注入 **gRPC Client**。Gateway 透過網路調用遠端的 GS/GMS 服務。

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
4.  **Task-Specific Timeout (任務級超時保護)**:
    *   針對關鍵業務（如 GMS 狀態機結算、GS 訂單處理），必須設定明確的**業務超時時間**（例如狀態機可能允許 30s 完成當前局，而普通請求僅允許 5s）。
    *   若超過此時間仍未完成，系統應記錄錯誤並強制釋放資源（或執行緊急保存），防止進程僵死導致部署卡住。

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
    *   **直連架構 (Direct Connection)**: 結合客戶端服務發現，內部服務間通訊 (Service-to-Service) 採用 **P2P 直連模式**，不經過任何中間代理（如 Nginx）。這消除了單點瓶頸，並顯著降低網絡跳數 (Network Hops) 與延遲。

