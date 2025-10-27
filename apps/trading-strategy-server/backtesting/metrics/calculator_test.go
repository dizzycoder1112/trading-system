package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dizzycode.xyz/trading-strategy-server/backtesting/simulator"
)

// TestMetricsCalculator_Calculate_ProfitableBacktest 测试盈利回测
func TestMetricsCalculator_Calculate_ProfitableBacktest(t *testing.T) {
	calculator := NewMetricsCalculator(10000.0)
	tracker := simulator.NewPositionTracker()
	feeRate := 0.0005

	baseTime := time.Now()

	// 模拟 5 笔交易：3 盈利 + 2 亏损
	trades := []struct {
		entryPrice float64
		closePrice float64
		size       float64
		duration   time.Duration
	}{
		{2500.0, 2510.0, 200.0, 10 * time.Minute}, // 盈利
		{2510.0, 2505.0, 200.0, 5 * time.Minute},  // 亏损
		{2505.0, 2515.0, 200.0, 15 * time.Minute}, // 盈利
		{2515.0, 2510.0, 200.0, 8 * time.Minute},  // 亏损
		{2510.0, 2520.0, 200.0, 12 * time.Minute}, // 盈利
	}

	balance := 10000.0
	calculator.RecordBalance(baseTime, balance)

	for i, trade := range trades {
		openTime := baseTime.Add(time.Duration(i) * 20 * time.Minute)
		closeTime := openTime.Add(trade.duration)

		// 开仓
		pos := tracker.AddPosition(trade.entryPrice, trade.size, openTime, trade.closePrice)

		// 计算盈亏
		priceChange := trade.closePrice - trade.entryPrice
		profitBeforeFee := (priceChange / trade.entryPrice) * trade.size
		fee := trade.size * feeRate * 2
		realizedPnL := profitBeforeFee - fee

		// 更新余额
		balance += realizedPnL

		// 平仓
		tracker.ClosePosition(pos.ID, trade.closePrice, closeTime, realizedPnL)

		// 记录资金快照
		calculator.RecordBalance(closeTime, balance)
	}

	// 计算指标
	result := calculator.Calculate(tracker, balance)

	// 验证基础指标
	assert.Equal(t, 10000.0, result.InitialBalance)
	assert.Greater(t, result.FinalBalance, result.InitialBalance) // 总体盈利
	assert.Equal(t, 5, result.TotalTrades)

	// 验证胜率
	// 3 盈利 / 5 总交易 = 60%
	assert.InDelta(t, 60.0, result.WinRate, 0.1)
	assert.Equal(t, 3, result.WinningTrades)
	assert.Equal(t, 2, result.LosingTrades)

	// 验证总收益率
	expectedReturn := ((result.FinalBalance - result.InitialBalance) / result.InitialBalance) * 100
	assert.InDelta(t, expectedReturn, result.TotalReturn, 0.01)

	// 验证盈亏比
	assert.Greater(t, result.ProfitFactor, 1.0) // 盈利 > 亏损

	// 验证净利润
	assert.Equal(t, result.FinalBalance-result.InitialBalance, result.NetProfit)

	// 验证平均持仓时长
	expectedAvgDuration := (10 + 5 + 15 + 8 + 12) * time.Minute / 5
	assert.Equal(t, expectedAvgDuration, result.AvgHoldDuration)

	t.Logf("✅ Profitable backtest metrics")
	t.Logf("   Initial: $%.2f → Final: $%.2f", result.InitialBalance, result.FinalBalance)
	t.Logf("   Total Return: %.2f%%", result.TotalReturn)
	t.Logf("   Max Drawdown: %.2f%%", result.MaxDrawdown)
	t.Logf("   Win Rate: %.2f%% (%d/%d)", result.WinRate, result.WinningTrades, result.TotalTrades)
	t.Logf("   Profit Factor: %.2f", result.ProfitFactor)
	t.Logf("   Net Profit: $%.2f", result.NetProfit)
	t.Logf("   Avg Hold Duration: %v", result.AvgHoldDuration)
}

