# 日誌系統架構與策略 (Game Production)

本文檔說明 Game Production 系統的日誌架構、設計策略以及在高併發場景下的最佳實踐。

## 1. 核心架構

日誌系統基於 `zerolog` 構建，並針對高併發場景進行了深度優化。

### 主要組件

*   **`pkg/logger`**: 核心封裝庫，提供統一的 API。
*   **`SmartWriter`**: 智能緩衝寫入器，負責高效能且可靠的日誌寫入。
*   **`lumberjack`**: 負責日誌檔案的自動輪轉 (Rotation) 與壓縮。

---

## 2. 寫入策略 (Smart Buffered I/O)

為了同時滿足「高效能」、「順序一致性」與「資料可靠性」，我們採用了基於 `SmartWriter` 的緩衝寫入策略：

### 為什麼放棄之前的混合策略 (Hybrid Async)？
之前的策略將 Info/Error 設為同步，Debug 設為非同步。這雖然解決了阻塞問題，但導致了**日誌順序錯亂**（Info 可能比 Debug 先寫入），增加了除錯難度。

### SmartWriter 解決方案
`SmartWriter` 是一個智慧緩衝寫入器，其工作原理如下：

1.  **統一緩衝 (Unified Buffer)**：所有等級的日誌 (Info, Debug, Error) 都先寫入同一個 256KB 的記憶體 Buffer。這保證了**嚴格的順序一致性**，且寫入速度極快（不阻塞 Goroutine）。
2.  **智慧刷新 (Smart Flush)**：
    *   **立即刷新 (Immediate Flush)**：當寫入的日誌包含 `Error` 或 `Fatal` 等級時，Buffer 會**立即**寫入磁碟。這保證了關鍵錯誤**絕對不會丟失**。
    *   **定期刷新 (Periodic Flush)**：背景 Goroutine 每隔 **1 秒** 自動刷新 Buffer。這保證了 Info/Debug 日誌也能及時持久化。
    *   **滿額刷新 (Full Flush)**：當 Buffer 滿時自動刷新。
    *   **Panic Flush**：在程式 Panic 時，通過 `defer` 機制確保緩衝區內容被寫入。

這種設計類似於 Google 的 `glog`，完美平衡了效能與可靠性。

---

## 3. 檔案管理 (Log Rotation)

為了避免單一日誌檔案過大導致磁碟空間耗盡或難以管理，我們整合了 `lumberjack` 進行自動輪轉。

*   **Monolith**: `logs/color_game/monolith.log`
*   **Test Robot**: `logs/color_game/test_robot.log`
*   **切割規則**: 當檔案大小超過 **100 MB** 時自動切割。
*   **保留策略**:
    *   最多保留 **3 個** 舊檔案。
    *   舊檔案最多保留 **28 天**。
*   **壓縮**: 舊檔案會自動壓縮為 `.gz` 格式。

---

## 4. 效能優化總結

我們解決了高併發下 (10k+ Users) 常見的 "Connection Reset by Peer" 問題，主要歸功於以下優化：

1.  **Smart Buffered I/O**: 雖然我們同時輸出到 Console 與 File，但因為使用了 `SmartWriter` 進行緩衝，主 Goroutine 僅需將日誌寫入記憶體，因此**不會被 Console 的慢速 I/O 阻塞**。
2.  **Console 格式**: 為了開發方便，我們啟用了 Console 格式。雖然比 JSON 稍慢，但在 SmartWriter 的緩衝下，對效能影響微乎其微。
3.  **智慧刷新機制**: 
    *   利用記憶體 Buffer 吸收流量峰值。
    *   Error/Fatal 立即刷新，Info 定期刷新，確保資料安全與順序一致。
    *   Panic Flush 確保即使程式崩潰，關鍵錯誤也能被記錄。

---

## 5. 使用範例

### 初始化 (在 main.go)

```go
// cmd/color_game/monolith/main.go

func main() {
    // 初始化日誌：輸出到文件和控制台，使用 Console 格式
    logger.InitWithFile("logs/color_game/monolith.log", "info", "console")
    
    // 確保程式退出時寫入剩餘日誌
    defer logger.Flush()

    // ...
}
```

### 記錄日誌

```go
// Info (寫入 Buffer，1秒內刷新)
logger.Info(ctx).Int64("user_id", 123).Msg("User logged in")

// Error (立即刷新到磁碟，不遺失)
logger.Error(ctx).Err(err).Msg("Database connection failed")

// Debug (寫入 Buffer，若 Level="info" 則被忽略)
logger.Debug(ctx).Msg("Processing step 1...")
```

---

## 6. 最佳實踐 (Best Practices)

### ✅ DO

```go
// 始終傳遞 context
func ProcessOrder(ctx context.Context, orderID string) error {
    logger.Info(ctx).Str("order_id", orderID).Msg("Processing order")
}

// 使用結構化欄位
logger.Info(ctx).
    Str("user_name", name).
    Int("age", age).
    Msg("User registered")

// 記錄關鍵操作
logger.Info(ctx).Msg("Bet placed successfully")
logger.Error(ctx).Err(err).Msg("Database query failed")
```

### ❌ DON'T

```go
// 缺少 context
func ProcessOrder(orderID string) {
    log.Println("Processing order", orderID)  // ❌
}

// 字串拼接 (效能較差且不易解析)
logger.Info(ctx).Msgf("User %s age %d", name, age)  // ❌

// 過度日誌
logger.Debug(ctx).Msg("Step 1")  // 如果不是必要的調試訊息
logger.Debug(ctx).Msg("Step 2")
```

---

## 7. 總結

日誌系統已全面集成到 Game Production 專案，具備以下特點：
1.  **高效能**：基於 SmartWriter 的緩衝 I/O，解決了高併發下的阻塞問題。
2.  **可靠性**：Error/Fatal/Panic 即時落盤，確保關鍵數據不丟失。
3.  **易用性**：簡化的 `InitWithFile` 和標準化的 API。
4.  **可觀測性**：完整的 Request ID 鏈路追蹤。
