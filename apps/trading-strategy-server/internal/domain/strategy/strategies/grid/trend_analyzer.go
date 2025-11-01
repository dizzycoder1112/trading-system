package grid

import (
	"dizzycode.xyz/shared/domain/value_objects"
)

// TrendState 趋势状态
type TrendState string

const (
	STRONG_UPTREND   TrendState = "STRONG_UPTREND"   // 强上升趋势
	STRONG_DOWNTREND TrendState = "STRONG_DOWNTREND" // 强下降趋势
	RANGING          TrendState = "RANGING"          // 震荡
)

// TrendAnalyzer 趋势分析器（领域服务）⭐
// 特点：
// 1. 无状态设计（纯函数）
// 2. 可独立测试
// 3. 可复用（其他策略也可以使用）
// 4. 遵循 DRY 原则
type TrendAnalyzer struct {
	emaThreshold       float64 // EMA 差距阈值（例如 0.005 = 0.5%）
	candleThreshold    float64 // 单根K线幅度阈值（例如 0.006 = 0.6%）
	emaShortPeriod     int     // 短期 EMA 周期（默认 20）
	emaLongPeriod      int     // 长期 EMA 周期（默认 50）
	priceDropThreshold float64 // 价格跌幅阈值（例如 0.02 = 2%）⭐ 新增
	consecutivePeriod  int     // 连续阴线检测周期（默认 10）⭐ 新增
}

// TrendAnalyzerConfig 趋势分析器配置
type TrendAnalyzerConfig struct {
	EMAThreshold       float64 // EMA 差距阈值
	CandleThreshold    float64 // 单根K线幅度阈值
	EMAShortPeriod     int     // 短期 EMA 周期
	EMALongPeriod      int     // 长期 EMA 周期
	PriceDropThreshold float64 // 价格跌幅阈值 ⭐ 新增
	ConsecutivePeriod  int     // 连续阴线检测周期 ⭐ 新增
}

// NewTrendAnalyzer 创建趋势分析器（工厂方法）
func NewTrendAnalyzer(config TrendAnalyzerConfig) *TrendAnalyzer {
	// 设置默认值
	if config.EMAThreshold <= 0 {
		config.EMAThreshold = 0.005 // 0.5%
	}
	if config.CandleThreshold <= 0 {
		config.CandleThreshold = 0.006 // 0.6%
	}
	if config.EMAShortPeriod <= 0 {
		config.EMAShortPeriod = 20
	}
	if config.EMALongPeriod <= 0 {
		config.EMALongPeriod = 50
	}
	if config.PriceDropThreshold <= 0 {
		config.PriceDropThreshold = 0.008 // 0.8% ⭐ 修改：更敏感
	}
	if config.ConsecutivePeriod <= 0 {
		config.ConsecutivePeriod = 5 // 5根K线 ⭐ 修改：更短周期
	}

	return &TrendAnalyzer{
		emaThreshold:       config.EMAThreshold,
		candleThreshold:    config.CandleThreshold,
		emaShortPeriod:     config.EMAShortPeriod,
		emaLongPeriod:      config.EMALongPeriod,
		priceDropThreshold: config.PriceDropThreshold, // ⭐ 新增
		consecutivePeriod:  config.ConsecutivePeriod,  // ⭐ 新增
	}
}

// DetectTrend 检测市场趋势（基于 EMA 交叉）⭐
// 参数：
//   - candles: K线历史数据（需要至少 50 根）
//
// 返回：
//   - TrendState: 趋势状态
func (ta *TrendAnalyzer) DetectTrend(candles []value_objects.Candle) TrendState {
	if len(candles) < ta.emaLongPeriod {
		return RANGING // 数据不足，默认震荡
	}

	// 计算 EMA
	emaShort := ta.calculateEMA(candles, ta.emaShortPeriod)
	emaLong := ta.calculateEMA(candles, ta.emaLongPeriod)

	// 计算差距百分比
	diff := (emaShort - emaLong) / emaLong

	if diff > ta.emaThreshold {
		return STRONG_UPTREND // EMA 20 明显高于 EMA 50
	} else if diff < -ta.emaThreshold {
		return STRONG_DOWNTREND // EMA 20 明显低于 EMA 50
	}

	return RANGING // 两条 EMA 接近，震荡行情
}

