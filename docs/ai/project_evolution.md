# Project Evolution & AI Context

這份文件專為 AI 助手設計，旨在記錄 **Game Production** 專案的演化歷史、設計哲學、安全性準則以及開發規範。未來的 AI 在接手任務前，應優先閱讀此文件以理解專案脈絡。

---

## 1. 專案演化史 (Evolution History)

### Phase 1: Monolith Foundation (單體基礎)
*   **目標**: 建立 Color Game 的核心邏輯與單體架構。
*   **成果**: 
    *   實現了 `color_game` 業務邏輯 (Betting, State Machine)。
    *   採用 Clean Architecture (Domain/UseCase/Adapter)。
    *   使用 `gin` 作為 HTTP 框架，`gorm` 作為 ORM。

### Phase 2: Gateway & WebSocket (通訊層)
*   **目標**: 實現即時通訊與連接管理。
*   **變更**:
    *   引入 `gateway` 模組，負責 WebSocket 連接。
    *   設計了 `Hub-Client` 模式的 WebSocket Manager。
    *   **關鍵決策**: 採用 `Header + Body` 的 JSON 格式標準化所有 WebSocket 消息。

### Phase 3: Microservices Preparation (微服務準備)
*   **目標**: 準備將單體拆分為微服務。
*   **變更**:
    *   引入 `protobuf` 定義服務介面 (User, Game, Wallet)。
    *   重構 `User` 和 `Gateway` 模組，使其同時支持 Local Adapter (單體) 和 gRPC Adapter (微服務)。
    *   **關鍵決策**: 簡化 Protobuf 命名 (Request -> Req, Response -> Rsp)。

### Phase 4: High Concurrency Optimization (高併發優化)
*   **目標**: 解決 10k+ 用戶壓測下的效能瓶頸。
*   **變更**:
    *   **日誌系統重構**: 從標準 log 遷移到 `zerolog` + `SmartWriter`。
    *   **Smart Buffered I/O**: 引入記憶體緩衝寫入，解決 Console I/O 阻塞問題。
    *   **Reliability**: 實作 Panic Flush 和 Graceful Shutdown Flush。

---

## 2. 設計哲學 (Design Philosophy)

### 2.1 Architecture
*   **Clean Architecture**: 嚴格遵守 `Domain` -> `UseCase` -> `Adapter` 的依賴方向。業務邏輯 (UseCase) 不應依賴外部框架 (HTTP, WebSocket, DB)。
*   **Modular Monolith**: 目前以單體形式運行，但模組間邊界清晰，隨時可拆分為微服務。

### 2.2 Performance
*   **Zero Allocation**: 在熱點路徑 (Hot Path) 盡量避免記憶體分配 (e.g., 使用 `zerolog`, 避免字串拼接)。
*   **Non-Blocking I/O**: 關鍵路徑 (如 WebSocket 廣播) 必須是非阻塞的。如果客戶端慢，直接斷開 (Fail-Fast)。

### 2.3 Reliability
*   **Fail-Fast**: 遇到無法恢復的錯誤 (如 Buffer Full)，立即失敗而不是拖慢系統。
*   **Observability**: 每個請求必須有 `request_id`，並貫穿整個調用鏈 (Context Propagation)。

---

## 3. 安全性準則 (Security Guidelines)

*   **Input Validation**: 所有外部輸入 (HTTP, WebSocket) 必須在邊界層 (Adapter) 進行驗證。
*   **Token Authentication**: WebSocket 連接必須在握手階段驗證 Token，無效則拒絕連接。
*   **Rate Limiting**: (未來實作) 針對 API 和 WebSocket 消息進行限流。
*   **State Validation**: 遊戲操作 (如下注) 必須驗證當前狀態 (State Machine)，非 Betting 狀態拒絕下注。

---

## 4. 開發規範 (Development Standards)

### 4.1 Logging
*   **Package**: 使用 `pkg/logger`。
*   **Initialization**: 使用 `logger.InitWithFile`。
*   **Context**: 始終傳遞 `context.Context` 以記錄 `request_id`。
*   **Format**: 開發環境用 `console`，生產環境用 `json`。
*   **Flush**: `main` 函數必須 `defer logger.Flush()`。

### 4.2 Error Handling
*   **Wrap Errors**: 使用 `fmt.Errorf("...: %w", err)` 包裝錯誤，保留原始堆疊。
*   **Log at Boundary**: 錯誤通常在最外層 (Adapter/Handler) 記錄，避免重複日誌。

### 4.3 Testing
*   **Unit Test**: 針對 UseCase 和 Domain Logic。
*   **Integration Test**: 針對 Database 和 External Service。
*   **Test Robot**: 使用 `cmd/color_game/test_robot` 進行端對端壓測。

---

## 5. 當前系統狀態 (Current State)

*   **架構**: Modular Monolith (Gateway, User, ColorGame, GMS, Wallet)。
*   **通訊**: WebSocket (Client <-> Gateway), gRPC (Internal, 準備中)。
*   **日誌**: Zerolog + SmartWriter (Async Buffered)。
*   **協議**: JSON (Header + Body)。

---

**給未來的 AI**:
當你接手此專案時，請務必維持上述的架構一致性。不要引入破壞 Clean Architecture 的依賴，並始終關注高併發下的效能與穩定性。
