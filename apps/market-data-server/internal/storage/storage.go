package storage

import (
	"context"

	"dizzycoder.xyz/market-data-service/internal/okx"
)

// MarketDataStorage 市場數據存儲接口（抽象層）
//
// 這個接口定義了存儲市場數據的方法，使得業務邏輯與具體存儲實現解耦。
// 可以輕鬆替換為 Redis, Kafka, RabbitMQ 等不同的存儲後端。
type MarketDataStorage interface {
	// ========== KV 存儲（Pull 模式，目前使用）==========

	// SaveLatestPrice 保存最新價格（Ticker 數據）
	// key 格式: price.latest.{instId}
	SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error

	// SaveLatestCandle 保存最新 K 線數據
	// key 格式: candle.latest.{bar}.{instId}
	SaveLatestCandle(ctx context.Context, candle okx.Candle) error

	// AppendCandleHistory 追加 K 線到歷史列表（僅已確認的 K 線）
	// key 格式: candle.history.{bar}.{instId}
	// maxLength: 保留的最大 K 線數量
	AppendCandleHistory(ctx context.Context, candle okx.Candle, maxLength int) error

	// ========== Pub/Sub 推送（Push 模式，保留接口）==========

	// PublishPrice 推送價格到 Pub/Sub 頻道（可選，目前未啟用）
	// channel 格式: market.ticker.{instId}
	PublishPrice(ctx context.Context, ticker okx.Ticker) error

	// PublishCandle 推送 K 線到 Pub/Sub 頻道（可選，目前未啟用）
	// channel 格式: market.candle.{bar}.{instId}
	PublishCandle(ctx context.Context, candle okx.Candle) error

	// ========== 管理 ==========

	// Cleanup 清理所有市場數據（服務關閉時調用）
	// 防止策略服務讀取到過時的價格數據
	Cleanup(ctx context.Context) error
}
