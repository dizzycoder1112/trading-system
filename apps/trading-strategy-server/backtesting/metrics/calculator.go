package metrics

import (
	"time"

	"dizzycode.xyz/trading-strategy-server/backtesting/simulator"
)

// BacktestResult 回测结果
type BacktestResult struct {
	InitialBalance  float64       // 初始资金
	FinalBalance    float64       // 最终资金
	TotalReturn     float64       // 总收益率 (%)
	MaxDrawdown     float64       // 最大回撤 (%)
	WinRate         float64       // 胜率 (%)
	TotalTrades     int           // 总交易次数
	WinningTrades   int           // 盈利交易次数
	LosingTrades    int           // 亏损交易次数
	AvgHoldDuration time.Duration // 平均持仓时长
	ProfitFactor    float64       // 盈亏比 (总盈利/总亏损)
	TotalProfit     float64       // 总盈利金额
	TotalLoss       float64       // 总亏损金额
	NetProfit       float64       // 净利润 (最终资金 - 初始资金)
}

// BalanceSnapshot 资金快照（用于计算最大回撤）
type BalanceSnapshot struct {
	Time    time.Time
	Balance float64
}

// MetricsCalculator 指标计算器
type MetricsCalculator struct {
	initialBalance   float64
	balanceSnapshots []BalanceSnapshot
}

// NewMetricsCalculator 创建指标计算器
func NewMetricsCalculator(initialBalance float64) *MetricsCalculator {
	return &MetricsCalculator{
		initialBalance:   initialBalance,
		balanceSnapshots: make([]BalanceSnapshot, 0),
	}
}

// RecordBalance 记录资金快照（用于最大回撤计算）
func (mc *MetricsCalculator) RecordBalance(timestamp time.Time, balance float64) {
	mc.balanceSnapshots = append(mc.balanceSnapshots, BalanceSnapshot{
		Time:    timestamp,
		Balance: balance,
	})
}

// Calculate 计算回测指标
//
// 参数：
//   - positionTracker: 仓位追踪器（包含所有已平仓记录）
//   - finalBalance: 最终资金
//
// 返回：
//   - BacktestResult: 回测结果
func (mc *MetricsCalculator) Calculate(
	positionTracker *simulator.PositionTracker,
	finalBalance float64,
) BacktestResult {
	closedPositions := positionTracker.GetClosedPositions()
	totalTrades := len(closedPositions)

	// 1. 计算总收益率
	// TotalReturn = (FinalBalance - InitialBalance) / InitialBalance * 100
	netProfit := finalBalance - mc.initialBalance
	totalReturn := 0.0
	if mc.initialBalance > 0 {
		totalReturn = (netProfit / mc.initialBalance) * 100
	}

	// 2. 计算最大回撤
	maxDrawdown := mc.calculateMaxDrawdown()

	// 3. 计算胜率、盈亏比
	winningTrades := 0
	losingTrades := 0
	totalProfit := 0.0
	totalLoss := 0.0

	for _, closed := range closedPositions {
		if closed.RealizedPnL > 0 {
			winningTrades++
			totalProfit += closed.RealizedPnL
		} else if closed.RealizedPnL < 0 {
			losingTrades++
			totalLoss += -closed.RealizedPnL // 转为正数
		}
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = (float64(winningTrades) / float64(totalTrades)) * 100
	}

	// 4. 计算盈亏比 (Profit Factor)
	// ProfitFactor = TotalProfit / TotalLoss
	profitFactor := 0.0
	if totalLoss > 0 {
		profitFactor = totalProfit / totalLoss
	} else if totalProfit > 0 {
		profitFactor = 999.99 // 无亏损，盈亏比极高
	}

	// 5. 计算平均持仓时长
	avgHoldDuration := positionTracker.GetAverageHoldDuration()

	return BacktestResult{
		InitialBalance:  mc.initialBalance,
		FinalBalance:    finalBalance,
		TotalReturn:     totalReturn,
		MaxDrawdown:     maxDrawdown,
		WinRate:         winRate,
		TotalTrades:     totalTrades,
		WinningTrades:   winningTrades,
		LosingTrades:    losingTrades,
		AvgHoldDuration: avgHoldDuration,
		ProfitFactor:    profitFactor,
		TotalProfit:     totalProfit,
		TotalLoss:       totalLoss,
		NetProfit:       netProfit,
	}
}

// calculateMaxDrawdown 计算最大回撤
//
// 最大回撤 = (历史最高资金 - 最低资金) / 历史最高资金 * 100
//
// 算法：
//  1. 遍历所有资金快照
//  2. 记录当前最高资金
//  3. 计算当前回撤 = (最高资金 - 当前资金) / 最高资金
//  4. 更新最大回撤
func (mc *MetricsCalculator) calculateMaxDrawdown() float64 {
	if len(mc.balanceSnapshots) == 0 {
		return 0.0
	}

	maxDrawdown := 0.0
	peak := mc.balanceSnapshots[0].Balance

	for _, snapshot := range mc.balanceSnapshots {
		// 更新历史最高资金
		if snapshot.Balance > peak {
			peak = snapshot.Balance
		}

		// 计算当前回撤
		if peak > 0 {
			drawdown := ((peak - snapshot.Balance) / peak) * 100
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}

	return maxDrawdown
}

// GetBalanceSnapshots 获取资金快照列表（用于绘图或调试）
func (mc *MetricsCalculator) GetBalanceSnapshots() []BalanceSnapshot {
	return mc.balanceSnapshots
}
