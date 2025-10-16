package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"dizzycode.xyz/logger"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/value_objects"
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
	onCandle func(candle value_objects.Candle) error,
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

// handleCandleMessage parses candle JSON and creates Candle value object
func (s *CandleSubscriber) handleCandleMessage(payload string, onCandle func(candle value_objects.Candle) error) error {
	var raw struct {
		Open      string `json:"open"`
		High      string `json:"high"`
		Low       string `json:"low"`
		Close     string `json:"close"`
		Timestamp string `json:"ts"`
	}

	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return fmt.Errorf("failed to parse candle JSON: %w", err)
	}

	// Parse prices
	open, err := strconv.ParseFloat(raw.Open, 64)
	if err != nil {
		return fmt.Errorf("invalid open price '%s': %w", raw.Open, err)
	}

	high, err := strconv.ParseFloat(raw.High, 64)
	if err != nil {
		return fmt.Errorf("invalid high price '%s': %w", raw.High, err)
	}

	low, err := strconv.ParseFloat(raw.Low, 64)
	if err != nil {
		return fmt.Errorf("invalid low price '%s': %w", raw.Low, err)
	}

	close, err := strconv.ParseFloat(raw.Close, 64)
	if err != nil {
		return fmt.Errorf("invalid close price '%s': %w", raw.Close, err)
	}

	// Parse timestamp (milliseconds since epoch)
	tsMs, err := strconv.ParseInt(raw.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp '%s': %w", raw.Timestamp, err)
	}
	timestamp := time.Unix(0, tsMs*int64(time.Millisecond))

	// Create Candle value object
	candle, err := value_objects.NewCandle(open, high, low, close, timestamp)
	if err != nil {
		return fmt.Errorf("failed to create candle: %w", err)
	}

	s.logger.Debug("Received candle", map[string]any{
		"open":  open,
		"high":  high,
		"low":   low,
		"close": close,
	})

	// Invoke callback
	if err := onCandle(candle); err != nil {
		return fmt.Errorf("candle handler failed: %w", err)
	}

	return nil
}
