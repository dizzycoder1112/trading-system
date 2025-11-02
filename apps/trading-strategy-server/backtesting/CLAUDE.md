# Backtesting Engine - 回測系統文檔

## 📖 專案概述

回測引擎，用於測試交易策略的歷史表現。

**核心設計原則** ⭐：
- ✅ **內建於 trading-strategy-server**（與策略代碼在同一倉庫）
- ✅ **使用真實的策略代碼**（確保回測結果與實盤一致）
- ✅ **通過 CLI 入口運行**（`cmd/backtest/main.go`）
- ✅ 快速迭代（可並行測試多種參數組合）
- ✅ 結果可重現（相同數據和參數 → 相同結果）

**架構決策**：參考業界實踐（QuantConnect, Backtrader, Jesse），將回測引擎與策略代碼放在同一個項目中，確保實盤和回測使用完全相同的策略邏輯。

---

## 🗂️ 專案結構

```
apps/trading-strategy-server/
├── cmd/
│   ├── main.go                    # 實盤策略服務入口
│   └── backtest/
│       └── main.go                # 回測 CLI 入口 ⭐
├── internal/
│   └── domain/
│       └── strategy/
│           └── strategies/
│               └── grid/          # Grid 策略（實盤和回測共用）⭐
├── backtesting/                   # 回測引擎模組 ⭐
│   ├── engine/                    # 回測引擎核心
│   │   ├── backtest_engine.go
│   │   └── backtest_engine_test.go
│   ├── simulator/                 # 成交模擬器 + 倉位追蹤器
│   │   ├── position.go
│   │   ├── position_test.go
│   │   ├── pnl_calculator.go      # ⭐ Single Source of Truth
│   │   ├── order_simulator.go
│   │   └── order_simulator_test.go
│   ├── metrics/                   # 指標計算器
│   │   ├── calculator.go
│   │   └── calculator_test.go
│   └── loader/                    # 歷史數據加載器
│       ├── candle_loader.go
│       └── candle_loader_test.go
├── data/                          # 歷史數據存放
│   ├── 20240930-20241001-5m-ETH-USDT-SWAP.json
│   └── 20240930-20241005-5m-ETH-USDT-SWAP.json
└── go.mod
```

---

## 🚀 快速開始

### 1. 運行回測

```bash
# 基本使用（使用默認參數）
go run cmd/backtest/main.go --data=data/20240930-20241001-5m-ETH-USDT-SWAP.json

# 或使用編譯後的二進制文件
go build -o bin/backtest cmd/backtest/main.go
./bin/backtest --data=data/20240930-20241001-5m-ETH-USDT-SWAP.json

# 自定義參數
./bin/backtest \
  --data=data/20240930-20241005-5m-ETH-USDT-SWAP.json \
  --initial-balance=20000 \
  --position-size=200 \
  --take-profit-min=0.002 \
  --take-profit-max=0.003 \
  --break-even-profit-min=1 \
  --break-even-profit-max=20 \
  --enable-trend-filter=true \
  --enable-red-candle-filter=true \
  --enable-auto-funding=true \
  --auto-funding-amount=5000 \
  --auto-funding-idle=288
```

### 2. 可用參數

| 參數 | 默認值 | 說明 |
|------|--------|------|
| `--data` | (必填) | 歷史數據文件路徑 |
| `--initial-balance` | 10000 | 初始資金 (USDT) |
| `--position-size` | 100 | 單次開倉大小 (USDT) |
| `--fee-rate` | 0.0005 | 手續費率 (0.05%) |
| `--take-profit-min` | 0.0015 | 最小止盈百分比 (0.15%) |
| `--take-profit-max` | 0.01 | 最大止盈百分比 (1%) |
| `--break-even-profit-min` | -0.1 | 打平最小目標盈利 (USDT) |
| `--break-even-profit-max` | 20.0 | 打平最大目標盈利 (USDT) |
| `--enable-trend-filter` | false | 是否啟用趨勢過濾 ⭐ |
| `--enable-red-candle-filter` | true | 虧損時只在紅K開倉 ⭐ |
| `--enable-auto-funding` | true | 是否啟用自動注資 ⭐ |
| `--auto-funding-amount` | 5000 | 自動注資金額 (USDT) |
| `--auto-funding-idle` | 12 | 觸發注資的閒置K線數 |

