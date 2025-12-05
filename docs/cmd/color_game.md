# Color Game Service Guide

本文檔詳細說明 Color Game 服務的架構、啟動方式以及運維指南。我們採用 **Modular Monolith** (模組化單體) 作為主要的開發與部署模式，同時保留了向微服務演進的能力。

## 1. 核心思維 (Core Philosophy)

### Modular Monolith First
在開發初期與中小型規模部署 (< 10k CCU) 時，我們優先推薦使用 **Monolith 模式**。

*   **開發效率**: 單一進程，無需處理複雜的 RPC 網路問題與服務發現。
*   **效能優勢**: 模組間調用 (e.g., Gateway -> Game) 是內存函數調用，零延遲。
*   **部署簡單**: 只有一個 Binary，運維成本極低。

### Microservices Ready
雖然是單體，但我們嚴格遵守 Clean Architecture 與模組邊界。當業務規模擴大時，可以輕鬆將 `GMS` (Game Machine Service) 或 `User` 服務拆分出去，只需切換 Adapter (Local -> gRPC)。

---

## 2. 目錄結構

```
cmd/color_game/
├── monolith/           # 單體模式 (推薦)
│   └── main.go         # 整合所有模組 (Gateway, User, ColorGame)
└── microservices/      # 微服務模式 (進階)
    ├── gms/            # Game Machine Service (遊戲核心邏輯)
    │   └── main.go
    ├── gateway/        # Gateway Service (接入層)
    │   └── main.go
    └── ...
```

---

## 3. 啟動指南 (Startup Guide)

### 3.1 環境變量配置
確保配置以下環境變量（或使用默認值）：

```bash
# 數據庫配置
export COLORGAME_DB_HOST=localhost
export COLORGAME_DB_PORT=5432
export COLORGAME_DB_USER=postgres
export COLORGAME_DB_PASSWORD=postgres
export COLORGAME_DB_NAME=game_product

# Redis 配置 (用於 Session 與 廣播)
export REDIS_ADDR=localhost:6379
```

### 3.2 Monolith 模式 (推薦)

```bash
# 1. 確保數據庫和 Redis 已啟動
# 2. 啟動服務
go run cmd/color_game/monolith/main.go

# 服務將在 :8081 啟動
# WebSocket: ws://localhost:8081/ws?token=YOUR_TOKEN
# API: http://localhost:8081/api
```

### 3.3 API 使用流程

#### 步驟 1: 註冊用戶

```bash
curl -X POST http://localhost:8081/api/users/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "password": "password123",
    "email": "player1@example.com"
  }'
```

**回應範例**:
```json
{
  "user_id": 1001,
  "success": true
}
```

#### 步驟 2: 登入獲取 Token

```bash
curl -X POST http://localhost:8081/api/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "password": "password123"
  }'
```

**回應範例**:
```json
{
  "user_id": 1001,
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### 步驟 3: 連接 WebSocket

使用獲取的 token 連接 WebSocket：

```javascript
// JavaScript 範例
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
const ws = new WebSocket(`ws://localhost:8081/ws?token=${token}`);

ws.onopen = () => {
  console.log("WebSocket 連接成功");
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log("收到訊息:", message);
  
  // 處理不同類型的訊息
  switch(message.command) {
    case "ColorGameRoundStateBRC":
      // 遊戲狀態更新
      console.log("遊戲狀態:", message.data.state);
      console.log("剩餘時間:", message.data.left_time, "秒");
      break;
    case "ColorGameSettlementBRC":
      // 結算通知
      console.log("結算:", message.data);
      if (message.data.is_winner) {
        console.log("恭喜！贏得:", message.data.win_amount);
      }
      break;
  }
};
```

#### 步驟 4: 下注

在收到 `GAME_STATE_BETTING` 狀態後，可以進行下注：

```javascript
// 下注請求
const betRequest = {
  game: "color_game",
  command: "ColorGamePlaceBetREQ",  // 使用 PascalCase
  data: {
    color: "red",      // 可選: red, green, blue, yellow
    amount: 100        // 下注金額
  }
};

