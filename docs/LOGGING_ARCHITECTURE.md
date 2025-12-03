# 日志系統架構與策略

本文檔說明 Color Game 專案的日誌系統架構、設計策略以及在高併發場景下的最佳實踐。

## 1. 核心架構

日誌系統基於 `zerolog` 構建，並針對高併發場景進行了深度優化。

### 主要組件

*   **`pkg/logger`**: 核心封裝庫，提供統一的 API。
*   **`AsyncWriter`**: 非同步寫入器，使用 Buffer 緩衝日誌，避免阻塞主程式。
*   **`LevelAsyncWriter`**: 智慧分級寫入器，根據日誌等級決定同步或非同步。
*   **`lumberjack`**: 負責日誌檔案的自動輪轉 (Rotation) 與壓縮。

---

## 2. 寫入策略 (LevelAsyncWriter)

為了在「效能」與「資料安全性」之間取得平衡，我們採用了混合寫入策略：

| 日誌等級 | 寫入模式 | 行為描述 | 適用場景 |
| :--- | :--- | :--- | :--- |
| **Debug** | **非同步 (Async)** | 寫入 Buffer。若 Buffer 滿 (10,000 條)，則**丟棄**新日誌。 | 開發除錯、詳細流程追蹤。高併發下允許遺失以保護系統。 |
| **Info** | **同步 (Sync)** | 直接寫入檔案 (os.File)。**保證不遺失**。 | 關鍵業務流程 (如：用戶登入、下注成功、遊戲結算)。 |
| **Warn** | **同步 (Sync)** | 直接寫入檔案。**保證不遺失**。 | 潛在問題、非預期但可恢復的錯誤。 |
| **Error** | **同步 (Sync)** | 直接寫入檔案。**保證不遺失**。 | 系統錯誤、資料庫失敗、API 呼叫失敗。 |
| **Fatal** | **同步 (Sync)** | 直接寫入檔案並退出程式。 | 嚴重錯誤導致系統無法繼續運行。 |

> **注意**：目前的 Monolith 設定已將全域 Level 設為 `Info`，因此 `Debug` 日誌會被直接忽略，不產生任何 I/O 開銷。

---

## 3. 檔案管理 (Log Rotation)

為了避免單一日誌檔案過大導致磁碟空間耗盡或難以管理，我們整合了 `lumberjack` 進行自動輪轉。

*   **檔案路徑**: `logs/color_game/monolith.log`
*   **切割規則**: 當檔案大小超過 **100 MB** 時自動切割。
*   **保留策略**:
    *   最多保留 **3 個** 舊檔案。
    *   舊檔案最多保留 **28 天**。
*   **壓縮**: 舊檔案會自動壓縮為 `.gz` 格式。

---

## 4. 效能優化總結

我們解決了高併發下 (4500+ Users) 常見的 "Connection Reset by Peer" 問題，主要歸功於以下優化：

1.  **移除 Console 輸出**: 終端機輸出是同步且極慢的，改為寫入檔案大幅提升了 I/O 吞吐量。
2.  **JSON 格式**: 機器可讀且序列化速度快，避免了 Console 美化輸出的 CPU 消耗。
3.  **智慧同步/非同步**: 
    *   關鍵日誌 (Info+) 走同步通道，確保數據完整性。
    *   非關鍵日誌 (Debug) 走非同步通道 (或直接忽略)，防止 I/O 阻塞 Goroutine。

## 5. 使用範例

### 初始化 (在 main.go)

```go
// 設定 Log Rotation
logFile := &lumberjack.Logger{
    Filename:   "logs/monolith.log",
    MaxSize:    100, // MB
    MaxBackups: 3,
    Compress:   true,
}

// 初始化 Logger
logger.Init(logger.Config{
    Level:  "info",    // 只記錄 Info 以上
    Format: "json",    // 使用 JSON 格式
    Output: logFile,   // 寫入檔案
    Async:  true,      // 啟用智慧分級寫入 (LevelAsyncWriter)
})
```

### 記錄日誌

```go
// Info (同步寫入，不遺失)
logger.Info(ctx).Int64("user_id", 123).Msg("User logged in")

// Error (同步寫入，不遺失)
logger.Error(ctx).Err(err).Msg("Database connection failed")

// Debug (非同步寫入，若 Level="info" 則被忽略)
logger.Debug(ctx).Msg("Processing step 1...")
```
