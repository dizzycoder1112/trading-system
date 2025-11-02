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
	CurrentRoundRealizedPnL float64 // 當前交易輪次的已實現盈虧（扣除手續費）⭐
	CurrentRoundClosedValue float64 // 當前交易輪次累積關倉價值（本金 + 盈虧）⭐
	UnrealizedPnL           float64 // 未實現盈虧（外部計算，已包含預估平倉費）⭐ 用於 ShouldBreakEven2
}

// NewPositionSummary 創建倉位摘要
//
// 參數：
//   - count: 持倉數量
//   - totalSize: 總倉位大小（USDT）
//   - avgPrice: 平均開倉價格
//   - feesPaid: 已支付的總手續費
//   - currentRoundRealizedPnL: 當前交易輪次的已實現盈虧（扣除手續費）
//   - currentRoundClosedValue: 當前交易輪次累積關倉價值（本金 + 盈虧）
//   - unrealizedPnL: 未實現盈虧（外部計算，已包含預估平倉費）
//
// 返回：
//   - PositionSummary
func NewPositionSummary(count int, totalSize float64, avgPrice float64, feesPaid float64, currentRoundRealizedPnL float64, currentRoundClosedValue float64, unrealizedPnL float64) PositionSummary {
	return PositionSummary{
		Count:                   count,
		TotalSize:               totalSize,
		AvgPrice:                avgPrice,
		FeesPaid:                feesPaid,
		CurrentRoundRealizedPnL: currentRoundRealizedPnL,
		CurrentRoundClosedValue: currentRoundClosedValue,
		UnrealizedPnL:           unrealizedPnL,
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
//
//	未實現盈虧 = TotalSize × (closePrice - avgPrice) / avgPrice
//	平倉手續費 = (TotalSize + 未實現盈虧) × feeRate
//	盈虧平衡條件：未實現盈虧 - 平倉手續費 - 已支付手續費 = 0
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

// ShouldBreakEven2 判斷是否應該盈虧平衡退出（使用外部計算的 unrealizedPnL）⭐
//
// 與 ShouldBreakEven 的差異：
//   - ShouldBreakEven: 內部計算 unrealizedPnL（簡化版，用平均價格）
//   - ShouldBreakEven2: 使用 ps.UnrealizedPnL（精確版，逐倉位計算，外部傳入）
//
// 判斷條件：
//   - 當前輪次總盈虧（已實現 + 未實現）在目標盈利範圍內
//   - 目標盈利範圍：0-20 USDT（可調整）
//
// 參數：
//   - targetProfitMin: 最小目標盈利（USDT，例如：0）
//   - targetProfitMax: 最大目標盈利（USDT，例如：20）- 保留參數但暫未使用
//
// 返回：
//   - shouldExit: 是否應該退出
//   - expectedProfit: 預期盈利（USDT）
func (ps PositionSummary) ShouldBreakEven(
	targetProfitMin float64,
	targetProfitMax float64,
) (shouldExit bool, expectedProfit float64) {
	if ps.Count == 0 {
		return false, 0
	}

	// ⭐ 如果本輪還沒有任何關倉，不觸發打平機制
	if ps.CurrentRoundClosedValue == 0 {
		return false, 0
	}

	// ⭐ 如果當前輪次已實現盈虧不為負值，不觸發打平機制（打平是防守機制，只在虧損時觸發）
	if ps.CurrentRoundRealizedPnL >= 0 {
		return false, 0
	}

	// ========== 計算預期淨利潤 ⭐ ==========
	//
	// 正確公式：
	// expectedProfit = CurrentRoundRealizedPnL + ps.UnrealizedPnL
	//
	// 說明：
	// - CurrentRoundRealizedPnL：本輪已平倉的淨盈虧（已扣除開倉費和平倉費）
	// - ps.UnrealizedPnL：外部計算好的未實現盈虧（已包含預估平倉費）
	//
	// ⭐ 重要：ps.UnrealizedPnL 是外部（BacktestEngine/OrderService）通過 PositionTracker 計算好的
	//         已經包含了預估平倉手續費，這裡不需要重複計算
	expectedProfit = ps.CurrentRoundRealizedPnL + ps.UnrealizedPnL

	// 判斷是否應該觸發打平機制
	// 條件：expectedProfit >= targetProfitMin（例如 >= 0）
	if expectedProfit > targetProfitMin {
		return true, expectedProfit
	}

	return false, expectedProfit
}
