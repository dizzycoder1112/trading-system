package redis

import (
	"context"
	"fmt"
	"time"

	"dizzycode.xyz/logger"
	"github.com/redis/go-redis/v9"
)

// Client Redis 客戶端封裝
type Client struct {
	rdb    *redis.Client
	logger logger.Logger
}

// Config Redis 配置
type Config struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
	Logger   logger.Logger
}

// NewClient 創建新的 Redis 客戶端
func NewClient(cfg Config) (*Client, error) {
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

	return &Client{
		rdb:    rdb,
		logger: cfg.Logger,
	}, nil
}

// Close 關閉 Redis 連接
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping 測試連接
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// GetClient 獲取底層 Redis 客戶端（供其他模組使用）
func (c *Client) GetClient() *redis.Client {
	return c.rdb
}