### 3. 輸出結果

回測完成後會在數據文件同目錄下生成：

```
data/
├── backtest_trades_pos{size}/
│   ├── trades.csv           # 交易日誌（包含每筆交易詳情）
│   ├── report.md            # 回測報告（Markdown格式）
│   └── rounds_detail.csv    # 打平輪次詳細記錄
```

---

## 🔧 核心架構

### 1. 回測流程

```
歷史K線數據
    ↓
┌─────────────────────────────────┐
│ BacktestEngine.Run()             │
│                                  │
│  for 每根K線:                     │
│    1. 檢查平倉（優先釋放資金）      │
│    2. 調用真實策略獲取開倉建議 ⭐   │
│    3. 模擬開倉（如果建議開倉）      │
│    4. 記錄資金快照                │
│                                  │
│  回測結束:                        │
│    5. 強制平倉所有未平倉位         │
│    6. 計算回測指標                │
└─────────────────────────────────┘
    ↓
BacktestResult
    ├── 資金狀況（初始、最終、總權益）
    ├── 倉位分析（開倉數、最大持倉值）
    ├── 交易統計（勝率、盈虧比、回撤）
    └── 詳細記錄（交易日誌、打平輪次）
```

### 2. 核心組件

#### **BacktestEngine** (回測引擎核心)
- 循環歷史數據
- **調用真實策略代碼** (`grid.GridAggregate`) ⭐
- 模擬交易執行
- 記錄資金曲線

#### **OrderSimulator** (成交模擬器)
- 模擬開倉：計算成本、手續費、驗證餘額
- 模擬平倉：計算盈虧、已實現PnL、實際收入
- **使用 PnLCalculator** (Single Source of Truth) ⭐

#### **PositionTracker** (倉位追蹤器)
- 管理未平倉/已平倉記錄
- **累進式計算平均成本** (買入時更新，賣出不變) ⭐
- 計算未實現盈虧（基於平均成本）

#### **MetricsCalculator** (指標計算器)
- 計算總收益率、勝率、盈虧比
- 計算最大回撤（基於資金快照）
- **淨利潤公式**: `totalProfitGross + unrealizedPnL - totalFeesPaid` ⭐

#### **PnLCalculator** (盈虧計算器) ⭐ Single Source of Truth
```go
type PnLCalculator struct{}

// 核心函數（所有價格變化率計算的唯一來源）
func (pc *PnLCalculator) CalculatePriceChangeRate(currentPrice, basePrice float64) float64
func (pc *PnLCalculator) CalculatePriceChangePercent(currentPrice, basePrice float64) float64
func (pc *PnLCalculator) CalculatePnL(closePrice, basePrice, coins float64) (pnlAmount, pnlPercent float64)
```

---

## 📊 回測邏輯

### 使用 Close 價格（當前版本）

每根K線結束時，用收盤價作為 `currentPrice`：

```go
for i, candle := range candles {
    currentPrice := candle.Close()

    // 檢查平倉
    for _, pos := range openPositions {
        if currentPrice >= pos.TargetClosePrice {
            closePosition(pos, currentPrice)
        }
    }

    // 調用策略
    advice := strategy.GetOpenAdvice(currentPrice, lastCandle, histories)
    if advice.ShouldOpen {
        openPosition(advice, currentPrice)
    }
}
```

**優點**：
- ✅ 簡單直觀
- ✅ 快速實現
- ✅ 適合中低頻策略

**缺點**：
- ⚠️ 每 5 分鐘才判斷一次
- ⚠️ 錯過K線中間的價格變化

