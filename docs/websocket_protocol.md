# WebSocket 協議規範

## 設計原則

### 1. Command 命名規範
- **所有 command 類型必須在 `colorgame.proto` 的 `CommandType` enum 中定義**
- 使用 **PascalCase** 命名
- 命名後綴規範：
  - `REQ` - 客戶端請求（Client → Server）
  - `RSP` - 服務端回應（Server → Client）
  - `BRC` - 服務端廣播（Server → All Clients or Specific User）

### 2. Data 內容規範
- **所有 `data` 欄位的內容必須對應 proto 中定義的 message**
- Gateway 負責將 proto message 轉換為 JSON 格式發送給客戶端
- 客戶端發送的 JSON 會被 Gateway 轉換為對應的 proto message

## 消息格式

### 標準信封格式
```json
{
  "game_code": "color_game",
  "command": "CommandString",
  "data": { /* proto message 的 JSON 表示 */ }
}
```

## Command 類型定義

### 客戶端 → 服務端（REQ）

#### ColorGamePlaceBetREQ
**Proto 定義**: `ColorGamePlaceBetReq`
```protobuf
message ColorGamePlaceBetReq {
  int64 user_id = 1;
  ColorGameReward color = 2;
  int64 amount = 3;
}
```

**JSON 範例**:
```json
{
  "game": "color_game",
  "command": "ColorGamePlaceBetREQ",
  "data": {
    "color": "red",
    "amount": 100
  }
}
```
*註：`color` 欄位輸入為字串（如 "red"），服務端會自動映射到 `ColorGameReward` 枚舉。*

#### ColorGameGetStateREQ
**Proto 定義**: `ColorGameGetStateReq`
```protobuf
message ColorGameGetStateReq {
  int64 user_id = 1;
}
```

**JSON 範例**:
```json
{
  "game": "color_game",
  "command": "ColorGameGetStateREQ",
  "data": {}
}
```

---

### 服務端 → 客戶端（RSP）

#### ColorGamePlaceBetRSP
**Proto 定義**: `ColorGamePlaceBetRsp`
```protobuf
message ColorGamePlaceBetRsp {
  common.ErrorCode error_code = 1;
  string bet_id = 2;
  string error = 3;
}
```

**成功回應**:
```json
{
  "game_code": "color_game",
  "command": "ColorGamePlaceBetRSP",
  "data": {
    "error_code": 0,
    "bet_id": "bet_20251205123456_1001_red",
    "error": ""
  }
}
```

**失敗回應**:
```json
{
  "game_code": "color_game",
  "command": "ColorGamePlaceBetRSP",
  "data": {
    "error_code": 5,
    "bet_id": "",
    "error": "下注時間已結束"
  }
}
```

#### ColorGameGetStateRSP
**Proto 定義**: `ColorGameGetStateRsp`
```protobuf
message ColorGameGetStateRsp {
  common.ErrorCode error_code = 1;
  bytes state_json = 2;
}
```

**JSON 範例**:
```json
{
  "game_code": "color_game",
  "command": "ColorGameGetStateRSP",
  "data": {
    "round_id": "20251205123456",
    "state": "GAME_STATE_BETTING",
    "betting_end_timestamp": 1733377991,
    "left_time": 10,
    "player_bets": [
      {"color": "REWARD_RED", "amount": 100}
    ]
  }
}
```

---

### 服務端 → 客戶端（BRC - 廣播）

#### ColorGameRoundStateBRC
**Proto 定義**: `ColorGameRoundStateBRC`
```protobuf
message ColorGameRoundStateBRC {
  string round_id = 1;
  ColorGameState state = 2;
  int64 betting_end_timestamp = 3;
  int64 left_time = 4;
}
```

**JSON 範例**:
```json
{
  "game_code": "color_game",
  "command": "ColorGameRoundStateBRC",
  "data": {
    "round_id": "20251205123456",
    "state": "GAME_STATE_BETTING",
    "betting_end_timestamp": 1733377991,
    "left_time": 10
  }
}
```

#### ColorGameSettlementBRC
**Proto 定義**: `ColorGameSettlementBRC`
```protobuf
message ColorGameSettlementBRC {
  string round_id = 1;
  ColorGameReward winning_color = 2;
  string bet_id = 3;
  ColorGameReward bet_color = 4;
  int64 bet_amount = 5;
  int64 win_amount = 6;
  bool is_winner = 7;
}
```