ws.send(JSON.stringify(betRequest));
```

**下注回應**（立即收到）:

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

**下注失敗回應**:
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

**ErrorCode 對照表**:
- `0` = SUCCESS
- `5` = INTERNAL_ERROR
- `302` = INVALID_BET_AMOUNT
- `301` = ROUND_NOT_ACTIVE
- 完整列表見 `shared/proto/common/common.proto`

#### 步驟 5: 接收結算通知

當回合結束後，會收到結算通知：

**有下注的玩家會收到兩次通知**：

1. **個人結算通知**（包含下注詳情）:
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "REWARD_RED",
    "bet_id": "bet_20251205123456_1001_red",
    "bet_color": "REWARD_RED",
    "bet_amount": 100,
    "win_amount": 200,
    "is_winner": true
  }
}
```

2. **全局廣播**（所有玩家都收到）:
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "REWARD_RED",
    "bet_id": "",
    "bet_color": "REWARD_UNSPECIFIED",
    "bet_amount": 0,
    "win_amount": 0,
    "is_winner": false
  }
}
```

**無下注的玩家只收到全局廣播**。

**前端處理建議**：
```javascript
let hasReceivedPersonalSettlement = false;

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  if (message.command === "ColorGameSettlementBRC") {
    // 如果有 bet_id，這是個人結算通知
    if (message.data.bet_id) {
      hasReceivedPersonalSettlement = true;
      showPersonalResult(message.data);
    } 
    // 如果沒有 bet_id，這是全局廣播
    else if (!hasReceivedPersonalSettlement) {
      // 只有沒收到個人通知的玩家才處理全局廣播
      showWinningColor(message.data.winning_color);
    }
  }
};
```

**欄位說明**:
- `round_id`: 回合 ID
- `winning_color`: 開獎顏色 (Enum String: `REWARD_RED`, etc.)
- `bet_id`: 下注 ID（無下注時為空）
- `bet_color`: 下注顏色 (Enum String)
- `bet_amount`: 下注金額（無下注時為 0）
- `win_amount`: 贏得金額（無下注或輸了時為 0）
- `is_winner`: 是否贏家（無下注時為 false）

### 3.4 完整流程範例 (cURL + wscat)

```bash
# 1. 註冊並登入
TOKEN=$(curl -s -X POST http://localhost:8081/api/users/login \
  -H "Content-Type: application/json" \
  -d '{"username":"player1","password":"password123"}' \
  | jq -r '.token')

# 2. 使用 wscat 連接 WebSocket
npm install -g wscat
wscat -c "ws://localhost:8081/ws?token=$TOKEN"

# 3. 等待收到 GAME_STATE_BETTING 狀態

# 4. 發送下注請求
{"game_code":"color_game","command":"ColorGamePlaceBetREQ","data":{"color":"red","amount":100}}

# 5. 等待開獎和結算
```

### 3.5 Microservices 模式

```bash
# 1. 啟動 GMS (遊戲核心)
go run cmd/color_game/microservices/gms/main.go
# GMS gRPC server: localhost:50051

# 2. 啟動 Gateway (接入層，需等待 GMS 啟動)
go run cmd/color_game/microservices/gateway/main.go
# Gateway HTTP server: localhost:8080
```

---

## 4. 架構與數據流

### Monolith 模式數據流
```
Client <-> [Gateway Module] <-> [User Module]
                  ^
                  | (Local Call)
                  v
           [ColorGame Module]
```
*   所有模組在同一個 Process 內。
*   Gateway 直接調用 ColorGame 的 UseCase。
*   ColorGame 直接通過 Go Channel 廣播事件給 Gateway。

### Microservices 模式數據流
```
Client <-> [Gateway Service] <-> [User Service]
                  ^
                  | (gRPC)
                  v
           [GMS Service]
