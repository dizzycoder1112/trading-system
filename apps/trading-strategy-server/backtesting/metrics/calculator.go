package metrics

import (
	"time"

	"dizzycode.xyz/trading-strategy-server/backtesting/simulator"
	"github.com/shopspring/decimal"
)

// BacktestResult 回测结果
type BacktestResult struct {
	InitialBalance float64 // 初始资金
	FinalBalance   float64 // 最终资金（可用余额）
	TotalEquity    float64 // 总权益（余额 + 未平仓价值 + 浮盈亏）⭐

	// 倉位分析
	TotalOpenedTrades    int     // 總開倉數量 ⭐ 新增
	TotalClosedTrades    int     // 總關倉數量 ⭐ 新增
	OpenPositionCount    int     // 未平仓数量
	OpenPositionValue    float64 // 未平仓总价值
	MaxOpenPositionValue float64 // 最大持倉價值（USDT）⭐ 新增
	FullPositionDays     int     // 持倉全滿的天數 ⭐ 新增

	// 交易統計
	TotalProfitGross       float64 // 總利潤-基於平均成本（未扣手續費）⭐
	TotalProfitGross_Entry float64 // 總利潤-基於單筆開倉價（未扣手續費）⭐ 新增
	TotalFeesOpen          float64 // 開倉總手續費 ⭐ 新增
	TotalFeesClose         float64 // 關倉總手續費 ⭐ 新增
	TotalFeesPaid          float64 // 總手續費（開倉 + 關倉）
	UnrealizedPnL          float64 // 未實現盈虧（含預估關倉手續費）⭐
	NetProfit              float64 // 淨利潤 = 總利潤 + 未實現盈虧 - 總手續費 ⭐
	TotalReturn      float64       // 總收益率 (%)
	ProfitFactor     float64       // 盈虧比（含未實現盈虧）⭐
	WinRate          float64       // 勝率 (%)
	AvgHoldDuration  time.Duration // 平均持倉時長
	MaxDrawdown      float64       // 最大回撤 (%)

	// 詳細統計（保留用於其他分析）
	TotalTrades   int     // 總交易次數（已平倉）
	WinningTrades int     // 盈利交易次數
	LosingTrades  int     // 虧損交易次數
	TotalProfit   float64 // 總盈利金額（已實現，已扣費）
	TotalLoss     float64 // 總虧損金額（已實現，已扣費）
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
//   - positionTracker: 仓位追踪器（包含所有已平仓记录和未平仓）
//   - finalBalance: 最终可用资金（不包含未平仓）
//   - lastPrice: 最后价格（用于计算未实现盈亏）
//   - totalOpenedTrades: 總開倉數量 ⭐ 新增
//   - totalProfitGross: 總利潤（未扣手續費）⭐ 新增
//   - totalFeesOpen: 開倉總手續費 ⭐ 新增
//   - totalFeesClose: 關倉總手續費 ⭐ 新增
//
// 返回：
//   - BacktestResult: 回测结果
func (mc *MetricsCalculator) Calculate(
	positionTracker *simulator.PositionTracker,
	finalBalance float64,
	lastPrice float64,
	totalOpenedTrades int,
	totalProfitGross float64,
	totalProfitGross_Entry float64, // ⭐ 新增：基于单笔开仓价的总利润
	totalFeesOpen float64,
	totalFeesClose float64,
) BacktestResult {
	closedPositions := positionTracker.GetClosedPositions()
	totalTrades := len(closedPositions)

	// 1. 计算未平仓信息 ⭐
	openPositions := positionTracker.GetOpenPositions()
	openPositionCount := len(openPositions)
	openPositionValue := positionTracker.GetTotalSize()

	// 计算未实现盈亏（使用最后价格，包含預估關倉手續費）
	feeRate := 0.0005 // OKX Taker 手续费
	unrealizedPnL := positionTracker.CalculateUnrealizedPnL(lastPrice, feeRate)

	// ⭐ 使用 decimal 計算，避免浮點誤差
	totalFeesOpenD := decimal.NewFromFloat(totalFeesOpen)
	totalFeesCloseD := decimal.NewFromFloat(totalFeesClose)
	totalProfitGrossD := decimal.NewFromFloat(totalProfitGross)
	unrealizedPnLD := decimal.NewFromFloat(unrealizedPnL)
	finalBalanceD := decimal.NewFromFloat(finalBalance)
	openPositionValueD := decimal.NewFromFloat(openPositionValue)
	initialBalanceD := decimal.NewFromFloat(mc.initialBalance)
	hundred := decimal.NewFromInt(100)

	// 2. 計算總手續費
	totalFeesPaidD := totalFeesOpenD.Add(totalFeesCloseD)
	totalFeesPaid := totalFeesPaidD.InexactFloat64()

	// 3. 計算淨利潤 ⭐
	// NetProfit = 已平倉淨利潤 + 未平倉淨盈虧
	netProfitD := totalProfitGrossD.Add(unrealizedPnLD).Sub(totalFeesPaidD)
	netProfit := netProfitD.InexactFloat64()

	// 4. 计算总权益（可用余额 + 未平仓价值 + 未实现盈亏）
	totalEquityD := finalBalanceD.Add(openPositionValueD).Add(unrealizedPnLD)
	totalEquity := totalEquityD.InexactFloat64()

	// 5. 计算总收益率（基于淨利潤）⭐
	// TotalReturn = NetProfit / InitialBalance * 100
	totalReturn := 0.0
	if mc.initialBalance > 0 {
		totalReturnD := netProfitD.Div(initialBalanceD).Mul(hundred)
		totalReturn = totalReturnD.Truncate(2).InexactFloat64() // 截斷到小數點後兩位
	}

	// 6. 计算最大回撤
	maxDrawdown := mc.calculateMaxDrawdown()

	// 7. 计算胜率、盈亏比（含未實現盈虧）⭐
	winningTrades := 0
	losingTrades := 0
	totalProfitRealizedD := decimal.Zero // 已實現盈利（已扣費）
	totalLossRealizedD := decimal.Zero   // 已實現虧損（已扣費）

	for _, closed := range closedPositions {
		realizedPnLD := decimal.NewFromFloat(closed.RealizedPnL)
		if closed.RealizedPnL > 0 {
			winningTrades++
			totalProfitRealizedD = totalProfitRealizedD.Add(realizedPnLD)
		} else if closed.RealizedPnL < 0 {
			losingTrades++
			totalLossRealizedD = totalLossRealizedD.Add(realizedPnLD.Neg()) // 转为正数
		}
	}

	// 勝率（只計算已平倉）
	winRate := 0.0
	if totalTrades > 0 {
		winningTradesD := decimal.NewFromInt(int64(winningTrades))
		totalTradesD := decimal.NewFromInt(int64(totalTrades))
		winRateD := winningTradesD.Div(totalTradesD).Mul(hundred)
		winRate = winRateD.InexactFloat64()
	}

	// 盈虧比（含未實現盈虧）⭐ 新邏輯
	// ProfitFactor = (TotalProfitRealized + UnrealizedProfit) / (TotalLossRealized + UnrealizedLoss)
	totalProfitWithUnrealizedD := totalProfitRealizedD
	totalLossWithUnrealizedD := totalLossRealizedD

	if unrealizedPnL > 0 {
		totalProfitWithUnrealizedD = totalProfitWithUnrealizedD.Add(unrealizedPnLD)
	} else if unrealizedPnL < 0 {
		totalLossWithUnrealizedD = totalLossWithUnrealizedD.Add(unrealizedPnLD.Neg())
	}

	profitFactor := 0.0
	if totalLossWithUnrealizedD.GreaterThan(decimal.Zero) {
		profitFactorD := totalProfitWithUnrealizedD.Div(totalLossWithUnrealizedD)
		profitFactor = profitFactorD.InexactFloat64()
	} else if totalProfitWithUnrealizedD.GreaterThan(decimal.Zero) {
		profitFactor = 999.99 // 无亏损，盈亏比极高
	}

	// 8. 计算平均持仓时长
	avgHoldDuration := positionTracker.GetAverageHoldDuration()

	return BacktestResult{
		InitialBalance: mc.initialBalance,
		FinalBalance:   finalBalance,
		TotalEquity:    totalEquity,

		// 倉位分析
		TotalOpenedTrades: totalOpenedTrades,
		TotalClosedTrades: totalTrades,
		OpenPositionCount: openPositionCount,
		OpenPositionValue: openPositionValue,

		// 交易統計
		TotalProfitGross:       totalProfitGross,
		TotalProfitGross_Entry: totalProfitGross_Entry, // ⭐ 新增
		TotalFeesOpen:          totalFeesOpen,
		TotalFeesClose:         totalFeesClose,
		TotalFeesPaid:          totalFeesPaid,
		UnrealizedPnL:          unrealizedPnL,
		NetProfit:              netProfit,
		TotalReturn:      totalReturn,
		ProfitFactor:     profitFactor,
		WinRate:          winRate,
		AvgHoldDuration:  avgHoldDuration,
		MaxDrawdown:      maxDrawdown,

		// 詳細統計
		TotalTrades:   totalTrades,
		WinningTrades: winningTrades,
		LosingTrades:  losingTrades,
		TotalProfit:   totalProfitRealizedD.InexactFloat64(),
		TotalLoss:     totalLossRealizedD.InexactFloat64(),
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

	// ⭐ 使用 decimal 計算，避免浮點誤差
	hundred := decimal.NewFromInt(100)
	maxDrawdownD := decimal.Zero
	peakD := decimal.NewFromFloat(mc.balanceSnapshots[0].Balance)

	for _, snapshot := range mc.balanceSnapshots {
		balanceD := decimal.NewFromFloat(snapshot.Balance)

		// 更新历史最高资金
		if balanceD.GreaterThan(peakD) {
			peakD = balanceD
		}

		// 计算当前回撤
		if peakD.GreaterThan(decimal.Zero) {
			// drawdown = (peak - balance) / peak * 100
			drawdownD := peakD.Sub(balanceD).Div(peakD).Mul(hundred)
			if drawdownD.GreaterThan(maxDrawdownD) {
				maxDrawdownD = drawdownD
			}
		}
	}

	return maxDrawdownD.InexactFloat64()
}

// GetBalanceSnapshots 获取资金快照列表（用于绘图或调试）
func (mc *MetricsCalculator) GetBalanceSnapshots() []BalanceSnapshot {
	return mc.balanceSnapshots
}
