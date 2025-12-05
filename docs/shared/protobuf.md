# Protobuf Design & Definitions

本文檔定義了 Game Production 專案中 gRPC 服務與 Protobuf 消息的設計規範與現狀。

## 1. 設計風格 (Design Style)

### 1.1 命名規範 (Naming Conventions)
為了保持簡潔與一致性，我們採用以下命名規則：

#### 基本規則
*   **Request 消息**: 使用 `Req` 後綴 (e.g., `ColorGamePlaceBetReq`)。
    *   *Why?* 比 `Request` 更短，且能有效區分消息類型。
*   **Response 消息**: 使用 `Rsp` 後綴 (e.g., `ColorGamePlaceBetRsp`)。
    *   *Why?* 比 `Response` 更短，且與 `Req` 對應。
*   **Broadcast 消息**: 使用 `BRC` 後綴 (e.g., `ColorGameSettlementBRC`)。
    *   *Why?* 明確標識單向廣播消息，與 REQ/RSP 區分。
*   **Service 方法**: 使用動詞開頭 (e.g., `Login`, `RecordBet`)。
*   **欄位命名**: 使用 `snake_case` (protobuf 預設)。

#### 模組前綴規範
*   **ColorGame 模組**: 所有消息必須使用 `ColorGame` 前綴
    *   範例: `ColorGamePlaceBetReq`, `ColorGameSettlementBRC`
    *   *Why?* 避免命名衝突，明確模組歸屬
*   **其他模組**: 根據需要添加模組前綴
    *   範例: `UserLoginReq`, `WalletGetBalanceReq`

### 1.2 檔案結構
所有 proto 文件位於 `shared/proto/` 目錄下，按模組分類。

```
shared/proto/
├── common/         # 通用定義（ErrorCode 等）
│   └── common.proto
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
    *   `PlaceBet(ColorGamePlaceBetReq) returns (ColorGamePlaceBetRsp)`: 玩家下注
    *   `GetState(ColorGameGetStateReq) returns (ColorGameGetStateRsp)`: 獲取遊戲狀態
*   **內部 RPC** (GMS ↔ GS):
    *   `RecordBet(ColorGameRecordBetReq) returns (ColorGameRecordBetRsp)`: GMS 記錄下注
    *   `GetCurrentRound(ColorGameGetCurrentRoundReq) returns (ColorGameGetCurrentRoundRsp)`: 獲取當前局資訊
*   **Broadcast Messages**:
    *   `ColorGameRoundStateBRC`: 遊戲狀態廣播
    *   `ColorGameSettlementBRC`: 結算廣播
    *   `ColorGameEvent`: 通用事件

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

## 3. 標準 Response 結構

所有 `Rsp` 消息必須包含以下標準欄位：

```protobuf
message XxxRsp {
  common.ErrorCode error_code = 1;  // 必須：錯誤代碼（int32）
  // ... 業務數據欄位
  string error = N;                  // 必須：錯誤訊息（成功時為空）
}
```

**ErrorCode 定義** (`common.proto`):
- `0` = SUCCESS
- `5` = INTERNAL_ERROR
- `200` = INSUFFICIENT_BALANCE
- `301` = ROUND_NOT_ACTIVE
- `302` = INVALID_BET_AMOUNT

---

## 4. 演化記錄 (Evolution Log)

### 2025-12-05: ColorGame 模組前綴規範化
為所有 ColorGame 相關的 proto message 添加 `ColorGame` 前綴。

**Before:**
```protobuf
message PlaceBetReq {
  int64 user_id = 1;
  string color = 2;
  int64 amount = 3;
}

message GameEvent {
  EventType type = 1;
  string round_id = 2;
}
```

**After:**
```protobuf
message ColorGamePlaceBetReq {
  int64 user_id = 1;
  string color = 2;
  int64 amount = 3;
}

message ColorGameEvent {
  ColorGameEventType type = 1;
  string round_id = 2;
}
```

**新增 Broadcast 消息**:
```protobuf
message ColorGameSettlementBRC {
  string round_id = 1;
  string winning_color = 2;
  string bet_id = 3;
  string bet_color = 4;
  int64 bet_amount = 5;
  int64 win_amount = 6;
  bool is_winner = 7;
}
```

**優勢**:
1.  **避免命名衝突**: `ColorGame` 前綴明確標識模組歸屬
2.  **提高可讀性**: 一眼就能看出 message 屬於哪個模組
3.  **統一規範**: 為未來其他遊戲模組建立命名標準
4.  **支持多遊戲**: 當有多個遊戲時，不會產生 message 名稱衝突

### 2025-12-01: Req/Rsp 簡化
將所有消息的後綴從 `Request/Response` 簡化為 `Req/Rsp`。

**Before:**
```protobuf
rpc RecordBet (RecordBetRequest) returns (RecordBetResponse);
```

**After:**
```protobuf
rpc RecordBet (ColorGameRecordBetReq) returns (ColorGameRecordBetRsp);
```

**優勢**:
1.  **代碼更簡潔**: `req := &pb.ColorGameRecordBetReq{}` vs `req := &pb.RecordBetRequest{}`。
2.  **減少命名衝突**: 更短的名稱降低了與其他 package 的衝突機率。
3.  **統一風格**: 強制所有新舊消息一致。

---

## 5. 最佳實踐

### 5.1 Message 設計
- ✅ **DO**: 使用模組前綴（如 `ColorGame`）
- ✅ **DO**: 所有 RSP 包含 `error_code` 和 `error`
- ✅ **DO**: 使用 `BRC` 後綴標識廣播消息
- ❌ **DON'T**: 在 message 中定義信封欄位（如 `game_code`, `command`）
- ❌ **DON'T**: 使用字符串表示 `error_code`

### 5.2 向後兼容性
- 添加新欄位時使用新的 field number
- 不要修改已有欄位的編號
- 不要刪除已使用的 field number
- 使用 `reserved` 關鍵字保護已刪除的欄位

### 5.3 文檔
- 為每個 message 和 service 添加註釋
- 說明欄位的用途和約束
- 記錄重要的演化變更

---

## 6. 參考資料

- [Protocol Buffers 官方文檔](https://protobuf.dev/)
- [gRPC 最佳實踐](https://grpc.io/docs/guides/performance/)
- 專案內部規範: `.agent/proto_design_guidelines.md`