```
*   Gateway 通過 gRPC Client 調用 GMS。
*   GMS 通過 gRPC Stream 推送事件給 Gateway。

---

## 5. 運維與監控

### 5.1 日誌 (Logging)
*   **格式**: 生產環境請使用 JSON 格式 (`logger.InitWithFile(..., "json")`)。
*   **關鍵日誌**:
    *   `Settlement completed successfully`: 結算成功。
    *   `Failed to deposit winnings`: 派彩失敗 (需人工介入)。

### 5.2 故障排查 (Troubleshooting)

**問題：玩家贏了但沒收到通知**
*   檢查日誌中是否有 `Failed to deposit winnings`。
*   如果錢包操作失敗，系統會有意不發送通知以避免誤導。
*   檢查 `bet_orders` 表確認結算記錄是否已寫入。

**問題：Connection Reset by Peer**
*   檢查是否開啟了 `Debug` 日誌且輸出到 Console (請改用 Info + JSON)。
*   檢查 `ulimit -n` 是否足夠大。

---

## 6. 最新功能更新 (2025-12)

### 數據庫重構
*   Repository 包名從 `postgres` 重構為 `db`。
*   引入 `BetOrderRepository` 用於結算記錄持久化。

### 結算流程優化
1.  **分批處理**: 每 500 筆下注為一個批次。
2.  **DB 寫入優先**: 確保數據持久化後才進行錢包操作。
3.  **條件通知**: 只有錢包派彩成功後才會發送贏家通知。

### 下注累加機制
*   同一個玩家在同一局中，對同一個區域（如 "red"）只能有一筆下注記錄。
*   重複下注會自動累加金額，保持 `BetID` 不變。

---

## 7. 遊戲狀態機流程 (Game State Machine)

### 7.1 狀態轉換流程

遊戲狀態機會自動循環執行以下階段：

```
GAME_STATE_ROUND_STARTED → GAME_STATE_BETTING → GAME_STATE_DRAWING → GAME_STATE_RESULT → GAME_STATE_ROUND_ENDED → (下一回合)
```

### 7.2 各階段時長配置

預設配置（可在 `StateMachine` 初始化時調整）：

| 階段 | 狀態 | 持續時間 | 說明 |
|------|------|----------|------|
| 1. 回合開始 | `GAME_STATE_ROUND_STARTED` | **2 秒** | 生成新的回合 ID，等待玩家準備 |
| 2. 下注階段 | `GAME_STATE_BETTING` | **10 秒** | 玩家可以下注，倒數計時顯示剩餘時間 |
| 3. 開獎階段 | `GAME_STATE_DRAWING` | **2 秒** | 停止下注，系統抽取結果 |
| 4. 結果公布 | `GAME_STATE_RESULT` | **5 秒** | 顯示開獎結果，觸發結算流程 |
| 5. 回合結束 | `GAME_STATE_ROUND_ENDED` | **3 秒** | 休息時間，準備下一回合 |

**總回合時長**: 約 **22 秒** (2 + 10 + 2 + 5 + 3)

### 7.3 狀態事件詳細說明

#### 1. GAME_STATE_ROUND_STARTED
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "GAME_STATE_ROUND_STARTED",
        "left_time": 2,  // 等待 2 秒後開始下注
        "betting_end_timestamp": 0
    },
    "game_code": "color_game"
}
```
- **left_time**: 表示距離下注開始還有 2 秒
- **用途**: 前端可以顯示「準備中，2 秒後開始下注」

#### 2. GAME_STATE_BETTING
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "GAME_STATE_BETTING",
        "left_time": 10,  // 距離下注結束還有幾秒
        "betting_end_timestamp": 1733377991  // 下注結束的 Unix 時間戳
    },
    "game_code": "color_game"
}
```
- **left_time**: 下注階段剩餘時間
- **betting_end_timestamp**: 下注結束的絕對時間
- **用途**: 前端顯示倒數計時，玩家可以下注

#### 3. GAME_STATE_DRAWING
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "GAME_STATE_DRAWING",
        "left_time": 2,  // 開獎階段持續 2 秒
        "betting_end_timestamp": 1733377991
    },
    "game_code": "color_game"
}
```
- **用途**: 停止接受下注，顯示開獎動畫

#### 4. GAME_STATE_RESULT
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "GAME_STATE_RESULT",
        "left_time": 5,  // 結果顯示持續 5 秒
        "betting_end_timestamp": 1733377991
    },
    "game_code": "color_game"
}
```
- **用途**: 顯示開獎結果，觸發玩家結算（結算結果通過 `ColorGameSettlementBRC` 發送）

#### 5. GAME_STATE_ROUND_ENDED
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "GAME_STATE_ROUND_ENDED",
        "left_time": 3,  // 休息時間 3 秒
        "betting_end_timestamp": 1733377991
    },
    "game_code": "color_game"
}
```
- **用途**: 回合結束，準備下一回合

### 7.4 自定義時長配置

如需調整各階段時長，可在啟動時修改：

```go
stateMachine := gmsMachine.NewStateMachine()
stateMachine.WaitDuration = 3 * time.Second      // 回合開始等待 3 秒
stateMachine.BettingDuration = 30 * time.Second  // 下注 30 秒
stateMachine.DrawingDuration = 3 * time.Second   // 開獎 3 秒
stateMachine.ResultDuration = 10 * time.Second   // 結果顯示 10 秒
stateMachine.RestDuration = 5 * time.Second      // 休息 5 秒
```


