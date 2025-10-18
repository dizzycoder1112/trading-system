package redis

import (
	"context"
	"fmt"
	"time"

	"dizzycode.xyz/logger"
	"github.com/redis/go-redis/v9"
)

// Config Redis 配置
type Config struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
	Logger   logger.Logger
}

// NewClient 創建新的 Redis 客戶端（直接返回 *redis.Client）
//
// 不使用 wrapper，直接返回 redis.Client：
// - redis.Client 已經提供了連接池、重試等功能
// - 不需要額外的抽象層（storage.RedisStorage 已經是封装层）
// - 避免暴露內部實現的問題
func NewClient(cfg Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// 測試連接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	cfg.Logger.Info("Connected to Redis successfully", map[string]any{
		"host": cfg.Addr,
		"db":   cfg.DB,
	})

	return rdb, nil
}
