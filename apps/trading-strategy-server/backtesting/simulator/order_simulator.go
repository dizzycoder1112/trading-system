package simulator

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// OrderSimulator 成交模擬器
type OrderSimulator struct {
	feeRate       float64        // OKX taker 手續費: 0.05% (0.0005)
	slippage      float64        // 滑點（簡單版設為 0）
	pnlCalculator *PnLCalculator // 盈虧計算器 ⭐ Single Source of Truth
}

// OpenAdvice 開倉建議（與 strategy-server 保持一致）
type OpenAdvice struct {
	ShouldOpen   bool    // 是否應該開倉
	CurrentPrice string  // 當前價格
	OpenPrice    string  // 建議開倉價格（精確字符串）
	ClosePrice   string  // 建議平倉價格（精確字符串）
	PositionSize float64 // 建議倉位大小（美元）
	TakeProfit   float64 // 建議停利百分比
	Reason       string  // 原因
}

// CloseResult 平倉結果（統一計算所有盈虧指標）⭐
type CloseResult struct {
	ClosedPosition ClosedPosition // 已平倉記錄

	// 基於單筆開倉價的盈虧
	PnL        float64 // 盈虧金額（未扣手續費）
	PnLPercent float64 // 盈虧百分比

	// 基於平均成本的盈虧（用於交易輪次統計）⭐
	PnL_Avg        float64 // 基於平均成本的盈虧金額（未扣手續費）
	PnLPercent_Avg float64 // 基於平均成本的盈虧百分比

	// 手續費和收入
	CloseFee   float64 // 平倉手續費
	CloseValue float64 // 平倉時的總價值（本金 + 盈虧）
	Revenue    float64 // 實際收入（closeValue - closeFee）
}

// NewOrderSimulator 創建成交模擬器
func NewOrderSimulator(feeRate, slippage float64) *OrderSimulator {
	return &OrderSimulator{
		feeRate:       feeRate,
		slippage:      slippage,
		pnlCalculator: NewPnLCalculator(), // 初始化盈虧計算器 ⭐
	}
}

// SimulateOpen 模擬開倉
//
// 功能：
//  1. 檢查餘額是否足夠
//  2. 計算開倉手續費
//  3. 計算實際成本（倉位大小 + 手續費）
//  4. 返回持倉記錄和實際成本
//
// 參數：
//   - advice: 開倉建議（包含開倉價格、倉位大小等）
//   - balance: 當前可用餘額
//
// 返回：
//   - Position: 持倉記錄
//   - float64: 實際成本（倉位大小 + 手續費）
//   - error: 錯誤信息
func (s *OrderSimulator) SimulateOpen(
	advice OpenAdvice,
	balance float64,
	openTime time.Time,
) (Position, float64, error) {
	// 1. 驗證是否應該開倉
	if !advice.ShouldOpen {
		return Position{}, 0, errors.New("advice indicates should not open")
	}

	// 2. 解析開倉價格和平倉價格（使用精確的 decimal 計算）
	openPriceDecimal, err := decimal.NewFromString(advice.OpenPrice)
	if err != nil {
		return Position{}, 0, fmt.Errorf("invalid open price: %w", err)
	}

	closePriceDecimal, err := decimal.NewFromString(advice.ClosePrice)
	if err != nil {
		return Position{}, 0, fmt.Errorf("invalid close price: %w", err)
	}

	openPrice := openPriceDecimal.InexactFloat64()
	closePrice := closePriceDecimal.InexactFloat64()
	positionSize := advice.PositionSize

	// 3. 計算開倉手續費（倉位大小 * 手續費率）
	fee := positionSize * s.feeRate

	// 4. 計算實際成本（倉位大小 + 手續費）
	actualCost := positionSize + fee

	// 5. 檢查餘額是否足夠
	if balance < actualCost {
		return Position{}, 0, fmt.Errorf(
			"insufficient balance: need %.2f USDT (position: %.2f + fee: %.2f), have %.2f USDT",
			actualCost, positionSize, fee, balance,
		)
	}

	// 6. 創建持倉記錄
	position := Position{
		ID:               fmt.Sprintf("backtest_pos_%d", time.Now().UnixNano()),
		EntryPrice:       openPrice,
		Size:             positionSize,
		OpenTime:         openTime,
		TargetClosePrice: closePrice,
	}

	return position, actualCost, nil
}

