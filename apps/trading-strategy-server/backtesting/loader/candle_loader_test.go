package loader

import (
	"testing"
)

func TestCandleLoader_Load(t *testing.T) {
	// 測試加載真實數據
	loader := NewCandleLoader("../../data/20240930-20241001-5m-ETH-USDT-SWAP.json")

	candles, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load candles: %v", err)
	}

	// 驗證數據不為空
	if len(candles) == 0 {
		t.Fatal("No candles loaded")
	}

	t.Logf("✅ Loaded %d candles", len(candles))

	// 驗證順序（從舊到新）
	if len(candles) >= 2 {
		first := candles[0]
		second := candles[1]

		if first.Timestamp().After(second.Timestamp()) {
			t.Fatal("Candles are not sorted from old to new")
		}

		t.Logf("✅ First candle timestamp: %s", first.Timestamp())
		t.Logf("✅ Last candle timestamp: %s", candles[len(candles)-1].Timestamp())
	}

	// 驗證價格數據
	firstCandle := candles[0]
	t.Logf("✅ First candle: O=%.2f H=%.2f L=%.2f C=%.2f",
		firstCandle.Open().Value(),
		firstCandle.High().Value(),
		firstCandle.Low().Value(),
		firstCandle.Close().Value(),
	)

	// 驗證業務規則：High >= Low
	for i, candle := range candles {
		if candle.High().Value() < candle.Low().Value() {
			t.Fatalf("Invalid candle at index %d: High < Low", i)
		}
	}

	t.Logf("✅ All candles valid (High >= Low)")
}
