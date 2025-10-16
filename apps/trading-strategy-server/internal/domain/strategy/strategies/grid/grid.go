package grid

import (
	"errors"
	"fmt"

	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/value_objects"
)

// GridAggregate 網格聚合根
// 特點：
// 1. 封裝業務規則
// 2. 保證不變性（Invariants）
// 3. 管理內部狀態
// 4. 不依賴任何技術實現
type GridAggregate struct {
	instID         string
	positionSize   float64 // 單次開倉大小（美元）
	takeProfitMin  float64 // 最小停利百分比
	takeProfitMax  float64 // 最大停利百分比
	lastCandle     *value_objects.Candle // 上一根K線
	calculator     *GridCalculator
}

// NewGridAggregate 創建網格聚合根（工廠方法）
func NewGridAggregate(instID string, positionSize, takeProfitMin, takeProfitMax float64) (*GridAggregate, error) {
	// 驗證業務規則
	if positionSize <= 0 {
		return nil, errors.New("position size must be positive")
	}

	if takeProfitMin <= 0 || takeProfitMax <= 0 {
		return nil, errors.New("take profit must be positive")
	}

	if takeProfitMin > takeProfitMax {
		return nil, errors.New("take profit min must be <= max")
	}

	return &GridAggregate{
		instID:        instID,
		positionSize:  positionSize,
		takeProfitMin: takeProfitMin,
		takeProfitMax: takeProfitMax,
		lastCandle:    nil,
		calculator:    NewGridCalculator(),
	}, nil
}

// ProcessCandle 處理新的K線（核心業務邏輯）
// 根據策略文件：開倉位置 = 前一根K線的MidLow
func (g *GridAggregate) ProcessCandle(candle value_objects.Candle) (*value_objects.Signal, error) {
	// 第一根K線，只記錄不生成信號
	if g.lastCandle == nil {
		g.lastCandle = &candle
		return nil, nil
	}

	// 計算開倉位置：上一根K線的MidLow
	openPrice := g.lastCandle.MidLow()

	// 檢查當前價格是否觸及開倉位置
	// 如果當前K線的低點 <= 開倉價格，則生成開倉信號
	if candle.Low().IsBelowOrEqual(openPrice) {
		signal := g.generateOpenSignal(openPrice)

		// 更新lastCandle
		g.lastCandle = &candle

		return &signal, nil
	}

	// 沒有觸發開倉條件
	g.lastCandle = &candle
	return nil, nil
}

// generateOpenSignal 生成開倉信號
func (g *GridAggregate) generateOpenSignal(openPrice value_objects.Price) value_objects.Signal {
	// 目前只實作多單（BUY）
	action := value_objects.ActionBuy

	// 使用中間值的停利
	takeProfit := (g.takeProfitMin + g.takeProfitMax) / 2.0

	reason := fmt.Sprintf("open_long_at_mid_low_%.2f", openPrice.Value())

	return value_objects.NewSignal(
		action,
		g.instID,
		openPrice,
		g.positionSize,
		takeProfit,
		reason,
	)
}

// GetState 獲取當前狀態（用於日誌或監控）
func (g *GridAggregate) GetState() map[string]any {
	state := map[string]any{
		"instID":        g.instID,
		"positionSize":  g.positionSize,
		"takeProfitMin": g.takeProfitMin,
		"takeProfitMax": g.takeProfitMax,
	}

	if g.lastCandle != nil {
		state["lastCandleClose"] = g.lastCandle.Close().Value()
	}

	return state
}

// GetName 獲取策略名稱
func (g *GridAggregate) GetName() string {
	return "grid"
}
