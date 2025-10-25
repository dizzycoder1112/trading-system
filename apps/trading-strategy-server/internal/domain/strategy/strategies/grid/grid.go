package grid

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/value_objects"
)

// OpenAdvice 開倉建議（領域值對象）
type OpenAdvice struct {
	ShouldOpen   bool // 是否應該開倉
	CurrentPrice string
	OpenPrice    string  // 建議開倉價格（精確字符串，例: "3889.94"）
	ClosePrice   string  // 建議平倉價格（精確字符串，例: "3895.78"）
	PositionSize float64 // 建議倉位大小（美元）
	TakeProfit   float64 // 建議停利百分比
	Reason       string  // 原因
}

// GridAggregate 網格聚合根（無狀態設計）⭐
// 特點：
// 1. 封裝業務規則
// 2. 保證不變性（Invariants）
// 3. 無狀態：不記錄 lastCandle（改為參數傳入）
// 4. 不依賴任何技術實現
type GridAggregate struct {
	InstID        string
	PositionSize  float64 // 單次開倉大小（美元）
	TakeProfitMin float64 // 最小停利百分比
	TakeProfitMax float64 // 最大停利百分比
	Calculator    *GridCalculator
	// ❌ 移除 lastCandle（改為參數傳入，無狀態設計）
}

// NewGridAggregate 創建網格聚合根（工廠方法）
func NewGridAggregate(instID string, takeProfitMin, takeProfitMax float64) (*GridAggregate, error) {
	// 驗證業務規則

	if takeProfitMin <= 0 || takeProfitMax <= 0 {
		return nil, errors.New("take profit must be positive")
	}

	if takeProfitMin > takeProfitMax {
		return nil, errors.New("take profit min must be <= max")
	}

	return &GridAggregate{
		InstID:        instID,
		PositionSize:  200,
		TakeProfitMin: takeProfitMin,
		TakeProfitMax: takeProfitMax,
		Calculator:    NewGridCalculator(),
	}, nil
}

// GetOpenAdvice 獲取開倉建議（被動諮詢方法）⭐
// 參數：
//   - currentPrice: 當前價格（Order Service 提供）
//   - lastCandle: 上一根 K 線（從 Redis 讀取）
//
// 返回：OpenAdvice（開倉建議）
func (g *GridAggregate) GetOpenAdvice(
	currentPrice value_objects.Price,
	lastCandle value_objects.Candle,
	candleHistories []value_objects.Candle,
) OpenAdvice {
	// 計算開倉位置：上一根 K 線的 MidLow
	// midLow := lastCandle.MidLow()

	// 判斷：當前價格是否 <= MidLow
	// if currentPrice.IsBelowOrEqual(midLow) {
	// 	// 應該開倉
	// 	takeProfit := (g.takeProfitMin + g.takeProfitMax) / 2.0

	// 	return OpenAdvice{
	// 		ShouldOpen:   true,
	// 		Price:        midLow.Value(),
	// 		PositionSize: g.positionSize,
	// 		TakeProfit:   takeProfit,
	// 		Reason:       fmt.Sprintf("hit_mid_low_%.2f", midLow.Value()),
	// 	}
	// }

	// 不應該開倉
	// return OpenAdvice{
	// 	ShouldOpen: false,
	// 	Reason:     fmt.Sprintf("price_%.2f_above_mid_low_%.2f", currentPrice.Value(), midLow.Value()),
	// }

	// ✅ 使用 decimal 进行精确计算
	currentPriceDecimal := decimal.NewFromFloat(currentPrice.Value())

	// 策略参数
	openDiscount := 0.001 // 开仓折扣：0.1%（在低于市价 0.1% 处挂单）
	takeProfit := 0.0015  // 止盈比例：0.15%（覆盖双边手续费 0.1% + 净利润 0.05%）

	// 计算因子
	openDiscountFactor := decimal.NewFromFloat(1 - openDiscount) // 1 - 0.001 = 0.999
	takeProfitFactor := decimal.NewFromFloat(1 + takeProfit)     // 1 + 0.0015 = 1.0015

	// 计算开仓价格：当前价格 * 0.999，无条件舍去到小数点第 2 位
	openPriceDecimal := currentPriceDecimal.Mul(openDiscountFactor).Truncate(2)

	// 计算平仓价格：开仓价格 * 1.0015，无条件进位到小数点第 2 位
	// 实现无条件进位：先乘以 100，向上取整，再除以 100
	closePriceDecimal := openPriceDecimal.Mul(takeProfitFactor)
	shift := decimal.NewFromInt(100)
	closePriceDecimal = closePriceDecimal.Mul(shift).Ceil().Div(shift)

	return OpenAdvice{
		ShouldOpen:   true,
		CurrentPrice: currentPriceDecimal.String(),
		OpenPrice:    openPriceDecimal.String(),  // 例: "3889.94" (舍去)
		ClosePrice:   closePriceDecimal.String(), // 例: "3895.78" (进位)
		PositionSize: 200.0,
		TakeProfit:   takeProfit, // 0.0015 (0.15%)
		Reason:       "simulated_advice",
	}
}

// ProcessCandle 處理新的K線（舊方法，保留用於向後兼容）
// ⚠️ 已棄用：請使用 GetOpenAdvice() 方法
// 根據策略文件：開倉位置 = 前一根K線的MidLow
func (g *GridAggregate) ProcessCandle(candle value_objects.Candle) (*value_objects.Signal, error) {
	// 舊方法已棄用，但保留代碼避免破壞現有依賴
	return nil, fmt.Errorf("ProcessCandle is deprecated, use GetOpenAdvice instead")
}

// GetState 獲取當前狀態（用於日誌或監控）
func (g *GridAggregate) GetState() map[string]any {
	return map[string]any{
		"instID":        g.InstID,
		"positionSize":  g.PositionSize,
		"takeProfitMin": g.TakeProfitMin,
		"takeProfitMax": g.TakeProfitMax,
	}
}

// GetName 獲取策略名稱
func (g *GridAggregate) GetName() string {
	return "grid"
}
