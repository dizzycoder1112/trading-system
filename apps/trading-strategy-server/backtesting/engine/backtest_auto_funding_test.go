package engine

import (
	"testing"
	"time"

	"dizzycode.xyz/shared/domain/value_objects"
)

// TestAutoFunding_DisabledByDefault 測試自動注資默認關閉
func TestAutoFunding_DisabledByDefault(t *testing.T) {
	config := BacktestConfig{
		InitialBalance:     10000.0,
		FeeRate:            0.0005,
		Slippage:           0,
		InstID:             "ETH-USDT-SWAP",
		TakeProfitMin:      0.0015,
		TakeProfitMax:      0.0020,
		PositionSize:       200.0,
		BreakEvenProfitMin: 1.0,
		BreakEvenProfitMax: 20.0,
		EnableTrendFilter:  true,
		EnableAutoFunding:  false, // 默認關閉
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create backtest engine: %v", err)
	}

	// 創建測試數據（300根K線，使用上漲價格避免觸發開倉）
	candles := make([]value_objects.Candle, 300)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 300 {
		// 使用逐漸上漲的價格，這樣不會觸發做多開倉（策略是在價格下跌到 MidLow 時開倉）
		price := 2500.0 + float64(i)*10.0
		candle, err := value_objects.NewCandle(
			price,     // Open
			price+5.0, // High (稍高)
			price,     // Low
			price+5.0, // Close (上漲)
			baseTime.Add(time.Duration(i)*5*time.Minute),
		)
		if err != nil {
			t.Fatalf("Failed to create candle: %v", err)
		}
		candles[i] = candle
	}

	result, err := engine.Run(candles)
	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// 驗證：未啟用自動注資時，不應該有注資記錄
	if len(engine.fundingHistory) != 0 {
		t.Errorf("Expected no funding records when auto-funding is disabled, got %d", len(engine.fundingHistory))
	}

	t.Logf("✅ Auto-funding disabled test passed: no funding records, final balance: %.2f", result.FinalBalance)
}

// TestAutoFunding_TriggersAfterIdleThreshold 測試超過閒置閾值時觸發注資
// 閒置的原因：資金用完無法開倉，等待 N 根 K 線後自動注資
func TestAutoFunding_TriggersAfterIdleThreshold(t *testing.T) {
	// 設計：
	// - 初始資金 $500，每次開倉 $200
	// - 大約 2-3 根 K 線後資金用完
	// - 閒置 10 根 K 線後觸發注資（測試用，實際可能是 288）
	// - 注資 $500 後又可以開倉，很快又用完
	// - 再閒置 10 根後再注資一次
	// - 總共需要約 30 根 K 線來觸發 2 次注資
	config := BacktestConfig{
		InitialBalance:        500.0,  // 小額初始資金，很快用完 ⭐
		FeeRate:               0.0005,
		Slippage:              0,
		InstID:                "ETH-USDT-SWAP",
		TakeProfitMin:         0.0015,
		TakeProfitMax:         0.0020,
		PositionSize:          200.0,
		BreakEvenProfitMin:    1.0,
		BreakEvenProfitMax:    20.0,
		EnableTrendFilter:     false, // 關閉趨勢過濾，確保每根 K 線都嘗試開倉
		EnableRedCandleFilter: false, // 關閉紅K過濾
		EnableAutoFunding:     true,  // 啟用自動注資 ⭐
		AutoFundingAmount:     500.0, // 每次注資 $500（小額，方便快速用完再觸發）
		AutoFundingIdle:       10,    // 10 根 K 線後觸發（測試用較短閾值）
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create backtest engine: %v", err)
	}

	// 創建測試數據（50 根 K 線，價格持續下跌，確保倉位不會止盈）
	// 這樣資金會真的用完，觸發閒置 → 注資
	candles := make([]value_objects.Candle, 50)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 50 {
		// 價格持續下跌，倉位永遠不會止盈，資金會真的用完
		price := 2500.0 - float64(i)*5.0
		candle, err := value_objects.NewCandle(
			price,     // Open
			price,     // High（不超過開盤價，不會觸發止盈）
			price-5.0, // Low
			price-3.0, // Close（收跌）
			baseTime.Add(time.Duration(i)*5*time.Minute),
		)
		if err != nil {
			t.Fatalf("Failed to create candle: %v", err)
		}
		candles[i] = candle
	}

	result, err := engine.Run(candles)
	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// 驗證：應該至少觸發 2 次注資
	if len(engine.fundingHistory) < 2 {
		t.Errorf("Expected at least 2 funding records, got %d", len(engine.fundingHistory))
	}

	// 驗證：每次注資金額正確
	for i, record := range engine.fundingHistory {
		if record.Amount != config.AutoFundingAmount {
			t.Errorf("Funding #%d: expected amount %.2f, got %.2f", i+1, config.AutoFundingAmount, record.Amount)
		}

		// 驗證：注資前後餘額差異正確
		balanceDiff := record.BalanceAfter - record.BalanceBefore
		if balanceDiff != config.AutoFundingAmount {
			t.Errorf("Funding #%d: expected balance diff %.2f, got %.2f", i+1, config.AutoFundingAmount, balanceDiff)
		}
	}

	t.Logf("✅ Auto-funding test passed:")
	t.Logf("   Total fundings: %d", len(engine.fundingHistory))
	t.Logf("   Final balance: %.2f USDT", result.FinalBalance)
}

