package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"dizzycode.xyz/logger"
)

// CandleSubscriber subscribes to candle data from Redis Pub/Sub
type CandleSubscriber struct {
	client *RedisClient
	logger logger.Logger
}

// NewCandleSubscriber creates a new CandleSubscriber
func NewCandleSubscriber(client *RedisClient, log logger.Logger) *CandleSubscriber {
	return &CandleSubscriber{
		client: client,
		logger: log,
	}
}

// Subscribe subscribes to candle data and invokes the callback for each candle
// Channel format: market.candle.{bar}.{instId}
// Example: market.candle.1m.ETH-USDT
func (s *CandleSubscriber) Subscribe(
	ctx context.Context,
	instID string,
	bar string,
	onCandle func(price float64) error,
) error {
	channel := fmt.Sprintf("market.candle.%s.%s", bar, instID)

	s.logger.Info("Subscribing to candle channel", map[string]any{
		"channel": channel,
		"instId":  instID,
		"bar":     bar,
	})

	pubsub := s.client.Client().Subscribe(ctx, channel)
	defer pubsub.Close()

	// Wait for subscription confirmation
	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to channel %s: %w", channel, err)
	}

	s.logger.Info("Successfully subscribed to candle channel", map[string]any{"channel": channel})

	// Process messages
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Candle subscription cancelled", map[string]any{"channel": channel})
			return ctx.Err()

		case msg := <-pubsub.Channel():
			if msg == nil {
				continue
			}

			if err := s.handleCandleMessage(msg.Payload, onCandle); err != nil {
				s.logger.Error("Failed to handle candle message", map[string]any{
					"error":   err,
					"channel": channel,
				})
				// Continue processing other messages
			}
		}
	}
}

// handleCandleMessage parses candle JSON and extracts price
func (s *CandleSubscriber) handleCandleMessage(payload string, onCandle func(price float64) error) error {
	var candle struct {
		Close string `json:"close"`
	}

	if err := json.Unmarshal([]byte(payload), &candle); err != nil {
		return fmt.Errorf("failed to parse candle JSON: %w", err)
	}

	price, err := strconv.ParseFloat(candle.Close, 64)
	if err != nil {
		return fmt.Errorf("invalid price value '%s': %w", candle.Close, err)
	}

	s.logger.Debug("Received candle", map[string]any{"price": price})

	// Invoke callback
	if err := onCandle(price); err != nil {
		return fmt.Errorf("candle handler failed: %w", err)
	}

	return nil
}
