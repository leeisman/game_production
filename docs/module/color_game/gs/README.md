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

GS 與 GMS 的交互採用 **雙向服務依賴 (Bidirectional Service Dependency)** 模式，而非單純的觀察者註冊。這種設計確保了介面的強型別與 RPC 的兼容性。

### 3.1 依賴介面定義
交互涉及兩個核心介面 (Defined in `pkg/service/color_game`):

1.  **GMS 服務介面** (GS 依賴 GMS):
    ```go
    type GMSService interface {
        GetCurrentRound(ctx context.Context, ...) (*pb.ColorGameGetCurrentRoundRsp, error)
    }
    ```
2.  **GS 服務介面** (GMS 依賴 GS):
    ```go
    type ColorGameGSService interface {
        RoundResult(ctx context.Context, ...) (*pb.ColorGameRoundResultRsp, error)
    }
    ```

### 3.2 交互流程詳解

#### 1. 下注驗證 (On-Demand Query)
GS 是 **無狀態 (Stateless)** 的，它不維護當前遊戲階段的副本。
*   當收到 `PlaceBet` 請求時，GS 會實時調用 `GMSService.GetCurrentRound`。
*   由 GMS 判斷當前是否處於 `GAME_STATE_BETTING` 以及剩餘時間是否足夠。
*   這種設計避免了分佈式系統中的狀態同步問題。

#### 2. 結算通知 (Result Push)
*   當 GMS 進入 `GAME_STATE_RESULT` 階段時，會主動調用 `ColorGameGSService.RoundResult`。
*   GS 收到此 RPC/Method Call 後，觸發內部的批次結算流程 (Settlement Process)。
*   這取代了傳統的事件訂閱模式，讓結算邏輯成為一個明確的服務入口 (Service Endpoint)。