本文檔詳細說明 Color Game 服務的架構、啟動方式以及運維指南。我們採用 **Modular Monolith** (模組化單體) 作為主要的開發與部署模式，同時保留了向微服務演進的能力。

## 1. 核心思維 (Core Philosophy)

### Modular Monolith First
在開發初期與中小型規模部署 (< 10k CCU) 時，我們優先推薦使用 **Monolith 模式**。

*   **開發效率**: 單一進程，無需處理複雜的 RPC 網路問題與服務發現。
*   **效能優勢**: 模組間調用 (e.g., Gateway -> Game) 是內存函數調用，零延遲。
*   **部署簡單**: 只有一個 Binary，運維成本極低。

### Microservices Ready
雖然是單體，但我們嚴格遵守 Clean Architecture 與模組邊界。當業務規模擴大時，可以輕鬆將 `GMS` (Game Machine Service) 或 `User` 服務拆分出去，只需切換 Adapter (Local -> gRPC)。

---

## 2. 目錄結構

```
cmd/color_game/
├── monolith/           # 單體模式 (推薦)
│   └── main.go         # 整合所有模組 (Gateway, User, ColorGame)
└── microservices/      # 微服務模式 (進階)
    ├── gms/            # Game Machine Service (遊戲核心邏輯)
    │   └── main.go
    ├── gateway/        # Gateway Service (接入層)
    │   └── main.go
    └── ...
```

---

## 3. 啟動指南 (Startup Guide)

### 3.1 環境變量配置
確保配置以下環境變量（或使用默認值）：

```bash
# 數據庫配置
export COLORGAME_DB_HOST=localhost
export COLORGAME_DB_PORT=5432
export COLORGAME_DB_USER=postgres
export COLORGAME_DB_PASSWORD=postgres
export COLORGAME_DB_NAME=game_product

# Redis 配置 (用於 Session 與 廣播)
export REDIS_ADDR=localhost:6379
```

### 3.2 Monolith 模式 (推薦)

```bash
# 1. 確保數據庫和 Redis 已啟動
# 2. 啟動服務
go run cmd/color_game/monolith/main.go

# 服務將在 :8081 啟動
# WebSocket: ws://localhost:8081/ws?token=YOUR_TOKEN
# API: http://localhost:8081/api
```

### 3.3 API 使用流程

#### 步驟 1: 註冊用戶

```bash
curl -X POST http://localhost:8081/api/users/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "password": "password123",
    "email": "player1@example.com"
  }'
```

**回應範例**:
```json
{
  "user_id": 1001,
  "success": true
}
```

#### 步驟 2: 登入獲取 Token

```bash
curl -X POST http://localhost:8081/api/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "player1",
    "password": "password123"
  }'
```

**回應範例**:
```json
{
  "user_id": 1001,
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### 步驟 3: 連接 WebSocket

使用獲取的 token 連接 WebSocket：

```javascript
// JavaScript 範例
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
const ws = new WebSocket(`ws://localhost:8081/ws?token=${token}`);

ws.onopen = () => {
  console.log("WebSocket 連接成功");
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log("收到訊息:", message);
  
  // 處理不同類型的訊息
  switch(message.command) {
    case "ColorGameRoundStateBRC":
      // 遊戲狀態更新
      console.log("遊戲狀態:", message.data.state);
      console.log("剩餘時間:", message.data.left_time, "秒");
      break;
    case "ColorGameResultBRC":
      // 開獎結果
      console.log("開獎結果:", message.data.winning_color);
      break;
    case "ColorGameSettlementBRC":
      // 結算通知
      console.log("結算:", message.data);
      if (message.data.is_winner) {
        console.log("恭喜！贏得:", message.data.win_amount);
      }
      break;
  }
};
```

#### 步驟 4: 下注

在收到 `BETTING_STARTED` 狀態後，可以進行下注：

```javascript
// 下注請求
const betRequest = {
  game: "color_game",
  command: "ColorGamePlaceBetREQ",  // 使用駝峰命名
  data: {
    color: "red",      // 可選: red, green, blue, yellow
    amount: 100        // 下注金額
  }
};

