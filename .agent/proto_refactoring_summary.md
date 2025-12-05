# Proto 重構完成總結

## 完成時間
2025-12-05

## 重構內容

### 1. 統一命名規範
所有 ColorGame 相關的 proto message 現在都使用 `ColorGame` 前綴：

#### Request/Response
- `ColorGamePlaceBetReq` / `ColorGamePlaceBetRsp`
- `ColorGameGetStateReq` / `ColorGameGetStateRsp`
- `ColorGameRecordBetReq` / `ColorGameRecordBetRsp`
- `ColorGameGetCurrentRoundReq` / `ColorGameGetCurrentRoundRsp`

#### Broadcast
- `ColorGameRoundStateBRC` - 遊戲狀態廣播
- `ColorGameSettlementBRC` - 結算廣播（新增）
- `ColorGameEvent` - 通用事件
- `ColorGameBetConfirmation` - 下注確認

#### Other
- `ColorGamePlayerBet` - 玩家下注信息
- `ColorGameEventType` - 事件類型枚舉
- `ColorGameSubscribeEventsReq` - 訂閱事件請求

### 2. 新增 ColorGameSettlementBRC

專門用於結算通知的廣播消息：

```protobuf
message ColorGameSettlementBRC {
  string round_id = 1;
  string winning_color = 2;
  
  // 以下欄位只有下注的玩家才有值
  string bet_id = 3;           // 下注 ID（無下注時為空）
  string bet_color = 4;        // 下注顏色（無下注時為空）
  int64 bet_amount = 5;        // 下注金額（無下注時為 0）
  int64 win_amount = 6;        // 贏得金額（無下注或輸了時為 0）
  bool is_winner = 7;          // 是否贏家（無下注時為 false）
}
```

**特點**：
- 有下注的玩家會收到完整的欄位
- 無下注的玩家只會看到 `round_id` 和 `winning_color`

### 3. 更新的 CommandType

```protobuf
enum ColorGameCommandType {
  COMMAND_TYPE_UNSPECIFIED = 0;
  
  // Client -> Server (REQ)
  COMMAND_TYPE_PLACE_BET_REQ = 1;
  COMMAND_TYPE_GET_STATE_REQ = 2;
  
  // Server -> Client (RSP)
  COMMAND_TYPE_PLACE_BET_RSP = 3;
  COMMAND_TYPE_GET_STATE_RSP = 4;
  
  // Server -> Client (BRC - Broadcast)
  COMMAND_TYPE_COLOR_GAME_STATE_BRC = 5;
  COMMAND_TYPE_RESULT_BRC = 6;
  COMMAND_TYPE_SETTLEMENT_BRC = 7;
}
```

## 更新的文件

### Proto 定義
- `shared/proto/colorgame/colorgame.proto`

### 自動生成的代碼
- `shared/proto/colorgame/colorgame.pb.go`
- `shared/proto/colorgame/colorgame_grpc.pb.go`

### 業務代碼（批量替換）
- `internal/modules/color_game/**/*.go` - 所有 GMS 和 GS 相關代碼
- `internal/modules/gateway/**/*.go` - Gateway 相關代碼
- `pkg/service/color_game/*.go` - Service 接口
- `cmd/**/*.go` - 所有命令行工具
- `tests/**/*.go` - 所有測試文件

## 驗證結果

✅ 所有測試通過：
```
ok  github.com/frankieli/game_product/tests/integration/color_game  1.299s
ok  github.com/frankieli/game_product/tests/integration/gateway     0.415s
```

## 文檔更新

### 新增文檔
- `.agent/proto_design_guidelines.md` - Proto 設計規範
- `.agent/proto_migration_guide.md` - 遷移指南
- `.agent/websocket_protocol.md` - WebSocket 協議規範

### 待更新文檔
- [ ] `docs/cmd/color_game.md` - 需要更新 message 名稱
- [ ] `docs/websocket_protocol.md` - 需要更新範例

## 規範總結

### 命名規則
1. **所有 message 使用 `ColorGame` 前綴**
2. **Request 使用 `Req` 後綴**
3. **Response 使用 `Rsp` 後綴**
4. **Broadcast 使用 `BRC` 後綴**

### 設計原則
1. **Message 即 Body** - proto message 對應 WebSocket 的 `data` 欄位
2. **統一錯誤處理** - 所有 RSP 必須包含 `error_code`（int32）和 `error`（string）
3. **Command 定義** - 所有 command 必須在 `CommandType` enum 中定義

## 後續工作

1. [ ] 實現 `ColorGameSettlementBRC` 的發送邏輯
2. [ ] 更新 Gateway 的 `convertEvent` 處理 `ColorGameSettlementBRC`
3. [ ] 更新用戶文檔中的所有 message 名稱
4. [ ] 考慮添加更多 BRC 類型（如需要）

## 注意事項

⚠️ **破壞性變更**：這是一個破壞性變更，所有使用舊 message 名稱的代碼都需要更新。

✅ **向前兼容**：新的命名規範更清晰，避免了命名衝突，為未來擴展提供了更好的基礎。
