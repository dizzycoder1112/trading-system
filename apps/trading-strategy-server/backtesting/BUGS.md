# Backtesting Engine - Bug 追蹤

記錄回測引擎發現的 bug 和待調查問題。

---

## 📋 Bug 追蹤流程

**採用 GitHub Issues + 本地索引的混合方案**

### 流程

1. **發現 bug** → 開 GitHub Issue
2. **調查過程** → 在 Issue 留言討論
3. **修復完成** → commit 訊息寫 `Fixes #N`
4. **更新索引** → 在下方表格加上 commit hash
5. **Push** → GitHub 自動關閉 Issue 並連結 commit

### Commit 訊息格式

```bash
# 修復並關閉 Issue
git commit -m "Fix xxx issue

Fixes #1"

# 只引用不關閉
git commit -m "Investigate xxx

Related to #2"
```

### 索引表格

| ID | 標題 | 狀態 | Issue | Commit | 開單 | 修復 |
|----|------|------|-------|--------|------|------|
| 001 | Max Drawdown 計算錯誤 | 🔍 | - | - | 12-04 | - |
| 002 | UnrealizedPnL 計算不一致 | 🔍 | - | - | 12-04 | - |
| 003 | 兩版本結果差異巨大 | 🔴 | - | - | 12-04 | - |
| 004 | 強制平倉被註解 | ⚠️ | - | - | 12-04 | - |

> **備註**：Issue 連結待 GitHub repo 建立後補上

---

## 詳細記錄

---

## BUG-001: Max Drawdown 計算錯誤 (99.99%)

**狀態**: 🔍 調查中

**現象**:
- Max Drawdown 顯示 96.99% ~ 99.99%
- 這個數字不合理，實際策略不可能虧這麼多

**根本原因**:
- `calculateMaxDrawdown()` 只用 `Balance`（可用餘額）計算
- 網格策略在單邊下跌時會不斷加倉，資金被鎖在持倉裡
- 可用餘額趨近於零，但總權益（餘額 + 持倉市值）還在

**正確計算方式**:
```
總權益 = 可用餘額 + 持倉市值 - 預估平倉手續費
Max Drawdown 應該基於總權益計算
```

**相關檔案**:
- `backtesting/metrics/calculator.go:214-238` - `calculateMaxDrawdown()`

**解決方案**:
- [ ] 方案 A: 回測結束時強制平倉（已實作但被註解）
- [ ] 方案 B: 修改 drawdown 計算，每次記錄快照時加入持倉市值

---

## BUG-002: UnrealizedPnL 計算方式不一致

**狀態**: 🔍 調查中

**現象**:
- 修改 `CalculateUnrealizedPnL` 後，淨利潤從 -$611 變成 -$2,999
- 但 CSV 中的 UnrealizedPnL 數值幾乎沒變（前 5 筆完全一樣）

**問題描述**:

原本的計算（可能有誤）:
```go
totalSize := pt.GetTotalSize()  // 開倉時投入的 USDT 總和
priceChangeRate := (currentPrice - avgCost) / avgCost
unrealizedPnL := totalSize * priceChangeRate
```

修正後的計算（邏輯正確）:
```go
totalCoins := pt.totalCoins
unrealizedPnL := totalCoins * (currentPrice - avgCost)
```

**疑點**:
1. 為什麼修改後 CSV 數值沒變？程式碼真的有重新編譯嗎？
2. 兩種算法在數學上應該等價，但實際結果差很多，為什麼？

**相關檔案**:
- `backtesting/simulator/position.go:178-196` - `CalculateUnrealizedPnL()`

**待驗證**:
- [ ] 加 debug log 確認程式碼有被執行
- [ ] 手動驗算兩種公式的差異

---

## BUG-003: 兩版本回測結果差異巨大

**狀態**: 🔴 嚴重

**現象**:

| 項目 | pos100_1 (舊) | pos100 (新) | 差異 |
|------|--------------|-------------|------|
| 總開倉數量 | 80,591 | 79,739 | -852 |
| 總關倉數量 | 80,591 | 79,597 | -994 |
| 未平倉數量 | 0 | 142 | +142 |
| 總利潤(平均成本) | $7,497.87 | $5,134.05 | -$2,364 |
| 總利潤(單筆) | $7,497.87 | $10,435.82 | +$2,938 |
| 總手續費 | $8,062.85 | $7,972.02 | -$91 |
| 淨利潤 | -$564.97 | -$2,999.84 | -$2,435 |
| 打平輪次 | 640 | 556 | -84 |

**奇怪的點**:
1. 開倉少 852 筆，但手續費只少 $91（應該少 ~$85）
2. 基於平均成本的利潤少了，但基於單筆的利潤反而多了
3. 打平機制觸發次數不同，代表整個交易路徑都變了

**可能原因**:
- `UnrealizedPnL` 計算方式改變 → 打平觸發時機改變 → 整個交易路徑都不同

**相關檔案**:
- `backtesting/engine/backtest_engine.go` - 打平機制判斷
- `internal/domain/strategy/strategies/grid/` - 策略邏輯

**待調查**:
- [ ] 確認 UnrealizedPnL 計算是否影響打平判斷
- [ ] 找出第一筆產生分歧的交易
- [ ] 驗證哪個版本的數字才是正確的

---

## BUG-004: 強制平倉程式碼被註解

**狀態**: ⚠️ 待確認

**現象**:
- `backtest_engine.go:666-692` 的強制平倉程式碼被註解掉了

**影響**:
- 回測結束時不會強制平倉
- 導致有未平倉位存在
- Max Drawdown 計算不準確

**相關檔案**:
- `backtesting/engine/backtest_engine.go:660-695`

**解決方案**:
- [ ] 取消註解，啟用強制平倉
- [ ] 或者提供 CLI flag 讓使用者選擇是否強制平倉

---

## 調查筆記

### 2025-12-04 調查過程

1. 發現 Max Drawdown 99.99% 不合理
2. 嘗試在回測結束時強制平倉
3. 發現 `CalculateUnrealizedPnL` 用 `totalSize` 而非 `totalCoins`
4. 修改後淨利潤從 -$611 變成 -$2,999
5. 比較兩份 CSV，發現 UnrealizedPnL 前 5 筆完全一樣
6. 懷疑程式碼沒有重新編譯，或有其他問題

### 待做事項

- [ ] 加 debug log 確認 `CalculateUnrealizedPnL` 有被執行
- [ ] 手動驗算第一筆交易的 UnrealizedPnL
- [ ] 確認哪個版本的淨利潤才是正確的
- [ ] 修復 Max Drawdown 計算

---

*最後更新: 2025-12-04*
