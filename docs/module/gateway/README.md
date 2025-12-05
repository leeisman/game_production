# Gateway Module

Gateway 模組是整個遊戲系統的入口，負責處理 WebSocket 連接、請求轉發、Token 驗證以及遊戲狀態廣播。它採用了 Clean Architecture (Domain/UseCase/Adapter) 設計，並提供了高效能的 WebSocket 實現。

## 1. 模組結構

```
internal/modules/gateway/
├── domain/                    # 介面定義
│   └── gateway.go
├── usecase/                   # 業務邏輯
│   ├── gateway_uc.go          # 核心邏輯 (通用)
│   └── gateway_color_game_uc.go # 特定遊戲轉發邏輯
├── adapter/
│   ├── http/                  # HTTP/WebSocket 適配器 (Client -> Gateway)
│   │   └── handler.go         # 握手、消息分發
│   ├── local/                 # 單體內部調用適配器 (Broadcaster)
│   │   └── handler.go         # 實現 GatewayService 接口
│   └── grpc/                  # 微服務 gRPC 客戶端適配器
└── ws/                        # WebSocket 基礎庫
    └── manager.go             # 連接管理、讀寫泵
```

---

## 2. 請求處理流程

### 2.1 啟動與初始化 (`gateway.NewService`)
- 初始化 `ws.Manager` 用於管理所有活躍連接。
- 注入 `UserService` (用於驗證) 和 `GameService` (用於轉發)。

### 2.2 WebSocket 握手與連接 (`http.Handler.HandleWebSocket`)
1.  **Token 驗證**: 從 URL query 或 Header 提取 token，調用 `UserService.ValidateToken`。
2.  **升級協議**: 驗證通過後，將 HTTP 請求升級為 WebSocket 連接。
3.  **註冊連接**: 將新連接封裝為 `ws.Connection` 並註冊到 `ws.Manager`。

### 2.3 消息路由與轉發 (`gateway_uc.HandleMessage`)
1.  **解析信封**: 解析 JSON 消息的 `Header` (`game_code`, `command`)。
2.  **路由**: 根據 `game_code` 將請求路由到對應的處理函數 (如 `handleColorGame`)。
3.  **轉換**: 將 JSON payload 轉換為具體的 Proto Request。
4.  **調用**: 調用後端服務 (如 GMS) 的業務方法。
5.  **響應**: 將後端返回的 Proto Response 轉換回 JSON，並通過 WebSocket 發送給用戶。

### 2.4 廣播機制 (`ws.Manager.Broadcast`)
- 提供 `Broadcast(msg []byte)` 和 `SendToUser(userID, msg []byte)` 接口。
- **Fail-Fast**: 對於廣播消息，如果客戶端 buffer 滿，直接斷開連接以保護系統。
- **Timeout**: 對於單發消息，提供 5 秒超時機制。

---

## 3. 核心函數設計

### 3.1 `ws.Manager`
負責底層連接管理，是 Gateway 的核心組件。

*   **Register(conn, userID)**: 
    *   處理新連接註冊。
    *   **互斥登入**: 如果該 UserID 已有連接，主動斷開舊連接 (Reason: `replaced_by_new_connection`)。
*   **Run()**: 
    *   單執行緒處理所有 `register`/`unregister` 操作，確保 `clients` map 的線程安全，無需對每個操作加鎖。
*   **Broadcast(msg)**:
    *   遍歷所有連接發送消息。
    *   使用 `Non-blocking Send`，如果 `Send` channel 滿了，視為客戶端阻塞，立即調用 `CloseWithReason` 清理。

### 3.2 `ws.Connection`
代表單個用戶連接，負責讀寫數據。

*   **ReadPump()**:
    *   從 WebSocket 讀取數據。
    *   設置 `ReadLimit` (4KB) 防止大包攻擊。
    *   設置 `PongHandler` 維持心跳。
    *   讀取到的消息通過回調函數傳遞給 `http.Handler` 處理。
*   **WritePump()**:
    *   從 `Send` channel 讀取數據寫入 WebSocket。
    *   定期發送 `Ping` (每 54s) 保持連接活躍。
    *   處理寫入超時 (`WriteDeadline`)。