// CanOpenLong 是否允许开多单 ⭐ 方案3：多信号组合检测
// 参数：
//   - candles: K线历史数据
//
// 返回：
//   - bool: true = 可以开多单，false = 禁止开多单
//
// 逻辑（任一条件触发则禁止开多单）：
//   1. 检查单根K线是否剧烈下跌（快速响应）
//   2. 检查价格跌幅（最近20根K线跌幅 > 2%）⭐ 新增
//   3. 检查连续阴线（最近10根K线中80%是阴线）⭐ 新增
//   4. 检查 EMA 趋势是否为下降趋势（整体判断）
func (ta *TrendAnalyzer) CanOpenLong(candles []value_objects.Candle) bool {
	if len(candles) < ta.emaLongPeriod {
		return true // 数据不足，默认允许（保守策略）
	}

	// 检查 1: 单根K线剧烈下跌 → 禁止开多单（快速响应）
	latestCandle := candles[len(candles)-1]
	candleChange := (latestCandle.Close().Value() - latestCandle.Open().Value()) / latestCandle.Open().Value()

	if candleChange < -ta.candleThreshold {
		// 当前K线大跌（例如 -0.6%），立即禁止开多单
		return false
	}

	// 检查 2: 价格跌幅检测 → 最近10根K线跌幅 > 0.8% 禁止开多单 ⭐ 修改：更短周期
	lookbackPeriod := 10 // 缩短到10根K线
	if len(candles) >= lookbackPeriod {
		priceChange := ta.calculatePriceChange(candles, lookbackPeriod)
		if priceChange < -ta.priceDropThreshold {
			// 价格持续下跌超过阈值，禁止开多单
			return false
		}
	}

	// 检查 3: 连续阴线检测 → 60%以上是阴线禁止开多单 ⭐ 修改：降低阈值
	if len(candles) >= ta.consecutivePeriod {
		bearishCount := ta.countConsecutiveBearish(candles, ta.consecutivePeriod)
		threshold := int(float64(ta.consecutivePeriod) * 0.6) // 60%阈值（更敏感）
		if bearishCount >= threshold {
			// 连续阴线过多，市场处于下跌趋势，禁止开多单
			return false
		}
	}

	// 检查 4: EMA 趋势检测 → 下降趋势禁止开多单
	trend := ta.DetectTrend(candles)
	if trend == STRONG_DOWNTREND {
		// 下降趋势，禁止开多单
		return false
	}

	// 通过所有检查，允许开多单
	return true
}

// CanOpenShort 是否允许开空单 ⭐
// 参数：
//   - candles: K线历史数据
//
// 返回：
//   - bool: true = 可以开空单，false = 禁止开空单
//
// 逻辑：
//   1. 检查单根K线是否剧烈上涨（快速响应）
//   2. 检查 EMA 趋势是否为上升趋势（整体判断）
//   3. 任一条件触发则禁止开空单
func (ta *TrendAnalyzer) CanOpenShort(candles []value_objects.Candle) bool {
	if len(candles) < ta.emaLongPeriod {
		return true // 数据不足，默认允许
	}

	// 检查 1: 单根K线剧烈上涨 → 禁止开空单
	latestCandle := candles[len(candles)-1]
	candleChange := (latestCandle.Close().Value() - latestCandle.Open().Value()) / latestCandle.Open().Value()

	if candleChange > ta.candleThreshold {
		return false
	}

	// 检查 2: EMA 趋势检测 → 上升趋势禁止开空单
	trend := ta.DetectTrend(candles)
	if trend == STRONG_UPTREND {
		return false
	}

	return true
}

// calculateEMA 计算指数移动平均线（EMA）⭐
// 参数：
//   - candles: K线历史数据
//   - period: EMA 周期
//
// 返回：
//   - float64: EMA 值
//
// 算法：
//   1. 先计算初始 SMA（简单移动平均）
//   2. 再使用指数加权递推计算 EMA
func (ta *TrendAnalyzer) calculateEMA(candles []value_objects.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}

	// 1. 计算初始 SMA（简单移动平均）
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close().Value()
	}
	ema := sum / float64(period)

	// 2. 计算 EMA（指数加权）
	// EMA(t) = (Close(t) - EMA(t-1)) * multiplier + EMA(t-1)
	// multiplier = 2 / (period + 1)
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(candles); i++ {
		ema = (candles[i].Close().Value()-ema)*multiplier + ema
	}

	return ema
}

