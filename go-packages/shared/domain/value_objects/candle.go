package value_objects

import (
	"fmt"
	"time"
)

// Candle K線值對象
// 包含開高低收及時間信息
type Candle struct {
	open      Price
	high      Price
	low       Price
	close     Price
	timestamp time.Time
}

// NewCandle 創建K線（工廠方法）
func NewCandle(open, high, low, close float64, timestamp time.Time) (Candle, error) {
	openPrice, err := NewPrice(open)
	if err != nil {
		return Candle{}, fmt.Errorf("invalid open price: %w", err)
	}

	highPrice, err := NewPrice(high)
	if err != nil {
		return Candle{}, fmt.Errorf("invalid high price: %w", err)
	}

	lowPrice, err := NewPrice(low)
	if err != nil {
		return Candle{}, fmt.Errorf("invalid low price: %w", err)
	}

	closePrice, err := NewPrice(close)
	if err != nil {
		return Candle{}, fmt.Errorf("invalid close price: %w", err)
	}

	// 驗證業務規則：high >= low
	if !highPrice.IsAboveOrEqual(lowPrice) {
		return Candle{}, fmt.Errorf("high price must be >= low price")
	}

	return Candle{
		open:      openPrice,
		high:      highPrice,
		low:       lowPrice,
		close:     closePrice,
		timestamp: timestamp,
	}, nil
}

// Getters
func (c Candle) Open() Price          { return c.open }
func (c Candle) High() Price          { return c.high }
func (c Candle) Low() Price           { return c.low }
func (c Candle) Close() Price         { return c.close }
func (c Candle) Timestamp() time.Time { return c.timestamp }

// BodyLow 返回實體低點（開盤價和收盤價中較小的）
func (c Candle) BodyLow() Price {
	if c.open.Value() < c.close.Value() {
		return c.open
	}
	return c.close
}

// BodyHigh 返回實體高點（開盤價和收盤價中較大的）
func (c Candle) BodyHigh() Price {
	if c.open.Value() > c.close.Value() {
		return c.open
	}
	return c.close
}

// MidLow 返回中間低點：(實體低點 + 影線低點) / 2
// 用於計算開倉位置
func (c Candle) MidLow() Price {
	bodyLow := c.BodyLow().Value()
	wickLow := c.Low().Value()
	midValue := (bodyLow + wickLow) / 2.0

	// 這裡不會出錯，因為 midValue 必然是正數
	price, _ := NewPrice(midValue)
	return price
}

// IsBullish 判斷是否為陽線
func (c Candle) IsBullish() bool {
	return c.close.Value() > c.open.Value()
}

// IsBearish 判斷是否為陰線
func (c Candle) IsBearish() bool {
	return c.close.Value() < c.open.Value()
}