*   **CloseWithReason(reason, err)**:
    *   統一的資源釋放入口。
    *   使用 `sync.Once` 確保冪等性。
    *   **注意**: 只關閉底層 TCP 連接，不關閉 Go Channel，避免並發寫入導致的 Panic。

### 3.3 `usecase.GatewayUseCase`
純業務邏輯層，不依賴 HTTP/WebSocket 實現。

*   **HandleMessage(ctx, userID, msg)**:
    *   定義了標準的消息信封結構 `RequestEnvelope`。
    *   負責 JSON -> Proto -> JSON 的轉換與路由分發。

---

## 4. 通訊協議設計 (Protocol Envelop)

Gateway 定義了統一的 JSON 信封格式，所有業務消息都必須封裝在此結構中。Gateway 僅解析信封頭部進行路由，不關心具體的業務數據內容。

### 4.1 通用請求格式 (Client -> Server)

```json
{
  "game_code": "string",  // [必須] 路由目標 (e.g., "color_game", "wallet")
  "command": "string",    // [必須] 操作指指令 (e.g., "PlaceBetREQ", "GetBalanceREQ")
  "data": { ... }         // [可選] 具體業務參數，由目標服務解析
}
```

### 4.2 通用響應格式 (Server -> Client)

```json
{
  "game_code": "string",  // 來源模組
  "command": "string",    // 對應的響應指令 (e.g., "PlaceBetRSP")
  "data": {
     "error_code": 0,     // 統一錯誤碼 (0=成功)
     "error": "",         // 錯誤信息
     ...                  // 業務響應數據
  }
}
```

### 4.3 廣播消息格式 (Server Event)

```json
{
  "game_code": "string",
  "command": "string",    // 事件類型 (e.g., "RoundStateBRC")
  "data": { ... }         // 事件數據
}
```

---

## 2. 請求處理流程

### 2.1 啟動
- `main.go` 調用 `gateway.NewService`。
- 啟動 `ws.Manager` 協程，負責管理連接和廣播。

### 2.2 WebSocket 連接
- 用戶請求 `/ws`。
- `gateway.Service` -> `http.Handler`。
- **Token 驗證**: 調用 `UseCase.ValidateToken` -> `UserService`。
- **連接升級**: 升級為 WebSocket 連接。
- **註冊**: 將連接註冊到 `ws.Manager`。

### 2.3 消息處理 (轉發)
- 用戶發送消息 -> `ws.Connection.ReadPump`。
- 回調 `http.Handler` 中的匿名函數。
- **轉發**: 調用 `UseCase.HandleMessage` -> `GameService.HandleMessage`。
- **響應**: `GameService` 返回響應 -> `ws.Manager.SendToUser` -> 用戶。

### 2.4 廣播 (GMS -> Users)
- GMS 調用 `Broadcaster.Broadcast`。
- 消息進入 `ws.Manager` 的 `broadcast` channel。
- `ws.Manager` 遍歷所有客戶端並發送消息。

---

## 3. 關鍵組件職責

- **gateway.Service**: 模組門面，對外提供統一接口，隱藏內部複雜性。
- **http.Handler**: 處理 HTTP 協議細節，Token 驗證，WebSocket 升級。
- **usecase.GatewayUseCase**: 純業務邏輯，負責協調 User 和 Game 服務，不依賴 HTTP 或 WebSocket 細節。
- **ws.Manager**: 負責底層的 WebSocket 連接管理、併發安全、心跳保活。

---

## 4. WebSocket 設計與實現

WebSocket 實現遵循標準的 **Hub-Client** 模式。

### 4.1 連接生命週期管理 (`ws.Manager`)
*   **單執行緒狀態變更**: 使用單一 Goroutine 處理 `register` 和 `unregister` channel，確保 `clients` map 的執行緒安全。
*   **重複登入處理**: 當相同 UserID 登入時，主動斷開舊連接 (`ReasonReplaced`)。

### 4.2 I/O 處理 (The Pumps)
每個連接啟動兩個 Goroutine：
*   **ReadPump**: 負責從 Socket 讀取數據。
    *   設置 `ReadLimit` 防止記憶體攻擊。
    *   設置 `ReadDeadline` 和 `PongHandler` 檢測死連接。
