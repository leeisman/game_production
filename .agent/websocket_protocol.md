# WebSocket 協議規範

## 核心原則

### 1. Command 定義規範
- **所有 command 必須在 `shared/proto/colorgame/colorgame.proto` 的 `CommandType` enum 中定義**
- 使用 **PascalCase** 命名（例如：`PlaceBetREQ`、`ColorGameStateBRC`）
- 命名後綴：
  - `REQ` - 客戶端請求（Client → Server）
  - `RSP` - 服務端回應（Server → Client）
  - `BRC` - 服務端廣播（Server → Clients）

### 2. Data 內容規範
- **所有 `data` 欄位的內容必須對應 proto 中定義的 message**
- Gateway 負責 proto message ↔ JSON 的雙向轉換
- 禁止在代碼中硬編碼 JSON 結構，必須基於 proto 定義

### 3. 錯誤處理規範
- **統一使用 `error_code` 欄位表示狀態**
- `error_code` 是 **int32 數字**，對應 `common.proto` 的 `ErrorCode` enum
- 常見值：
  - `0` = SUCCESS
  - `5` = INTERNAL_ERROR
  - `301` = ROUND_NOT_ACTIVE
  - `302` = INVALID_BET_AMOUNT
- **禁止使用 `success: true/false` 欄位**
- **禁止使用字符串形式的 error_code**（例如 `"SUCCESS"`）

## ErrorCode 對照表

完整定義見 `shared/proto/common/common.proto`：

| Code | Name | 說明 |
|------|------|------|
| 0 | SUCCESS | 成功 |
| 5 | INTERNAL_ERROR | 內部錯誤 |
| 200 | INSUFFICIENT_BALANCE | 餘額不足 |
| 301 | ROUND_NOT_ACTIVE | 回合未激活 |
| 302 | INVALID_BET_AMOUNT | 下注金額無效 |

## 標準消息格式

```json
{
  "game_code": "color_game",
  "command": "CommandType",
  "data": {
    "error_code": 0,  // 所有 RSP 必須包含（int32 數字）
    // ... 其他 proto message 欄位
  }
}
```

## Command 類型映射表

| Command | Proto Message | 方向 | 說明 |
|---------|---------------|------|------|
| `PlaceBetREQ` | `PlaceBetReq` | Client → Server | 下注請求 |
| `PlaceBetRSP` | `PlaceBetRsp` | Server → Client | 下注回應 |
| `GetStateREQ` | `GetStateReq` | Client → Server | 獲取狀態請求 |
| `GetStateRSP` | `GetStateRsp` | Server → Client | 獲取狀態回應 |
| `ColorGameStateBRC` | `ColorGameRoundStateBRC` | Server → Clients | 遊戲狀態廣播 |
| `result` | `GameEvent` | Server → Clients | 開獎結果 |
| `settlement` | 待定義 | Server → Client | 結算通知 |

## 添加新 Command 的流程

1. **定義 Proto**
   ```protobuf
   // 在 colorgame.proto 中
   enum CommandType {
     COMMAND_TYPE_NEW_FEATURE_REQ = 10;
     COMMAND_TYPE_NEW_FEATURE_RSP = 11;
   }
   
   message NewFeatureReq {
     int64 user_id = 1;
     string param = 2;
   }
   
   message NewFeatureRsp {
     common.ErrorCode error_code = 1;
     string result = 2;
     string error = 3;
   }
   ```

2. **生成代碼**
   ```bash
   protoc --go_out=. --go_opt=paths=source_relative shared/proto/colorgame/colorgame.proto
   ```

3. **實現 Gateway 處理**
   - 在 `gateway/usecase/gateway_uc.go` 的 `handleColorGame` 中添加 case
   - 如果是廣播，在 `gateway/adapter/local/handler.go` 的 `convertEvent` 中添加轉換邏輯

4. **更新文檔**
   - 更新本文檔
   - 更新 `docs/cmd/color_game.md`

## 範例：完整下注流程

### 請求
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

### 成功回應
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

### 失敗回應
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

## 重要提醒

1. ❌ **禁止**：硬編碼 command 字符串
2. ❌ **禁止**：使用 `success: true/false`
3. ❌ **禁止**：使用字符串形式的 `error_code`（例如 `"SUCCESS"`）
4. ❌ **禁止**：在 JSON 中添加 proto 未定義的欄位
5. ✅ **必須**：所有 command 來自 `CommandType` enum
6. ✅ **必須**：所有 RSP 包含 `error_code` 欄位（int32 數字）
7. ✅ **必須**：data 結構對應 proto message
