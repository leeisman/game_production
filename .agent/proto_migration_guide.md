# Proto Message 重命名遷移指南

## 背景

所有 ColorGame 相關的 proto message 現在都添加了 `ColorGame` 前綴，以保持命名一致性和避免命名衝突。

## 重命名對照表

### Request/Response Messages

| 舊名稱 | 新名稱 |
|--------|--------|
| `PlaceBetReq` | `ColorGamePlaceBetReq` |
| `PlaceBetRsp` | `ColorGamePlaceBetRsp` |
| `GetStateReq` | `ColorGameGetStateReq` |
| `GetStateRsp` | `ColorGameGetStateRsp` |
| `RecordBetReq` | `ColorGameRecordBetReq` |
| `RecordBetRsp` | `ColorGameRecordBetRsp` |
| `GetCurrentRoundReq` | `ColorGameGetCurrentRoundReq` |
| `GetCurrentRoundRsp` | `ColorGameGetCurrentRoundRsp` |
| `SubscribeEventsReq` | `ColorGameSubscribeEventsReq` |

### Broadcast Messages

| 舊名稱 | 新名稱 |
|--------|--------|
| `GameEvent` | `ColorGameEvent` |
| `BetConfirmation` | `ColorGameBetConfirmation` |
| `ColorGameRoundStateBRC` | `ColorGameRoundStateBRC` (不變) |
| `SettlementBRC` | `ColorGameSettlementBRC` |

### Other Messages

| 舊名稱 | 新名稱 |
|--------|--------|
| `PlayerBet` | `ColorGamePlayerBet` |

### Enums

| 舊名稱 | 新名稱 |
|--------|--------|
| `EventType` | `ColorGameEventType` |
| `CommandType` | `ColorGameCommandType` (待添加) |

## 需要更新的文件列表

### 1. Service Interfaces
- [ ] `pkg/service/color_game/gs.go`
- [ ] `pkg/service/color_game/gms.go`

### 2. GS Adapters
- [ ] `internal/modules/color_game/gs/adapter/local/handler.go`
- [ ] `internal/modules/color_game/gs/adapter/grpc/handler.go`
- [ ] `internal/modules/color_game/gs/adapter/grpc/client.go`

### 3. GMS Adapters
- [ ] `internal/modules/color_game/gms/adapter/local/handler.go`
- [ ] `internal/modules/color_game/gms/usecase/gms_uc.go`
- [ ] `internal/modules/color_game/gms/machine/state_machine.go`

### 4. Gateway
- [ ] `internal/modules/gateway/usecase/gateway_uc.go`
- [ ] `internal/modules/gateway/adapter/local/handler.go`
- [ ] `internal/modules/gateway/adapter/grpc/colorgame_client.go`

### 5. Tests
- [ ] `tests/integration/color_game/*.go`
- [ ] `tests/integration/gateway/*.go`

## 自動替換腳本

可以使用以下命令進行批量替換（謹慎使用）：

```bash
# 替換 PlaceBetReq
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.PlaceBetReq/pbColorGame.ColorGamePlaceBetReq/g' {} +
find . -name "*.go" -type f -exec sed -i '' 's/colorgame\.PlaceBetReq/colorgame.ColorGamePlaceBetReq/g' {} +

# 替換 PlaceBetRsp
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.PlaceBetRsp/pbColorGame.ColorGamePlaceBetRsp/g' {} +
find . -name "*.go" -type f -exec sed -i '' 's/colorgame\.PlaceBetRsp/colorgame.ColorGamePlaceBetRsp/g' {} +

# 替換 GetStateReq
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.GetStateReq/pbColorGame.ColorGameGetStateReq/g' {} +

# 替換 GetStateRsp
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.GetStateRsp/pbColorGame.ColorGameGetStateRsp/g' {} +

# 替換 RecordBetReq
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.RecordBetReq/pbColorGame.ColorGameRecordBetReq/g' {} +

# 替換 RecordBetRsp
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.RecordBetRsp/pbColorGame.ColorGameRecordBetRsp/g' {} +

# 替換 GetCurrentRoundReq
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.GetCurrentRoundReq/pbColorGame.ColorGameGetCurrentRoundReq/g' {} +

# 替換 GetCurrentRoundRsp
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.GetCurrentRoundRsp/pbColorGame.ColorGameGetCurrentRoundRsp/g' {} +

# 替換 GameEvent
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.GameEvent/pbColorGame.ColorGameEvent/g' {} +

# 替換 EventType
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.EventType/pbColorGame.ColorGameEventType/g' {} +

# 替換 PlayerBet
find . -name "*.go" -type f -exec sed -i '' 's/pbColorGame\.PlayerBet/pbColorGame.ColorGamePlayerBet/g' {} +
```

## 手動更新步驟

1. **重新生成 proto 代碼**（已完成）
   ```bash
   protoc --go_out=. --go_opt=paths=source_relative shared/proto/colorgame/colorgame.proto
   ```

2. **更新 Service 接口**
   - 修改方法簽名使用新的 message 名稱

3. **更新所有實現**
   - 逐個文件更新 import 和使用的 message 類型

4. **運行測試**
   ```bash
   go test ./...
   ```

5. **修復編譯錯誤**
   - 根據編譯器提示逐個修復

## 注意事項

1. **向後兼容性**：這是一個破壞性變更，需要同時更新所有相關代碼
2. **gRPC Service**：Service 定義中的方法簽名也需要更新
3. **測試**：確保所有測試都更新並通過
4. **文檔**：更新所有相關文檔中的 message 名稱

## 驗證清單

- [ ] 所有 Go 文件編譯通過
- [ ] 所有單元測試通過
- [ ] 所有整合測試通過
- [ ] 文檔已更新
- [ ] `.agent` 規範文檔已更新
