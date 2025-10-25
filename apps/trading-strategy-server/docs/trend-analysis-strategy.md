# 盤勢分析策略設計文檔

> **創建日期**: 2025-10-19
> **狀態**: 🔄 設計中
> **架構模式**: Strategy Pattern（策略模式）

---

## 一、盤勢定義

### 1.1 五種盤勢類型

| 盤勢     | 英文       | 描述                 | 策略影響               |
| -------- | ---------- | -------------------- | ---------------------- |
| **急漲** | Rapid Rise | 短時間內大幅上漲     | 可能暫停開多單         |
| **緩漲** | Slow Rise  | 溫和上漲趨勢         | 正常開多單             |
| **平盤** | Flat       | 橫盤整理，無明顯趨勢 | 網格策略最佳時機 ⭐    |
| **緩跌** | Slow Fall  | 溫和下跌趨勢         | 正常開多單（逢低買入） |
| **急跌** | Rapid Fall | 短時間內大幅下跌     | 可能暫停開倉           |

### 1.2 盤勢枚舉定義

```go
// Trend 盤勢類型
type Trend int

const (
    TrendRapidRise Trend = iota  // 急漲
    TrendSlowRise                // 緩漲
    TrendFlat                    // 平盤
    TrendSlowFall                // 緩跌
    TrendRapidFall               // 急跌
)

func (t Trend) String() string {
    return [...]string{
        "rapid_rise",
        "slow_rise",
        "flat",
        "slow_fall",
        "rapid_fall",
    }[t]
}
```

---

## 二、策略模式設計 ⭐

### 2.1 架構圖

```
                    ┌─────────────────────┐
                    │  TrendAnalyzer      │
                    │   (Interface)       │
                    └──────────┬──────────┘
                               │
                               │ implements
          ┌────────────────────┼────────────────────┐
          │                    │                    │
          ▼                    ▼                    ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│ PriceChangeRate  │ │ SlopeVolatility  │ │ CandlePattern    │
│    Analyzer      │ │    Analyzer      │ │    Analyzer      │
│   (方案 A)        ││   (方案 B)        │  │  (方案 C)        │
└──────────────────┘ └──────────────────┘ └──────────────────┘
```

### 2.2 介面定義

```go
// TrendAnalyzer 盤勢分析器介面（策略模式）
type TrendAnalyzer interface {
    // AnalyzeTrend 分析盤勢
    // 參數：最近 N 根 K 線
    // 返回：盤勢類型
    AnalyzeTrend(candles []value_objects.Candle) Trend

    // GetName 獲取分析器名稱
    GetName() string
}
```

### 2.3 使用方式

```go
// 創建分析器（可切換！）
var analyzer TrendAnalyzer

// 使用方案 A（推薦）
analyzer = NewPriceChangeRateAnalyzer(config)

// 或使用方案 B
// analyzer = NewSlopeVolatilityAnalyzer(config)

// 或使用方案 C
// analyzer = NewCandlePatternAnalyzer(config)

// 分析盤勢
trend := analyzer.AnalyzeTrend(recentCandles)

// 根據盤勢決策
if trend == TrendFlat {
    // 平盤時開倉
}
```

---

## 三、方案 A：價格變化率分析器 ⭐ 推薦

### 3.1 原理

計算最近 N 根 K 線的價格變化百分比：

```
priceChange = (最新收盤價 - N 根前收盤價) / N 根前收盤價 × 100
```

### 3.2 參數配置

