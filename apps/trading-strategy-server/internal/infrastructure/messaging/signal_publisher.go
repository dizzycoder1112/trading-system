package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"dizzycode.xyz/logger"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy"
)

// RedisSignalPublisher implements the SignalPublisher port from application layer
type RedisSignalPublisher struct {
	client *RedisClient
	logger logger.Logger
}

// NewRedisSignalPublisher creates a new RedisSignalPublisher
func NewRedisSignalPublisher(client *RedisClient, log logger.Logger) *RedisSignalPublisher {
	return &RedisSignalPublisher{
		client: client,
		logger: log,
	}
}

// Publish implements application.SignalPublisher interface
// Publishes signal to Redis Pub/Sub channel: strategy.signals.{instId}
func (p *RedisSignalPublisher) Publish(ctx context.Context, signal strategy.Signal) error {
	channel := fmt.Sprintf("strategy.signals.%s", signal.InstID())

	// Signal already implements MarshalJSON, serialize directly
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	// Publish to Redis
	if err := p.client.Client().Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish signal to channel %s: %w", channel, err)
	}

	p.logger.Info("Signal published", map[string]any{
		"channel": channel,
		"action":  signal.Action(),
		"price":   signal.Price().Value(),
		"reason":  signal.Reason(),
	})

	return nil
}
