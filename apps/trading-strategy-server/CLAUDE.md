# Trading Strategy Server - 開發進度與計劃

## 服務概述

Trading Strategy Server 是交易系統的**策略信號生成器**，負責：

- **主動監控市場數據**並生成交易信號 ⭐ 核心定位
- 從 Redis 訂閱最新市場數據（Candle/Price）
- 計算網格策略邏輯（開倉點位、趨勢過濾、動態止盈）
- 發布交易信號到 Redis Pub/Sub
- **無狀態設計**：不知道倉位，不管理持倉，不執行交易
- **單一職責**：只負責策略計算和信號生成

## 架構設計 ⭐ Hybrid Model

**採用 DDD (Domain-Driven Design) + Hybrid Communication Pattern**

### 通信模式：Strategy 推送信號，Order 驗證執行

```
┌─────────────────────────────────────────┐
│ Market Data Service                      │
│ (OKX WebSocket → Redis Pub/Sub)          │
└─────────────┬───────────────────────────┘
              ↓ Redis Pub/Sub
              │ market.ticker.{instId}
              │ market.candle.5m.{instId}
              ↓
┌─────────────────────────────────────────┐
│ Trading Strategy Service                 │
│ - 訂閱市場數據                            │
│ - 計算策略邏輯（MidLow、趨勢過濾）          │
│ - 生成交易信號                            │
└─────────────┬───────────────────────────┘
              ↓ Redis Pub/Sub
              │ strategy.signals.{instId}
              ↓
┌─────────────────────────────────────────┐
│ Order Service                            │
│ - 訂閱交易信號                            │
│ - 驗證信號可行性（餘額、倉位、冷卻期）       │
│ - 計算訂單數量                            │
│ - 執行訂單（OKX API）                     │
└─────────────────────────────────────────┘
```

### 為什麼採用 Hybrid Model？

**核心原則**：Strategy 專注"應該交易嗎？"，Order 專注"可以交易嗎？+ 如何交易？"

| 職責         | Trading Strategy Service  | Order Service                 |
| ------------ | ------------------------- | ----------------------------- |
| **信號生成** | ✅ 主動監控價格並生成信號 | ❌                            |
| **倉位狀態** | ❌ 不知道當前倉位         | ✅ 持有 API Key，知道所有倉位 |
| **風險驗證** | ❌ 不做風險檢查           | ✅ 驗證餘額、倉位限制、冷卻期 |
| **訂單執行** | ❌                        | ✅ 計算數量並執行訂單         |

**優勢**：

- ✅ Strategy 持續監控，不錯過交易機會
- ✅ Order 在源頭控制風險（避免無效信號）
- ✅ 清晰的職責分離
- ✅ 可擴展（多個服務可訂閱同一信號）

---

## 策略核心邏輯 ⭐ 2025-10-26 修正版

### 1. **冷卻期機制**

**邏輯**：

- **優先**：等待倉位完成（平倉）才重新開倉
- **或者**：5分鐘到了 + 價格脫離掛單位置比較遠 → 可以開新倉

**實現細節**：

```go
type CoolingPeriod struct {
    lastOrderPrice float64  // 上一次掛單價格
    lastCloseTime  time.Time
}

func (cp *CoolingPeriod) CanOpen(currentPrice float64, hasOpenPosition bool) bool {
    // 1. 如果有未平倉位，必須等平倉
    if hasOpenPosition {
        return false
    }

    // 2. 檢查5分鐘冷卻 + 價格脫離
    if time.Since(cp.lastCloseTime) >= 5*time.Minute {
        priceDiff := math.Abs(currentPrice - cp.lastOrderPrice) / cp.lastOrderPrice
        if priceDiff > 0.003 { // 例如：> 0.3%
            return true
        }
    }

    return false
}
```

**註**：此邏輯在 Order Service 實現，Strategy Service 不負責冷卻期檢查

---

### 2. **大趨勢處理**

**策略**：第一版直接規避大趨勢，不做單 ⭐

**檢測方法**：

