# ColorGame Proto 重構完成記錄

## 日期
2025-12-05

## 重構目標
統一所有 ColorGame 相關的 proto message 命名規範，使用 `ColorGame` 前綴，並實現完整的 WebSocket 協議規範。

---

## 完成的工作

### 1. Proto Message 重命名

所有 ColorGame 相關的 message 現在都使用 `ColorGame` 前綴：

#### Request/Response Messages
| 舊名稱 | 新名稱 | 用途 |
|--------|--------|------|
| `PlaceBetReq` | `ColorGamePlaceBetReq` | 下注請求 |
| `PlaceBetRsp` | `ColorGamePlaceBetRsp` | 下注回應 |
| `GetStateReq` | `ColorGameGetStateReq` | 獲取狀態請求 |
| `GetStateRsp` | `ColorGameGetStateRsp` | 獲取狀態回應 |
| `RecordBetReq` | `ColorGameRecordBetReq` | GMS 記錄下注請求 |
| `RecordBetRsp` | `ColorGameRecordBetRsp` | GMS 記錄下注回應 |
| `GetCurrentRoundReq` | `ColorGameGetCurrentRoundReq` | 獲取當前回合請求 |
| `GetCurrentRoundRsp` | `ColorGameGetCurrentRoundRsp` | 獲取當前回合回應 |

#### Broadcast Messages
| 舊名稱 | 新名稱 | 用途 |
|--------|--------|------|
| `ColorGameRoundStateBRC` | `ColorGameRoundStateBRC` | 遊戲狀態廣播（不變） |
| - | `ColorGameSettlementBRC` | 結算廣播（新增） |
| `GameEvent` | `ColorGameEvent` | 通用事件 |

#### Other Messages
| 舊名稱 | 新名稱 | 用途 |
|--------|--------|------|
| `PlayerBet` | `ColorGamePlayerBet` | 玩家下注信息 |
| `EventType` | `ColorGameEventType` | 事件類型枚舉 |

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
- 有下注的玩家收到完整信息
- 無下注的玩家收到簡化信息（bet_id 為空，金額為 0）

### 3. WebSocket Command 規範

#### CommandType Enum
```protobuf
enum CommandType {
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

#### 實際使用的 Command 字符串
- `ColorGamePlaceBetREQ` - 下注請求
- `ColorGamePlaceBetRSP` - 下注回應
- `ColorGameGetStateREQ` - 獲取狀態請求
- `ColorGameGetStateRSP` - 獲取狀態回應
- `ColorGameStateBRC` - 遊戲狀態廣播
- `ColorGameResultBRC` - 開獎結果廣播
- `ColorGameSettlementBRC` - 結算廣播

### 4. 錯誤處理規範

#### 統一使用 error_code (int32)
- ✅ 所有 RSP 消息必須包含 `error_code` 和 `error` 欄位
- ❌ 禁止使用 `success: true/false`
- ❌ 禁止使用字符串形式的 error_code

#### ErrorCode 對照表
```
0   = SUCCESS
5   = INTERNAL_ERROR
200 = INSUFFICIENT_BALANCE
301 = ROUND_NOT_ACTIVE
302 = INVALID_BET_AMOUNT
```

### 5. 代碼更新

#### 批量替換
使用 `sed` 命令批量更新了所有文件：
- `internal/modules/color_game/**/*.go`
- `internal/modules/gateway/**/*.go`
- `pkg/service/**/*.go`
- `cmd/**/*.go`
- `tests/**/*.go`

#### 關鍵文件更新
1. **GS UseCase** (`gs/usecase/gs_uc.go`)
   - 將結算通知從 `ColorGameEvent` 改為 `ColorGameSettlementBRC`
   - 移除 JSON 字符串拼接，使用結構化 proto message

2. **Gateway UseCase** (`gateway/usecase/gateway_uc.go`)
   - 更新 command 處理：`PlaceBetREQ` → `ColorGamePlaceBetREQ`
   - 添加完整的錯誤日誌（Error/Warn 級別）

3. **Gateway Handler** (`gateway/adapter/local/handler.go`)
   - 添加 `ColorGameSettlementBRC` 的轉換處理
   - 正確映射所有欄位到 JSON

4. **測試文件**
   - 更新所有測試使用新的 message 類型
   - 更新 command 名稱

### 6. 文檔更新

#### 用戶文檔
- `docs/cmd/color_game.md` - 更新所有 command 和範例
- `docs/shared/protobuf.md` - 添加 ColorGame 前綴規範和演化記錄
- `docs/websocket_protocol.md` - 完整的協議規範

#### AI 助手規範（.agent/）
- `proto_design_guidelines.md` - Proto 設計規範
- `proto_migration_guide.md` - 遷移指南
- `proto_refactoring_summary.md` - 重構總結
- `websocket_protocol.md` - WebSocket 協議規範
- `settlement_brc_implementation.md` - 結算廣播實現需求

---

## 命名規範總結

### 格式
```
ColorGame + 功能名 + REQ/RSP/BRC
```

### 範例
- `ColorGamePlaceBetREQ` = ColorGame + PlaceBet + REQ
- `ColorGamePlaceBetRSP` = ColorGame + PlaceBet + RSP
- `ColorGameSettlementBRC` = ColorGame + Settlement + BRC

### 規則
1. **所有 message 使用 `ColorGame` 前綴**
2. **Request 使用 `REQ` 後綴**
3. **Response 使用 `RSP` 後綴**
4. **Broadcast 使用 `BRC` 後綴**
5. **使用 PascalCase 命名**

---

## 測試驗證

### 測試結果
```bash
✅ ok  github.com/frankieli/game_product/tests/integration/color_game  1.077s
✅ ok  github.com/frankieli/game_product/tests/integration/gateway     0.650s
```

### 驗證的功能
- ✅ 下注請求/回應使用新的 command
- ✅ 結算廣播使用 `ColorGameSettlementBRC`
- ✅ 錯誤處理使用 `error_code` (int32)
- ✅ 所有 WebSocket 消息格式正確

---

## 重要提醒

### Message 即 Body
- Proto message 對應 WebSocket 的 `data` 欄位
- 不要在 message 中定義信封欄位（`game_code`, `command`）

### 向後兼容性
- 這是一個破壞性變更
- 所有使用舊 message 名稱的代碼都需要更新
- 客戶端也需要更新 command 名稱

### 日誌規範
所有錯誤都添加了日誌：
- `Error` - 客戶端錯誤或服務端內部錯誤
- `Warn` - 正常的業務錯誤（如餘額不足）

---

## 後續工作

### 已完成
- ✅ Proto 定義更新
- ✅ 代碼批量更新
- ✅ 測試更新
- ✅ 文檔更新
- ✅ 結算廣播實現

### 待實現（優先級 P2）
- [ ] 向所有在線玩家（包括沒下注的）廣播結算通知
- [ ] Gateway 維護房間玩家列表
- [ ] 實現去重邏輯（避免重複通知）

詳見：`.agent/settlement_brc_implementation.md`

---

## 相關文件

### Proto 定義
- `shared/proto/colorgame/colorgame.proto`
- `shared/proto/common/common.proto`

### 核心實現
- `internal/modules/color_game/gs/usecase/gs_uc.go`
- `internal/modules/gateway/usecase/gateway_uc.go`
- `internal/modules/gateway/adapter/local/handler.go`

### 文檔
- `docs/cmd/color_game.md`
- `docs/shared/protobuf.md`
- `docs/websocket_protocol.md`

### AI 助手規範
- `.agent/proto_design_guidelines.md`
- `.agent/websocket_protocol.md`
- `.agent/settlement_brc_implementation.md`
