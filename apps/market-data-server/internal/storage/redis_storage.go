package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/okx"
)

// RedisStorage Redis 存储实现（实现 MarketDataStorage 接口）
type RedisStorage struct {
	client *redis.Client
	logger logger.Logger
}

// NewRedisStorage 创建 Redis 存储实例
func NewRedisStorage(client *redis.Client, logger logger.Logger) *RedisStorage {
	return &RedisStorage{
		client: client,
		logger: logger,
	}
}

// SaveLatestPrice 保存最新价格到 Redis
func (s *RedisStorage) SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error {
	key := fmt.Sprintf(KeyPatternTickerLatest, ticker.InstID)

	// 序列化为 JSON
	data, err := json.Marshal(ticker)
	if err != nil {
		return fmt.Errorf("failed to marshal ticker: %w", err)
	}

	// 写入 Redis 并设置 TTL
	if err := s.client.Set(ctx, key, data, 60*time.Second).Err(); err != nil {
		s.logger.Error("Failed to save latest price to Redis",
			"error", err,
			"key", key,
			"instId", ticker.InstID)
		return fmt.Errorf("failed to save latest price: %w", err)
	}

	return nil
}

// SaveLatestCandle 保存最新 K 线到 Redis
func (s *RedisStorage) SaveLatestCandle(ctx context.Context, candle okx.Candle) error {
	key := fmt.Sprintf(KeyPatternCandleLatest, candle.Bar, candle.InstID)

	// 序列化为 JSON
	data, err := json.Marshal(candle)
	if err != nil {
		return fmt.Errorf("failed to marshal candle: %w", err)
	}

	// 根据 bar 计算 TTL
	ttl := calculateCandleTTL(candle.Bar)

	// 写入 Redis 并设置 TTL
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		s.logger.Error("Failed to save latest candle to Redis",
			"error", err,
			"key", key,
			"instId", candle.InstID,
			"bar", candle.Bar)
		return fmt.Errorf("failed to save latest candle: %w", err)
	}

	return nil
}

// AppendCandleHistory 追加 K 线到历史列表
func (s *RedisStorage) AppendCandleHistory(ctx context.Context, candle okx.Candle, maxLength int) error {
	key := fmt.Sprintf(KeyPatternCandleHistory, candle.Bar, candle.InstID)

	// 序列化为 JSON
	data, err := json.Marshal(candle)
	if err != nil {
		return fmt.Errorf("failed to marshal candle: %w", err)
	}

	// 使用 Pipeline 提高性能
	pipe := s.client.Pipeline()

	// 1. 将新 K 线推入列表头部（最新的在前）
	pipe.LPush(ctx, key, data)

	// 2. 只保留最近 maxLength 根 K 线
	pipe.LTrim(ctx, key, 0, int64(maxLength-1))

	// 执行 Pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Error("Failed to append candle to history",
			"error", err,
			"key", key,
			"instId", candle.InstID,
			"bar", candle.Bar)
		return fmt.Errorf("failed to append candle history: %w", err)
	}

	s.logger.Debug("Appended candle to history",
		"key", key,
		"instId", candle.InstID,
		"bar", candle.Bar,
		"maxLength", maxLength)

	return nil
}

// ========== Pub/Sub 推送（Push 模式，保留接口）==========

// PublishPrice 推送价格到 Pub/Sub 频道
//
// channel 格式: market.ticker.{instId}
// 目前未启用，保留接口供未来使用
func (s *RedisStorage) PublishPrice(ctx context.Context, ticker okx.Ticker) error {
	channel := fmt.Sprintf(ChannelPatternTicker, ticker.InstID)

	// 序列化为 JSON
	data, err := json.Marshal(ticker)
	if err != nil {
		return fmt.Errorf("failed to marshal ticker: %w", err)
	}

	// 发布到 Redis Pub/Sub
	if err := s.client.Publish(ctx, channel, data).Err(); err != nil {
		s.logger.Error("Failed to publish price to channel",
			"error", err,
			"channel", channel,
			"instId", ticker.InstID)
		return fmt.Errorf("failed to publish price: %w", err)
	}

	s.logger.Debug("Published price to channel",
		"channel", channel,
		"instId", ticker.InstID,
		"last", ticker.Last)

	return nil
}

// PublishCandle 推送 K 线到 Pub/Sub 频道
//
// channel 格式: market.candle.{bar}.{instId}
// 目前未启用，保留接口供未来使用
func (s *RedisStorage) PublishCandle(ctx context.Context, candle okx.Candle) error {
	channel := fmt.Sprintf(ChannelPatternCandle, candle.Bar, candle.InstID)

	// 序列化为 JSON
	data, err := json.Marshal(candle)
	if err != nil {
		return fmt.Errorf("failed to marshal candle: %w", err)
	}

	// 发布到 Redis Pub/Sub
	if err := s.client.Publish(ctx, channel, data).Err(); err != nil {
		s.logger.Error("Failed to publish candle to channel",
			"error", err,
			"channel", channel,
			"instId", candle.InstID,
			"bar", candle.Bar)
		return fmt.Errorf("failed to publish candle: %w", err)
	}

	s.logger.Debug("Published candle to channel",
		"channel", channel,
		"instId", candle.InstID,
		"bar", candle.Bar,
		"confirm", candle.Confirm)

	return nil
}

// Cleanup 清理所有市场数据（关机时调用）
//
// 清理以下 key pattern：
// - price:latest:*       (Ticker 数据)
// - candle:latest:*      (最新 K 线)
// - candle:history:*     (历史 K 线)
//
// 防止策略服务读到过时的价格数据
func (s *RedisStorage) Cleanup(ctx context.Context) error {
	patterns := CleanupPatterns()

	var deletedCount int64

	for _, pattern := range patterns {
		// 使用 SCAN 命令获取所有匹配的 key（避免 KEYS 阻塞）
		iter := s.client.Scan(ctx, 0, pattern, 0).Iterator()
		keys := []string{}

		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}

		if err := iter.Err(); err != nil {
			s.logger.Error("Failed to scan keys", "pattern", pattern, "error", err)
			continue
		}

		// 批量删除 key
		if len(keys) > 0 {
			deleted, err := s.client.Del(ctx, keys...).Result()
			if err != nil {
				s.logger.Error("Failed to delete keys", "pattern", pattern, "error", err)
				continue
			}

			deletedCount += deleted
			s.logger.Info("Cleaned up market data",
				"pattern", pattern,
				"deleted", deleted)
		}
	}

	s.logger.Info("Market data cleanup completed",
		"totalDeleted", deletedCount)

	return nil
}

// calculateCandleTTL 根据 K 线周期计算合适的 TTL
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
		// 默认 60 秒
		return 60 * time.Second
	}
}