// calculatePriceChange 计算价格变化百分比 ⭐
// 参数：
//   - candles: K线历史数据
//   - period: 回溯周期
//
// 返回：
//   - float64: 价格变化百分比（例如 -0.02 = -2%）
func (ta *TrendAnalyzer) calculatePriceChange(candles []value_objects.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}

	startPrice := candles[len(candles)-period].Close().Value()
	endPrice := candles[len(candles)-1].Close().Value()

	return (endPrice - startPrice) / startPrice
}

// countConsecutiveBearish 统计连续阴线数量 ⭐
// 参数：
//   - candles: K线历史数据
//   - period: 检测周期
//
// 返回：
//   - int: 连续阴线数量
func (ta *TrendAnalyzer) countConsecutiveBearish(candles []value_objects.Candle, period int) int {
	if len(candles) < period {
		return 0
	}

	bearishCount := 0
	startIdx := len(candles) - period

	for i := startIdx; i < len(candles); i++ {
		if candles[i].IsBearish() { // 收盘价 < 开盘价
			bearishCount++
		}
	}

	return bearishCount
}

// GetTrendInfo 获取趋势详细信息（用于日志调试）⭐
// 参数：
//   - candles: K线历史数据
//
// 返回：
//   - TrendInfo: 趋势详细信息
func (ta *TrendAnalyzer) GetTrendInfo(candles []value_objects.Candle) TrendInfo {
	if len(candles) < ta.emaLongPeriod {
		return TrendInfo{
			Status:      "insufficient_data",
			MinRequired: ta.emaLongPeriod,
			Current:     len(candles),
		}
	}

	latestCandle := candles[len(candles)-1]
	candleChange := (latestCandle.Close().Value() - latestCandle.Open().Value()) / latestCandle.Open().Value()

	emaShort := ta.calculateEMA(candles, ta.emaShortPeriod)
	emaLong := ta.calculateEMA(candles, ta.emaLongPeriod)
	emaDiff := (emaShort - emaLong) / emaLong

	trend := ta.DetectTrend(candles)

	// 计算新增的检测指标 ⭐
	var priceChange20 float64
	var bearishCount int
	if len(candles) >= 20 {
		priceChange20 = ta.calculatePriceChange(candles, 20)
	}
	if len(candles) >= ta.consecutivePeriod {
		bearishCount = ta.countConsecutiveBearish(candles, ta.consecutivePeriod)
	}

	return TrendInfo{
		Status:            string(trend),
		EMAShort:          emaShort,
		EMALong:           emaLong,
		EMADiffPercent:    emaDiff * 100,
		CandleChange:      candleChange * 100,
		PriceChange20:     priceChange20 * 100,    // ⭐ 新增：最近20根K线价格变化
		BearishCount:      bearishCount,           // ⭐ 新增：连续阴线数量
		BearishThreshold:  ta.consecutivePeriod,   // ⭐ 新增：检测周期
		CanOpenLong:       ta.CanOpenLong(candles),
		CanOpenShort:      ta.CanOpenShort(candles),
		LatestPrice:       latestCandle.Close().Value(),
		MinRequired:       ta.emaLongPeriod,
		Current:           len(candles),
	}
}

// TrendInfo 趋势详细信息（用于日志）
type TrendInfo struct {
	Status           string  // 趋势状态
	EMAShort         float64 // 短期 EMA
	EMALong          float64 // 长期 EMA
	EMADiffPercent   float64 // EMA 差距百分比
	CandleChange     float64 // 最新K线变化百分比
	PriceChange20    float64 // ⭐ 新增：最近20根K线价格变化百分比
	BearishCount     int     // ⭐ 新增：连续阴线数量
	BearishThreshold int     // ⭐ 新增：阴线检测周期
	CanOpenLong      bool    // 是否允许开多单
	CanOpenShort     bool    // 是否允许开空单
	LatestPrice      float64 // 最新价格
	MinRequired      int     // 最少需要的K线数量
	Current          int     // 当前K线数量
}
