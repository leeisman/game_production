# Protobuf Design & Definitions

本文檔定義了 Game Production 專案中 gRPC 服務與 Protobuf 消息的設計規範與現狀。

## 1. 設計風格 (Design Style)

### 1.1 命名規範 (Naming Conventions)
為了保持簡潔與一致性，我們採用以下命名規則：

*   **Request 消息**: 使用 `Req` 後綴 (e.g., `LoginReq`, `PlaceBetReq`)。
    *   *Why?* 比 `Request` 更短，且能有效區分消息類型。
*   **Response 消息**: 使用 `Rsp` 後綴 (e.g., `LoginRsp`, `PlaceBetRsp`)。
    *   *Why?* 比 `Response` 更短，且與 `Req` 對應。
*   **Service 方法**: 使用動詞開頭 (e.g., `Login`, `RecordBet`)。
*   **欄位命名**: 使用 `snake_case` (protobuf 預設)。

### 1.2 檔案結構
所有 proto 文件位於 `shared/proto/` 目錄下，按模組分類。

```
shared/proto/
├── user/           # 用戶與認證服務
│   └── user.proto
├── colorgame/      # 顏色遊戲邏輯
│   └── colorgame.proto
├── game/           # 通用遊戲接口
│   └── game.proto
└── wallet/         # 錢包服務
    └── wallet.proto
```

---

## 2. 服務定義 (Service Definitions)

### 2.1 User Service (`user.proto`)
負責用戶認證、Token 管理與個人資料。

*   **Service**: `UserService`
*   **Methods**:
    *   `Register(RegisterReq) returns (RegisterRsp)`
    *   `Login(LoginReq) returns (LoginRsp)`
    *   `ValidateToken(ValidateTokenReq) returns (ValidateTokenRsp)`
    *   `GetProfile(GetProfileReq) returns (GetProfileRsp)`

### 2.2 Color Game Service (`colorgame.proto`)
負責顏色遊戲的核心邏輯與狀態管理。

*   **Service**: `ColorGameService`
*   **Methods**:
    *   `RecordBet(RecordBetReq) returns (RecordBetRsp)`: GMS 記錄下注。
    *   `GetCurrentRound(GetCurrentRoundReq) returns (GetCurrentRoundRsp)`: 獲取當前局資訊。
    *   `SubscribeEvents(SubscribeEventsReq) returns (stream GameEvent)`: 訂閱遊戲事件流。

### 2.3 Game Service (`game.proto`)
通用的遊戲服務接口，用於 Gateway 轉發消息。

*   **Service**: `GameService`
*   **Methods**:
    *   `HandleMessage(HandleMessageReq) returns (HandleMessageRsp)`: 處理客戶端 WebSocket 消息。
    *   `UserConnected(UserConnectedReq) returns (UserConnectedRsp)`
    *   `UserDisconnected(UserDisconnectedReq) returns (UserDisconnectedRsp)`

### 2.4 Wallet Service (`wallet.proto`)
負責用戶餘額與交易。

*   **Service**: `WalletService`
*   **Methods**:
    *   `GetBalance(GetBalanceReq) returns (GetBalanceRsp)`
    *   `PlaceBet(PlaceBetReq) returns (PlaceBetRsp)`: 扣款下注。
    *   `SettleWin(SettleWinReq) returns (SettleWinRsp)`: 結算派彩。
    *   `Rollback(RollbackReq) returns (RollbackRsp)`: 交易回滾。

---

## 3. 演化記錄 (Evolution Log)

### 2025-12-01: Req/Rsp 簡化
將所有消息的後綴從 `Request/Response` 簡化為 `Req/Rsp`。

**Before:**
```protobuf
rpc RecordBet (RecordBetRequest) returns (RecordBetResponse);
```

**After:**
```protobuf
rpc RecordBet (RecordBetReq) returns (RecordBetRsp);
```

**優勢**:
1.  **代碼更簡潔**: `req := &pb.RecordBetReq{}` vs `req := &pb.RecordBetRequest{}`。
2.  **減少命名衝突**: 更短的名稱降低了與其他 package 的衝突機率。
3.  **統一風格**: 強制所有新舊消息一致。