- 5分鐘內價格變化 > ±0.6% → 大趨勢
- 大趨勢時 → 跳過交易（不生成信號）

**實現位置**：Trading Strategy Service

```go
type TrendFilter struct {
    threshold float64 // 0.006 = 0.6%
}

func (tf *TrendFilter) ShouldSkipTrading(currentCandle Candle) bool {
    changeRate := (currentCandle.Close - currentCandle.Open) / currentCandle.Open

    // 大漲或大跌時跳過
    if math.Abs(changeRate) > tf.threshold {
        return true // ⭐ 直接跳過，不生成信號
    }

    return false
}
```

---

### 3. **趨勢過濾方向** ⭐ 順勢交易

**修正後邏輯**：

- **大跌時** → 做空✅，做多❌
- **大漲時** → 做多✅，做空❌
- **震盪時** → 兩個方向都可以

**實現**：

```go
func (tf *TrendFilter) ShouldOpenLong(trend TrendState) bool {
    // 大跌時禁止做多（順勢：大跌只做空）
    return trend != STRONG_DOWNTREND
}

func (tf *TrendFilter) ShouldOpenShort(trend TrendState) bool {
    // 大漲時禁止做空（順勢：大漲只做多）
    return trend != STRONG_UPTREND
}
```

---

### 4. **平平出場邏輯** ⭐ Break-Even Exit

**邏輯**：`closedPnL + unrealizedPnL >= 1-20 USDT`

**實現位置**：Order Service

```go
func (os *OrderService) ShouldBreakEven(closedPnL, unrealizedPnL float64) bool {
    totalPnL := closedPnL + unrealizedPnL

    // 總盈虧達到 1-20 USDT → 保本出場
    return totalPnL >= 1.0 && totalPnL <= 20.0
}
```

---

### 5. **動態止盈計算**

**邏輯**：

- 波動大 → 放寬止盈（例如：0.2%）
- 波動小 → 縮緊止盈（例如：0.15%）

**實現位置**：Trading Strategy Service

```go
func (g *GridAggregate) CalculateDynamicTakeProfit(volatility float64) float64 {
    // 基於波動率動態調整
    if volatility > 0.01 { // 高波動
        return 0.002 // 0.2%
    } else if volatility < 0.005 { // 低波動
        return 0.0015 // 0.15%
    }

    return 0.0018 // 默認 0.18%
}
```

---

## 策略參數規格

| 參數           | 值                               | 說明                   |
| -------------- | -------------------------------- | ---------------------- |
| **開倉點位**   | MidLow = `(low + close) / 2`     | 上一根K線的中低點      |
| **止盈範圍**   | 0.15% ~ 0.2%                     | 動態調整（基於波動率） |
| **倉位大小**   | $200 USDT                        | 固定倉位               |
| **手續費率**   | 0.05% (Taker)                    | OKX USDT 永續合約      |
| **冷卻期**     | 完成才重開 OR (5分鐘 + 價格脫離) | Order Service 控制     |
| **大趨勢閾值** | ±0.6% (5分鐘K線)                 | 超過則跳過交易         |
| **趨勢過濾**   | 順勢交易（大跌做空，大漲做多）   | Strategy Service 實現  |
| **平平出場**   | 總盈虧 1-20 USDT                 | Order Service 判斷     |

| 策略要素        | 具体逻辑                                        | 参数                                                                                    |
| --------------- | ----------------------------------------------- | --------------------------------------------------------------------------------------- |
| 1. 高频开仓     | 每个tick开200美仓位                             | 200 USDT/tick                                                                           |
| 2. 动态停利     | 基础0.12%，但看前面K线振幅调整                  | 振幅>0.3% → 停利0.25%                                                                   |
| 3. 快速重开     | 一个tick在5分K走到一半完成，立即在现价-0.2%再开 | 30秒开→1分45秒关→再开                                                                   |
| 4. 冷却期       | 基于前面仓位是否完成交易                        | 完成才重开，不然就是5分鐘到了考慮要不要再次開倉，如果價格脫離了掛單位置比較遠，我會開倉 |
| 5. 多单同时成交 | 高点坠落时10笔同时成交                          | 最高2000美持仓                                                                          |
| 6. 网格持续     | 价格坠落后继续开仓，成本往下摊                  | -                                                                                       |
| 7. 打平出场     | closePnL + unrealizedPnL ≥ 1-20美               | 1-20 USDT                                                                               |
| 8. 大行情调整   | 5分振幅23% → 停利2-3%，1000U/仓                 | 我其實希望第一版可以做到大趨勢直接規避掉，不要做單                                      |
| 9. 趋势过滤     | 避免逆向追，可以正向追                          | 大跌时做空✅，做多❌                                                                    |

