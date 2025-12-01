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
| `--break-even-profit-max` | 20.0 | ⚠️ Deprecated，目前未使用 |
| `--enable-trend-filter` | false | 是否啟用趨勢過濾（實測會降低獲利，不建議啟用）|
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

## 🎯 當前改進任務 ⭐ 2025-11-03

### 優先級 1：平倉檢查邏輯修正（高優先級）

**問題**：當前回測引擎使用 `Close` 價格檢查平倉，不夠準確

**影響**：
- ❌ 錯過 K 線內部的止盈觸發
- ❌ 平倉價格不準確（應該用止盈價，而不是收盤價）
- ❌ 回測結果偏離真實交易表現

**改進方案**：

```go
// 當前實現（不準確）
if currentCandle.Close() >= pos.TargetClosePrice {
    closePosition(pos, currentCandle.Close())  // ❌ 使用收盤價
}

// 應該改為（更準確）
if currentCandle.High() >= pos.TargetClosePrice {
    closePosition(pos, pos.TargetClosePrice)  // ✅ 使用止盈價
}
```

**原因**：
- K 線的 `High` 表示該時段內的最高價
- 如果 `High >= 止盈價`，說明價格已經觸及止盈
- 止盈是限價單，觸及即成交，成交價應該是止盈價

**相關文件**：
- `backtesting/engine/backtest_engine.go:326-376`

**狀態**：待實現 ⭐

---

### 優先級 2：參數化混合時間框架回測 ⭐ 已確認設計

**目標**：使用更細粒度的 K 線作為輸入源，保持固定的交易節奏

**核心設計**：

#### 1. 參數配置

```go
type BacktestConfig struct {
    // ... 現有配置

    // 新增 ⭐
    TickBarSize    int  // 輸入K線周期（秒），例如：60 = 1分K, 1 = 1秒K
    CooldownPeriod int  // 冷卻時間（秒），決定固定開倉節奏，例如：300 = 5分鐘
}
```

**示例配置**：

```go
// 示例1: 1分K + 5分鐘冷卻（默認）
config := BacktestConfig{
    TickBarSize:    60,   // 1分鐘 = 60秒
    CooldownPeriod: 300,  // 5分鐘 = 300秒
    // 聚合：300/60 = 5根1分K → 5分K
}

// 示例2: 1分K + 3分鐘冷卻
config := BacktestConfig{
    TickBarSize:    60,   // 1分鐘
    CooldownPeriod: 180,  // 3分鐘
    // 聚合：180/60 = 3根1分K → 3分K
}

// 示例3: 1秒K + 5分鐘冷卻（未來支持）
config := BacktestConfig{
    TickBarSize:    1,    // 1秒
    CooldownPeriod: 300,  // 5分鐘
    // 聚合：300/1 = 300根1秒K → 5分K
}
```

#### 2. CLI 參數

```bash
go run cmd/backtest/main.go \
  --data=data/1m-ETH-USDT-SWAP.json \
  --tick-bar-size=60 \
  --cooldown-period=300
```

**向後兼容** ⭐：
- 如果不提供 `--tick-bar-size` 和 `--cooldown-period`，使用當前 K 線的時間周期
- 例如：`data/5m-ETH-USDT-SWAP.json` → 自動設置 `TickBarSize=300`, `CooldownPeriod=300`
- 保持現有回測行為不變

**驗證規則** ⭐：
- 必須滿足：`CooldownPeriod % TickBarSize == 0`
- 避免無法整除的情況（例如：60秒tick + 70秒冷卻）

#### 3. 開倉規則（兩種時機）

**規則 A: 冷卻邊界開倉**（固定節奏）
- 每 `CooldownPeriod` 秒無條件詢問策略一次
- 無論當前有多少個持倉
- 使用聚合的 K 線供策略決策
- 使用當前 tick K 線的收盤價作為 `currentPrice`

**規則 B: 平倉後立即重開**（額外機會）
- 任何時刻平倉後立即詢問策略
- 使用當前 tick K 線（**不聚合**）⭐
- 使用當前 tick K 線的收盤價作為 `currentPrice`
- 不影響冷卻邊界的固定節奏

**時間線示例（1分K + 5分鐘冷卻）**：
```
時間: 00:00  00:01  00:02  00:03  00:04  00:05  00:06  00:07
     [1分K] [1分K] [1分K] [1分K] [1分K] [1分K] [1分K] [1分K]
     └────────5分K1──────────┘  └────────5分K2──────────┘

00:00 冷卻邊界（5分K邊界）→ 開倉A（固定節奏）
      持倉：[A]

00:02 檢查平倉 → A觸及止盈，平倉A
      平倉後立即詢問策略（使用當前1分K）→ 開倉B（額外機會）
      持倉：[B]

00:05 冷卻邊界（5分K邊界）→ 開倉C（固定節奏，無論B是否還在）
      持倉：[B, C]  ⭐ 同時持有2個

00:07 檢查平倉 → B觸及止盈，平倉B
      平倉後立即詢問策略 → 開倉D（額外機會）
      持倉：[C, D]  ⭐ 同時持有2個
```

#### 4. 平倉規則（每根 tick K 檢查）

