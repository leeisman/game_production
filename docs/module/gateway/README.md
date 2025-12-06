# WebSocket 協議規範

## 1. 設計原則

### Command 命名規範
- **所有 command 類型必須在 `colorgame.proto` 中定義**
- 使用 **PascalCase** 命名
- 命名後綴規範：
  - `REQ` - 客戶端請求（Client → Server）
  - `RSP` - 服務端回應（Server → Client）
  - `BRC` - 服務端廣播（Server → All Clients or Specific User）

### Data 內容規範
- **所有 `data` 欄位的內容必須對應 proto 中定義的 message**
- Gateway 負責將 proto message 轉換為 JSON 格式發送給客戶端
- 客戶端發送的 JSON 會被 Gateway 轉換為對應的 proto message

---

## 2. 消息格式 (Message Envelope)

標準信封格式：

```json
{
  "game_code": "color_game",
  "command": "CommandString",
  "data": { /* proto message 的 JSON 表示 */ }
}
```

---

## 3. Command 類型定義

### 客戶端 → 服務端（REQ）

#### ColorGamePlaceBetREQ
**Proto 定義**: `ColorGamePlaceBetReq`

```json
{
  "game_code": "color_game",
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

```json
{
  "game_code": "color_game",
  "command": "ColorGameGetStateREQ",
  "data": {}
}
```

### 服務端 → 客戶端（RSP）

#### ColorGamePlaceBetRSP
**Proto 定義**: `ColorGamePlaceBetRsp`

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

```json
{
  "game_code": "color_game",
  "command": "ColorGameGetStateRSP",
  "data": {
    "error_code": 0,
    "round_id": "20251205123456",
    "state": "GAME_STATE_BETTING",
    "betting_end_timestamp": 1733377991,
    "left_time": 10
  }
}
```
*(注意: state_json 字段在某些實作中可能會被展開為具體字段，具體視 Gateway 邏輯而定)*

### 服務端廣播（BRC）

#### ColorGameRoundStateBRC
**Proto 定義**: `ColorGameRoundStateBRC`

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
(包含結算結果)

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

---

## 4. 錯誤代碼 (Error Codes)

定義於 `shared/proto/common/common.proto`：

| Code | Name | 說明 |
|------|------|------|
| 0 | SUCCESS | 成功 |
| 1 | UNKNOWN_ERROR | 未知錯誤 |
| 2 | INVALID_PARAMS | 參數錯誤 |
| 3 | UNAUTHORIZED | 未授權 |
| 5 | INTERNAL_ERROR | 內部錯誤 |
| 100 | INVALID_CREDENTIALS | 憑證無效 |
| 101 | TOKEN_EXPIRED | Token 過期 |
| 200 | INSUFFICIENT_BALANCE | 餘額不足 |
| 301 | ROUND_NOT_ACTIVE | 回合未激活 |
| 302 | INVALID_BET_AMOUNT | 下注金額無效 |
| 303 | INVALID_BET_OPTION | 下注選項無效 |

---

## 5. 實現規範

### Gateway 職責
1. **接收客戶端 JSON**：解析 `command` 欄位，根據字符串匹配路由到對應的處理邏輯。
2. **Proto 轉換**：負責 JSON 與 Proto Message 之間的雙向轉換。
3. **Command 映射**：Gateway 內部維護 Command String 到業務邏輯的映射。

### 添加新 Command 的步驟
1. 在 `colorgame.proto` 中定義新的 message。
2. 運行 `protoc` (或 `make proto`) 重新生成 Go 代碼。
3. 在 Gateway 的 `handleColorGame` 中添加對應的 case 處理。
4. 如果是廣播消息，在 Gateway 的 `convertEvent` 中添加轉換邏輯。
5. 更新本文檔。

## 6. 注意事項

1. **命名一致性**：Command 必須保持全局唯一性，建議使用 `GamePrefix` + `Action` + `Type` (e.g. `ColorGamePlaceBetREQ`)。
2. **枚舉處理**：Protocol Buffers 的枚舉在 JSON 中通常表現為字符串名稱 (e.g. "REWARD_RED")。
3. **版本兼容性**：修改 Proto 時請遵守向前兼容原則 (新增字段，不修改舊字段編號)。

