package storage

// Redis Key Patterns
//
// 定義所有市場數據相關的 Redis key 格式
// 便於統一管理和修改
const (
	// ========== KV 存儲 Key（Pull 模式）==========

	// Ticker 相關
	KeyPatternTickerLatest = "price.latest.%s" // %s = instId
	KeyPatternTickerAll    = "price.latest.*"  // 用於清理

	// Candle 相關
	KeyPatternCandleLatest  = "candle.latest.%s.%s"  // %s = bar, %s = instId
	KeyPatternCandleHistory = "candle.history.%s.%s" // %s = bar, %s = instId

	KeyPatternCandleLatestAll  = "candle.latest.*"  // 用於清理
	KeyPatternCandleHistoryAll = "candle.history.*" // 用於清理

	// ========== Pub/Sub Channel（Push 模式）==========

	// Ticker Pub/Sub 頻道
	ChannelPatternTicker = "market.ticker.%s" // %s = instId

	// Candle Pub/Sub 頻道
	ChannelPatternCandle = "market.candle.%s.%s" // %s = bar, %s = instId
)

// CleanupPatterns 返回所有需要清理的 key pattern
func CleanupPatterns() []string {
	return []string{
		KeyPatternTickerAll,
		KeyPatternCandleLatestAll,
		KeyPatternCandleHistoryAll,
	}
}