- 使用 tick K 線的 `High` 價格檢查是否觸及止盈
- 使用止盈價平倉（而不是收盤價或 High）
- 平倉後立即觸發**規則 B**（詢問策略是否重開）

#### 5. 動態 K 線聚合

```go
// 聚合任意數量的K線
func aggregateCandles(candles []value_objects.Candle) (value_objects.Candle, error) {
    if len(candles) == 0 {
        return nil, fmt.Errorf("需要至少1根K線")
    }

    // 只有1根K線，直接返回
    if len(candles) == 1 {
        return candles[0], nil
    }

    open := candles[0].Open()
    close := candles[len(candles)-1].Close()

    // 找最高價和最低價
    high := candles[0].High()
    low := candles[0].Low()

    for i := 1; i < len(candles); i++ {
        if candles[i].High().Value() > high.Value() {
            high = candles[i].High()
        }
        if candles[i].Low().Value() < low.Value() {
            low = candles[i].Low()
        }
    }

    timestamp := candles[len(candles)-1].Timestamp()

    return value_objects.NewCandle(open, high, low, close, timestamp), nil
}
```

#### 6. 主循環實現偽代碼

```go
func (e *BacktestEngine) Run(candles []value_objects.Candle) {
    aggregationCount := e.config.CooldownPeriod / e.config.TickBarSize

    for i := 0; i < len(candles); i++ {
        currentCandle := candles[i]
        currentTime := currentCandle.Timestamp()

        // === 步驟 1: 檢查平倉（每根tick K）===
        beforeCloseCount := len(openPositions)

        for _, pos := range openPositions {
            if currentCandle.High().Value() >= pos.TargetClosePrice {
                closePosition(pos, pos.TargetClosePrice, currentTime)
            }
        }

        afterCloseCount := len(openPositions)
        justClosed := (beforeCloseCount > afterCloseCount)

        // === 步驟 2: 開倉檢查 ===

        // 情況A: 平倉後立即重開（使用當前單根K線，不聚合）
        if justClosed {
            currentPrice := currentCandle.Close()

            advice := strategy.GetOpenAdvice(currentPrice, currentCandle, ...)

            if advice.ShouldOpen {
                openPosition(advice, advice.OpenPrice, currentTime)
            }
        }

        // 情況B: 冷卻邊界固定開倉
        if e.isCooldownBoundary(i) {
            // 獲取需要聚合的K線範圍
            candlesToAggregate := e.getAggregationRange(i, candles)

            // 聚合K線
            aggregatedCandle, err := aggregateCandles(candlesToAggregate)
            if err != nil {
                continue
            }

            currentPrice := currentCandle.Close()

            advice := strategy.GetOpenAdvice(currentPrice, aggregatedCandle, ...)

            if advice.ShouldOpen {
                openPosition(advice, advice.OpenPrice, currentTime)
            }
        }
    }
}

// 判斷是否為冷卻邊界
func (e *BacktestEngine) isCooldownBoundary(index int) bool {
    aggregationCount := e.config.CooldownPeriod / e.config.TickBarSize

    if index < aggregationCount-1 {
        return false
    }

    return index % aggregationCount == (aggregationCount - 1)
}

// 獲取需要聚合的K線範圍
func (e *BacktestEngine) getAggregationRange(index int, candles []value_objects.Candle) []value_objects.Candle {
    aggregationCount := e.config.CooldownPeriod / e.config.TickBarSize

    start := index - aggregationCount + 1
    if start < 0 {
        start = 0
    }

    return candles[start:index+1]
}
```

#### 7. 掛單邏輯（簡化處理）⭐

**當前階段**：假設所有掛單都成交（不檢查 `Low` 價格）

```go
// 簡化版本（當前實現）
if advice.ShouldOpen {
    openPosition(advice, advice.OpenPrice)  // 假設成交
}
```

**未來改進**（低優先級）：
```go
// 完整版本（未來改進）
if advice.ShouldOpen {
    if currentCandle.Low() <= advice.OpenPrice {
        openPosition(advice, advice.OpenPrice)  // ✅ 成交
    } else {
        // ❌ 未成交，跳過（不保留掛單）
    }
}
```

**為什麼是低優先級**：
- 策略的掛單價格通常很接近當前價（0.1%），成交概率高
- 對回測結果影響相對較小

#### 8. 持倉管理

- ✅ **允許同時持有多個倉位**（如果止盈不觸發，倉位會累積）
- ✅ **冷卻期內只開倉一次**（邊界時刻的固定開倉）
- ✅ **額外開倉不計入節奏**（平倉後的重開）
- ⚠️ 倉位增長由其他機制保護（餘額限制、打平機制等）

#### 9. 優勢

- ✅ **通用性**：支持任意時間框架組合（1秒K、1分K、5分K等）
- ✅ **靈活性**：可以輕鬆調整冷卻時間測試不同策略
- ✅ **可擴展**：未來可以支持秒K、分鐘K、小時K等
- ✅ **向後兼容**：默認參數保持現有行為不變
- ✅ **更頻繁的平倉檢查**：每根tick K檢查一次
- ✅ **更準確的平倉價格**：使用止盈價而非收盤價
- ✅ **平倉後立即重開**：不錯過交易機會

**狀態**：設計已確認 ✅，待實現

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
