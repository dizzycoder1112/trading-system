package messaging

import (
	"context"
	"fmt"

	"dizzycode.xyz/logger"
	"github.com/redis/go-redis/v9"
)

// RedisClient wraps redis.Client with logging and health check
type RedisClient struct {
	rdb    *redis.Client
	logger logger.Logger
}

// NewRedisClient creates a new Redis client with connection validation
func NewRedisClient(addr, password string, db int, log logger.Logger) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: 10,
	})

	// Ping to verify connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}

	log.Info("Redis client connected", map[string]any{
		"addr": addr,
		"db":   db,
	})

	return &RedisClient{
		rdb:    rdb,
		logger: log,
	}, nil
}

// Client returns the underlying redis.Client for direct access
func (c *RedisClient) Client() *redis.Client {
	return c.rdb
}

// Close closes the Redis connection
func (c *RedisClient) Close() error {
	c.logger.Info("Closing Redis connection", nil)
	return c.rdb.Close()
}

// Ping checks if the connection is alive
func (c *RedisClient) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}
