package simulator

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// OrderSimulator 成交模擬器
type OrderSimulator struct {
	feeRate  float64 // OKX taker 手續費: 0.05% (0.0005)
	slippage float64 // 滑點（簡單版設為 0）
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

// NewOrderSimulator 創建成交模擬器
func NewOrderSimulator(feeRate, slippage float64) *OrderSimulator {
	return &OrderSimulator{
		feeRate:  feeRate,
		slippage: slippage,
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

// SimulateClose 模擬平倉
//
// 功能：
//  1. 計算平倉收入（倉位大小 * 價格變化比例）
//  2. 計算平倉手續費
//  3. 計算已實現盈虧（扣除開倉和平倉手續費）
//  4. 返回已平倉記錄和實際收入
//
// 參數：
//   - position: 持倉記錄
//   - closePrice: 平倉價格
//   - closeTime: 平倉時間
//
// 返回：
//   - ClosedPosition: 已平倉記錄
//   - float64: 實際收入（倉位大小 + 盈虧 - 手續費）
//   - error: 錯誤信息
func (s *OrderSimulator) SimulateClose(
	position Position,
	closePrice float64,
	closeTime time.Time,
) (ClosedPosition, float64, error) {
	// 1. 驗證輸入
	if closePrice <= 0 {
		return ClosedPosition{}, 0, errors.New("close price must be positive")
	}

	// 2. 計算價格變化比例
	// priceChangeRate = (closePrice - entryPrice) / entryPrice
	priceChange := closePrice - position.EntryPrice
	priceChangeRate := priceChange / position.EntryPrice

	// 3. 計算盈虧（未扣除手續費）
	// profit = positionSize * priceChangeRate
	profitBeforeFee := position.Size * priceChangeRate

	// 4. 計算開倉和平倉手續費
	// 開倉手續費已在開倉時扣除，這裡只計算平倉手續費
	closeFee := position.Size * s.feeRate
	openFee := position.Size * s.feeRate // 用於計算總手續費

	// 5. 計算已實現盈虧（扣除雙邊手續費）
	realizedPnL := profitBeforeFee - openFee - closeFee

	// 6. 計算實際收入
	// 實際收入 = 原始倉位大小 + 盈虧 - 平倉手續費
	// （開倉手續費已在開倉時扣除，這裡只扣平倉手續費）
	actualRevenue := position.Size + profitBeforeFee - closeFee

	// 7. 創建已平倉記錄
	closedPosition := ClosedPosition{
		Position:     position,
		ClosePrice:   closePrice,
		CloseTime:    closeTime,
		RealizedPnL:  realizedPnL,
		HoldDuration: closeTime.Sub(position.OpenTime),
	}

	return closedPosition, actualRevenue, nil
}
