# Logger 使用指南

## 特性

✅ **Request ID 追踪** - 整条链路唯一标识  
✅ **文件名和行号** - 自动记录调用位置  
✅ **结构化日志** - JSON 格式，易于解析  
✅ **Context 传递** - 通过 Context 传递 Request ID  
✅ **高性能** - 基于 zerolog，零分配  
✅ **多级别** - Debug, Info, Warn, Error, Fatal  

## 快速开始

### 1. 初始化日志系统

```go
package main

import (
    "github.com/frankieli/game_product/pkg/logger"
)

func main() {
    // 开发环境 - Console 格式，带颜色
    logger.Init(logger.Config{
        Level:  "debug",
        Format: "console",
    })
    
    // 生产环境 - JSON 格式
    logger.Init(logger.Config{
        Level:  "info",
        Format: "json",
    })
}
```

### 2. 在 HTTP Handler 中使用

```go
// 添加 Middleware
r := gin.Default()
r.Use(logger.GinMiddleware()) // 自动生成和注入 Request ID

r.GET("/api/users", func(c *gin.Context) {
    ctx := c.Request.Context()
    
    // 使用 context 打印日志（自动带上 Request ID、文件名、行号）
    logger.Info(ctx).Msg("Getting users")
    
    logger.Info(ctx).
        Int("user_id", 123).
        Str("action", "fetch").
        Msg("User operation")
    
    c.JSON(200, gin.H{"status": "ok"})
})
```

### 3. 在 UseCase/Service 中使用

```go
package usecase

import (
    "context"
    "github.com/frankieli/game_product/pkg/logger"
)

type PlayerUseCase struct {
    // ...
}

func (uc *PlayerUseCase) PlaceBet(ctx context.Context, userID int64, amount int64) error {
    // 日志会自动带上 Request ID、文件名、行号
    logger.Info(ctx).
        Int64("user_id", userID).
        Int64("amount", amount).
        Msg("Placing bet")
    
    // 业务逻辑
    err := uc.repository.Save(...)
    if err != nil {
        logger.Error(ctx).
            Err(err).
            Int64("user_id", userID).
            Msg("Failed to save bet")
        return err
    }
    
    logger.Info(ctx).
        Int64("user_id", userID).
        Msg("Bet placed successfully")
    
    return nil
}
```

### 4. 在 WebSocket 中使用

```go
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // 创建带 Request ID 的 context
    ctx := logger.WebSocketContext(r)
    
    logger.Info(ctx).Msg("WebSocket connection established")
    
    // 连接处理
    conn, _ := upgrader.Upgrade(w, r, nil)
    defer conn.Close()
    
    for {
        _, message, err := conn.ReadMessage()
        if err != nil {
            logger.Error(ctx).Err(err).Msg("Read message failed")
            break
        }
        
        // 处理消息时可以创建新的 Request ID
        msgCtx := logger.WithRequestID(ctx, logger.GenerateRequestID())
        logger.Info(msgCtx).
            Str("message", string(message)).
            Msg("Received message")
    }
}
```

### 5. 手动创建 Request ID

```go
func SomeBackgroundJob() {
    // 创建带 Request ID 的 context
    requestID := logger.GenerateRequestID()
    ctx := logger.WithRequestID(context.Background(), requestID)
    
    logger.Info(ctx).Msg("Background job started")
    
    // 在子函数中继续传递
    processData(ctx)
}

func processData(ctx context.Context) {
    // 自动带上父 context 的 Request ID
    logger.Info(ctx).Msg("Processing data")
}
```

### 6. 添加额外字段

```go
func HandleRequest(ctx context.Context) {
    // 添加持久化字段到 context
    ctx = logger.WithFields(ctx, map[string]interface{}{
        "user_id": 123,
        "tenant_id": "acme",
    })
    
    // 后续日志都会带上这些字段
    logger.Info(ctx).Msg("Step 1")
    logger.Info(ctx).Msg("Step 2")
}
```

## 日志输出格式

### Console 格式（开发环境）

```
2023-12-01T10:28:22+08:00 INF Request started file=handler.go line=45 request_id=20231201102822-000001-a3f2b1 method=GET path=/api/users
2023-12-01T10:28:22+08:00 INF Getting users file=handler.go line=48 request_id=20231201102822-000001-a3f2b1
2023-12-01T10:28:22+08:00 INF Request completed file=handler.go line=55 request_id=20231201102822-000001-a3f2b1 status=200 duration=15ms
```