*   **WritePump**: 負責寫入數據到 Socket。
    *   定期發送 `Ping` (每 54s) 保持連接活躍。
    *   處理 `Send` channel 中的消息。

### 4.3 消息投遞策略
*   **SendToUser**: 非阻塞發送，帶有超時回退 (Fallback Timeout)。如果 Buffer 滿了，等待 5 秒，若仍無法寫入則斷開連接。
*   **Broadcast**: 快速失敗 (Fail-Fast)。如果某個客戶端的 Buffer 滿了，**立即斷開該客戶端**，避免阻塞廣播流程影響其他用戶。

### 4.4 錯誤處理與資源清理 (`CloseWithReason`)

我們設計了 `CloseWithReason` 方法來統一處理連接關閉，其背後有幾個關鍵的設計考量：

1.  **集中化日誌 (Centralized Logging)**:
    *   每次斷線都會記錄明確的 `reason` (e.g., `buffer_full`, `timeout`, `read_error`)。
    *   這對於生產環境排查問題至關重要，我們可以清楚知道是因為網路問題、客戶端太慢、還是服務端主動踢人。

2.  **冪等性 (Idempotency)**:
    *   使用 `sync.Once` 確保清理邏輯只執行一次。
    *   無論是 ReadPump 出錯、WritePump 出錯、還是 Manager 主動踢人，都可以安全地調用此方法，不用擔心重複關閉導致的錯誤。

3.  **為什麼不關閉 Send Channel? (Panic Prevention)**:
    *   這是一個常見的 Go Concurrency 陷阱。如果我們在 `CloseWithReason` 中 `close(c.Send)`，那麼其他的 Goroutine (如廣播) 可能正試圖寫入這個 Channel，這會導致 **Panic: send on closed channel**。
    *   **解決方案**: 我們只關閉底層的 TCP 連接 (`c.Conn.Close()`)。
    *   **結果**: `WritePump` 會因為 TCP 寫入錯誤而退出，或者因為 `Send` channel 不再有數據而最終被 GC 回收。這是一種更安全、更健壯的資源釋放方式。

```go
func (c *Connection) CloseWithReason(r CloseReason, err error) {
    c.closeOnce.Do(func() {
        logger.Error(context.Background()).
            Int64("user_id", c.UserID).
            Str("reason", string(r)).
            Err(err).
            Msg("ws connection closed")
        
        // CRITICAL: Do NOT close the channel here.
        // Closing the channel while other goroutines might be writing to it 
        // will cause a panic. Let the GC handle the channel.
        // close(c.Send) 
        
        c.Conn.Close()
    })
}
```

---

## 5. 通訊協議 (Standardized API)

所有 WebSocket 消息 (Client Requests 和 Server Events) 均遵循統一的 **Header + Body** 結構。

**基本結構:**
```json
{
  "game": "string",     // [Header] 路由目標 (e.g., "color_game")
  "command": "string",  // [Header] 動作或事件類型 (e.g., "place_bet", "game_state")
  "data": { ... }       // [Body] 具體 Payload
}
```

### 5.1 Client Requests

#### Place Bet (下注)
```json
{
  "game": "color_game",
  "command": "place_bet",
  "data": {
    "color": "red",       // "red", "green", "blue"
    "amount": 100
  }
}
```

### 5.2 Server Events (Broadcasts)

#### Betting Started (開始下注)
```json
{
  "game": "color_game",
  "command": "betting_started",
  "data": {
    "round_id": "20231204120000",
    "start_time": 1701662400
  }
}
```

#### Game State Update (狀態更新)
```json
{
  "game": "color_game",
  "command": "game_state",
  "data": {
    "round_id": "20231204120000",
    "state": "BETTING", // "WAITING", "BETTING", "RESULTING"
    "countdown": 10
  }
}
```

#### Game Result (開獎結果)
```json
{
  "game": "color_game",
  "command": "result",
  "data": {
    "round_id": "20231204120000",
    "winner": "red"
  }
}
```

#### Settlement (結算)
```json
{
  "game": "color_game",
  "command": "settlement",
  "data": {
    "round_id": "20231204120000",
    "user_id": 123,
    "win_amount": 200,
    "balance": 1500
  }
}
```
