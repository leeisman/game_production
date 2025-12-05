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
    *   **Infrastructure**：框架和驅動程式（Main、Config、DB 連接）。

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