// SimulateClose 模擬平倉（統一計算所有盈虧指標）⭐
//
// 功能：
//  1. 計算關閉的幣數（核心邏輯）
//  2. 計算基於開倉價的盈虧
//  3. 計算基於平均成本的盈虧
//  4. 計算手續費和實際收入
//  5. 返回完整的 CloseResult
//
// 參數：
//   - position: 持倉記錄
//   - closePrice: 平倉價格
//   - closeTime: 平倉時間
//   - avgCost: 當前平均成本（用於計算 PnL_Avg）
//
// 返回：
//   - CloseResult: 包含所有盈虧指標的完整結果
//   - error: 錯誤信息
func (s *OrderSimulator) SimulateClose(
	position Position,
	closePrice float64,
	closeTime time.Time,
	avgCost float64,
) (CloseResult, error) {
	// 1. 驗證輸入
	if closePrice <= 0 {
		return CloseResult{}, errors.New("close price must be positive")
	}
	if avgCost <= 0 {
		return CloseResult{}, errors.New("avgCost must be positive")
	}

	// ⭐ 2. 計算關閉的幣數（核心邏輯 - 必須用 EntryPrice）
	// 重要：pos.Size 是該筆開倉投入的 USDT 金額
	// 該筆開倉實際買入的幣數 = Size / EntryPrice
	closedCoins := position.Size / position.EntryPrice

	// ⭐ 3. 使用 PnLCalculator 計算兩套盈虧 (Single Source of Truth)
	//
	// 3.1 基於單筆開倉價的盈虧（用於分析單笔交易表現）
	pnlAmount, pnlPercent := s.pnlCalculator.CalculatePnL(
		closePrice,
		position.EntryPrice, // 參數名清楚表明：基於開倉價
		closedCoins,
	)

	// 3.2 基於平均成本的盈虧（用於真實賬戶盈虧計算）⭐ 核心計算
	pnlAmount_Avg, pnlPercent_Avg := s.pnlCalculator.CalculatePnL(
		closePrice,
		avgCost, // 參數名清楚表明：基於平均成本
		closedCoins,
	)

	// ⭐ 5. 計算平倉時的實際價值和手續費
	closeValue := position.Size + pnlAmount        // 平倉時的總價值（本金 + 盈虧）
	closeFee := closeValue * s.feeRate             // 平倉手續費基於總價值
	openFee := position.Size * s.feeRate           // 開倉手續費
	realizedPnL := pnlAmount_Avg - openFee - closeFee // 已實現盈虧（基於平均成本）

	// ⭐ 6. 計算實際收入
	// 實際收入 = 平倉價值 - 平倉手續費
	// （開倉手續費已在開倉時扣除，這裡只扣平倉手續費）
	revenue := closeValue - closeFee

	// 7. 創建已平倉記錄
	closedPosition := ClosedPosition{
		Position:     position,
		ClosePrice:   closePrice,
		CloseTime:    closeTime,
		RealizedPnL:  realizedPnL, // 基於平均成本的盈虧（用於勝率計算）
		HoldDuration: closeTime.Sub(position.OpenTime),
	}

	// ⭐ 8. 返回完整結果
	return CloseResult{
		ClosedPosition: closedPosition,
		PnL:            pnlAmount,
		PnLPercent:     pnlPercent,
		PnL_Avg:        pnlAmount_Avg,
		PnLPercent_Avg: pnlPercent_Avg,
		CloseFee:       closeFee,
		CloseValue:     closeValue,
		Revenue:        revenue,
	}, nil
}