### JSON 格式（生产环境）

```json
{
  "level":"info",
  "request_id":"20231201102822-000001-a3f2b1",
  "file":"handler.go",
  "line":"45",
  "method":"GET",
  "path":"/api/users",
  "time":"2023-12-01T10:28:22+08:00",
  "message":"Request started"
}
{
  "level":"info",
  "request_id":"20231201102822-000001-a3f2b1",
  "file":"handler.go",
  "line":"48",
  "time":"2023-12-01T10:28:22+08:00",
  "message":"Getting users"
}
{
  "level":"info",
  "request_id":"20231201102822-000001-a3f2b1",
  "file":"handler.go",
  "line":"55",
  "status":200,
  "duration":15,
  "time":"2023-12-01T10:28:22+08:00",
  "message":"Request completed"
}
```

## API 参考

### 初始化

```go
logger.Init(logger.Config{
    Level:  "info",     // debug, info, warn, error
    Format: "json",     // json, console
    Output: os.Stdout,  // 可选，默认 stdout
})
```

### 日志方法

```go
logger.Debug(ctx).Msg("Debug message")
logger.Info(ctx).Msg("Info message")
logger.Warn(ctx).Msg("Warning message")
logger.Error(ctx).Err(err).Msg("Error occurred")
logger.Fatal(ctx).Msg("Fatal error") // 会退出程序

// 添加字段
logger.Info(ctx).
    Str("key", "value").
    Int("count", 10).
    Bool("success", true).
    Dur("duration", time.Since(start)).
    Msg("Message with fields")
```

### Context 操作

```go
// 创建带 Request ID 的 context
ctx := logger.WithRequestID(context.Background(), requestID)

// 获取 Request ID
requestID := logger.GetRequestID(ctx)

// 添加字段到 context
ctx = logger.WithFields(ctx, map[string]interface{}{
    "user_id": 123,
})
```

### 无 Context 使用（不推荐，但可用）

```go
logger.InfoGlobal().Msg("No context available")
logger.ErrorGlobal().Err(err).Msg("Error without context")
```

## 最佳实践

### 1. 始终传递 Context

```go
// ✅ Good
func ProcessOrder(ctx context.Context, orderID string) error {
    logger.Info(ctx).Str("order_id", orderID).Msg("Processing order")
    // ...
}

// ❌ Bad - 缺少 Request ID 追踪
func ProcessOrder(orderID string) error {
    log.Println("Processing order", orderID)
    // ...
}
```

### 2. 在链路入口生成 Request ID

```go
// HTTP Handler
r.Use(logger.GinMiddleware()) // 自动生成

// WebSocket
ctx := logger.WebSocketContext(r)

// Background Job
ctx := logger.WithRequestID(context.Background(), logger.GenerateRequestID())
```

### 3. 记录关键操作

```go
logger.Info(ctx).
    Int64("user_id", userID).
    Int64("amount", amount).
    Msg("Bet placed") // 下注成功

logger.Error(ctx).
    Err(err).
    Int64("user_id", userID).
    Msg("Failed to deduct wallet") // 错误
```

### 4. 使用结构化字段而非字符串拼接

```go
// ✅ Good - 易于查询和分析
logger.Info(ctx).
    Str("user_name", userName).
    Int("age", age).
    Msg("User registered")

// ❌ Bad - 难以解析
logger.Info(ctx).Msgf("User %s registered, age: %d", userName, age)
```

## 日志查询（生产环境）

使用 JSON 格式后，可以用 `jq` 等工具查询：

```bash
# 查找特定 Request ID 的所有日志
cat app.log | jq 'select(.request_id == "20231201102822-000001-a3f2b1")'

# 查找错误日志
cat app.log | jq 'select(.level == "error")'

# 查找特定用户的操作
cat app.log | jq 'select(.user_id == 123)'

# 统计请求耗时
cat app.log | jq 'select(.duration) | .duration' | awk '{sum+=$1; count++} END {print sum/count}'
```

## 性能

- **零分配**：使用 zerolog，避免内存分配
- **异步写入**：可选配置异步写入文件
- **采样**：高流量时可配置日志采样

```go
// 高性能配置
logger := zerolog.New(os.Stdout).
    Sample(&zerolog.BurstSampler{
        Burst:  5,
        Period: 1 * time.Second,
    })
```
