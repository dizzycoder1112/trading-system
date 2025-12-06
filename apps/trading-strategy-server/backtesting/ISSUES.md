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
| 005 | 計算邏輯分散，不符合 Single Source of Truth | 📋 | - | - | 12-07 | - |
| 006 | 使用 float64 計算導致精度問題 | ✅ | - | - | 12-07 | 12-07 |

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

---

## BUG-005: 計算邏輯分散，不符合 Single Source of Truth

**狀態**: 📋 待重構

**現象**:
- 同一個計算邏輯散落在多個文件
- 勝率在 `position.go` 和 `metrics/calculator.go` 都有計算
- 手續費汇總分散在 `order_simulator` 和 `metrics`
- `MetricsCalculator` 做了太多計算，而不是只匯報

**目前的計算分布**:
```
pnl_calculator.go          order_simulator.go
├── 價格變化率              ├── 開倉手續費
├── 盈虧金額                ├── 平倉手續費
└── 盈虧百分比              ├── 實際成本
                           └── 收入

position.go                metrics/calculator.go
├── 平均成本 ⭐             ├── 總手續費 (又加一次)
├── 總幣數                  ├── 淨利潤 (又算一次)
├── 未實現盈虧              ├── 總收益率
├── 已實現盈虧汇總          ├── 最大回撤
└── 勝率 ⭐                 ├── 勝率 ⭐ (重複！)
                           └── 盈虧比
```

**理想架構**:
```
PnLCalculator (Single Source of Truth)
    ↑ 所有盈虧計算
    │
PositionTracker (倉位狀態)
    ├── 調用 PnLCalculator
    ├── 提供：平均成本、總幣數、未實現盈虧、已實現盈虧
    ├── 提供：勝率、盈虧比（唯一來源）
    └── 提供：總手續費（唯一來源）
    │
MetricsCalculator (只做匯報)
    └── 從 PositionTracker 取值，不自己計算
```

**解決方案**:
- [ ] 把 `metrics/calculator.go` 的計算邏輯移到 `PositionTracker`
- [ ] `MetricsCalculator` 只負責格式化和生成報告
- [ ] 刪除重複的勝率計算

**相關檔案**:
- `backtesting/simulator/position.go`
- `backtesting/simulator/pnl_calculator.go`
- `backtesting/simulator/order_simulator.go`
- `backtesting/metrics/calculator.go`

**優先級**: 低（先完成 decimal 重構）

---

## ISSUE-006: 使用 float64 計算導致精度問題

**狀態**: ✅ 已修復

**現象**:
- 金額計算使用 `float64`，會有浮點數精度問題
- 例如：`0.1 + 0.2 = 0.30000000000000004`
- 累積多次計算後誤差會放大

**解決方案**:
- [x] `position.go` 改用 `shopspring/decimal`
- [x] `pnl_calculator.go` 改用 `shopspring/decimal`
- [x] `order_simulator.go` 改用 `shopspring/decimal`
- [x] `metrics/calculator.go` 改用 `shopspring/decimal`

**原則**:
- 計算過程用 `decimal.Decimal`
- 儲存時用 `.InexactFloat64()` 轉回 `float64`
- API 簽名保持 `float64` 不變

**相關檔案**:
- `backtesting/simulator/position.go` ✅
- `backtesting/simulator/pnl_calculator.go` ✅
- `backtesting/simulator/order_simulator.go` ✅
- `backtesting/metrics/calculator.go` ✅

---

*最後更新: 2025-12-07*
