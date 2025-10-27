package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"dizzycode.xyz/logger"
	"dizzycode.xyz/shared/domain/value_objects"
)

type CandleData struct {
	InstID  string `json:"instId"`
	Bar     string `json:"bar"`
	Open    string `json:"open"`
	High    string `json:"high"`
	Low     string `json:"low"`
	Close   string `json:"close"`
	Confirm string `json:"confirm"`
	Ts      string `json:"ts"` // Timestamp in milliseconds
}

// MarketDataReader 從 Redis 讀取市場數據
type MarketDataReader struct {
	client *RedisClient
	logger logger.Logger
}

// NewMarketDataReader 創建 MarketDataReader
func NewMarketDataReader(client *RedisClient, log logger.Logger) *MarketDataReader {
	return &MarketDataReader{
		client: client,
		logger: log,
	}
}

// GetLatestCandle 從 Redis 讀取最新的 Candle（包括未確認的）
// Key format: candle.latest.{bar}.{instId}
// 用於即時監控，不用於策略計算
func (r *MarketDataReader) GetLatestCandle(ctx context.Context, instID string, bar string) (value_objects.Candle, error) {
	key := fmt.Sprintf("candle.latest.%s.%s", bar, instID)

	// Get from Redis
	val, err := r.client.Client().Get(ctx, key).Result()
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("failed to get candle from Redis (key: %s): %w", key, err)
	}

	// Parse JSON
	var candleData CandleData

	if err := json.Unmarshal([]byte(val), &candleData); err != nil {
		return value_objects.Candle{}, fmt.Errorf("failed to parse candle JSON: %w", err)
	}

	candle, err := parseCandleData(candleData)
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("failed to convert candle %w", err)
	}

	r.logger.Debug("Retrieved candle from Redis", map[string]any{
		"key":   key,
		"close": candle.Close().Value(),
		"low":   candle.Low().Value(),
	})

	return *candle, nil
}

func (r *MarketDataReader) GetCandleHistories(ctx context.Context, instID string, bar string) ([]value_objects.Candle, error) {
	key := fmt.Sprintf("candle.history.%s.%s", bar, instID)

	// Get from Redis
	val, err := r.client.Client().LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get candle from Redis (key: %s): %w", key, err)
	}

	r.logger.Debug("Retrieved candle from Redis", map[string]any{
		"key":   key,
		"value": val,
	})

	// Parse JSON
	// ✅ 预分配长度（不只是容量），因为使用索引赋值
	candleDataSet := make([]value_objects.Candle, len(val))

	for i, candle := range val {

		var candleData CandleData

		if err := json.Unmarshal([]byte(candle), &candleData); err != nil {
			return nil, fmt.Errorf("failed to parse candle at index %d (instId: %s, bar: %s): %w",
				i, instID, bar, err) // ✅ 提供详细错误信息
		}

		result, err := parseCandleData(candleData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert candle at index %d: %w", i, err)
		}
		candleDataSet[i] = *result

	}

	r.logger.Debug("Retrieved candle from Redis", map[string]any{
		"key": key,
	})

	return candleDataSet, nil
}

// GetLatestPrice 從 Redis 讀取最新價格（用於模擬 Order Service）
// Key format: price.latest.{instId}
func (r *MarketDataReader) GetLatestPrice(ctx context.Context, instID string) (value_objects.Price, error) {
	key := fmt.Sprintf("price.latest.%s", instID)

	val, err := r.client.Client().Get(ctx, key).Result()
	if err != nil {
		return value_objects.Price{}, fmt.Errorf("failed to get price from Redis (key: %s): %w", key, err)
	}

	// Parse JSON
	var priceData struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal([]byte(val), &priceData); err != nil {
		return value_objects.Price{}, fmt.Errorf("failed to parse price JSON: %w", err)
	}

	// Convert to float64
	currentPrice, err := strconv.ParseFloat(priceData.Last, 64)
	if err != nil {
		return value_objects.Price{}, fmt.Errorf("invalid price value: %w", err)
	}

	price, err := value_objects.NewPrice(currentPrice)
	if err != nil {

		return value_objects.Price{}, fmt.Errorf("invalid price value: %w", err)
	}

	return price, nil
}

func parseCandleData(candleData CandleData) (*value_objects.Candle, error) {

	// Convert strings to float64
	open, err := strconv.ParseFloat(candleData.Open, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid open price: %w", err)
	}

	high, err := strconv.ParseFloat(candleData.High, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid high price: %w", err)
	}

	low, err := strconv.ParseFloat(candleData.Low, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid low price: %w", err)
	}

	close, err := strconv.ParseFloat(candleData.Close, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid close price: %w", err)
	}

	// Parse timestamp (OKX uses milliseconds)
	tsMs, err := strconv.ParseInt(candleData.Ts, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}
	timestamp := time.Unix(0, tsMs*int64(time.Millisecond))

	// Convert to domain Candle value object
	candle, err := value_objects.NewCandle(open, high, low, close, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to create candle: %w", err)
	}

	return &candle, nil

}