```go
type PriceChangeRateConfig struct {
    SampleSize int     // 取樣數量（建議 5 根）

    // 閾值（百分比）
    RapidRiseThreshold float64  // 急漲閾值（> 2%）
    SlowRiseThreshold  float64  // 緩漲閾值（> 0.5%）
    FlatThreshold      float64  // 平盤閾值（± 0.5%）
    SlowFallThreshold  float64  // 緩跌閾值（< -0.5%）
    RapidFallThreshold float64  // 急跌閾值（< -2%）
}

// 默認配置（基於 5 分鐘 K 線）
func DefaultPriceChangeRateConfig() PriceChangeRateConfig {
    return PriceChangeRateConfig{
        SampleSize:         5,     // 5 根 5 分鐘 K 線 = 25 分鐘
        RapidRiseThreshold: 2.0,   // 25 分鐘內漲 > 2%
        SlowRiseThreshold:  0.5,   // 25 分鐘內漲 > 0.5%
        FlatThreshold:      0.5,   // 25 分鐘內變化 < ± 0.5%
        SlowFallThreshold:  -0.5,  // 25 分鐘內跌 > 0.5%
        RapidFallThreshold: -2.0,  // 25 分鐘內跌 > 2%
    }
}
```

### 3.3 判斷邏輯

```go
func (a *PriceChangeRateAnalyzer) AnalyzeTrend(candles []value_objects.Candle) Trend {
    if len(candles) < a.config.SampleSize {
        return TrendFlat  // 數據不足，默認平盤
    }

    // 取最新 N 根
    recent := candles[len(candles)-a.config.SampleSize:]

    // 計算價格變化率
    oldPrice := recent[0].Close().Value()
    newPrice := recent[len(recent)-1].Close().Value()
    priceChange := (newPrice - oldPrice) / oldPrice * 100

    // 判斷盤勢
    switch {
    case priceChange > a.config.RapidRiseThreshold:
        return TrendRapidRise
    case priceChange > a.config.SlowRiseThreshold:
        return TrendSlowRise
    case priceChange < a.config.RapidFallThreshold:
        return TrendRapidFall
    case priceChange < a.config.SlowFallThreshold:
        return TrendSlowFall
    default:
        return TrendFlat
    }
}
```

### 3.4 示例

**場景 1：急漲（ETH 5 分鐘線）**

```
時間        收盤價    變化
10:00      3800      -
10:05      3820      +0.5%
10:10      3840      +1.05%
10:15      3860      +1.58%
10:20      3880      +2.11%  ← 5 根累計 +2.11% → 急漲 ⚡
```

**場景 2：平盤（橫盤整理）**

```
時間        收盤價    變化
10:00      3800      -
10:05      3805      +0.13%
10:10      3798      -0.05%
10:15      3802      +0.05%
10:20      3806      +0.16%  ← 5 根累計 +0.16% → 平盤 📊
```

### 3.5 優點與缺點

**優點** ✅

- 計算簡單，性能高
- 參數容易理解和調整
- 對短期趨勢反應靈敏

**缺點** ⚠️

- 對突發價格跳動敏感（如大單）
- 沒有考慮波動率
- 可能受異常值影響

---

## 四、方案 B：斜率波動率分析器

### 4.1 原理

結合兩個指標：

1. **斜率（線性回歸）** - 判斷趨勢方向
2. **波動率（標準差）** - 判斷趨勢強度

### 4.2 參數配置

```go
type SlopeVolatilityConfig struct {
    SampleSize int  // 取樣數量

    // 斜率閾值
    RiseSlopeThreshold float64   // 上漲斜率閾值
    FallSlopeThreshold float64   // 下跌斜率閾值

    // 波動率閾值
    HighVolatilityThreshold float64  // 高波動率閾值（急）
}

func DefaultSlopeVolatilityConfig() SlopeVolatilityConfig {
    return SlopeVolatilityConfig{
        SampleSize:              5,
        RiseSlopeThreshold:      0.5,   // 斜率 > 0.5 為上漲
        FallSlopeThreshold:      -0.5,  // 斜率 < -0.5 為下跌
        HighVolatilityThreshold: 30.0,  // 標準差 > 30 為高波動
    }
}
```

### 4.3 計算方法

**線性回歸斜率**：

```
斜率 = Σ(x - x̄)(y - ȳ) / Σ(x - x̄)²

其中：
  x = K 線索引（1, 2, 3, 4, 5）
  y = 收盤價
  x̄ = x 的平均值
  ȳ = y 的平均值
```

