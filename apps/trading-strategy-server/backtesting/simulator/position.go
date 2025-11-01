package simulator

import (
	"fmt"
	"time"
)

// Position 單筆持倉
type Position struct {
	ID               string    // 持倉ID
	EntryPrice       float64   // 開倉價格
	Size             float64   // 倉位大小（美元）
	OpenTime         time.Time // 開倉時間
	TargetClosePrice float64   // 目標平倉價格
}

// ClosedPosition 已平倉記錄
type ClosedPosition struct {
	Position              // 嵌入原始持倉信息
	ClosePrice   float64  // 實際平倉價格
	CloseTime    time.Time // 平倉時間
	RealizedPnL  float64  // 已實現盈虧（扣除手續費後）
	HoldDuration time.Duration // 持倉時長
}

// PositionTracker 倉位追蹤器
type PositionTracker struct {
	openPositions   []Position       // 未平倉持倉
	closedPositions []ClosedPosition // 已平倉記錄
	nextID          int              // 用於生成持倉ID
	avgCost         float64          // ⭐ 累進的平均成本
	totalCoins      float64          // ⭐ 總持倉幣數
}

// NewPositionTracker 創建倉位追蹤器
func NewPositionTracker() *PositionTracker {
	return &PositionTracker{
		openPositions:   make([]Position, 0),
		closedPositions: make([]ClosedPosition, 0),
		nextID:          1,
		avgCost:         0,
		totalCoins:      0,
	}
}

// AddPosition 添加新持倉（使用累進式計算平均成本）⭐
func (pt *PositionTracker) AddPosition(
	entryPrice float64,
	size float64,
	openTime time.Time,
	targetClosePrice float64,
) Position {
	// ⭐ 計算新買入的幣數
	newCoins := size / entryPrice

	// ⭐ 累進公式更新平均成本
	// 新平均成本 = (原平均成本 × 原幣數 + 新價格 × 新幣數) / (原幣數 + 新幣數)
	if pt.totalCoins > 0 {
		pt.avgCost = (pt.avgCost*pt.totalCoins + entryPrice*newCoins) / (pt.totalCoins + newCoins)
	} else {
		// 第一次開倉，平均成本就是開倉價
		pt.avgCost = entryPrice
	}

	// ⭐ 更新總幣數
	pt.totalCoins += newCoins

	// 創建持倉記錄
	position := Position{
		ID:               fmt.Sprintf("pos_%d", pt.nextID),
		EntryPrice:       entryPrice,
		Size:             size,
		OpenTime:         openTime,
		TargetClosePrice: targetClosePrice,
	}

	pt.openPositions = append(pt.openPositions, position)
	pt.nextID++

	return position
}

// ClosePosition 平倉（關倉只減少幣數，不改變平均成本）⭐
func (pt *PositionTracker) ClosePosition(
	positionID string,
	closePrice float64,
	closeTime time.Time,
	realizedPnL float64,
) error {
	// 查找並移除開倉記錄
	foundIndex := -1
	var position Position

	for i, p := range pt.openPositions {
		if p.ID == positionID {
			foundIndex = i
			position = p
			break
		}
	}

	if foundIndex == -1 {
		return fmt.Errorf("position not found: %s", positionID)
	}

	// ⭐ 計算減少的幣數（用該倉位的開倉價計算）
	// 重要：必須用 EntryPrice 而不是 avgCost，因為：
	// - position.Size 是該筆開倉投入的 USDT 金額
	// - 該筆開倉實際買入的幣數 = Size / EntryPrice
	// - 平倉時應該平掉實際買入的幣數，而不是用平均成本計算的幣數
	closedCoins := position.Size / position.EntryPrice

	// ⭐ 只減少總幣數，平均成本不變
	pt.totalCoins -= closedCoins

	// ⭐ 如果所有倉位都關閉了，重置平均成本
	if pt.totalCoins <= 0.00001 { // 使用小值避免浮點誤差
		pt.avgCost = 0
		pt.totalCoins = 0
	}

	// 從開倉列表中移除
	pt.openPositions = append(pt.openPositions[:foundIndex], pt.openPositions[foundIndex+1:]...)

	// 添加到已平倉列表
	closedPosition := ClosedPosition{
		Position:     position,
		ClosePrice:   closePrice,
		CloseTime:    closeTime,
		RealizedPnL:  realizedPnL,
		HoldDuration: closeTime.Sub(position.OpenTime),
	}

	pt.closedPositions = append(pt.closedPositions, closedPosition)

	return nil
}

