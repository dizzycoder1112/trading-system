package grid

import (
	"testing"
	"time"

	"dizzycode.xyz/shared/domain/value_objects"
)

// TestTrendAnalyzer_DetectTrend 测试趋势检测功能
func TestTrendAnalyzer_DetectTrend(t *testing.T) {
	analyzer := NewTrendAnalyzer(TrendAnalyzerConfig{
		EMAThreshold:    0.005, // 0.5%
		CandleThreshold: 0.006, // 0.6%
		EMAShortPeriod:  20,
		EMALongPeriod:   50,
	})

	tests := []struct {
		name     string
		candles  []value_objects.Candle
		expected TrendState
	}{
		{
			name:     "震荡行情 - 价格波动小",
			candles:  generateRangingCandles(60, 2500.0, 0.001), // 波动 0.1%
			expected: RANGING,
		},
		{
			name:     "上升趋势 - 持续上涨",
			candles:  generateTrendingCandles(60, 2500.0, 0.01), // 上涨 1%/根
			expected: STRONG_UPTREND,
		},
		{
			name:     "下降趋势 - 持续下跌",
			candles:  generateTrendingCandles(60, 2500.0, -0.01), // 下跌 1%/根
			expected: STRONG_DOWNTREND,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.DetectTrend(tt.candles)
			if result != tt.expected {
				t.Errorf("DetectTrend() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestTrendAnalyzer_CanOpenLong 测试是否允许开多单
func TestTrendAnalyzer_CanOpenLong(t *testing.T) {
	analyzer := NewTrendAnalyzer(TrendAnalyzerConfig{
		EMAThreshold:    0.005,
		CandleThreshold: 0.006,
		EMAShortPeriod:  20,
		EMALongPeriod:   50,
	})

	tests := []struct {
		name     string
		candles  []value_objects.Candle
		expected bool
	}{
		{
			name:     "震荡行情 - 允许开多",
			candles:  generateRangingCandles(60, 2500.0, 0.001),
			expected: true,
		},
		{
			name:     "下降趋势 - 禁止开多",
			candles:  generateTrendingCandles(60, 2500.0, -0.01),
			expected: false,
		},
		{
			name:     "上升趋势 - 允许开多",
			candles:  generateTrendingCandles(60, 2500.0, 0.01),
			expected: true,
		},
		{
			name:     "单根大跌 - 禁止开多",
			candles:  generateCandlesWithSharpMove(60, 2500.0, -0.008), // 最后一根跌 0.8%
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.CanOpenLong(tt.candles)
			if result != tt.expected {
				info := analyzer.GetTrendInfo(tt.candles)
				t.Errorf("CanOpenLong() = %v, want %v\nTrendInfo: %+v", result, tt.expected, info)
			}
		})
	}
}

// TestTrendAnalyzer_CalculateEMA 测试 EMA 计算
func TestTrendAnalyzer_CalculateEMA(t *testing.T) {
	analyzer := NewTrendAnalyzer(TrendAnalyzerConfig{})

	// 创建简单的测试数据：价格恒定为 100
	candles := make([]value_objects.Candle, 30)
	for i := range candles {
		candle, _ := value_objects.NewCandle(100.0, 100.0, 100.0, 100.0, time.Now())
		candles[i] = candle
	}

	ema := analyzer.calculateEMA(candles, 20)

	// EMA 应该接近 100
	if ema < 99.9 || ema > 100.1 {
		t.Errorf("calculateEMA() = %v, want ~100", ema)
	}
}

// TestTrendAnalyzer_GetTrendInfo 测试获取趋势信息
func TestTrendAnalyzer_GetTrendInfo(t *testing.T) {
	analyzer := NewTrendAnalyzer(TrendAnalyzerConfig{
		EMAThreshold:    0.005,
		CandleThreshold: 0.006,
		EMAShortPeriod:  20,
		EMALongPeriod:   50,
	})

	candles := generateRangingCandles(60, 2500.0, 0.001)
	info := analyzer.GetTrendInfo(candles)

	if info.Status == "" {
		t.Error("GetTrendInfo() returned empty status")
	}

	if info.EMAShort == 0 || info.EMALong == 0 {
		t.Error("GetTrendInfo() returned zero EMA values")
	}

	t.Logf("Trend Info: %+v", info)
}

// === 辅助函数：生成测试数据 ===

// generateRangingCandles 生成震荡行情的K线数据
func generateRangingCandles(count int, startPrice float64, volatility float64) []value_objects.Candle {
	candles := make([]value_objects.Candle, count)
	price := startPrice

	for i := range candles {
		// 随机波动（使用简单的正弦函数模拟）
		change := volatility * float64(i%10-5) / 5.0
		open := price
		close := price * (1 + change)
		high := max(open, close) * 1.001
		low := min(open, close) * 0.999

		candle, _ := value_objects.NewCandle(open, high, low, close, time.Now().Add(time.Duration(i)*5*time.Minute))
		candles[i] = candle

		price = close
	}

	return candles
}

// generateTrendingCandles 生成趋势行情的K线数据
func generateTrendingCandles(count int, startPrice float64, trendRate float64) []value_objects.Candle {
	candles := make([]value_objects.Candle, count)
	price := startPrice

	for i := range candles {
		open := price
		close := open * (1 + trendRate)
		high := max(open, close) * 1.001
		low := min(open, close) * 0.999

		candle, _ := value_objects.NewCandle(open, high, low, close, time.Now().Add(time.Duration(i)*5*time.Minute))
		candles[i] = candle

		price = close
	}

	return candles
}

// generateCandlesWithSharpMove 生成最后一根K线有剧烈波动的数据
func generateCandlesWithSharpMove(count int, startPrice float64, lastMove float64) []value_objects.Candle {
	// 前面的K线都是正常震荡
	candles := generateRangingCandles(count-1, startPrice, 0.001)

	// 最后一根K线剧烈波动
	lastCandle := candles[len(candles)-1]
	open := lastCandle.Close().Value()
	close := open * (1 + lastMove)
	high := max(open, close) * 1.001
	low := min(open, close) * 0.999

	sharpCandle, _ := value_objects.NewCandle(open, high, low, close, time.Now())
	candles = append(candles, sharpCandle)

	return candles
}

// min 返回两个浮点数中的较小值
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max 返回两个浮点数中的较大值
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
