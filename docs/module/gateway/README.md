# Gateway Module

Gateway 模組是整個遊戲系統的入口，負責處理 WebSocket 連接、請求轉發、Token 驗證以及遊戲狀態廣播。它採用了 Clean Architecture (Domain/UseCase/Adapter) 設計，並提供了高效能的 WebSocket 實現。

## 1. 模組結構

```
gateway/
├── gateway.go              # 模組入口 (Facade)
├── domain/                 # 介面定義
│   └── gateway.go          # GatewayUseCase, ConnectionManager
├── usecase/                # 業務邏輯
│   └── gateway_uc.go       # 轉發邏輯 (Auth, Game)
├── adapter/
│   └── http/               # HTTP/WebSocket 適配器
│       └── handler.go      # WebSocket 握手、消息處理
└── ws/                     # WebSocket 基礎庫
    └── manager.go          # 連接管理、讀寫泵
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
