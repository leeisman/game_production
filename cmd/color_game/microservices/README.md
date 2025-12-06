# Color Game - Monolith vs Microservices

本目錄包含 Color Game 的**微服務架構**實作。如果您要運行**單體應用（Monolith）**，請使用 `cmd/color_game/monolith/main.go`。

## 架構選擇

### Monolith（單體應用）
- **路徑**: `cmd/color_game/monolith/main.go`
- **Gateway 端口**: **8081**
- **適用場景**: 開發、測試、小規模部署
- **優點**: 簡單、易於調試
- **啟動**: `go run ./cmd/color_game/monolith/main.go`

### Microservices（微服務）
- **路徑**: `cmd/color_game/microservices/`
- **Gateway 端口**: **8081** (與 Monolith 相同)
- **適用場景**: 生產環境、高可用部署
- **優點**: 可擴展、獨立部署
- **需要**: Nacos 服務發現

---

## 統一端口配置

為了簡化測試和開發，**Monolith 和 Microservices 使用相同的端口**：

### HTTP/WebSocket 服務

| 服務名稱 | 端口 | 環境變數 | 說明 |
|---------|-----|---------|------|
| **Gateway** (Monolith & Microservices) | **8081** | `GATEWAY_PORT` | WebSocket 網關 |
| **User Service HTTP** | **8082** | `AUTH_HTTP_PORT` | 用戶認證 HTTP API |

> ✅ **統一端口的好處**: test_robot 和其他測試工具無需區分 Monolith/Microservices 模式，直接連接 `localhost:8081`。

### gRPC 服務（僅 Microservices）

所有 gRPC 服務使用**隨機端口**並通過 Nacos 進行服務發現。

| 服務名稱 | Nacos 服務名 | 說明 |
|---------|-------------|------|
| **User Service (gRPC)** | `user-service` | 用戶認證 gRPC 服務 |
| **GMS** | `color-game-service` | 遊戲管理服務（Game Management Service） |
| **GS** | `color-game-service` | 遊戲服務（Game Service） |

> **注意**: GMS 和 GS 註冊為同一個服務名 `color-game-service`，但使用不同的 metadata 區分：
> - GMS: `{"type": "gms"}`
> - GS: `{"type": "gs"}`

### 管理工具

| 工具名稱 | 默認端口 | 說明 |
|---------|---------|------|
| **OPS Center** | 7090 | 運維管理工具，支持多環境/多遊戲切換 |

## 服務依賴

### 基礎設施

所有微服務依賴以下基礎設施：

- **Nacos**: `localhost:8848` (服務發現與配置中心)
- **PostgreSQL**: `localhost:5432` (數據庫)
- **Redis**: `localhost:6379` (緩存與消息隊列)

### 環境變數

#### 通用配置

```bash
# Nacos
export NACOS_HOST=localhost
export NACOS_PORT=8848
export NACOS_NAMESPACE=public

# Database
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=your_password
export DB_NAME=game_product

# Redis
export REDIS_HOST=localhost
export REDIS_PORT=6379
```

#### Gateway 配置（Monolith & Microservices 共用）

```bash
export GATEWAY_PORT=8081
```

#### User Service 配置

```bash
export AUTH_SERVER_PORT=50051  # gRPC 端口（實際會使用隨機端口）
export AUTH_HTTP_PORT=8082     # HTTP API 端口
export JWT_SECRET=your_jwt_secret
export JWT_DURATION=24h
```

## 啟動順序

### 方案 A: 啟動 Monolith（推薦用於開發）

```bash
# 1. 啟動基礎設施
docker-compose up -d postgres redis

# 2. 啟動 Monolith
go run ./cmd/color_game/monolith/main.go
```

### 方案 B: 啟動 Microservices（用於生產環境測試）

```bash
# 1. 啟動基礎設施（包含 Nacos）
docker-compose up -d nacos postgres redis

# 2. 啟動核心微服務
go run ./cmd/color_game/microservices/user/main.go
go run ./cmd/color_game/microservices/gms/main.go
# go run ./cmd/color_game/microservices/gs/main.go  # 目前需要修復

# 3. 啟動 Gateway
go run ./cmd/color_game/microservices/gateway/main.go
```

### 啟動 OPS 工具（可選）

```bash
go run ./cmd/ops/main.go
```

訪問: `http://localhost:7090`

## 測試工具

### Test Robot

測試機器人默認連接到 `localhost:8081`，自動適配 Monolith 或 Microservices：

```bash
# 啟動 100 個機器人
go run ./cmd/color_game/test_robot/main.go -count 100

# 自定義參數
go run ./cmd/color_game/test_robot/main.go \
  -host localhost:8081 \
  -count 500 \
  -log-level info
```

## 服務通信架構

### Monolith 架構

```
客戶端 (WebSocket)
    ↓
Gateway (8081) ← 所有邏輯在同一進程
    ├─ User Module
    ├─ GMS Module
    └─ GS Module
```

### Microservices 架構

```
客戶端 (WebSocket)
    ↓
Gateway (8081)
    ↓
    ├─→ User Service (gRPC) - 用戶驗證
    ├─→ GMS (gRPC) - 遊戲回合管理
    └─→ GS (gRPC) - 玩家下注與結算
```

## 服務發現

所有 gRPC 服務通過 Nacos 進行服務發現：

1. 服務啟動時使用**隨機端口**
2. 向 Nacos 註冊服務名和實際端口
3. 客戶端通過 Nacos 查詢服務地址
4. 支持負載均衡和故障轉移

## 開發與調試

### 查看服務狀態

使用 OPS Center 的 Service Discovery 頁面查看所有註冊的服務。

### 測試 gRPC 調用

使用 OPS Center 的 RPC Console 進行 gRPC 調用測試：

1. 選擇環境/遊戲
2. 選擇方法（如 `ValidateToken`, `GetBalance`）
3. 輸入 JSON payload
4. 執行並查看響應

### 端口衝突解決

如果遇到端口衝突：

1. **Gateway (8081)**: 通過 `GATEWAY_PORT` 環境變數修改
2. **User HTTP (8082)**: 通過 `AUTH_HTTP_PORT` 環境變數修改
3. **gRPC 服務**: 自動使用隨機端口，無需手動配置

## 故障排查

### 服務無法啟動

1. 檢查 Nacos 是否運行: `docker ps | grep nacos`
2. 檢查數據庫連接: `psql -h localhost -U postgres -d game_product`
3. 檢查 Redis 連接: `redis-cli ping`

### 服務無法發現（僅 Microservices）

1. 訪問 Nacos 控制台: `http://localhost:8848/nacos`
2. 檢查服務列表中是否有註冊的服務
3. 檢查 namespace 是否正確（默認 `public`）

### WebSocket 連接失敗

1. 確認 Gateway 運行在 8081 端口
2. 檢查 Token 是否有效
3. 查看 Gateway 日誌

## 未來擴展

### 添加新遊戲

1. 在 `cmd/ops/main.go` 的 `initProjects` 中添加新遊戲配置
2. 指定對應的 Nacos namespace
3. 重啟 OPS 工具

### 部署到生產環境

1. 修改環境變數指向生產環境的 Nacos/DB/Redis
2. 使用 Docker 容器化部署
3. 配置負載均衡器（如 Nginx）
4. 啟用 HTTPS/WSS

## 相關文檔

- [系統架構文檔](../../../.agent/system_architecture_20251206.md)
- [GMS/GS 實作總結](../../../.agent/gms_gs_microservices_summary.md)
- [Proto 定義](../../../shared/proto/)
