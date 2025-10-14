package strategy

import (
	"errors"
	"fmt"
)

// GridAggregate 網格聚合根
// 特點：
// 1. 封裝業務規則
// 2. 保證不變性（Invariants）
// 3. 管理內部狀態
// 4. 不依賴任何技術實現
type GridAggregate struct {
	instID       string
	upperBound   Price
	lowerBound   Price
	gridLevels   int
	gridLines    []float64
	currentPrice Price
	lastPrice    Price
	calculator   *GridCalculator
	// 可選：追蹤已觸發的網格線
	triggeredLines map[int]bool
}

// NewGridAggregate 創建網格聚合根（工廠方法）
func NewGridAggregate(instID string, upper, lower float64, levels int) (*GridAggregate, error) {
	// 創建價格值對象
	upperPrice, err := NewPrice(upper)
	if err != nil {
		return nil, fmt.Errorf("invalid upper bound: %w", err)
	}

	lowerPrice, err := NewPrice(lower)
	if err != nil {
		return nil, fmt.Errorf("invalid lower bound: %w", err)
	}

	// 驗證業務規則
	if upperPrice.IsBelow(lowerPrice) || upperPrice.Equals(lowerPrice) {
		return nil, errors.New("upper bound must be greater than lower bound")
	}

	if levels < 2 {
		return nil, errors.New("must have at least 2 grid levels")
	}

	calculator := NewGridCalculator()
	gridLines := calculator.CalculateGridLines(upper, lower, levels)

	// 初始價格設為中間值
	midPrice, _ := NewPrice((upper + lower) / 2)

	return &GridAggregate{
		instID:         instID,
		upperBound:     upperPrice,
		lowerBound:     lowerPrice,
		gridLevels:     levels,
		gridLines:      gridLines,
		currentPrice:   midPrice,
		lastPrice:      midPrice,
		calculator:     calculator,
		triggeredLines: make(map[int]bool),
	}, nil
}

// ProcessPriceUpdate 處理價格更新（核心業務邏輯）
// 返回信號或 nil
func (g *GridAggregate) ProcessPriceUpdate(newPrice Price) (*Signal, error) {
	// 1. 驗證價格在網格範圍內
	if newPrice.IsAbove(g.upperBound) {
		return nil, fmt.Errorf("price %.2f above upper bound %.2f",
			newPrice.Value(), g.upperBound.Value())
	}
	if newPrice.IsBelow(g.lowerBound) {
		return nil, fmt.Errorf("price %.2f below lower bound %.2f",
			newPrice.Value(), g.lowerBound.Value())
	}

	// 2. 檢測是否穿越網格線
	crossedLine := g.calculator.DetectCrossedLine(
		newPrice.Value(),
		g.lastPrice.Value(),
		g.gridLines,
	)

	// 3. 更新狀態
	g.lastPrice = g.currentPrice
	g.currentPrice = newPrice

	// 4. 如果沒有穿越，返回 nil
	if crossedLine == -1 {
		return nil, nil
	}

	// 5. 生成交易信號
	signal := g.generateSignal(newPrice, crossedLine)

	// 6. 標記網格線已觸發
	g.triggeredLines[crossedLine] = true

	return &signal, nil
}

// generateSignal 生成交易信號（私有方法）
func (g *GridAggregate) generateSignal(price Price, gridLineIndex int) Signal {
	var action SignalAction
	var reason string

	// 業務規則：向上穿越 → SELL，向下穿越 → BUY
	if price.IsAbove(g.lastPrice) {
		action = ActionSell
		reason = fmt.Sprintf("grid_cross_up_line_%d", gridLineIndex)
	} else {
		action = ActionBuy
		reason = fmt.Sprintf("grid_cross_down_line_%d", gridLineIndex)
	}

	// 固定數量（實際應用中可能需要根據資金動態計算）
	quantity := 0.01

	return NewSignal(action, g.instID, price, quantity, reason)
}

// GetState 獲取當前狀態（用於日誌或監控）
func (g *GridAggregate) GetState() map[string]interface{} {
	return map[string]interface{}{
		"instID":       g.instID,
		"upperBound":   g.upperBound.Value(),
		"lowerBound":   g.lowerBound.Value(),
		"gridLevels":   g.gridLevels,
		"currentPrice": g.currentPrice.Value(),
		"gridLines":    g.gridLines,
		"triggered":    len(g.triggeredLines),
	}
}

// Validate 驗證聚合根的不變性（Invariants）
func (g *GridAggregate) Validate() error {
	if g.upperBound.IsBelow(g.lowerBound) || g.upperBound.Equals(g.lowerBound) {
		return errors.New("invariant violated: upper must be greater than lower")
	}
	if g.gridLevels < 2 {
		return errors.New("invariant violated: must have at least 2 grid levels")
	}
	if len(g.gridLines) != g.gridLevels {
		return errors.New("invariant violated: grid lines count mismatch")
	}
	return nil
}