**JSON 範例**:
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "REWARD_RED",
    "bet_id": "bet_123",
    "bet_color": "REWARD_RED",
    "bet_amount": 100,
    "win_amount": 200,
    "is_winner": true
  }
}
```
*註：`winning_color` 和 `bet_color` 返回枚舉的字符串名稱（如 `REWARD_RED`）。*

#### ColorGameBetConfirmation
**Proto 定義**: `ColorGameBetConfirmation`
```protobuf
message ColorGameBetConfirmation {
  string round_id = 1;
  string bet_id = 2;
  int64 user_id = 3;
  ColorGameReward color = 4;
  int64 amount = 5;
}
```
*（此消息主要用於服務端廣播下注確認，可配置為單發給用戶或廣播給所有人）*

## 實現規範

### Gateway 職責
1. **接收客戶端 JSON**：解析 `command` 欄位，根據字串匹配（如 `ColorGamePlaceBetREQ`）路由到對應的處理邏輯
2. **Command 映射**：Gateway 內部維護 Command String 到業務邏輯的映射。

### 添加新 Command 的步驟
1. 在 `colorgame.proto` 的 `CommandType` enum 中添加新的 command 類型
2. 定義對應的 proto message
3. 運行 `protoc` 重新生成 Go 代碼
4. 在 Gateway 的 `handleColorGame` 中添加對應的 case 處理
5. 在 Gateway 的 `convertEvent` 中添加對應的轉換邏輯（如果是廣播消息）
6. 更新本文檔

## 注意事項

1. **Command 命名**：請使用 `ColorGame` 前綴，保持全局唯一性。
2. **枚舉處理**：輸入時支持簡化字符串（如 "red"），輸出時通常為枚舉全名（如 "REWARD_RED"）或整數值，視具體實現而定（目前統一為枚舉名稱字符串）。


## 設計原則

### 1. Command 命名規範
- **所有 command 類型必須在 `colorgame.proto` 的 `CommandType` enum 中定義**
- 使用 **PascalCase** 命名（例如：`PlaceBetREQ`、`ColorGameRoundStateBRC`）
- 命名後綴規範：
  - `REQ` - 客戶端請求（Client → Server）
  - `RSP` - 服務端回應（Server → Client）
  - `BRC` - 服務端廣播（Server → All Clients or Specific User）

### 2. Data 內容規範
- **所有 `data` 欄位的內容必須對應 proto 中定義的 message**
- Gateway 負責將 proto message 轉換為 JSON 格式發送給客戶端
- 客戶端發送的 JSON 會被 Gateway 轉換為對應的 proto message

## 消息格式

### 標準信封格式
```json
{
  "game_code": "color_game",
  "command": "CommandType",
  "data": { /* proto message 的 JSON 表示 */ }
}
```

## Command 類型定義

### 客戶端 → 服務端（REQ）

#### PlaceBetREQ
**Proto 定義**: `PlaceBetReq`
```protobuf
message PlaceBetReq {
  int64 user_id = 1;
  string color = 2;
  int64 amount = 3;
}
```

**JSON 範例**:
```json
{
  "game": "color_game",
  "command": "PlaceBetREQ",
  "data": {
    "color": "red",
    "amount": 100
  }
}
```

#### GetStateREQ
**Proto 定義**: `GetStateReq`
```protobuf
message GetStateReq {
  int64 user_id = 1;
}
```

**JSON 範例**:
```json
{
  "game": "color_game",
  "command": "GetStateREQ",
  "data": {}
}
```

---

### 服務端 → 客戶端（RSP）

#### PlaceBetRSP
**Proto 定義**: `PlaceBetRsp`
```protobuf
message PlaceBetRsp {
  common.ErrorCode error_code = 1;
  string bet_id = 2;
  string error = 3;
}
```

**成功回應**:
```json
{
  "game_code": "color_game",
  "command": "PlaceBetRSP",
  "data": {
    "error_code": 0,
    "bet_id": "bet_20251205123456_1001_red",
    "error": ""
  }
}
```

**失敗回應**:
```json
{
  "game_code": "color_game",
  "command": "PlaceBetRSP",
  "data": {
    "error_code": 5,
    "bet_id": "",
    "error": "下注時間已結束"
  }
}
```

**ErrorCode 對照表**（定義在 `shared/proto/common/common.proto`）:

| Code | Name | 說明 |
|------|------|------|
| 0 | SUCCESS | 成功 |
| 1 | UNKNOWN_ERROR | 未知錯誤 |
| 2 | INVALID_PARAMS | 參數錯誤 |
| 3 | UNAUTHORIZED | 未授權 |
| 4 | NOT_FOUND | 未找到 |
| 5 | INTERNAL_ERROR | 內部錯誤 |
| 100 | INVALID_CREDENTIALS | 憑證無效 |
| 101 | TOKEN_EXPIRED | Token 過期 |
| 200 | INSUFFICIENT_BALANCE | 餘額不足 |
| 201 | TRANSACTION_FAILED | 交易失敗 |
| 300 | GAME_CLOSED | 遊戲關閉 |
| 301 | ROUND_NOT_ACTIVE | 回合未激活 |
| 302 | INVALID_BET_AMOUNT | 下注金額無效 |
| 303 | INVALID_BET_OPTION | 下注選項無效 |

#### GetStateRSP
**Proto 定義**: `GetStateRsp`
```protobuf
message GetStateRsp {
  common.ErrorCode error_code = 1;
  bytes state_json = 2;
}
```

**JSON 範例**:
```json
{
  "game_code": "color_game",
  "command": "GetStateRSP",
  "data": {
    "round_id": "20251205123456",
    "state": "BETTING",
    "betting_end_timestamp": 1733377991,
    "left_time": 10,
    "player_bets": [
      {"color": "REWARD_RED", "amount": 100}
    ]
  }
}
```

---

### 服務端 → 客戶端（BRC - 廣播）

#### ColorGameRoundStateBRC
**Proto 定義**: `ColorGameRoundStateBRC`
```protobuf
message ColorGameRoundStateBRC {
  string round_id = 1;
  ColorGameState state = 2;
  int64 betting_end_timestamp = 3;
  int64 left_time = 4;
}
```

**JSON 範例**:
```json
{
  "game_code": "color_game",
  "command": "ColorGameRoundStateBRC",
  "data": {
    "round_id": "20251205123456",
    "state": "GAME_STATE_BETTING",
    "betting_end_timestamp": 1733377991,
    "left_time": 10
  }
}
```

#### ColorGameSettlementBRC
**Proto 定義**: `ColorGameSettlementBRC`
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

**JSON 範例**:
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "REWARD_RED",
    "bet_id": "bet_123",
    "bet_color": "REWARD_RED",
    "bet_amount": 100,
    "win_amount": 200,
    "is_winner": true
  }
}
```