// CloseAllPositions 平倉所有持倉（回測結束時使用）
func (pt *PositionTracker) CloseAllPositions(
	closePrice float64,
	closeTime time.Time,
	feeRate float64,
) {
	for _, position := range pt.openPositions {
		// 計算盈虧
		priceChange := closePrice - position.EntryPrice
		profitBeforeFee := (priceChange / position.EntryPrice) * position.Size
		fee := position.Size * feeRate * 2 // 開倉 + 平倉手續費
		realizedPnL := profitBeforeFee - fee

		// 平倉
		pt.ClosePosition(position.ID, closePrice, closeTime, realizedPnL)
	}
}

// GetOpenPositions 獲取所有未平倉持倉
func (pt *PositionTracker) GetOpenPositions() []Position {
	return pt.openPositions
}

// GetClosedPositions 獲取所有已平倉記錄
func (pt *PositionTracker) GetClosedPositions() []ClosedPosition {
	return pt.closedPositions
}

// HasOpenPositions 是否有未平倉持倉
func (pt *PositionTracker) HasOpenPositions() bool {
	return len(pt.openPositions) > 0
}

// GetOpenPositionCount 獲取未平倉數量
func (pt *PositionTracker) GetOpenPositionCount() int {
	return len(pt.openPositions)
}

// CalculateAverageCost 計算平均成本（直接返回累進計算的結果）⭐
func (pt *PositionTracker) CalculateAverageCost() float64 {
	return pt.avgCost
}

// CalculateUnrealizedPnL 計算未實現盈虧（含預估平倉手續費）
//
// 重要：開倉手續費已經在開倉時從餘額中扣除，不應該在這裡再扣一次！
// UnrealizedPnL 只應該包含：
//   1. 價格變化帶來的浮動盈虧
//   2. 預估的平倉手續費（因為還沒平倉）
//
// currentPrice: 當前市場價格
// feeRate: 手續費率（用於估算平倉成本）
func (pt *PositionTracker) CalculateUnrealizedPnL(currentPrice float64, feeRate float64) float64 {
	if len(pt.openPositions) == 0 {
		return 0
	}

	totalPnL := 0.0

	for _, pos := range pt.openPositions {
		// ⭐ 1. 計算關閉的幣數（和 OrderSimulator 使用相同邏輯）
		closedCoins := pos.Size / pos.EntryPrice

		// ⭐ 2. 計算價格變化和盈虧（基於幣數）
		priceChange := currentPrice - pos.EntryPrice
		profitBeforeFee := closedCoins * priceChange

		// ⭐ 3. 計算平倉手續費（只計算平倉費，開倉費已經在開倉時扣過了）
		closeValue := pos.Size + profitBeforeFee // 平倉時的總價值
		closeFee := closeValue * feeRate         // 平倉手續費基於平倉價值

		// ⭐ 4. 未實現盈虧 = 浮動盈虧 - 預估平倉費（不包含開倉費）
		pnl := profitBeforeFee - closeFee

		totalPnL += pnl
	}

	return totalPnL
}

// CalculateTotalRealizedPnL 計算總已實現盈虧
func (pt *PositionTracker) CalculateTotalRealizedPnL() float64 {
	total := 0.0
	for _, closed := range pt.closedPositions {
		total += closed.RealizedPnL
	}
	return total
}

// GetTotalSize 獲取總倉位大小（美元）
func (pt *PositionTracker) GetTotalSize() float64 {
	total := 0.0
	for _, pos := range pt.openPositions {
		total += pos.Size
	}
	return total
}

// GetAverageHoldDuration 獲取平均持倉時長
func (pt *PositionTracker) GetAverageHoldDuration() time.Duration {
	if len(pt.closedPositions) == 0 {
		return 0
	}

	totalDuration := time.Duration(0)
	for _, closed := range pt.closedPositions {
		totalDuration += closed.HoldDuration
	}

	return totalDuration / time.Duration(len(pt.closedPositions))
}

// GetWinRate 獲取勝率
func (pt *PositionTracker) GetWinRate() float64 {
	if len(pt.closedPositions) == 0 {
		return 0
	}

	winCount := 0
	for _, closed := range pt.closedPositions {
		if closed.RealizedPnL > 0 {
			winCount++
		}
	}

	return float64(winCount) / float64(len(pt.closedPositions))
}
