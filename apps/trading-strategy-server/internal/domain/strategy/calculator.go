package strategy

import "math"

// GridCalculator 網格計算器（領域服務）
// 特點：無狀態、純函數、可獨立測試
type GridCalculator struct{}

// NewGridCalculator 創建計算器
func NewGridCalculator() *GridCalculator {
	return &GridCalculator{}
}

// CalculateGridLines 計算網格線價格
// 使用等差數列分配網格線
func (c *GridCalculator) CalculateGridLines(upperBound, lowerBound float64, levels int) []float64 {
	if levels < 2 {
		return []float64{}
	}

	gridLines := make([]float64, levels)
	step := (upperBound - lowerBound) / float64(levels-1)

	for i := 0; i < levels; i++ {
		gridLines[i] = lowerBound + float64(i)*step
	}

	return gridLines
}

// DetectCrossedLine 檢測價格穿越了哪條網格線
// 返回 -1 表示沒有穿越
func (c *GridCalculator) DetectCrossedLine(currentPrice, lastPrice float64, gridLines []float64) int {
	for i, line := range gridLines {
		// 向上穿越
		if lastPrice < line && currentPrice >= line {
			return i
		}
		// 向下穿越
		if lastPrice > line && currentPrice <= line {
			return i
		}
	}
	return -1
}

// CalculatePositionSize 計算每格應該交易的數量
func (c *GridCalculator) CalculatePositionSize(totalCapital, upperBound, lowerBound float64, levels int) float64 {
	// 簡單策略：總資金 / 網格數
	// 實際應用中可能需要更複雜的計算
	midPrice := (upperBound + lowerBound) / 2
	return (totalCapital / float64(levels)) / midPrice
}

// CalculateGridSpacing 計算網格間距（百分比）
func (c *GridCalculator) CalculateGridSpacing(upperBound, lowerBound float64, levels int) float64 {
	if levels < 2 {
		return 0
	}
	step := (upperBound - lowerBound) / float64(levels-1)
	midPrice := (upperBound + lowerBound) / 2
	return (step / midPrice) * 100 // 返回百分比
}

// RoundPrice 價格四捨五入（避免精度問題）
func (c *GridCalculator) RoundPrice(price float64, decimals int) float64 {
	multiplier := math.Pow10(decimals)
	return math.Round(price*multiplier) / multiplier
}
