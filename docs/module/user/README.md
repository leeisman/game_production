# User Module Design & Implementation

本文檔說明 User 模組的設計架構、安全機制以及在高併發場景下的保護策略。

## 1. 核心架構

User 模組負責處理用戶的註冊、登入、Token 驗證以及 Session 管理。

### 組件職責

*   **UseCase (`UserUseCase`)**: 核心業務邏輯，包括密碼雜湊、JWT 生成與驗證。
*   **Repository (`UserRepository`)**: 資料庫存取，負責 User 資料的 CRUD。
*   **Handler (`http.Handler`)**: 處理 HTTP 請求，參數驗證，以及**限流保護**。

---

## 2. 安全機制

### 密碼存儲
*   使用 **Bcrypt** 演算法進行密碼雜湊。
*   Bcrypt 自動處理 Salt，能有效防禦彩虹表攻擊。
*   **注意**: Bcrypt 是 CPU 密集型操作，在高併發登入時會消耗大量 CPU 資源。

### 身份驗證 (Authentication)
*   使用 **JWT (JSON Web Token)** 進行無狀態驗證。
*   Token 包含 `user_id` 與過期時間 (`exp`)。
*   Server 端使用密鑰 (`Secret`) 簽名，防止篡改。

---

## 3. 高併發保護策略 (Rate Limiting)

為了防止惡意攻擊或瞬間流量高峰 (如 8500 個機器人同時啟動) 導致服務崩潰，我們在登入接口實作了 **Token Bucket (令牌桶)** 限流機制。

### 限流參數

| 參數 | 設定值 | 說明 |
| :--- | :--- | :--- |
| **Limit (RPS)** | **100** | 每秒允許 100 個登入請求。 |
| **Burst** | **50** | 允許瞬間突發 50 個請求。 |

### 工作原理

1.  **請求到達**: 當用戶呼叫 `/api/users/login` 時。
2.  **檢查令牌**: `Handler` 檢查令牌桶中是否有足夠的令牌。
3.  **通過**: 若有令牌，扣除一個並繼續處理登入邏輯 (Bcrypt 驗證等)。
4.  **拒絕**: 若無令牌，直接返回 HTTP `429 Too Many Requests`，並記錄 Warning 日誌。

### 程式碼實作 (`adapter/http/handler.go`)

```go
func NewHandler(svc domain.UserUseCase) *Handler {
    return &Handler{
        svc: svc,
        // Limit: 100 requests per second, Burst: 50 requests
        loginLimiter: rate.NewLimiter(rate.Limit(100), 50),
    }
}

func (h *Handler) Login(c *gin.Context) {
    // Rate Limiting Check
    if !h.loginLimiter.Allow() {
        logger.Warn(c.Request.Context()).Msg("Login: rate limit exceeded")
        c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login requests"})
        return
    }
    // ... 繼續處理登入
}
```

---

## 4. 客戶端重試策略 (Test Robot)

為了配合 Server 端的限流，客戶端 (Test Robot) 實作了智慧重試機制：

1.  **重試次數**: 最多重試 **5 次**。
2.  **隨機退避 (Jitter)**: 每次重試前隨機等待 **100ms ~ 300ms**。這能有效將瞬間流量分散開來，避免所有客戶端在同一時間重試 (Thundering Herd Problem)。
3.  **429 處理**: 明確識別 `429` 錯誤並觸發重試。

### 效果

透過 Server 端限流與 Client 端重試的配合，系統能夠在保護自身資源 (CPU/DB) 的同時，最終讓所有合法用戶都能成功登入，實現**削峰填谷**的效果。
