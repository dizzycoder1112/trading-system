package storage

// Redis Key Patterns
//
// 定义所有市场数据相关的 Redis key 格式
// 便于统一管理和修改
const (
	// ========== KV 存储 Key（Pull 模式）==========

	// Ticker 相关
	KeyPatternTickerLatest = "price.latest.%s" // %s = instId
	KeyPatternTickerAll    = "price.latest.*"  // 用于清理

	// Candle 相关
	KeyPatternCandleLatest  = "candle.latest.%s.%s"  // %s = bar, %s = instId
	KeyPatternCandleHistory = "candle.history.%s.%s" // %s = bar, %s = instId

	KeyPatternCandleLatestAll  = "candle.latest.*"  // 用于清理
	KeyPatternCandleHistoryAll = "candle.history.*" // 用于清理

	// ========== Pub/Sub Channel（Push 模式）==========

	// Ticker Pub/Sub 频道
	ChannelPatternTicker = "market.ticker.%s" // %s = instId

	// Candle Pub/Sub 频道
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
