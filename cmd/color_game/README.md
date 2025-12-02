# Color Game 部署指南

## 目錄結構

```
cmd/color_game/
├── monolith/           # 單體模式（推薦用於開發和小規模部署）
│   └── main.go
└── microservices/      # 微服務模式（推薦用於生產環境）
    ├── gms/            # Game Machine Service (遊戲核心邏輯)
    │   └── main.go
    ├── gs/             # Game Service (玩家下注與結算邏輯 - 未來獨立)
    │   └── main.go
    └── gateway/        # Gateway Service (包含 GS 邏輯)
        └── main.go
```

## 最新功能更新 (2025-12)

### 1. 數據庫重構
- Repository 包名從 `postgres` 重構為 `db`，提供更好的抽象層，支持未來擴展。
- 引入 `BetOrderRepository` 用於結算記錄持久化。

### 2. 結算流程優化
為了確保數據一致性和用戶體驗，結算流程已優化為流式分批處理：
1. **收集下注**：獲取當前局的所有下注。
2. **分批處理**：每 500 筆下注為一個批次。
3. **DB 寫入優先**：每批次先寫入數據庫 (`bet_orders`)，確保數據持久化。
4. **錢包操作**：DB 寫入成功後，立即處理該批次的錢包派彩。
5. **條件通知**：
   - **贏家**：只有在錢包派彩成功後才會收到通知。
   - **輸家**：直接收到結算通知。
   - **失敗處理**：若錢包操作失敗，記錄詳細日誌但不誤導玩家（不發送贏錢通知）。

### 3. 下注累加機制
- **規則**：同一個玩家在同一局中，對同一個區域（如 "red"）只能有一筆下注記錄。
- **行為**：如果玩家再次對同一區域下注，系統會自動將新金額累加到現有的下注記錄中，保持 `BetID` 不變。
- **優點**：簡化結算邏輯，減少數據庫記錄數。

## 模式選擇

### Monolith 模式

**適用場景：**
- 開發環境
- 小規模部署（< 10,000 並發用戶）
- 快速原型驗證

**優點：**
- 部署簡單（單一進程）
- 無網路延遲
- 易於調試

**缺點：**
- 無法獨立擴展
- 單點故障

### Microservices 模式

**適用場景：**
- 生產環境
- 大規模部署（> 10,000 並發用戶）
- 需要高可用性

**優點：**
- 可獨立擴展（GMS 可以單獨擴展）
- 故障隔離
- 技術棧靈活

**缺點：**
- 部署複雜
- 網路延遲
- 需要服務發現

## 啟動方式

### 環境變量配置

確保配置以下環境變量（或使用默認值）：

```bash
# 數據庫配置
export COLORGAME_DB_HOST=localhost
export COLORGAME_DB_PORT=5432
export COLORGAME_DB_USER=postgres
export COLORGAME_DB_PASSWORD=postgres
export COLORGAME_DB_NAME=game_product

# Redis 配置
export REDIS_ADDR=localhost:6379
```

### Monolith 模式

```bash
# 1. 確保數據庫和 Redis 已啟動
# 2. 啟動服務
go run cmd/color_game/monolith/main.go

# 服務將在 :8080 啟動
# WebSocket: ws://localhost:8080/ws?token=YOUR_TOKEN
# API: http://localhost:8080/api
```

### Microservices 模式

```bash
# 1. 啟動 GMS
go run cmd/color_game/microservices/gms/main.go
# GMS gRPC server: localhost:50051

# 2. 啟動 Gateway（需要等 GMS 啟動完成）
go run cmd/color_game/microservices/gateway/main.go
# Gateway HTTP server: localhost:8080
```

## 事件廣播機制

### Monolith 模式

```
StateMachine → RoundUseCase → WebSocketBroadcaster → Gateway → 玩家
```

- 直接通過 Go channel 廣播
- 零網路延遲

### Microservices 模式

```
StateMachine → RoundUseCase → gRPC Handler → [網路] → Gateway gRPC Client → WebSocket → 玩家
```

- 通過 gRPC Server Streaming 推送事件
- Gateway 自動重連機制

## 遊戲事件類型

所有玩家會收到以下事件：

1. **round_started** - 新回合開始
2. **betting_started** - 投注階段開始
3. **drawing** - 開獎階段
4. **result** - 結果公布
5. **settlement** - 個人結算結果（私有事件）
6. **round_ended** - 回合結束

### 事件格式示例

**投注開始：**
```json
{
  "type": "betting_started",
  "round_id": "20231130120000",
  "data": "2023-11-30T12:00:10+08:00"
}
```

**結算結果（個人）：**
```json
{
  "type": "settlement",
  "round_id": "20231130120000",
  "data": "{\"round_id\":\"20231130120000\",\"winning_color\":\"red\",\"bet_amount\":100,\"win_amount\":200}"
}
```

## 監控建議

1. **Prometheus Metrics**
   - 在線玩家數
   - 每秒下注數
   - 結算批次處理時間
   - 事件廣播延遲

2. **日誌**
   - 結構化日誌（JSON）
   - 關注 `PlayerUseCase` 中的結算日誌 (`Settlement completed successfully`)

3. **追蹤**
   - OpenTelemetry
   - Jaeger

## 故障排查

### 常見問題

**問題：玩家贏了但沒收到通知**
- 檢查日誌中是否有 `Failed to deposit winnings`。
- 如果錢包操作失敗，系統會有意不發送通知以避免誤導。
- 檢查 `bet_orders` 表確認結算記錄是否已寫入。

**問題：Gateway 無法連接 GMS**
- 檢查 GMS 是否啟動：`grpcurl -plaintext localhost:50051 list`
- 檢查網路連接
- 查看 Gateway 日誌中的重連訊息

## 下一步

1. 添加服務發現（Consul/Nacos）
2. 添加配置中心
3. 添加健康檢查
4. 添加 Metrics 和 Tracing
5. 實現 GS 微服務（目前 GS 仍在 Gateway 內）