**波動率（標準差）**：

```
σ = √(Σ(price - avgPrice)² / N)
```

### 4.4 判斷邏輯

```go
func (a *SlopeVolatilityAnalyzer) AnalyzeTrend(candles []value_objects.Candle) Trend {
    if len(candles) < a.config.SampleSize {
        return TrendFlat
    }

    recent := candles[len(candles)-a.config.SampleSize:]

    // 1. 計算斜率
    slope := a.calculateSlope(recent)

    // 2. 計算波動率
    volatility := a.calculateVolatility(recent)

    // 3. 判斷盤勢
    isHighVolatility := volatility > a.config.HighVolatilityThreshold

    switch {
    case slope > a.config.RiseSlopeThreshold && isHighVolatility:
        return TrendRapidRise
    case slope > a.config.RiseSlopeThreshold:
        return TrendSlowRise
    case slope < a.config.FallSlopeThreshold && isHighVolatility:
        return TrendRapidFall
    case slope < a.config.FallSlopeThreshold:
        return TrendSlowFall
    default:
        return TrendFlat
    }
}
```

### 4.5 優點與缺點

**優點** ✅

- 更準確（考慮波動性）
- 抗噪音（線性回歸平滑數據）
- 適合趨勢交易

**缺點** ⚠️

- 計算複雜，性能較低
- 參數調整需要大量回測
- 對短期變化反應較慢

---

## 五、方案 C：K 線形態分析器

### 5.1 原理

基於最近 3 根 K 線的形態組合判斷盤勢。

### 5.2 參數配置

```go
type CandlePatternConfig struct {
    SampleSize          int     // 取樣數量（固定 3 根）
    BigBodyThreshold    float64 // 大實體閾值（> 1%）
    SmallBodyThreshold  float64 // 小實體閾值（< 0.3%）
}

func DefaultCandlePatternConfig() CandlePatternConfig {
    return CandlePatternConfig{
        SampleSize:         3,
        BigBodyThreshold:   1.0,   // 實體 > 1% 為大陽/大陰
        SmallBodyThreshold: 0.3,   // 實體 < 0.3% 為小波動
    }
}
```

### 5.3 判斷邏輯

```go
func (a *CandlePatternAnalyzer) AnalyzeTrend(candles []value_objects.Candle) Trend {
    if len(candles) < 3 {
        return TrendFlat
    }

    recent := candles[len(candles)-3:]

    // 計算每根 K 線的實體百分比
    bodies := make([]float64, 3)
    bullishCount := 0
    bearishCount := 0
    bigBodyCount := 0

    for i, candle := range recent {
        bodyPct := (candle.Close().Value() - candle.Open().Value()) / candle.Open().Value() * 100
        bodies[i] = bodyPct

        if bodyPct > 0 {
            bullishCount++
        } else if bodyPct < 0 {
            bearishCount++
        }

        if math.Abs(bodyPct) > a.config.BigBodyThreshold {
            bigBodyCount++
        }
    }

    // 判斷盤勢
    switch {
    case bullishCount == 3 && bigBodyCount >= 2:
        return TrendRapidRise  // 3 連陽 + 大實體
    case bullishCount >= 2:
        return TrendSlowRise   // 2-3 陽線
    case bearishCount == 3 && bigBodyCount >= 2:
        return TrendRapidFall  // 3 連陰 + 大實體
    case bearishCount >= 2:
        return TrendSlowFall   // 2-3 陰線
    default:
        return TrendFlat       // 混雜或小實體
    }
}
```

### 5.4 優點與缺點

**優點** ✅

- 最簡單，易於理解
- 符合交易者直覺
- 計算速度最快

**缺點** ⚠️

- 過於粗糙
- 容易誤判（如假突破）
- 不考慮價格變化幅度

---

## 六、策略選擇建議

### 6.1 選擇矩陣

