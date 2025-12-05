# ColorGameSettlementBRC 實現需求

## 需求描述

所有連接到遊戲的玩家（無論是否下注）都應該收到 `ColorGameSettlementBRC` 廣播。

## 當前狀態（2025-12-05 更新）

### ✅ 已實現
1. **Proto 定義** - `ColorGameSettlementBRC` 已定義，支持有/無下注兩種情況
2. **有下注玩家通知** - 通過 `SendToUser` 發送個人結算通知
3. **全局廣播** - 通過 `Broadcast` 向所有在線玩家發送結算通知

### ⚠️ 當前行為
- **有下注的玩家會收到兩次通知**：
  1. 個人通知（包含下注詳情）- 通過 `SendToUser`
  2. 全局廣播（通用開獎結果）- 通過 `Broadcast`
- **無下注的玩家只收到全局廣播**

### 📝 前端處理
前端需要去重，檢查 `bet_id` 欄位：
```javascript
if (message.data.bet_id) {
  // 這是個人通知，優先處理
  showPersonalResult(message.data);
} else if (!hasReceivedPersonalSettlement) {
  // 這是全局廣播，只有沒收到個人通知的才處理
  showWinningColor(message.data.winning_color);
}
```

## 待優化方案

### 方案 1：Gateway 維護在線玩家列表（推薦）

**目標**：有下注的玩家只收到一次通知

**實現步驟**：
1. 在 `GatewayService` 接口添加 `GetOnlineUsers(gameCode string) []int64`
2. Gateway 的 `ws.Manager` 實現 `GetConnectedUsers()` 方法
3. GS 在結算時：
   ```go
   onlineUsers := uc.gatewayBroadcaster.GetOnlineUsers("color_game")
   bettedUserIDs := map[int64]bool{...}  // 從 allBetOrders 收集
   
   for _, userID := range onlineUsers {
       if !bettedUserIDs[userID] {
           uc.gatewayBroadcaster.SendToUser(userID, "color_game", settlementMsg)
       }
   }
   ```

**優點**：
- 每個玩家只收到一次通知
- 職責清晰：GS 負責決定發給誰，Gateway 負責維護在線狀態

**缺點**：
- 需要實現 `ws.Manager.GetConnectedUsers()` 方法
- 需要維護在線玩家列表

### 方案 2：保持當前實現（臨時方案）

**優點**：
- 實現簡單
- 無需修改 Gateway

**缺點**：
- 有下注的玩家收到兩次通知
- 前端需要處理去重

## 決策

**當前採用方案 2（臨時方案）**，待後續優化時實現方案 1。

## 相關文件

- `internal/modules/color_game/gs/usecase/gs_uc.go` - 結算邏輯
- `internal/modules/gateway/adapter/local/handler.go` - Gateway 廣播
- `shared/proto/colorgame/colorgame.proto` - Proto 定義
- `docs/cmd/color_game.md` - 用戶文檔

## 優先級

- **P1**: 更新使用 `ColorGameSettlementBRC`（替換 `ColorGameEvent`）
- **P2**: 實現向所有在線玩家廣播
- **P3**: 優化去重邏輯

## 注意事項

1. **性能考慮**: 如果房間人數很多，需要考慮批量發送
2. **一致性**: 確保有下注和無下注的玩家收到的 `round_id` 和 `winning_color` 一致
3. **時序**: 確保個人結算通知在房間廣播之前發送