---

## 項目結構

```
trading-strategy-server/
├── cmd/
│   └── main.go                           # 應用入口
├── internal/
│   ├── domain/                           # 🎯 領域層
│   │   └── strategy/
│   │       ├── strategies/
│   │       │   ├── strategy.go          # 策略介面
│   │       │   └── grid/
│   │       │       ├── grid.go          # GridAggregate
│   │       │       ├── calculator.go    # GridCalculator
│   │       │       └── trend_analyzer.go # 趨勢分析器 ⭐
│   │       └── instance/
│   │           ├── instance.go          # 策略實例
│   │           └── manager.go           # 實例管理器
│   │
│   ├── application/                      # 📋 應用層
│   │   └── strategy_service.go          # 策略應用服務
│   │
│   └── infrastructure/                   # 🔧 基礎設施層
│       ├── config/
│       │   └── config.go
│       ├── logger/
│       │   └── factory.go
│       └── messaging/
│           ├── redis_client.go
│           ├── candle_subscriber.go      # 訂閱市場數據
│           ├── market_data_reader.go     # 讀取最新數據
│           └── signal_publisher.go       # 發布交易信號
├── domain/                               # 公開領域對象
│   └── value_objects/
│       ├── price.go
│       ├── candle.go
│       └── signal.go
├── docs/
│   ├── strategy-improvements.md          # 策略改進文檔
│   └── trend-analysis-strategy.md        # 趨勢分析文檔
├── .env
└── go.mod

外部依賴：
├── go-packages/logger/                   # 統一 Logger
```

---

## DDD 分層說明

### 🎯 **領域層 (Domain Layer)**

- **職責**：封裝核心策略邏輯
- **特點**：
  - 完全獨立，不依賴 Redis/外部服務
  - 可單獨測試
  - 包含 GridAggregate、GridCalculator、TrendAnalyzer
- **範例**：`GridAggregate.ProcessCandleUpdate()` - 純業務邏輯

### 📋 **應用層 (Application Layer)**

- **職責**：編排領域對象，處理用例
- **特點**：
  - 定義端口介面（Port）
  - 協調基礎設施
  - 不包含業務邏輯
- **範例**：`StrategyService.HandleCandleUpdate()` - 編排流程

### 🔧 **基礎設施層 (Infrastructure Layer)**

- **職責**：提供技術實現（Adapter）
- **特點**：
  - 實現應用層定義的介面
  - Redis、Config、Logger
  - 可替換
- **範例**：`RedisSignalPublisher` - 實現 SignalPublisher 介面

---

## ✅ 已完成的功能

### Phase 1: DDD 領域層實作 (2025-10-14)

#### 1. **值對象** (`domain/value_objects/`)

- ✅ **Price** - 價格值對象（驗證 > 0）
- ✅ **Candle** - K線值對象（OHLC + MidLow計算）
- ✅ **Signal** - 信號值對象（Action, Price, Quantity, Reason）

#### 2. **領域服務** (`internal/domain/strategy/`)

- ✅ **GridCalculator** - 網格線計算（等差數列）
- ✅ **GridAggregate** - 網格聚合根（穿越檢測、信號生成）

#### 3. **應用層** (`internal/application/`)

- ✅ **StrategyService** - 策略應用服務（編排領域邏輯）
- ✅ 定義 SignalPublisher 介面（端口）

#### 4. **基礎設施層** (`internal/infrastructure/`)

