# ColorGame 快速參考指南

## WebSocket Command 速查表

### 客戶端 → 服務端（REQ）

#### 下注
```json
{
  "game": "color_game",
  "command": "ColorGamePlaceBetREQ",
  "data": {
    "color": "red",
    "amount": 100
  }
}
```

#### 獲取狀態
```json
{
  "game": "color_game",
  "command": "ColorGameGetStateREQ",
  "data": {}
}
```

---

### 服務端 → 客戶端（RSP）

#### 下注成功
```json
{
  "game_code": "color_game",
  "command": "ColorGamePlaceBetRSP",
  "data": {
    "error_code": 0,
    "bet_id": "bet_xxx",
    "error": ""
  }
}
```

#### 下注失敗
```json
{
  "game_code": "color_game",
  "command": "ColorGamePlaceBetRSP",
  "data": {
    "error_code": 200,
    "bet_id": "",
    "error": "餘額不足"
  }
}
```

---

### 服務端 → 客戶端（BRC - 廣播）

#### 遊戲狀態
```json
{
  "game_code": "color_game",
  "command": "ColorGameStateBRC",
  "data": {
    "round_id": "20251205123456",
    "state": "EVENT_TYPE_BETTING_STARTED",
    "betting_end_timestamp": 1733377991,
    "left_time": 10
  }
}
```

#### 開獎結果
```json
{
  "game_code": "color_game",
  "command": "ColorGameResultBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "red",
    "left_time": 5,
    "timestamp": 1733377993
  }
}
```

#### 結算通知（有下注）
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "red",
    "bet_id": "bet_xxx",
    "bet_color": "red",
    "bet_amount": 100,
    "win_amount": 200,
    "is_winner": true
  }
}
```

#### 結算通知（無下注）
```json
{
  "game_code": "color_game",
  "command": "ColorGameSettlementBRC",
  "data": {
    "round_id": "20251205123456",
    "winning_color": "red",
    "bet_id": "",
    "bet_color": "",
    "bet_amount": 0,
    "win_amount": 0,
    "is_winner": false
  }
}
```

---

## ErrorCode 速查表

| Code | Name | 說明 | 使用場景 |
|------|------|------|---------|
| 0 | SUCCESS | 成功 | 所有成功的操作 |
| 5 | INTERNAL_ERROR | 內部錯誤 | 服務端異常 |
| 200 | INSUFFICIENT_BALANCE | 餘額不足 | 下注金額超過餘額 |
| 301 | ROUND_NOT_ACTIVE | 回合未激活 | 在非下注時間下注 |
| 302 | INVALID_BET_AMOUNT | 下注金額無效 | 金額 <= 0 或超過限制 |

完整列表見：`shared/proto/common/common.proto`

---

## 遊戲狀態流程

```
ROUND_STARTED (2秒)
    ↓
BETTING_STARTED (10秒) ← 可以下注
    ↓
DRAWING (2秒)
    ↓
RESULT (5秒) ← 顯示開獎結果
    ↓
SETTLEMENT ← 發送結算通知
    ↓
ROUND_ENDED (3秒)
    ↓
(循環)
```

**總回合時長**: 約 22 秒

---

## 前端處理範例

```javascript
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  switch(message.command) {
    case "ColorGameStateBRC":
      // 更新遊戲狀態
      updateGameState(message.data);
      break;
      
    case "ColorGamePlaceBetRSP":
      // 處理下注回應
      if (message.data.error_code === 0) {
        showSuccess("下注成功");
      } else {
        showError(message.data.error);
      }
      break;
      
    case "ColorGameResultBRC":
      // 顯示開獎動畫
      showResult(message.data.winning_color);
      break;
      
    case "ColorGameSettlementBRC":
      // 顯示結算結果
      if (message.data.is_winner) {
        showWin(message.data.win_amount);
      } else if (message.data.bet_amount > 0) {
        showLose();
      }
      break;
  }
};
```

---

## 常見問題

### Q: 為什麼要使用 ColorGame 前綴？
A: 避免命名衝突，當有多個遊戲時（如 SlotGame, PokerGame），可以清楚區分。

### Q: error_code 為什麼是數字而不是字符串？
A: 
1. 節省帶寬
2. 更容易做國際化（前端根據 code 顯示對應語言的錯誤訊息）
3. 更容易做錯誤統計和監控

### Q: 為什麼有 REQ/RSP/BRC 三種後綴？
A:
- `REQ` - 客戶端請求，需要服務端回應
- `RSP` - 服務端回應，對應某個 REQ
- `BRC` - 服務端廣播，主動推送，無需請求

### Q: 沒下注的玩家為什麼也要收到結算通知？
A: 讓所有玩家都知道開獎結果，即使沒參與下注也能看到遊戲進行。

---

## 開發檢查清單

添加新功能時：

- [ ] 在 proto 中定義 message
- [ ] 在 `CommandType` enum 中添加 command
- [ ] 運行 `protoc` 生成代碼
- [ ] 實現業務邏輯
- [ ] 在 Gateway 中添加處理
- [ ] 添加錯誤日誌
- [ ] 編寫測試
- [ ] 更新文檔

---

## 相關文檔

- 完整規範：`.agent/proto_design_guidelines.md`
- 協議詳情：`.agent/websocket_protocol.md`
- 重構記錄：`.agent/colorgame_proto_refactoring_2025-12-05.md`
- 用戶文檔：`docs/cmd/color_game.md`