---

## 🎯 待優化功能

### 1. 支持 1 分鐘 K 線數據（更精確的回測）⭐

**當前問題**：5分鐘K線顆粒度較粗，錯過快速波動

**改進方案**：
- 下載 1 分鐘 K 線歷史數據
- **用1分K的收盤價模擬5分K內的交易** ⭐
- 每根5分K內包含5根1分K，可以更精確地捕捉價格變化
- 使用相同的回測引擎（無需修改核心邏輯）

**實現邏輯**：
```go
// 讀取1分K線數據
candles1m := loader.LoadFromJSON("data/1m-ETH-USDT-SWAP.json")

// 每根5分K = 5根1分K
for i := 0; i < len(candles1m); i += 5 {
    // 取5根1分K的收盤價進行模擬
    for j := 0; j < 5 && i+j < len(candles1m); j++ {
        currentPrice := candles1m[i+j].Close()

        // 檢查平倉
        checkClose(positions, currentPrice)

        // 檢查開倉
        if shouldOpen(currentPrice) {
            openPosition(currentPrice)
        }
    }
}
```

**優勢**：
- ✅ 更接近真實交易（每分鐘都有機會觸發）
- ✅ 捕捉更細緻的價格變化
- ✅ 減少未觸發止盈的情況
- ✅ 回測結果更準確

**注意事項**：
- ⚠️ 數據量增加5倍，回測時間會變長
- ⚠️ 需要下載對應時間範圍的1分K線數據

---

### 2. 添加更多指標

**當前指標**：
- ✅ 總收益率、勝率、盈虧比
- ✅ 最大回撤、平均持倉時長

**待添加指標**：
- [ ] **夏普比率 (Sharpe Ratio)**：衡量風險調整後收益
  ```go
  sharpeRatio = (avgReturn - riskFreeRate) / stdDevReturns
  ```

- [ ] **Sortino 比率**：只考慮下行波動
  ```go
  sortinoRatio = (avgReturn - riskFreeRate) / downside_std
  ```

- [ ] **最大連續虧損天數**：風險控制指標
- [ ] **平均盈利/虧損金額**：評估每筆交易質量
- [ ] **卡爾馬比率 (Calmar Ratio)**：收益/最大回撤

---

### 3. 支持參數優化（網格搜索）⭐

**目標**：自動化測試多組參數，找到最優組合

**改進方案**：
```go
type ParamGrid struct {
    PositionSizes   []float64  // [100, 200, 300]
    TakeProfitMins  []float64  // [0.001, 0.0015, 0.002]
    TakeProfitMaxs  []float64  // [0.005, 0.01, 0.015]
    BreakEvenMins   []float64  // [0, 1, 5]
    BreakEvenMaxs   []float64  // [10, 20, 50]
}

// 網格搜索
results := []BacktestResult{}
for _, posSize := range grid.PositionSizes {
    for _, tpMin := range grid.TakeProfitMins {
        for _, tpMax := range grid.TakeProfitMaxs {
            config := BacktestConfig{
                PositionSize:  posSize,
                TakeProfitMin: tpMin,
                TakeProfitMax: tpMax,
            }
            result := RunBacktest(config, candles)
            results = append(results, result)
        }
    }
}

// 排序找到最優參數
sort.Slice(results, func(i, j int) bool {
    return results[i].NetProfit > results[j].NetProfit
})
```

**優勢**：
- ✅ 系統化測試參數組合
- ✅ 避免手動調參的主觀性
- ✅ 發現最優策略配置

**注意事項**：
- ⚠️ 避免過擬合（overfitting）
- ⚠️ 需要訓練集/測試集劃分
- ⚠️ 考慮樣本外驗證（out-of-sample testing）

---

### 4. 開放 API 供 Web UI 調用（可選）

**現況**：
- ✅ Web UI 已完成（可視化資金曲線、交易分布、參數調整）
- ✅ 目前通過讀取 CSV/JSON 結果文件展示數據

