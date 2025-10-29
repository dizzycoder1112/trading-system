package value_objects

// PositionSummary 倉位摘要（用於策略輸入）
//
// 設計目的：
// - Order Service 會根據實際持倉計算並生成 PositionSummary
// - Strategy Service 使用 PositionSummary 來做決策（例如：盈虧平衡退出）
//
// 使用場景：
// - 計算盈虧平衡點（break-even）
// - 決定是否需要調整策略參數
// - 風險控制判斷
type PositionSummary struct {
	Count                   int     // 持倉數量（未平倉的倉位數）
	TotalSize               float64 // 總倉位大小（所有未平倉的本金總和，單位：USDT）
	AvgPrice                float64 // 平均開倉價格（加權平均）
	FeesPaid                float64 // 已支付的總手續費（包含開倉和已平倉的手續費）
	CurrentRoundRealizedPnL float64 // 當前交易輪次的已實現盈虧（未扣手續費）⭐
}

// NewPositionSummary 創建倉位摘要
//
// 參數：
//   - count: 持倉數量
//   - totalSize: 總倉位大小（USDT）
//   - avgPrice: 平均開倉價格
//   - feesPaid: 已支付的總手續費
//   - currentRoundRealizedPnL: 當前交易輪次的已實現盈虧（未扣手續費）
//
// 返回：
//   - PositionSummary
func NewPositionSummary(count int, totalSize float64, avgPrice float64, feesPaid float64, currentRoundRealizedPnL float64) PositionSummary {
	return PositionSummary{
		Count:                   count,
		TotalSize:               totalSize,
		AvgPrice:                avgPrice,
		FeesPaid:                feesPaid,
		CurrentRoundRealizedPnL: currentRoundRealizedPnL,
	}
}

// IsEmpty 是否沒有持倉
func (ps PositionSummary) IsEmpty() bool {
	return ps.Count == 0
}

// TotalCost 計算總成本（倉位大小 + 手續費）
func (ps PositionSummary) TotalCost() float64 {
	return ps.TotalSize + ps.FeesPaid
}

// CalculateBreakEvenPrice 計算盈虧平衡價格
//
// 盈虧平衡價格 = 需要讓所有倉位以某個價格平倉後，收益剛好抵銷手續費
//
// 公式：
//   未實現盈虧 = TotalSize × (closePrice - avgPrice) / avgPrice
//   平倉手續費 = (TotalSize + 未實現盈虧) × feeRate
//   盈虧平衡條件：未實現盈虧 - 平倉手續費 - 已支付手續費 = 0
//
// 參數：
//   - feeRate: 手續費率（例如：0.0005 = 0.05%）
//
// 返回：
//   - breakEvenPrice: 盈虧平衡價格
func (ps PositionSummary) CalculateBreakEvenPrice(feeRate float64) float64 {
	if ps.Count == 0 || ps.AvgPrice == 0 {
		return 0
	}

	// 簡化計算：
	// breakEvenPrice = avgPrice × (1 + totalFees / totalSize)
	//
	// 其中：
	// - totalFees = 已支付手續費 + 預期平倉手續費
	// - 預期平倉手續費 ≈ totalSize × feeRate（簡化假設，實際會稍高）
	//
	// 更精確的計算需要解方程式，這裡使用近似值
	totalFeesNeeded := ps.FeesPaid + (ps.TotalSize * feeRate)
	breakEvenPriceRatio := 1 + (totalFeesNeeded / ps.TotalSize)

	return ps.AvgPrice * breakEvenPriceRatio
}

// ShouldBreakEven 判斷是否應該盈虧平衡退出
//
// 判斷條件：
//   - 當前輪次總盈虧（已實現 + 未實現 - 手續費）在目標盈利範圍內
//   - 目標盈利範圍：0-20 USDT（可調整）
//
// 參數：
//   - currentPrice: 當前市場價格
//   - feeRate: 手續費率
//   - targetProfitMin: 最小目標盈利（USDT，例如：0）
//   - targetProfitMax: 最大目標盈利（USDT，例如：20）
//
// 返回：
//   - shouldExit: 是否應該退出
//   - expectedProfit: 預期盈利（USDT）
func (ps PositionSummary) ShouldBreakEven(
	currentPrice float64,
	feeRate float64,
	targetProfitMin float64,
	targetProfitMax float64,
) (shouldExit bool, expectedProfit float64) {
	if ps.Count == 0 {
		return false, 0
	}

	// ========== 計算未實現盈虧 ==========
	priceChange := currentPrice - ps.AvgPrice
	unrealizedPnL := ps.TotalSize * (priceChange / ps.AvgPrice)

	// ========== 計算預期平倉手續費 ==========
	closeValue := ps.TotalSize + unrealizedPnL
	closeFee := closeValue * feeRate

	// ========== 計算預期淨利潤 ⭐ 包含當前輪次已實現盈虧 ==========
	// expectedProfit = 當前輪次已實現盈虧 + 未實現盈虧 - 預期平倉手續費 - 已支付手續費
	expectedProfit = ps.CurrentRoundRealizedPnL + unrealizedPnL - closeFee - ps.FeesPaid

	// 判斷是否在目標盈利範圍內
	if expectedProfit >= targetProfitMin && expectedProfit <= targetProfitMax {
		return true, expectedProfit
	}

	return false, expectedProfit
}
