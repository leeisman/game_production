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

# 服務將在 :8080 啟動
# WebSocket: ws://localhost:8080/ws?token=YOUR_TOKEN
# API: http://localhost:8080/api
```

### 3.3 Microservices 模式

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
