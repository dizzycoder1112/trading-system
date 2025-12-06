package simulator

import "github.com/shopspring/decimal"

// PnLCalculator 盈亏计算器
//
// 提供两套盈亏计算方法，确保 Single Source of Truth：
//
// 1. 基于单笔开仓价 (EntryPrice)
//    - 用途：分析单笔交易的实际表现（策略触发点分析）
//    - 适用场景：回测报告中查看每笔开仓的独立获利情况
//
// 2. 基于平均成本 (AvgCost)
//    - 用途：计算真实账户盈亏（考虑多次开仓的摊平成本）
//    - 适用场景：核心计算（realizedPnL, netProfit, balance）
//
// 核心原则：
//   - 所有价格变化率计算都通过此计算器统一实现
//   - 避免在多处重复实现相同的计算逻辑
//   - 保证公式修改时只需要修改一处
//   - ⭐ 內部使用 decimal 計算，避免浮點誤差
type PnLCalculator struct{}

// NewPnLCalculator 创建盈亏计算器实例
func NewPnLCalculator() *PnLCalculator {
	return &PnLCalculator{}
}

// CalculatePriceChangeRate 计算价格变化率
//
// 公式: (currentPrice - basePrice) / basePrice
//
// 参数:
//   - currentPrice: 当前价格（或平仓价格）
//   - basePrice: 基准价格（开仓价或平均成本）
//
// 返回: 价格变化率（小数形式，如 0.02 表示 2%）
//
// 示例:
//
//	rate := calc.CalculatePriceChangeRate(2510, 2500)
//	// rate = 0.004 (0.4%)
func (pc *PnLCalculator) CalculatePriceChangeRate(currentPrice, basePrice float64) float64 {
	// ⭐ 使用 decimal 計算，避免浮點誤差
	currentPriceD := decimal.NewFromFloat(currentPrice)
	basePriceD := decimal.NewFromFloat(basePrice)

	// (currentPrice - basePrice) / basePrice
	rateD := currentPriceD.Sub(basePriceD).Div(basePriceD)
	return rateD.InexactFloat64()
}

// CalculatePriceChangePercent 计算价格变化百分比
//
// 公式: (currentPrice - basePrice) / basePrice * 100
//
// 参数:
//   - currentPrice: 当前价格（或平仓价格）
//   - basePrice: 基准价格（开仓价或平均成本）
//
// 返回: 价格变化百分比（如 2.5 表示 2.5%）
//
// 示例:
//
//	percent := calc.CalculatePriceChangePercent(2510, 2500)
//	// percent = 0.4 (0.4%)
func (pc *PnLCalculator) CalculatePriceChangePercent(currentPrice, basePrice float64) float64 {
	// ⭐ 使用 decimal 計算，避免浮點誤差
	currentPriceD := decimal.NewFromFloat(currentPrice)
	basePriceD := decimal.NewFromFloat(basePrice)
	hundred := decimal.NewFromInt(100)

	// (currentPrice - basePrice) / basePrice * 100
	percentD := currentPriceD.Sub(basePriceD).Div(basePriceD).Mul(hundred)
	return percentD.InexactFloat64()
}

// CalculatePnL 计算盈亏（通用函数）⭐ Single Source of Truth
//
// 参数:
//   - closePrice: 平仓价格（或当前价格）
//   - basePrice: 基准价格（可以是开仓价 entryPrice 或平均成本 avgCost）
//   - coins: 持仓币数
//
// 返回:
//   - pnlAmount: 盈亏金额（未扣手续费）
//   - pnlPercent: 盈亏百分比
//
// 用途说明:
//   1. 基于单笔开仓价（分析单笔交易表现）:
//      amount, percent := calc.CalculatePnL(closePrice, position.EntryPrice, coins)
//
//   2. 基于平均成本（真实账户盈亏）:
//      amount, percent := calc.CalculatePnL(closePrice, avgCost, coins)
//
// 示例:
//
//	// 基于开仓价
//	amount, percent := calc.CalculatePnL(2510, 2500, 0.08)
//	// amount = 0.8 USDT, percent = 0.4%
//
//	// 基于平均成本
//	amount, percent := calc.CalculatePnL(2510, 2490, 0.08)
//	// amount = 1.6 USDT, percent = 0.8%
func (pc *PnLCalculator) CalculatePnL(closePrice, basePrice, coins float64) (pnlAmount, pnlPercent float64) {
	// ⭐ 使用 decimal 計算，避免浮點誤差
	closePriceD := decimal.NewFromFloat(closePrice)
	basePriceD := decimal.NewFromFloat(basePrice)
	coinsD := decimal.NewFromFloat(coins)
	hundred := decimal.NewFromInt(100)

	// priceChange = closePrice - basePrice
	priceChangeD := closePriceD.Sub(basePriceD)

	// pnlPercent = (closePrice - basePrice) / basePrice * 100
	pnlPercentD := priceChangeD.Div(basePriceD).Mul(hundred)

	// pnlAmount = coins * priceChange
	pnlAmountD := coinsD.Mul(priceChangeD)

	return pnlAmountD.InexactFloat64(), pnlPercentD.InexactFloat64()
}