- ✅ **Config** - 配置管理
- ✅ **Logger** - 日誌工廠
- ✅ **RedisClient** - Redis 客戶端
- ✅ **CandleSubscriber** - 訂閱市場數據
- ✅ **SignalPublisher** - 發布交易信號
- ✅ **MarketDataReader** - 讀取最新市場數據

---

## 📋 當前任務 ⭐ 優先級：高

### **任務 1: 實現趨勢過濾器（TrendAnalyzer）**

**檔案**: `internal/domain/strategy/strategies/grid/trend_analyzer.go`

**功能**：

1. 檢測大趨勢（5分鐘K線變化 > ±0.6%）
2. 大趨勢時跳過交易
3. 趨勢方向過濾（大跌只做空，大漲只做多）
4. 計算波動率（用於動態止盈）

**實現**：

```go
type TrendState string

const (
    STRONG_UPTREND   TrendState = "STRONG_UPTREND"
    STRONG_DOWNTREND TrendState = "STRONG_DOWNTREND"
    RANGING          TrendState = "RANGING"
)

type TrendAnalyzer struct {
    threshold float64 // 0.006 = 0.6%
}

func (ta *TrendAnalyzer) DetectTrend(candle value_objects.Candle) TrendState {
    changeRate := (candle.Close() - candle.Open()) / candle.Open()

    if changeRate > ta.threshold {
        return STRONG_UPTREND
    } else if changeRate < -ta.threshold {
        return STRONG_DOWNTREND
    }

    return RANGING
}

func (ta *TrendAnalyzer) ShouldSkipTrading(trend TrendState) bool {
    return trend == STRONG_UPTREND || trend == STRONG_DOWNTREND
}

func (ta *TrendAnalyzer) CalculateVolatility(candles []value_objects.Candle) float64 {
    // 計算最近10根K線的標準差
}
```

---

### **任務 2: 更新 GridAggregate 整合趨勢過濾**

**檔案**: `internal/domain/strategy/strategies/grid/grid.go`

**修改**：

1. 添加 `TrendAnalyzer` 依賴
2. 在 `ProcessCandleUpdate()` 中檢查趨勢
3. 大趨勢時不生成信號
4. 根據趨勢方向過濾做多/做空

**範例**：

```go
type GridAggregate struct {
    instID        string
    trendAnalyzer *TrendAnalyzer  // ⭐ 新增
    // ...
}

func (g *GridAggregate) ProcessCandleUpdate(candle value_objects.Candle) (*value_objects.Signal, error) {
    // 1. 檢測趨勢
    trend := g.trendAnalyzer.DetectTrend(candle)

    // 2. 大趨勢時跳過
    if g.trendAnalyzer.ShouldSkipTrading(trend) {
        return nil, nil // ⭐ 不生成信號
    }

    // 3. 計算開倉點位（MidLow）
    midLow := candle.MidLow()

    // 4. 判斷是否觸及
    if currentPrice.IsBelowOrEqual(midLow) {
        // 5. 趨勢方向過濾
        if trend == STRONG_DOWNTREND {
            // 只允許做空
            return g.generateShortSignal(midLow)
        } else if trend == STRONG_UPTREND {
            // 只允許做多
            return g.generateLongSignal(midLow)
        } else {
            // 震盪，兩個方向都可以
            return g.generateSignal(midLow)
        }
    }

    return nil, nil
}
```

---

### **任務 3: 添加動態止盈計算**

**修改**: `GridAggregate.CalculateDynamicTakeProfit()`

```go
func (g *GridAggregate) CalculateDynamicTakeProfit(candles []value_objects.Candle) float64 {
    volatility := g.trendAnalyzer.CalculateVolatility(candles)

    if volatility > 0.01 {
        return 0.002 // 0.2% (高波動)
    } else if volatility < 0.005 {
        return 0.0015 // 0.15% (低波動)
    }

    return 0.0018 // 0.18% (默認)
}
```

---

### **任務 4: 更新文檔**

- [x] 更新 CLAUDE.md（刪除過時內容，添加策略修正）
- [ ] 更新 strategy-improvements.md（添加趨勢過濾細節）
- [ ] 創建測試用例

