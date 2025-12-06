# Game Service (GS)

GS (Game Service) 負責處理玩家的業務請求（如下注）以及核心的遊戲結算邏輯。它是連接用戶、錢包與遊戲核心 (GMS) 的橋樑。

## 1. 核心邏輯

### 1.1 下注處理 (Betting Process)
當收到 `ColorGamePlaceBetREQ` 時，GS 執行以下檢查與操作：

1.  **狀態驗證**: 檢查當前遊戲回合是否處於 `GAME_STATE_BETTING` 狀態。
2.  **參數驗證**: 檢查下注金額是否有效，顏色選項是否正確。
3.  **下注累加機制**:
    *   同一個玩家在同一局中，對同一個區域（如 "red"）只能有一筆下注記錄。
    *   重複下注會自動累加金額，保持 `BetID` 不變。
4.  **扣款與記錄**: 調用 User Service 扣除餘額，並寫入 `bet_orders` 表。

### 1.2 結算流程 (Settlement Process)
GS 監聽 GMS 的 `GAME_STATE_RESULT` 事件來觸發結算流程。

#### 優化策略 (2025-12 更新)
1.  **分批處理**: 系統將下注訂單每 **500 筆** 為一個批次進行處理，以避免鎖表與內存溢出。
2.  **DB 寫入優先**: 確保結算結果持久化到數據庫後，才調用各種外部服務（如錢包派彩）。
3.  **條件通知**:
    *   只有在錢包派彩成功 (`Deposit`) 後，才會向贏家發送 `ColorGameSettlementBRC` 通知。
    *   輸家只會收到全局的開獎廣播，不會收到個人結算通知。

## 2. 數據模型與持久化
*   **Repository**: 重構後的 `db` 包提供了 `BetOrderRepository`。
*   **數據一致性**: 使用數據庫事務 (Transaction) 確保下注扣款與訂單創建的一致性。

## 3. 與 GMS 的交互 (Interaction with GMS)

GS並不直接依賴 GMS 的具體實作，而是通過 `pkg/service` 定義的介面進行交互。這確保了系統可以靈活地切換部署模式（單體或微服務）。

### 3.1 依賴介面
GS 依賴於 `pkg/service/color_game/gms.go` 中定義的 `GMSService` 介面：

```go
type GMSService interface {
    GetCurrentRound(ctx context.Context) (*domain.Round, error)
    RegisterEventHandler(handler func(event domain.GameEvent))
}
```

### 3.2 交互模式
1.  **狀態監聽 (Observer)**:
    *   GS 通過 `RegisterEventHandler` 註冊回調函數。
    *   在 **Monolith 模式** 下，這是直接的內存函數調用，效能極高。
    *   在 **Microservices 模式** 下，Adapter 會將 gRPC Stream 或 Redis Pub/Sub 消息轉換為此回調調用。

2.  **主動查詢**:
    *   當玩家下注時，GS 調用 `GetCurrentRound` 來驗證當前狀態是否允許下注 (`CanAcceptBet`)。

### 3.3 狀態響應邏輯
*   `GAME_STATE_BETTING`: 觸發 System Unblock，開放下注 API。
*   `GAME_STATE_DRAWING`: 觸發 System Block，拒絕新的下注請求。
*   `GAME_STATE_RESULT`: 獲取開獎結果，啟動結算流程 (Settlement)。