**待優化**：
考慮是否需要開放 API 讓 Web UI 可以：
- 🔄 **實時觸發回測**（不需手動運行CLI）
- 📊 **動態獲取回測結果**（不需刷新文件）
- 🎛️ **在線調整參數並立即查看結果**

**API 設計方案**：
```go
// POST /api/backtest - 觸發回測
type BacktestRequest struct {
    DataFile          string  `json:"data_file"`
    InitialBalance    float64 `json:"initial_balance"`
    PositionSize      float64 `json:"position_size"`
    TakeProfitMin     float64 `json:"take_profit_min"`
    TakeProfitMax     float64 `json:"take_profit_max"`
    // ... 其他參數
}

// GET /api/backtest/:id - 獲取回測結果
// GET /api/backtest/:id/trades - 獲取交易日誌
// GET /api/backtest/history - 獲取歷史回測列表
```

**優勢**：
- ✅ Web UI 更加靈活和互動
- ✅ 支持多用戶同時使用
- ✅ 可以實現回測任務隊列

**考量**：
- ⚠️ 需要處理並發回測（資源管理）
- ⚠️ 需要考慮安全性（避免濫用）
- ⚠️ 是否真的需要？（目前CLI + CSV已經足夠）

---

## 📊 歷史數據

### 已下載數據

| 文件 | 時間範圍 | K 線週期 | 數據條數 |
|------|---------|---------|---------|
| `20240930-20241001-5m-ETH-USDT-SWAP.json` | 2024-09-30 ~ 2024-10-01 | 5m | ~300 |
| `20240930-20241005-5m-ETH-USDT-SWAP.json` | 2024-09-30 ~ 2024-10-05 | 5m | ~1640 |

### 下載工具

使用 TypeScript 腳本：`scripts/download_okx_history.ts`

```bash
pnpm download:okx \
  --inst-id=ETH-USDT-SWAP \
  --bar=5m \
  --after=2024-10-01T00:00:00 \
  --before=2024-10-05T00:00:00
```

---

## 🏗️ 架構決策記錄

### 為什麼將 Backtesting 整合到 Strategy Server 內部？⭐

**問題發現過程**：
1. **初始架構**：Backtesting 是獨立的 app (`apps/backtesting`)
2. **遇到問題**：無法訪問 `trading-strategy-server/internal/domain/strategy`
3. **錯誤方案**：將策略移到公開包（違反封裝原則）
4. **反思質疑**：「為什麼外部服務可以直接訪問策略實體？這符合 DDD 嗎？」

**最終解決方案**：參考業界實踐（QuantConnect, Backtrader, Jesse），將 Backtesting 整合到 Strategy Server 內部

**為什麼這樣做？**

✅ **符合業界實踐**：
- QuantConnect: 策略和回測在同一項目
- Backtrader: 策略和回測在同一項目
- Jesse: 策略和回測在同一項目

✅ **解決封裝問題**：
- 策略保持 `internal`（不對外暴露）
- 回測引擎可以訪問（同一項目內）

✅ **確保一致性**：
- 實盤和回測使用完全相同的策略代碼
- 策略修改後，回測自動使用新邏輯

✅ **符合 DDD**：
- Domain Layer (`internal/domain/strategy`) 保持封裝
- Backtesting 是工具模組，不是獨立的 Bounded Context

**學到的教訓**：
1. 遇到技術限制時，先質疑架構合理性
2. "獨立性"需要有充分理由（Backtesting 只為策略服務）
3. 參考業界實踐很重要

---

## 🔗 相關文檔

- [項目整體架構](../../CLAUDE.md)
- [Trading Strategy Server](../CLAUDE.md)
- [下載腳本使用說明](../../scripts/README.md)

---

*文檔創建: 2025-10-26*
*最後更新: 2025-11-03*
*當前狀態: ✅ 核心功能完成，待優化改進*