---

## 數據流（完整）

### **開倉流程**

```
========== 市場數據 ==========
Market Data Service
    ↓ OKX WebSocket
    ↓ Publish: market.candle.5m.ETH-USDT-SWAP
Redis Pub/Sub

========== 策略計算 ==========
Trading Strategy Service (訂閱)
    ↓ Candle: {open: 2500, close: 2510, low: 2490}
    ↓
    ↓ TrendAnalyzer.DetectTrend()
    ↓   changeRate = (2510 - 2500) / 2500 = 0.4%
    ↓   → RANGING ✅ (< 0.6%)
    ↓
    ↓ GridAggregate.ProcessCandleUpdate()
    ↓   MidLow = (2490 + 2510) / 2 = 2500
    ↓   currentPrice = 2498
    ↓   2498 <= 2500? Yes ✅
    ↓
    ↓ GenerateSignal()
    ↓   Signal: {
    ↓     Action: BUY,
    ↓     Price: 2500,
    ↓     PositionSize: 200,
    ↓     TakeProfit: 0.0018,
    ↓     Reason: "hit_mid_low_2500"
    ↓   }
    ↓
    ↓ Publish: strategy.signals.ETH-USDT-SWAP
Redis Pub/Sub

========== 訂單執行 ==========
Order Service (訂閱)
    ↓ Receive Signal
    ↓
    ↓ Validate:
    ↓   - 餘額充足? ✅
    ↓   - 倉位未滿? ✅
    ↓   - 冷卻期結束? ✅
    ↓
    ↓ Calculate Quantity:
    ↓   quantity = 200 / 2500 = 0.08 ETH
    ↓
    ↓ Execute:
OKX API: PlaceOrder(BUY, 0.08 ETH, 2500)
```

---

## 📚 相關文檔

- [項目整體架構](../../CLAUDE.md)
- [Market Data Service](../market-data-server/CLAUDE.md)
- [Order Service](../order-service/CLAUDE.md)
- [Backtesting Engine](../backtesting/CLAUDE.md)

---

## 🤝 開發規範

### Git Commit 規範

```
feat: 新增功能
fix: 修復 bug
refactor: 重構代碼
docs: 文檔更新
test: 測試相關
chore: 其他雜項
```

### 代碼規範

- 使用 `gofmt` 格式化代碼
- 每個 public 函數都需要註釋
- 錯誤處理不能忽略
- 使用 context 管理生命週期
- **依賴注入優先於全局變量**
- **領域層完全獨立，不依賴基礎設施**
- **應用層定義端口，基礎設施層實現適配器**

---

## 🏆 設計原則

1. **單一職責** - 只負責策略計算和信號生成
2. **關注點分離** - 數據訂閱、策略邏輯、信號發布分離
3. **可測試性** - 策略邏輯可獨立測試（不需要 Redis）
4. **可擴展性** - 易於添加新策略類型
5. **依賴反轉** - 應用層定義介面，基礎設施層實現

---

## 💡 為什麼選擇 DDD？

**Trading Strategy Server 使用 DDD 的原因**：

- ✅ 網格策略是**複雜的業務邏輯**（網格線計算、穿越檢測、趨勢過濾）
- ✅ 需要**高度可測試性**（獨立測試業務邏輯，無需 Redis）
- ✅ 策略算法會**頻繁變化**（優化、回測、參數調整）
- ✅ 將來要添加**多種策略**（Grid, DCA, Martingale）

**對比 Market Data Service**：

- Market Data Service 主要是**數據轉發**（OKX → Redis），業務邏輯簡單
- Trading Strategy Server 是**業務核心**（策略計算），需要 DDD 保護業務邏輯純粹性

---

_最後更新: 2025-10-26_
_架構模式: DDD + Hybrid Model（Strategy 推送信號，Order 驗證執行）⭐_
_當前進度: 基礎架構完成，待實現趨勢過濾器和動態止盈_
_下一步: 實現 TrendAnalyzer，更新 GridAggregate，添加測試_
