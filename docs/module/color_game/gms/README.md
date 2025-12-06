# Game Machine Service (GMS)

GMS (Game Machine Service) 負責驅動遊戲的核心狀態與週期 (Round Loop)。它不處理玩家的下注邏輯，而是專注於維護 "現在是什麼階段" 以及 "開獎結果是什麼"。

## 1. 遊戲狀態機 (Game State Machine)

### 1.1 狀態轉換流程

遊戲狀態機會自動循環執行以下階段：

```
GAME_STATE_ROUND_STARTED → GAME_STATE_BETTING → GAME_STATE_DRAWING → GAME_STATE_RESULT → GAME_STATE_ROUND_ENDED → (下一回合)
```

### 1.2 設計模式：觀察者模式 (Observer Pattern)

狀態機使用 **Observer Pattern** 來解耦核心邏輯與外部通知。

*   **機制**: `StateMachine` 維護一個 `EventHandler` 列表。
*   **註冊**: 外部模組 (如 `GMSUseCase`) 通過 `RegisterEventHandler` 註冊回調函數。
*   **通知 (`emitEvent`)**:
    *   當狀態發生變化時，`emitEvent` 會被調用。
    *   **非阻塞設計**: 系統會為每個註冊的 handler 啟動一個 **Goroutine** (`go handler(event)`) 進行異步通知。
    *   這確保了狀態機的計時循環不會因為外部處理（如寫入 DB 或網絡廣播）的延遲而被阻塞。

### 1.3 實現與併發模型 (Concurrency Model)

狀態機在 `runRound` 中使用 `time.Sleep` 來控制階段時長。這在 Go 語言中是 **高效且安全** 的設計：

*   **非阻塞 OS 線程**: Go 的 `time.Sleep` 僅會掛起當前 Goroutine (`G`)，並讓出底層 OS 線程 (`M`) 去執行其他任務（如下注請求）。
*   **Timer 機制**: Go Runtime 使用全局堆 (Heap) 管理 Timer，時間到後自動喚醒 Goroutine，開銷極低。
*   **Graceful Shutdown**: 當調用 `Stop()` 時，狀態機會等待當前階段 (`Sleep`) 結束後才檢查停止標誌，這確保了**回合的完整性**，不會在下注一半時突然中斷。

### 1.4 各階段時長配置

預設配置（可在 `StateMachine` 初始化時調整）：

| 階段 | 狀態 | 持續時間 | 說明 |
|------|------|----------|------|
| 1. 回合開始 | `GAME_STATE_ROUND_STARTED` | **2 秒** | 生成新的回合 ID，等待玩家準備 |
| 2. 下注階段 | `GAME_STATE_BETTING` | **10 秒** | 玩家可以下注，倒數計時顯示剩餘時間 |
| 3. 開獎階段 | `GAME_STATE_DRAWING` | **2 秒** | 停止下注，系統抽取結果 |
| 4. 結果公布 | `GAME_STATE_RESULT` | **5 秒** | 顯示開獎結果，觸發結算流程 |
| 5. 回合結束 | `GAME_STATE_ROUND_ENDED` | **3 秒** | 休息時間，準備下一回合 |

**總回合時長**: 約 **22 秒** (2 + 10 + 2 + 5 + 3)

### 1.5 狀態事件詳細說明

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

### 1.6 自定義時長配置

如需調整各階段時長，可在啟動時修改：

```go
stateMachine := gmsMachine.NewStateMachine()
stateMachine.WaitDuration = 3 * time.Second      // 回合開始等待 3 秒
stateMachine.BettingDuration = 30 * time.Second  // 下注 30 秒
stateMachine.DrawingDuration = 3 * time.Second   // 開獎 3 秒
stateMachine.ResultDuration = 10 * time.Second   // 結果顯示 10 秒
stateMachine.RestDuration = 5 * time.Second      // 休息 5 秒
```