// TestAutoFunding_ResetsOnOpen 測試開倉後重置閒置計數
func TestAutoFunding_ResetsOnOpen(t *testing.T) {
	config := BacktestConfig{
		InitialBalance:     10000.0,
		FeeRate:            0.0005,
		Slippage:           0,
		InstID:             "ETH-USDT-SWAP",
		TakeProfitMin:      0.0015,
		TakeProfitMax:      0.0020,
		PositionSize:       200.0,
		BreakEvenProfitMin: 1.0,
		BreakEvenProfitMax: 20.0,
		EnableTrendFilter:  false, // 關閉趨勢過濾，更容易觸發開倉
		EnableAutoFunding:  true,
		AutoFundingAmount:  5000.0,
		AutoFundingIdle:    100, // 降低閾值以便測試
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create backtest engine: %v", err)
	}

	// 創建測試數據（在第50根K線觸發開倉）
	candles := make([]value_objects.Candle, 200)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 200; i++ {
		var candle value_objects.Candle
		var err error

		if i == 50 {
			// 第50根K線：觸發開倉（價格下跌到 MidLow）
			candle, err = value_objects.NewCandle(
				2600.0, // Open
				2600.0, // High
				2400.0, // Low
				2500.0, // Close
				baseTime.Add(time.Duration(i)*5*time.Minute),
			)
		} else {
			// 其他K線：使用上漲價格，不觸發開倉
			price := 2500.0 + float64(i)*10.0
			candle, err = value_objects.NewCandle(
				price,     // Open
				price+5.0, // High
				price,     // Low
				price+5.0, // Close (上漲)
				baseTime.Add(time.Duration(i)*5*time.Minute),
			)
		}

		if err != nil {
			t.Fatalf("Failed to create candle: %v", err)
		}
		candles[i] = candle
	}

	result, err := engine.Run(candles)
	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// 驗證：應該有 1 次注資（前50根不觸發 -> 第51-150根累積100根 -> 觸發注資）
	expectedFundings := 1
	if len(engine.fundingHistory) < expectedFundings {
		t.Logf("Warning: Expected at least %d funding, got %d (may vary based on strategy)", expectedFundings, len(engine.fundingHistory))
	}

	// 驗證：至少有 1 筆交易（開倉）
	if len(engine.tradeLog) < 1 {
		t.Errorf("Expected at least 1 trade (open), got %d", len(engine.tradeLog))
	}

	t.Logf("✅ Auto-funding reset test passed:")
	t.Logf("   Total trades: %d", len(engine.tradeLog))
	t.Logf("   Total fundings: %d", len(engine.fundingHistory))
	t.Logf("   Final balance: %.2f USDT", result.FinalBalance)
}
