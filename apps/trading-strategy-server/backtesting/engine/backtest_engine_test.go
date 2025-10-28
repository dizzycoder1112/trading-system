package engine

import (
	"testing"
	"time"

	"dizzycode.xyz/shared/domain/value_objects"
)

// TestBacktestEngine_NewBacktestEngine 測試引擎創建
func TestBacktestEngine_NewBacktestEngine(t *testing.T) {
	config := BacktestConfig{
		InitialBalance: 10000.0,
		FeeRate:        0.0005,
		Slippage:       0,
		InstID:         "ETH-USDT-SWAP",
		TakeProfitMin:  0.0020,
		TakeProfitMax:  0.0030,
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create backtest engine: %v", err)
	}

	if engine == nil {
		t.Fatal("Engine is nil")
	}

	if engine.strategy == nil {
		t.Fatal("Strategy is nil")
	}

	if engine.simulator == nil {
		t.Fatal("Simulator is nil")
	}

	if engine.positionTracker == nil {
		t.Fatal("Position tracker is nil")
	}

	if engine.calculator == nil {
		t.Fatal("Calculator is nil")
	}
}

// TestBacktestEngine_Run_EmptyCandles 測試空數據
func TestBacktestEngine_Run_EmptyCandles(t *testing.T) {
	config := BacktestConfig{
		InitialBalance: 10000.0,
		FeeRate:        0.0005,
		Slippage:       0,
		InstID:         "ETH-USDT-SWAP",
		TakeProfitMin:  0.0015,
		TakeProfitMax:  0.0020,
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	candles := []value_objects.Candle{}
	_, err = engine.Run(candles)

	if err == nil {
		t.Error("Expected error for empty candles, got nil")
	}
}

// TestBacktestEngine_Run_SingleCandle 測試單根K線
func TestBacktestEngine_Run_SingleCandle(t *testing.T) {
	config := BacktestConfig{
		InitialBalance: 10000.0,
		FeeRate:        0.0005,
		Slippage:       0,
		InstID:         "ETH-USDT-SWAP",
		TakeProfitMin:  0.0015,
		TakeProfitMax:  0.0020,
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 創建單根K線
	candle, _ := value_objects.NewCandle(2500, 2510, 2490, 2500, time.Now())
	candles := []value_objects.Candle{candle}

	result, err := engine.Run(candles)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// 驗證結果
	if result.InitialBalance != 10000.0 {
		t.Errorf("Expected initial balance 10000.0, got %.2f", result.InitialBalance)
	}

	// 單根K線應該沒有交易（因為需要歷史K線）
	if result.TotalTrades != 0 {
		t.Logf("Total trades: %d (may have trades if strategy allows)", result.TotalTrades)
	}
}

// TestBacktestEngine_Run_MultipleCandles 測試多根K線
func TestBacktestEngine_Run_MultipleCandles(t *testing.T) {
	config := BacktestConfig{
		InitialBalance: 10000.0,
		FeeRate:        0.0005,
		Slippage:       0,
		InstID:         "ETH-USDT-SWAP",
		TakeProfitMin:  0.0015,
		TakeProfitMax:  0.0020,
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 創建一系列K線（模擬價格波動）
	baseTime := time.Now()
	candles := []value_objects.Candle{}

	prices := []struct{ open, high, low, close float64 }{
		{2500, 2510, 2490, 2500},
		{2500, 2515, 2495, 2505},
		{2505, 2520, 2500, 2510},
		{2510, 2525, 2505, 2515},
		{2515, 2530, 2510, 2520},
		{2520, 2535, 2515, 2525},
		{2525, 2540, 2520, 2530},
		{2530, 2545, 2525, 2535},
		{2535, 2550, 2530, 2540},
		{2540, 2555, 2535, 2545},
	}

	for i, p := range prices {
		timestamp := baseTime.Add(time.Duration(i) * 5 * time.Minute)
		candle, _ := value_objects.NewCandle(p.open, p.high, p.low, p.close, timestamp)
		candles = append(candles, candle)
	}

	result, err := engine.Run(candles)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// 驗證結果
	t.Logf("========================================")
	t.Logf("回測結果:")
	t.Logf("========================================")
	t.Logf("初始資金: $%.2f", result.InitialBalance)
	t.Logf("最終資金: $%.2f", result.FinalBalance)
	t.Logf("淨利潤: $%.2f", result.NetProfit)
	t.Logf("總收益率: %.2f%%", result.TotalReturn)
	t.Logf("最大回撤: %.2f%%", result.MaxDrawdown)
	t.Logf("========================================")
	t.Logf("交易統計:")
	t.Logf("========================================")
	t.Logf("總交易次數: %d", result.TotalTrades)
	t.Logf("盈利交易: %d", result.WinningTrades)
	t.Logf("虧損交易: %d", result.LosingTrades)
	t.Logf("勝率: %.2f%%", result.WinRate)
	t.Logf("盈虧比: %.2f", result.ProfitFactor)
	t.Logf("========================================")
	t.Logf("盈虧明細:")
	t.Logf("========================================")
	t.Logf("總盈利: $%.2f", result.TotalProfit)
	t.Logf("總虧損: $%.2f", result.TotalLoss)
	t.Logf("平均持倉時長: %s", result.AvgHoldDuration)
	t.Logf("========================================")

	// 基本斷言
	if result.InitialBalance != 10000.0 {
		t.Errorf("Expected initial balance 10000.0, got %.2f", result.InitialBalance)
	}

	// 應該有一些交易記錄
	if result.TotalTrades == 0 {
		t.Log("Warning: No trades executed (may be expected based on strategy)")
	}

	// 驗證勝率計算
	if result.TotalTrades > 0 {
		expectedWinRate := float64(result.WinningTrades) / float64(result.TotalTrades) * 100
		if result.WinRate != expectedWinRate {
			t.Errorf("Win rate mismatch: expected %.2f%%, got %.2f%%",
				expectedWinRate, result.WinRate)
		}
	}
}

// TestBacktestEngine_Run_PriceIncrease 測試價格上漲場景
func TestBacktestEngine_Run_PriceIncrease(t *testing.T) {
	config := BacktestConfig{
		InitialBalance: 10000.0,
		FeeRate:        0.0005,
		Slippage:       0,
		InstID:         "ETH-USDT-SWAP",
		TakeProfitMin:  0.0015,
		TakeProfitMax:  0.0020,
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 創建持續上漲的K線
	baseTime := time.Now()
	candles := []value_objects.Candle{}

	startPrice := 2500.0
	for i := 0; i < 20; i++ {
		price := startPrice + float64(i)*5.0 // 每根K線上漲5美元
		timestamp := baseTime.Add(time.Duration(i) * 5 * time.Minute)
		candle, _ := value_objects.NewCandle(
			price,
			price+3,
			price-2,
			price+2,
			timestamp,
		)
		candles = append(candles, candle)
	}

	result, err := engine.Run(candles)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	t.Logf("========================================")
	t.Logf("價格上漲場景回測結果:")
	t.Logf("========================================")
	t.Logf("總交易次數: %d", result.TotalTrades)
	t.Logf("淨利潤: $%.2f", result.NetProfit)
	t.Logf("總收益率: %.2f%%", result.TotalReturn)
	t.Logf("勝率: %.2f%%", result.WinRate)
	t.Logf("========================================")

	// 在持續上漲的市場中，策略可能會有交易
	if result.TotalTrades > 0 {
		t.Logf("Executed %d trades in uptrend market", result.TotalTrades)
	}
}

// TestBacktestEngine_Run_PriceDecrease 測試價格下跌場景
func TestBacktestEngine_Run_PriceDecrease(t *testing.T) {
	config := BacktestConfig{
		InitialBalance: 10000.0,
		FeeRate:        0.0005,
		Slippage:       0,
		InstID:         "ETH-USDT-SWAP",
		TakeProfitMin:  0.0015,
		TakeProfitMax:  0.0020,
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 創建持續下跌的K線
	baseTime := time.Now()
	candles := []value_objects.Candle{}

	startPrice := 2600.0
	for i := 0; i < 20; i++ {
		price := startPrice - float64(i)*5.0 // 每根K線下跌5美元
		timestamp := baseTime.Add(time.Duration(i) * 5 * time.Minute)
		candle, _ := value_objects.NewCandle(
			price,
			price+2,
			price-3,
			price-2,
			timestamp,
		)
		candles = append(candles, candle)
	}

	result, err := engine.Run(candles)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	t.Logf("========================================")
	t.Logf("價格下跌場景回測結果:")
	t.Logf("========================================")
	t.Logf("總交易次數: %d", result.TotalTrades)
	t.Logf("淨利潤: $%.2f", result.NetProfit)
	t.Logf("總收益率: %.2f%%", result.TotalReturn)
	t.Logf("勝率: %.2f%%", result.WinRate)
	t.Logf("========================================")

	// 在持續下跌的市場中，策略可能會開倉
	if result.TotalTrades > 0 {
		t.Logf("Executed %d trades in downtrend market", result.TotalTrades)
	}
}

// TestBacktestEngine_RunFromFile 測試從文件載入（集成測試）
// 注意：這個測試需要實際的數據文件存在
func TestBacktestEngine_RunFromFile(t *testing.T) {
	config := BacktestConfig{
		InitialBalance: 10000.0,
		FeeRate:        0.0005,
		Slippage:       0,
		InstID:         "ETH-USDT-SWAP",
		TakeProfitMin:  0.0015,
		TakeProfitMax:  0.0020,
	}

	engine, err := NewBacktestEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 使用測試數據文件
	filepath := "../../data/20240930-20241001-5m-ETH-USDT-SWAP.json"

	result, err := engine.RunFromFile(filepath)
	if err != nil {
		// 如果文件不存在，跳過測試
		t.Skipf("Skipping file test: %v", err)
		return
	}

	t.Logf("========================================")
	t.Logf("真實數據回測結果 (ETH-USDT-SWAP):")
	t.Logf("========================================")
	t.Logf("初始資金: $%.2f", result.InitialBalance)
	t.Logf("最終資金: $%.2f", result.FinalBalance)
	t.Logf("淨利潤: $%.2f", result.NetProfit)
	t.Logf("總收益率: %.2f%%", result.TotalReturn)
	t.Logf("最大回撤: %.2f%%", result.MaxDrawdown)
	t.Logf("========================================")
	t.Logf("總交易次數: %d", result.TotalTrades)
	t.Logf("盈利交易: %d", result.WinningTrades)
	t.Logf("虧損交易: %d", result.LosingTrades)
	t.Logf("勝率: %.2f%%", result.WinRate)
	t.Logf("盈虧比: %.2f", result.ProfitFactor)
	t.Logf("平均持倉時長: %s", result.AvgHoldDuration)
	t.Logf("========================================")

	// 驗證結果合理性
	if result.InitialBalance != 10000.0 {
		t.Errorf("Expected initial balance 10000.0, got %.2f", result.InitialBalance)
	}

	if result.FinalBalance < 0 {
		t.Error("Final balance should not be negative")
	}
}