// TestMetricsCalculator_Calculate_LosingBacktest 测试亏损回测
func TestMetricsCalculator_Calculate_LosingBacktest(t *testing.T) {
	calculator := NewMetricsCalculator(10000.0)
	tracker := simulator.NewPositionTracker()
	feeRate := 0.0005

	baseTime := time.Now()

	// 模拟 4 笔交易：1 盈利 + 3 亏损
	trades := []struct {
		entryPrice float64
		closePrice float64
		size       float64
		duration   time.Duration
	}{
		{2500.0, 2490.0, 200.0, 10 * time.Minute}, // 亏损
		{2490.0, 2480.0, 200.0, 5 * time.Minute},  // 亏损
		{2480.0, 2490.0, 200.0, 15 * time.Minute}, // 盈利
		{2490.0, 2475.0, 200.0, 8 * time.Minute},  // 亏损
	}

	balance := 10000.0
	calculator.RecordBalance(baseTime, balance)

	for i, trade := range trades {
		openTime := baseTime.Add(time.Duration(i) * 20 * time.Minute)
		closeTime := openTime.Add(trade.duration)

		// 开仓
		pos := tracker.AddPosition(trade.entryPrice, trade.size, openTime, trade.closePrice)

		// 计算盈亏
		priceChange := trade.closePrice - trade.entryPrice
		profitBeforeFee := (priceChange / trade.entryPrice) * trade.size
		fee := trade.size * feeRate * 2
		realizedPnL := profitBeforeFee - fee

		// 更新余额
		balance += realizedPnL

		// 平仓
		tracker.ClosePosition(pos.ID, trade.closePrice, closeTime, realizedPnL)

		// 记录资金快照
		calculator.RecordBalance(closeTime, balance)
	}

	// 计算指标
	result := calculator.Calculate(tracker, balance)

	// 验证基础指标
	assert.Equal(t, 10000.0, result.InitialBalance)
	assert.Less(t, result.FinalBalance, result.InitialBalance) // 总体亏损
	assert.Equal(t, 4, result.TotalTrades)

	// 验证胜率
	// 1 盈利 / 4 总交易 = 25%
	assert.InDelta(t, 25.0, result.WinRate, 0.1)
	assert.Equal(t, 1, result.WinningTrades)
	assert.Equal(t, 3, result.LosingTrades)

	// 验证总收益率（负数）
	assert.Less(t, result.TotalReturn, 0.0)

	// 验证盈亏比
	assert.Less(t, result.ProfitFactor, 1.0) // 盈利 < 亏损

	// 验证净利润（负数）
	assert.Less(t, result.NetProfit, 0.0)

	t.Logf("✅ Losing backtest metrics")
	t.Logf("   Initial: $%.2f → Final: $%.2f", result.InitialBalance, result.FinalBalance)
	t.Logf("   Total Return: %.2f%%", result.TotalReturn)
	t.Logf("   Max Drawdown: %.2f%%", result.MaxDrawdown)
	t.Logf("   Win Rate: %.2f%% (%d/%d)", result.WinRate, result.WinningTrades, result.TotalTrades)
	t.Logf("   Profit Factor: %.2f", result.ProfitFactor)
	t.Logf("   Net Profit: $%.2f", result.NetProfit)
}

// TestMetricsCalculator_Calculate_NoTrades 测试无交易
func TestMetricsCalculator_Calculate_NoTrades(t *testing.T) {
	calculator := NewMetricsCalculator(10000.0)
	tracker := simulator.NewPositionTracker()

	result := calculator.Calculate(tracker, 10000.0)

	// 验证无交易情况
	assert.Equal(t, 10000.0, result.InitialBalance)
	assert.Equal(t, 10000.0, result.FinalBalance)
	assert.Equal(t, 0.0, result.TotalReturn)
	assert.Equal(t, 0.0, result.MaxDrawdown)
	assert.Equal(t, 0.0, result.WinRate)
	assert.Equal(t, 0, result.TotalTrades)
	assert.Equal(t, 0, result.WinningTrades)
	assert.Equal(t, 0, result.LosingTrades)
	assert.Equal(t, 0.0, result.ProfitFactor)

	t.Logf("✅ No trades scenario")
}