| 場景         | 推薦方案 | 理由               |
| ------------ | -------- | ------------------ |
| **初期開發** | 方案 A   | 簡單、快速、易調參 |
| **生產環境** | 方案 A   | 性能好、準確度足夠 |
| **趨勢交易** | 方案 B   | 更準確識別趨勢     |
| **快速驗證** | 方案 C   | 最簡單             |

### 6.2 切換策略

```go
// 通過配置選擇策略
func CreateTrendAnalyzer(strategyType string, config interface{}) TrendAnalyzer {
    switch strategyType {
    case "price_change_rate":
        return NewPriceChangeRateAnalyzer(config.(PriceChangeRateConfig))
    case "slope_volatility":
        return NewSlopeVolatilityAnalyzer(config.(SlopeVolatilityConfig))
    case "candle_pattern":
        return NewCandlePatternAnalyzer(config.(CandlePatternConfig))
    default:
        return NewPriceChangeRateAnalyzer(DefaultPriceChangeRateConfig())
    }
}
```

---

## 七、整合到網格策略

### 7.1 GridAggregate 整合

```go
type GridAggregate struct {
    instID        string
    positionSize  float64
    takeProfitMin float64
    takeProfitMax float64
    trendAnalyzer TrendAnalyzer  // ⭐ 新增
    calculator    *GridCalculator
}

func NewGridAggregate(
    instID string,
    positionSize, takeProfitMin, takeProfitMax float64,
    trendAnalyzer TrendAnalyzer,  // ⭐ 注入
) (*GridAggregate, error) {
    return &GridAggregate{
        instID:        instID,
        positionSize:  positionSize,
        takeProfitMin: takeProfitMin,
        takeProfitMax: takeProfitMax,
        trendAnalyzer: trendAnalyzer,  // ⭐ 保存
        calculator:    NewGridCalculator(),
    }, nil
}
```

### 7.2 開倉建議整合盤勢判斷

```go
func (g *GridAggregate) GetOpenAdvice(
    currentPrice value_objects.Price,
    lastCandle value_objects.Candle,
    recentCandles []value_objects.Candle,  // ⭐ 新增：用於盤勢分析
) OpenAdvice {
    // 1. 分析盤勢 ⭐
    trend := g.trendAnalyzer.AnalyzeTrend(recentCandles)

    // 2. 根據盤勢決定是否開倉
    if trend == TrendRapidRise || trend == TrendRapidFall {
        // 急漲/急跌時不開倉
        return OpenAdvice{
            ShouldOpen: false,
            Reason:     fmt.Sprintf("trend_too_volatile_%s", trend.String()),
        }
    }

    // 3. 計算開倉位置
    midLow := lastCandle.MidLow()

    // 4. 判斷價格是否觸及
    if currentPrice.IsBelowOrEqual(midLow) {
        return OpenAdvice{
            ShouldOpen:   true,
            Price:        midLow.Value(),
            PositionSize: g.positionSize,
            TakeProfit:   (g.takeProfitMin + g.takeProfitMax) / 2.0,
            Reason:       fmt.Sprintf("hit_mid_low_trend_%s", trend.String()),
            Trend:        trend,  // ⭐ 新增：返回盤勢信息
        }
    }

    return OpenAdvice{
        ShouldOpen: false,
        Reason:     fmt.Sprintf("price_above_mid_low_trend_%s", trend.String()),
    }
}
```

### 7.3 OpenAdvice 結構更新

```go
type OpenAdvice struct {
    ShouldOpen   bool
    Price        float64
    PositionSize float64
    TakeProfit   float64
    Reason       string
    Trend        Trend    // ⭐ 新增：盤勢信息
}
```

---

## 八、數據需求

### 8.1 MarketDataReader 介面更新