## 實現規範

### Gateway 職責
1. **接收客戶端 JSON**：解析 `command` 欄位，根據 `CommandType` 路由到對應的處理邏輯
2. **JSON → Proto 轉換**：將客戶端發送的 JSON `data` 轉換為對應的 proto message
3. **Proto → JSON 轉換**：將服務端返回的 proto message 轉換為 JSON 發送給客戶端
4. **Command 映射**：維護 `CommandType` enum 值與字符串的映射關係

### 添加新 Command 的步驟
1. 在 `colorgame.proto` 的 `CommandType` enum 中添加新的 command 類型
2. 定義對應的 proto message（如果需要新的數據結構）
3. 運行 `protoc` 重新生成 Go 代碼
4. 在 Gateway 的 `handleColorGame` 中添加對應的 case 處理
5. 在 Gateway 的 `convertEvent` 中添加對應的轉換邏輯（如果是廣播消息）
6. 更新本文檔

## 範例：完整下注流程

### 1. 客戶端發送下注請求
```json
{
  "game": "color_game",
  "command": "PlaceBetREQ",
  "data": {
    "color": "red",
    "amount": 100
  }
}
```

### 2. Gateway 處理
- 解析 `command: "PlaceBetREQ"`
- 將 `data` 轉換為 `PlaceBetReq` proto message
- 調用 `ColorGameService.PlaceBet()`

### 3. 服務端回應
```json
{
  "game_code": "color_game",
  "command": "PlaceBetRSP",
  "data": {
    "success": true,
    "bet_id": "bet_20251205123456_1001_red",
    "color": "red",
    "amount": 100
  }
}
```

## 注意事項

1. **不要硬編碼 command 字符串**：所有 command 必須來自 `CommandType` enum
2. **保持 Proto 與 JSON 的一致性**：JSON 欄位名稱應與 proto 欄位名稱保持一致（使用 snake_case 或 camelCase）
3. **版本兼容性**：添加新欄位時使用新的 field number，不要修改已有欄位的編號
4. **錯誤處理**：所有回應都應包含錯誤處理機制（`error_code` 或 `success` 欄位）