// TestMetricsCalculator_CalculateMaxDrawdown 测试最大回撤计算
func TestMetricsCalculator_CalculateMaxDrawdown(t *testing.T) {
	calculator := NewMetricsCalculator(10000.0)
	baseTime := time.Now()

	// 模拟资金曲线：
	// 10000 → 10500 (peak) → 9500 (drawdown 9.52%) → 11000 (new peak) → 10000 (drawdown 9.09%)
	snapshots := []struct {
		balance float64
		delay   time.Duration
	}{
		{10000.0, 0},
		{10500.0, 1 * time.Hour},  // Peak 1
		{9500.0, 2 * time.Hour},   // Drawdown 1: (10500 - 9500) / 10500 = 9.52%
		{11000.0, 3 * time.Hour},  // Peak 2
		{10000.0, 4 * time.Hour},  // Drawdown 2: (11000 - 10000) / 11000 = 9.09%
		{10800.0, 5 * time.Hour},  // Recovery
	}

	for _, snapshot := range snapshots {
		calculator.RecordBalance(baseTime.Add(snapshot.delay), snapshot.balance)
	}

	// 计算最大回撤
	maxDrawdown := calculator.calculateMaxDrawdown()

	// 最大回撤应该是 9.52% (从 10500 跌到 9500)
	expectedMaxDrawdown := ((10500.0 - 9500.0) / 10500.0) * 100
	assert.InDelta(t, expectedMaxDrawdown, maxDrawdown, 0.01)

	t.Logf("✅ Max drawdown calculation")
	t.Logf("   Expected: %.2f%%", expectedMaxDrawdown)
	t.Logf("   Actual: %.2f%%", maxDrawdown)
}

// TestMetricsCalculator_ProfitFactor_AllWins 测试全胜情况下的盈亏比
func TestMetricsCalculator_ProfitFactor_AllWins(t *testing.T) {
	calculator := NewMetricsCalculator(10000.0)
	tracker := simulator.NewPositionTracker()
	feeRate := 0.0005

	baseTime := time.Now()

	// 模拟 3 笔全盈利交易
	trades := []struct {
		entryPrice float64
		closePrice float64
		size       float64
	}{
		{2500.0, 2510.0, 200.0},
		{2510.0, 2520.0, 200.0},
		{2520.0, 2530.0, 200.0},
	}

	balance := 10000.0

	for i, trade := range trades {
		openTime := baseTime.Add(time.Duration(i) * 10 * time.Minute)
		closeTime := openTime.Add(5 * time.Minute)

		// 开仓
		pos := tracker.AddPosition(trade.entryPrice, trade.size, openTime, trade.closePrice)

		// 计算盈亏
		priceChange := trade.closePrice - trade.entryPrice
		profitBeforeFee := (priceChange / trade.entryPrice) * trade.size
		fee := trade.size * feeRate * 2
		realizedPnL := profitBeforeFee - fee

		balance += realizedPnL

		// 平仓
		tracker.ClosePosition(pos.ID, trade.closePrice, closeTime, realizedPnL)
	}

	// 计算指标
	result := calculator.Calculate(tracker, balance)

	// 验证全胜情况
	assert.Equal(t, 3, result.WinningTrades)
	assert.Equal(t, 0, result.LosingTrades)
	assert.Equal(t, 100.0, result.WinRate)

	// 验证盈亏比（无亏损，应为极高值）
	assert.Equal(t, 999.99, result.ProfitFactor)

	t.Logf("✅ All winning trades")
	t.Logf("   Win Rate: %.2f%%", result.WinRate)
	t.Logf("   Profit Factor: %.2f (no losses)", result.ProfitFactor)
}

// TestMetricsCalculator_RecordBalance 测试资金快照记录
func TestMetricsCalculator_RecordBalance(t *testing.T) {
	calculator := NewMetricsCalculator(10000.0)
	baseTime := time.Now()

	// 记录多个快照
	calculator.RecordBalance(baseTime, 10000.0)
	calculator.RecordBalance(baseTime.Add(1*time.Hour), 10500.0)
	calculator.RecordBalance(baseTime.Add(2*time.Hour), 9500.0)

	snapshots := calculator.GetBalanceSnapshots()

	// 验证快照数量
	assert.Equal(t, 3, len(snapshots))

	// 验证快照内容
	assert.Equal(t, 10000.0, snapshots[0].Balance)
	assert.Equal(t, 10500.0, snapshots[1].Balance)
	assert.Equal(t, 9500.0, snapshots[2].Balance)

	t.Logf("✅ Balance snapshots recorded")
	t.Logf("   Snapshots: %d", len(snapshots))
}