```go
type MarketDataReader interface {
    // 獲取最新的已確認 K 線
    GetLastConfirmedCandle(ctx context.Context, instID string, bar string) (*value_objects.Candle, error)

    // ⭐ 新增：獲取最近 N 根已確認 K 線
    GetRecentCandles(ctx context.Context, instID string, bar string, count int) ([]value_objects.Candle, error)
}
```

### 8.2 Redis 讀取

```go
// 從 candle.history.{bar}.{instId} List 讀取最近 N 根
func (r *MarketDataReader) GetRecentCandles(
    ctx context.Context,
    instID string,
    bar string,
    count int,
) ([]value_objects.Candle, error) {
    key := fmt.Sprintf("candle.history.%s.%s", bar, instID)

    // LRANGE key 0 count-1
    vals, err := r.client.Client().LRange(ctx, key, 0, int64(count-1)).Result()
    if err != nil {
        return nil, err
    }

    // 解析並返回（注意：需要反轉順序，因為 LPUSH 是新的在前）
    candles := make([]value_objects.Candle, 0, len(vals))
    for i := len(vals) - 1; i >= 0; i-- {  // 反轉
        // 解析 JSON...
        candles = append(candles, candle)
    }

    return candles, nil
}
```

---

## 九、配置管理

### 9.1 環境變量

```bash
# 盤勢分析配置
TREND_ANALYZER_TYPE=price_change_rate        # 策略類型
TREND_SAMPLE_SIZE=5                          # 取樣數量
TREND_RAPID_RISE_THRESHOLD=2.0               # 急漲閾值
TREND_SLOW_RISE_THRESHOLD=0.5                # 緩漲閾值
TREND_FLAT_THRESHOLD=0.5                     # 平盤閾值
```

### 9.2 Config 結構

```go
type StrategyConfig struct {
    // ... 現有配置 ...

    // ⭐ 新增：盤勢分析配置
    Trend TrendAnalysisConfig
}

type TrendAnalysisConfig struct {
    Type       string  // "price_change_rate", "slope_volatility", "candle_pattern"
    SampleSize int

    // 方案 A 參數
    RapidRiseThreshold float64
    SlowRiseThreshold  float64
    FlatThreshold      float64
    SlowFallThreshold  float64
    RapidFallThreshold float64
}
```

---

## 十、測試計劃

### 10.1 單元測試

```go
func TestPriceChangeRateAnalyzer_AnalyzeTrend(t *testing.T) {
    analyzer := NewPriceChangeRateAnalyzer(DefaultPriceChangeRateConfig())

    // 測試急漲
    t.Run("rapid_rise", func(t *testing.T) {
        candles := createTestCandles([]float64{3800, 3820, 3840, 3860, 3880})
        trend := analyzer.AnalyzeTrend(candles)
        assert.Equal(t, TrendRapidRise, trend)
    })

    // 測試平盤
    t.Run("flat", func(t *testing.T) {
        candles := createTestCandles([]float64{3800, 3805, 3798, 3802, 3806})
        trend := analyzer.AnalyzeTrend(candles)
        assert.Equal(t, TrendFlat, trend)
    })
}
```

### 10.2 回測

使用歷史數據測試各方案的準確率。

---

## 十一、未來優化

### 11.1 機器學習方案

訓練模型自動識別盤勢：

- 輸入：最近 N 根 K 線的 OHLC
- 輸出：盤勢分類（5 種）

### 11.2 自適應閾值

根據歷史波動率動態調整閾值。

### 11.3 多時間週期確認

結合 1m、5m、15m 多個時間週期判斷。

---

## 十二、決策記錄

| 日期       | 決策                   | 原因                   |
| ---------- | ---------------------- | ---------------------- |
| 2025-10-19 | 採用策略模式           | 易於切換、測試不同方案 |
| 2025-10-19 | 初期使用方案 A         | 簡單、性能好、易調參   |
| 待定       | 是否整合盤勢到開倉邏輯 | 需要討論策略影響       |

---

**文檔版本**: 1.0
**最後更新**: 2025-10-19
**狀態**: 🔄 等待確認參數和實作
