package types

import (
	"dizzycode.xyz/shared/domain/value_objects"
)

// OpenAdvice 開倉建議
type OpenAdvice struct {
	ShouldOpen   bool                // 是否應該開倉
	Price        value_objects.Price // 建議開倉價格
	PositionSize float64             // 建議倉位大小
	TakeProfit   float64             // 建議停利價格（絕對價格）
	Reason       string              // 原因
}

// Strategy 策略接口（跨專案共用）
//
// 用途：
//   - Trading Strategy Server: 實現具體策略邏輯
//   - Backtesting Engine: 使用策略進行回測
//   - Order Service: 調用策略獲取交易建議
type Strategy interface {
	// GetOpenAdvice 獲取開倉建議
	//
	// 參數：
	//   - currentPrice: 當前價格
	//   - lastCandle: 上一根 K 線
	//   - histories: 歷史 K 線（最多100根，用於趨勢分析）
	//
	// 返回：
	//   - OpenAdvice: 開倉建議
	GetOpenAdvice(
		currentPrice value_objects.Price,
		lastCandle value_objects.Candle,
		histories []value_objects.Candle,
	) OpenAdvice
}
