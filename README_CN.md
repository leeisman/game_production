# 遊戲生產平台 (Game Production Platform)

[![English](https://img.shields.io/badge/Language-English-blue.svg)](./README.md)

> 一個用於構建可擴展實時多人遊戲的生產級 Go 語言框架。

本專案是一個用於構建高併發遊戲後端的 **參考架構 (Reference Architecture)**。我們使用 **"顏色遊戲 (Color Game)"**（一款快節奏的多人下注遊戲）作為具體的實作範例，展示如何處理狀態同步、原子結算和廣播風暴等現實世界的挑戰。

## 🌟 願景與目標 (Vision & Goal)

我們的目標是為遊戲開發者提供一個「身經百戰 (Battle-Tested)」的基礎，開箱即用地解決常見的基礎設施問題，讓您可以專注於遊戲玩法 (Gameplay) 的開發。

*   **生產就緒 (Production Ready)**: 不僅僅是一個玩具專案。包含優雅關機、結構化日誌、指標鉤子 (Metrics Hooks) 和 Docker 構建。
*   **架構優先 (Architecture First)**: 在簡單性 (Monolith) 與可擴展性 (Microservices) 之間取得平衡。
*   **實時演示 (Live Demo Case)**: "顏色遊戲" 模組展示了一個下注遊戲的完整生命週期（回合 -> 下注 -> 結算）。

### 1. 模組化單體架構 (Modular Monolith Architecture)
*   **整潔架構 (Clean Architecture)**: 嚴格遵循領域驅動設計 (DDD)，分層清晰 (Domain, Usecase, Adapter, Repository)。
*   **靈活部署 (Flexible Deployment)**: 同一套代碼支持 **Monolith** (單體) 與 **Microservices** (微服務) 兩種部署模式。
    *   **Monolith**: 適合開發與中小型部署（零 RPC 開銷，運維簡單）。
    *   **Microservices**: 適合大規模擴展，模組間通過 gRPC 通訊。

### 2. 協議驅動開發 (Proto-Driven Development)
*   **單一真理來源 (Single Source of Truth)**: 所有的 API、事件、數據結構均由 `shared/proto` 定義。
*   **類型安全 (Type Safety)**: 自動生成 Go 代碼，保證前後端與服務間通訊的類型安全。
*   **標準化協議 (Standardized Protocol)**: 統一的 WebSocket 信封格式 (`game_code`, `command`, `data`) 與錯誤碼規範。

### 3. 高效能網關 (High Performance Gateway)
*   **10k+ 同時在線用戶**: 經壓測驗證，單節點可支撐 10,000+ 同時在線玩家。
*   **快速失敗廣播 (Fail-Fast Broadcast)**: 廣播機制採用快速失敗策略，防止慢連接阻塞整體系統。
*   **優化 I/O**: 使用 `epoll` (via net/http) 與優化的 WebSocket 讀寫泵。

### 4. 智能集中式日誌 (Smart Centralized Logging)
*   **SmartWriter**: 自研日誌緩衝機制。
    *   **Normal**: 異步寫入，減少 I/O 阻塞。
    *   **Panic/Error**: 同步立即寫入，確保關鍵日誌不丟失。
*   **零分配 (Zero Allocation)**: 基於 Zerolog 封裝，極致的性能表現。

### 5. 穩健的遊戲核心 (Robust Game Core)
*   **狀態機 (State Machine)**: 使用嚴謹的狀態機管理遊戲流程 (開始 -> 下注 -> 開獎 -> 結果)。
*   **原子結算 (Atomic Settlement)**: 支持批量結算與事務處理，確保錢包扣款與派彩的原子性。
*   **重連安全 (Reconnection Safe)**: 玩家重連後可立即獲取當前完整狀態。

---

## 📚 文檔 (Documentation)

詳細文檔位於 `docs/` 目錄：

*   **架構**: [設計原則](docs/shared/design_principles.md) | [專案演進](docs/ai/project_evolution.md)
*   **模組**: [網關模組 (Gateway)](docs/module/gateway/README.md) | [用戶模組 (User)](docs/module/user/README.md)
*   **指南**: [服務啟動指南](docs/cmd/color_game.md) | [WebSocket 協議](docs/websocket_protocol.md)
*   **效能**: [基準測試報告](docs/performance/color_game_benchmark.md)

---

## 🚀 效能亮點 (Performance Highlights)

我們已在本地開發機器上成功對系統進行了 **12,500+ 同時在線用戶 (CCU)** 的基準測試。

*   **優化歷程**:
    1.  **登入**: 通過預熱策略解決了 bcrypt CPU 峰值問題。
    2.  **連接**: 調優了操作系統限制 (`ulimit`, 臨時端口)。
    3.  **日誌**: 實作了 `SmartWriter` 以消除控制台 I/O 阻塞，這是導致 4500 用戶時效能瓶頸的主要原因。

詳情請參閱 [基準測試報告](docs/performance/color_game_benchmark.md)。

---

## 🏗 專案結構 (Project Structure)

```
game_product/
├── cmd/
│   └── color_game/
│       ├── monolith/           # 單體啟動入口 (推薦)
│       └── microservices/      # 微服務啟動入口
├── internal/
│   └── modules/
│       ├── color_game/         # 遊戲業務模組 (GMS, GS)
│       ├── gateway/            # WebSocket 網關
│       └── user/               # 用戶與認證模組
├── pkg/                        # 公共基礎庫 (Logger, Service interfaces)
├── shared/
│   └── proto/                  # Protobuf 定義 (API 契約)
└── docs/                       # 項目文檔
```

---

## 📡 協議快照 (Protocol Snapshot)

平台使用標準化的 WebSocket JSON 協議：

**客戶端請求 (下注):**
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

**伺服器廣播 (遊戲狀態):**
```json
{
  "game_code": "color_game",
  "command": "ColorGameRoundStateBRC",
  "data": {
    "round_id": "20231204120000",
    "state": "GAME_STATE_BETTING",
    "left_time": 10
  }
}
```

---

## 🚀 快速開始 (Getting Started)

### 先決條件
*   Go 1.24+
*   PostgreSQL
*   Redis (可選但推薦)

### 運行單體模式 (Monolith)

```bash
# 啟動服務
go run cmd/color_game/monolith/main.go
```

服務將在端口 `8081` 啟動。

### 運行微服務模式 (Microservices) (新!)

我們現在支持完整的微服務部署，包含 Nacos 服務發現與 gRPC 通訊。

查看 [微服務部署指南](docs/cmd/color_game/microservices/readme.md) 了解詳細設置步驟。

```bash
# 1. 啟動基礎設施 (Nacos, Redis, Postgres)
docker-compose up -d

# 2. 啟動各服務 (在不同終端中)
go run cmd/color_game/microservices/gateway/main.go
go run cmd/color_game/microservices/gms/main.go
go run cmd/color_game/microservices/gs/main.go
```

### 🛠 OPS 運維控制台 (OPS Console)

使用內建的 OPS 工具輕鬆調試 gRPC 微服務：

```bash
go run cmd/ops/main.go
# 瀏覽器打開 http://localhost:7090
```
- **測試 (Tests)**: 手動觸發廣播。
- **監控 (Inspect)**: 查看服務狀態與路由表。

---

## 📝 授權 (License)

專有軟體 (Proprietary)
