# Protocol Buffers 設計規範

## 核心設計原則

### 1. 消息命名規範

#### REQ/RSP 模式（請求-回應）
- **請求消息**：以 `Req` 結尾（例如：`PlaceBetReq`）
- **回應消息**：以 `Rsp` 結尾（例如：`PlaceBetRsp`）
- **配對使用**：每個 REQ 必須有對應的 RSP

範例：
```protobuf
message PlaceBetReq {
  int64 user_id = 1;
  string color = 2;
  int64 amount = 3;
}

message PlaceBetRsp {
  common.ErrorCode error_code = 1;
  string bet_id = 2;
  string error = 3;
}
```

#### BRC 模式（廣播）
- **廣播消息**：以 `BRC` 結尾（例如：`ColorGameRoundStateBRC`）
- **用途**：服務端主動推送給客戶端的消息
- **特點**：單向通訊，無需回應

範例：
```protobuf
message ColorGameRoundStateBRC {
  string round_id = 1;
  string state = 2;
  int64 betting_end_timestamp = 3;
  int64 left_time = 4;
}
```

### 2. Message 即 Body

- **定義的 message 就是 WebSocket 消息的 body（data 欄位）**
- Gateway 負責將 proto message 轉換為 JSON
- 不要在 message 中重複定義 `game_code`、`command` 等信封欄位

**錯誤示範**（❌）:
```protobuf
message PlaceBetReq {
  string game_code = 1;  // ❌ 不要定義信封欄位
  string command = 2;    // ❌ 不要定義信封欄位
  string color = 3;
  int64 amount = 4;
}
```

**正確示範**（✅）:
```protobuf
message PlaceBetReq {
  int64 user_id = 1;  // ✅ 只定義業務欄位
  string color = 2;
  int64 amount = 3;
}
```

### 3. 標準 RSP 結構

所有 RSP 消息必須包含：
1. `error_code` - 錯誤代碼（int32，來自 `common.ErrorCode`）
2. `error` - 錯誤訊息（string，成功時為空）
3. 業務數據欄位

範例：
```protobuf
message PlaceBetRsp {
  common.ErrorCode error_code = 1;  // 必須
  string bet_id = 2;                 // 業務數據
  string error = 3;                  // 必須
}
```

### 4. CommandType Enum

所有 WebSocket command 必須在 `CommandType` enum 中定義：

```protobuf
enum CommandType {
  COMMAND_TYPE_UNSPECIFIED = 0;
  
  // REQ (Client → Server)
  COMMAND_TYPE_PLACE_BET_REQ = 1;
  COMMAND_TYPE_GET_STATE_REQ = 2;
  
  // RSP (Server → Client)
  COMMAND_TYPE_PLACE_BET_RSP = 3;
  COMMAND_TYPE_GET_STATE_RSP = 4;
  
  // BRC (Server → Clients)
  COMMAND_TYPE_COLOR_GAME_STATE_BRC = 5;
  COMMAND_TYPE_RESULT = 6;
  COMMAND_TYPE_SETTLEMENT = 7;
}
```

命名規則：
- 格式：`COMMAND_TYPE_<功能名稱>_<REQ|RSP|BRC>`
- 使用 UPPER_SNAKE_CASE
- REQ/RSP/BRC 成對或成組定義

## 完整範例

### 1. 定義 Proto

```protobuf
// 在 colorgame.proto 中

// 1. 定義 Command Type
enum CommandType {
  COMMAND_TYPE_PLACE_BET_REQ = 1;
  COMMAND_TYPE_PLACE_BET_RSP = 2;
}

// 2. 定義 Request Message
message PlaceBetReq {
  int64 user_id = 1;
  string color = 2;
  int64 amount = 3;
}

// 3. 定義 Response Message
message PlaceBetRsp {
  common.ErrorCode error_code = 1;  // 必須
  string bet_id = 2;
  string error = 3;                  // 必須
}

// 4. 定義 Broadcast Message（如需要）
message BetConfirmationBRC {
  string round_id = 1;
  string bet_id = 2;
  int64 user_id = 3;
  string color = 4;
  int64 amount = 5;
}
```

### 2. WebSocket 消息格式

#### 客戶端請求
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
- `data` 對應 `PlaceBetReq` message（不包含 `user_id`，由 Gateway 從 token 提取）

#### 服務端回應
```json
{
  "game_code": "color_game",
  "command": "PlaceBetRSP",
  "data": {
    "error_code": 0,
    "bet_id": "bet_xxx",
    "error": ""
  }
}
```
- `data` 對應 `PlaceBetRsp` message

#### 服務端廣播
```json
{
  "game_code": "color_game",
  "command": "BetConfirmationBRC",
  "data": {
    "round_id": "20251205123456",
    "bet_id": "bet_xxx",
    "user_id": 1001,
    "color": "red",
    "amount": 100
  }
}
```
- `data` 對應 `BetConfirmationBRC` message

## 設計檢查清單

添加新功能時，請確認：

- [ ] 定義了 `<Feature>Req` message
- [ ] 定義了 `<Feature>Rsp` message
- [ ] `Rsp` 包含 `error_code` 和 `error` 欄位
- [ ] 在 `CommandType` enum 中添加了對應的 command
- [ ] 如需廣播，定義了 `<Feature>BRC` message
- [ ] Message 只包含業務欄位，不包含信封欄位
- [ ] 運行 `protoc` 重新生成代碼
- [ ] 在 Gateway 中實現對應的處理邏輯
- [ ] 更新文檔

## 常見錯誤

### ❌ 錯誤 1：在 message 中定義信封欄位
```protobuf
message PlaceBetReq {
  string game_code = 1;  // ❌ 錯誤
  string command = 2;    // ❌ 錯誤
  string color = 3;
}
```

### ❌ 錯誤 2：RSP 缺少標準欄位
```protobuf
message PlaceBetRsp {
  string bet_id = 1;  // ❌ 缺少 error_code 和 error
}
```

### ❌ 錯誤 3：使用字符串表示 error_code
```protobuf
message PlaceBetRsp {
  string error_code = 1;  // ❌ 應該使用 common.ErrorCode
}
```

### ✅ 正確示範
```protobuf
message PlaceBetReq {
  int64 user_id = 1;
  string color = 2;
  int64 amount = 3;
}

message PlaceBetRsp {
  common.ErrorCode error_code = 1;
  string bet_id = 2;
  string error = 3;
}
```

## 參考文件

- `shared/proto/common/common.proto` - 通用定義（ErrorCode）
- `shared/proto/colorgame/colorgame.proto` - 遊戲協議定義
- `.agent/websocket_protocol.md` - WebSocket 協議規範