ws.send(JSON.stringify(betRequest));
```

**下注回應**（立即收到）:

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

**下注失敗回應**:
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

**ErrorCode 對照表**:
- `0` = SUCCESS
- `5` = INTERNAL_ERROR
- `302` = INVALID_BET_AMOUNT
- `301` = ROUND_NOT_ACTIVE
- 完整列表見 `shared/proto/common/common.proto`

#### 步驟 5: 接收結算通知

當回合結束後，會收到結算通知：

**有下注的玩家會收到兩次通知**：

1. **個人結算通知**（包含下注詳情）:
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "red",
    "bet_id": "bet_20251205123456_1001_red",
    "bet_color": "red",
    "bet_amount": 100,
    "win_amount": 200,
    "is_winner": true
  }
}
```

2. **全局廣播**（所有玩家都收到）:
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "red",
    "bet_id": "",
    "bet_color": "",
    "bet_amount": 0,
    "win_amount": 0,
    "is_winner": false
  }
}
```

**無下注的玩家只收到全局廣播**。

**前端處理建議**：
```javascript
let hasReceivedPersonalSettlement = false;

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  if (message.command === "ColorGameSettlementBRC") {
    // 如果有 bet_id，這是個人結算通知
    if (message.data.bet_id) {
      hasReceivedPersonalSettlement = true;
      showPersonalResult(message.data);
    } 
    // 如果沒有 bet_id，這是全局廣播
    else if (!hasReceivedPersonalSettlement) {
      // 只有沒收到個人通知的玩家才處理全局廣播
      showWinningColor(message.data.winning_color);
    }
  }
};
```

**欄位說明**:
- `round_id`: 回合 ID
- `winning_color`: 開獎顏色
- `bet_id`: 下注 ID（無下注時為空）
- `bet_color`: 下注顏色（無下注時為空）
- `bet_amount`: 下注金額（無下注時為 0）
- `win_amount`: 贏得金額（無下注或輸了時為 0）
- `is_winner`: 是否贏家（無下注時為 false）

### 3.4 完整流程範例 (cURL + wscat)

```bash
# 1. 註冊並登入
TOKEN=$(curl -s -X POST http://localhost:8081/api/users/login \
  -H "Content-Type: application/json" \
  -d '{"username":"player1","password":"password123"}' \
  | jq -r '.token')

# 2. 使用 wscat 連接 WebSocket
npm install -g wscat
wscat -c "ws://localhost:8081/ws?token=$TOKEN"

# 3. 等待收到 BETTING_STARTED 狀態

# 4. 發送下注請求
{"game":"color_game","command":"ColorGamePlaceBetREQ","data":{"color":"red","amount":100}}

# 5. 等待開獎和結算
```

### 3.5 Microservices 模式

```bash
# 1. 啟動 GMS (遊戲核心)
go run cmd/color_game/microservices/gms/main.go
# GMS gRPC server: localhost:50051

# 2. 啟動 Gateway (接入層，需等待 GMS 啟動)
go run cmd/color_game/microservices/gateway/main.go
# Gateway HTTP server: localhost:8080
```

---

## 4. 架構與數據流

### Monolith 模式數據流
```
Client <-> [Gateway Module] <-> [User Module]
                  ^
                  | (Local Call)
                  v
           [ColorGame Module]
```
*   所有模組在同一個 Process 內。
*   Gateway 直接調用 ColorGame 的 UseCase。
*   ColorGame 直接通過 Go Channel 廣播事件給 Gateway。

### Microservices 模式數據流
```
Client <-> [Gateway Service] <-> [User Service]
                  ^
                  | (gRPC)
                  v
           [GMS Service]
