package storage

import (
	"context"

	"dizzycoder.xyz/market-data-service/internal/okx"
)

// MarketDataStorage 市场数据存储接口（抽象层）
//
// 这个接口定义了存储市场数据的方法，使得业务逻辑与具体存储实现解耦。
// 可以轻松替换为 Redis, Kafka, RabbitMQ 等不同的存储后端。
type MarketDataStorage interface {
	// SaveLatestPrice 保存最新价格（Ticker 数据）
	// key 格式: price:latest:{instId}
	SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error

	// SaveLatestCandle 保存最新 K 线数据
	// key 格式: candle:latest:{bar}:{instId}
	SaveLatestCandle(ctx context.Context, candle okx.Candle) error

	// AppendCandleHistory 追加 K 线到历史列表（仅已确认的 K 线）
	// key 格式: candle:history:{bar}:{instId}
	// maxLength: 保留的最大 K 线数量
	AppendCandleHistory(ctx context.Context, candle okx.Candle, maxLength int) error

	// Cleanup 清理所有市场数据（服务关闭时调用）
	// 防止策略服务读取到过时的价格数据
	Cleanup(ctx context.Context) error
}
