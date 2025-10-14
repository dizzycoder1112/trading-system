package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/okx"
)

// Publisher Redis Pub/Sub 發布器
type Publisher struct {
	client *Client
	logger logger.Logger
}

// NewPublisher 創建新的發布器
func NewPublisher(client *Client, logger logger.Logger) *Publisher {
	return &Publisher{
		client: client,
		logger: logger,
	}
}

// PublishTicker 發布 Ticker 數據到 Redis Pub/Sub
// Channel 格式: market:ticker:{instId}
// 例如: market:ticker:BTC-USDT
func (p *Publisher) PublishTicker(ctx context.Context, ticker okx.Ticker) error {
	channel := fmt.Sprintf("market:ticker:%s", ticker.InstID)

	// 序列化為 JSON
	data, err := json.Marshal(ticker)
	if err != nil {
		return fmt.Errorf("failed to marshal ticker: %w", err)
	}

	// 發布到 Redis Pub/Sub
	if err := p.client.rdb.Publish(ctx, channel, data).Err(); err != nil {
		p.logger.Error("Failed to publish ticker to Redis",
			"error", err,
			"channel", channel,
			"instId", ticker.InstID)
		return fmt.Errorf("failed to publish ticker: %w", err)
	}

	// Debug 日誌（只在 LOG_LEVEL=debug 時顯示）
	p.logger.Debug("Published ticker to Redis",
		"channel", channel,
		"instId", ticker.InstID,
		"last", ticker.Last)

	return nil
}

// PublishCandle 發布 Candle 數據到 Redis Pub/Sub
// Channel 格式: market:candle:{bar}:{instId}
// 例如: market:candle:1m:BTC-USDT
func (p *Publisher) PublishCandle(ctx context.Context, candle okx.Candle) error {
	channel := fmt.Sprintf("market:candle:%s:%s", candle.Bar, candle.InstID)

	// 序列化為 JSON
	data, err := json.Marshal(candle)
	if err != nil {
		return fmt.Errorf("failed to marshal candle: %w", err)
	}

	// 發布到 Redis Pub/Sub
	if err := p.client.rdb.Publish(ctx, channel, data).Err(); err != nil {
		p.logger.Error("Failed to publish candle to Redis",
			"error", err,
			"channel", channel,
			"instId", candle.InstID,
			"bar", candle.Bar)
		return fmt.Errorf("failed to publish candle: %w", err)
	}

	// Debug 日誌（只在 LOG_LEVEL=debug 時顯示）
	p.logger.Debug("Published candle to Redis",
		"channel", channel,
		"instId", candle.InstID,
		"bar", candle.Bar,
		"close", candle.Close)

	return nil
}

// CacheLatestTicker 快取最新 Ticker 數據到 Redis
// Key 格式: price:latest:{instId}
// TTL: 60 秒
func (p *Publisher) CacheLatestTicker(ctx context.Context, ticker okx.Ticker) error {
	key := fmt.Sprintf("price:latest:%s", ticker.InstID)

	// 序列化為 JSON
	data, err := json.Marshal(ticker)
	if err != nil {
		return fmt.Errorf("failed to marshal ticker: %w", err)
	}

	// 寫入 Redis 並設置 TTL
	if err := p.client.rdb.Set(ctx, key, data, 60*time.Second).Err(); err != nil {
		p.logger.Error("Failed to cache ticker to Redis",
			"error", err,
			"key", key,
			"instId", ticker.InstID)
		return fmt.Errorf("failed to cache ticker: %w", err)
	}

	return nil
}

// CacheLatestCandle 快取最新 Candle 數據到 Redis
// Key 格式: candle:latest:{bar}:{instId}
// TTL: 根據週期動態設置（1m=60s, 5m=300s, 1H=3600s）
func (p *Publisher) CacheLatestCandle(ctx context.Context, candle okx.Candle) error {
	key := fmt.Sprintf("candle:latest:%s:%s", candle.Bar, candle.InstID)

	// 序列化為 JSON
	data, err := json.Marshal(candle)
	if err != nil {
		return fmt.Errorf("failed to marshal candle: %w", err)
	}

	// 根據 bar 計算 TTL
	ttl := calculateCandleTTL(candle.Bar)

	// 寫入 Redis 並設置 TTL
	if err := p.client.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		p.logger.Error("Failed to cache candle to Redis",
			"error", err,
			"key", key,
			"instId", candle.InstID,
			"bar", candle.Bar)
		return fmt.Errorf("failed to cache candle: %w", err)
	}

	return nil
}

// calculateCandleTTL 根據 K線週期計算合適的 TTL
func calculateCandleTTL(bar string) time.Duration {
	switch bar {
	case "1s":
		return 2 * time.Second
	case "1m":
		return 120 * time.Second
	case "3m":
		return 360 * time.Second
	case "5m":
		return 600 * time.Second
	case "15m":
		return 1800 * time.Second
	case "30m":
		return 3600 * time.Second
	case "1H":
		return 7200 * time.Second
	case "2H":
		return 14400 * time.Second
	case "4H":
		return 28800 * time.Second
	default:
		// 預設 60 秒
		return 60 * time.Second
	}
}