```
*   Gateway 通過 gRPC Client 調用 GMS。
*   GMS 通過 gRPC Stream 推送事件給 Gateway。

---

## 5. 運維與監控

### 5.1 日誌 (Logging)
*   **格式**: 生產環境請使用 JSON 格式 (`logger.InitWithFile(..., "json")`)。
*   **關鍵日誌**:
    *   `Settlement completed successfully`: 結算成功。
    *   `Failed to deposit winnings`: 派彩失敗 (需人工介入)。

### 5.2 故障排查 (Troubleshooting)

**問題：玩家贏了但沒收到通知**
*   檢查日誌中是否有 `Failed to deposit winnings`。
*   如果錢包操作失敗，系統會有意不發送通知以避免誤導。
*   檢查 `bet_orders` 表確認結算記錄是否已寫入。

**問題：Connection Reset by Peer**
*   檢查是否開啟了 `Debug` 日誌且輸出到 Console (請改用 Info + JSON)。
*   檢查 `ulimit -n` 是否足夠大。

---

## 6. 最新功能更新 (2025-12)

### 數據庫重構
*   Repository 包名從 `postgres` 重構為 `db`。
*   引入 `BetOrderRepository` 用於結算記錄持久化。

### 結算流程優化
1.  **分批處理**: 每 500 筆下注為一個批次。
2.  **DB 寫入優先**: 確保數據持久化後才進行錢包操作。
3.  **條件通知**: 只有錢包派彩成功後才會發送贏家通知。

### 下注累加機制
*   同一個玩家在同一局中，對同一個區域（如 "red"）只能有一筆下注記錄。
*   重複下注會自動累加金額，保持 `BetID` 不變。

---

## 7. 遊戲狀態機流程 (Game State Machine)

### 7.1 狀態轉換流程

遊戲狀態機會自動循環執行以下階段：

```
ROUND_STARTED → BETTING_STARTED → DRAWING → RESULT → ROUND_ENDED → (下一回合)
```

### 7.2 各階段時長配置

預設配置（可在 `StateMachine` 初始化時調整）：

| 階段 | 狀態 | 持續時間 | 說明 |
|------|------|----------|------|
| 1. 回合開始 | `ROUND_STARTED` | **2 秒** | 生成新的回合 ID，等待玩家準備 |
| 2. 下注階段 | `BETTING_STARTED` | **10 秒** | 玩家可以下注，倒數計時顯示剩餘時間 |
| 3. 開獎階段 | `DRAWING` | **2 秒** | 停止下注，系統抽取結果 |
| 4. 結果公布 | `RESULT` | **5 秒** | 顯示開獎結果，觸發結算流程 |
| 5. 回合結束 | `ROUND_ENDED` | **3 秒** | 休息時間，準備下一回合 |

**總回合時長**: 約 **22 秒** (2 + 10 + 2 + 5 + 3)

### 7.3 狀態事件詳細說明

#### 1. ROUND_STARTED
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "EVENT_TYPE_ROUND_STARTED",
        "left_time": 2,  // 等待 2 秒後開始下注
        "betting_end_timestamp": 0
    },
    "game_code": "color_game"
}
```
- **left_time**: 表示距離下注開始還有 2 秒
- **用途**: 前端可以顯示「準備中，2 秒後開始下注」

#### 2. BETTING_STARTED
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "EVENT_TYPE_BETTING_STARTED",
        "left_time": 10,  // 距離下注結束還有幾秒
        "betting_end_timestamp": 1733377991  // 下注結束的 Unix 時間戳
    },
    "game_code": "color_game"
}
```
- **left_time**: 下注階段剩餘時間
- **betting_end_timestamp**: 下注結束的絕對時間
- **用途**: 前端顯示倒數計時，玩家可以下注

#### 3. DRAWING
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "EVENT_TYPE_DRAWING",
        "left_time": 2,  // 開獎階段持續 2 秒
        "betting_end_timestamp": 1733377991
    },
    "game_code": "color_game"
}
```
- **用途**: 停止接受下注，顯示開獎動畫

#### 4. RESULT
```json
{
    "command": "ColorGameResultBRC",
    "data": {
        "round_id": "20251205123456",
        "winning_color": "red",  // 開獎結果
        "left_time": 5,  // 結果顯示持續 5 秒
        "timestamp": 1733377993
    },
    "game_code": "color_game"
}
```
- **用途**: 顯示開獎結果，觸發玩家結算

#### 5. ROUND_ENDED
```json
{
    "command": "ColorGameRoundStateBRC",
    "data": {
        "round_id": "20251205123456",
        "state": "EVENT_TYPE_ROUND_ENDED",
        "left_time": 3,  // 休息時間 3 秒
        "betting_end_timestamp": 1733377991
    },
    "game_code": "color_game"
}
```
- **用途**: 回合結束，準備下一回合

### 7.4 自定義時長配置

如需調整各階段時長，可在啟動時修改：

```go
stateMachine := gmsMachine.NewStateMachine()
stateMachine.WaitDuration = 3 * time.Second      // 回合開始等待 3 秒
stateMachine.BettingDuration = 30 * time.Second  // 下注 30 秒
stateMachine.DrawingDuration = 3 * time.Second   // 開獎 3 秒
stateMachine.ResultDuration = 10 * time.Second   // 結果顯示 10 秒
stateMachine.RestDuration = 5 * time.Second      // 休息 5 秒
```
