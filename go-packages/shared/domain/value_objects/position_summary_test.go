package value_objects

// import (
// 	"testing"
// )

// // TestShouldBreakEven_NoPositions 測試沒有持倉時不觸發
// func TestShouldBreakEven_NoPositions(t *testing.T) {
// 	ps := PositionSummary{
// 		Count:                   0,
// 		TotalSize:               0,
// 		AvgPrice:                0,
// 		FeesPaid:                0,
// 		CurrentRoundRealizedPnL: 0,
// 		CurrentRoundClosedValue: 0,
// 		UnrealizedPnL:           0,
// 	}

// 	shouldExit, expectedProfit := ps.ShouldBreakEven(0, 20)

// 	if shouldExit {
// 		t.Errorf("Expected shouldExit=false when no positions, got true")
// 	}
// 	if expectedProfit != 0 {
// 		t.Errorf("Expected expectedProfit=0, got %.2f", expectedProfit)
// 	}
// }

// // TestShouldBreakEven_NoClosedPositions 測試本輪沒有關倉時不觸發
// func TestShouldBreakEven_NoClosedPositions(t *testing.T) {
// 	// 計算 unrealizedPnL (currentPrice=2510, avgPrice=2500, totalSize=500)
// 	// closedCoins = 500 / 2500 = 0.2
// 	// priceChange = 2510 - 2500 = 10
// 	// profitBeforeFee = 0.2 * 10 = 2.0
// 	// closeValue = 500 + 2.0 = 502.0
// 	// closeFee = 502.0 * 0.0005 = 0.251
// 	// unrealizedPnL = 2.0 - 0.251 = 1.749
// 	ps := PositionSummary{
// 		Count:                   5,
// 		TotalSize:               500,
// 		AvgPrice:                2500,
// 		FeesPaid:                0.5,
// 		CurrentRoundRealizedPnL: 0,
// 		CurrentRoundClosedValue: 0,     // ⭐ 本輪還沒關倉
// 		UnrealizedPnL:           1.749, // ⭐ 已計算好的未實現盈虧
// 	}

// 	shouldExit, _ := ps.ShouldBreakEven(0, 20)

// 	if shouldExit {
// 		t.Errorf("Expected shouldExit=false when no closed positions in current round, got true")
// 	}
// }

// // TestShouldBreakEven_PositiveRealizedPnL 測試本輪盈利時不觸發
// func TestShouldBreakEven_PositiveRealizedPnL(t *testing.T) {
// 	// unrealizedPnL = 1.749 (same calculation as previous test)
// 	ps := PositionSummary{
// 		Count:                   5,
// 		TotalSize:               500,
// 		AvgPrice:                2500,
// 		FeesPaid:                1.0,
// 		CurrentRoundRealizedPnL: 5.0,   // ⭐ 本輪盈利
// 		CurrentRoundClosedValue: 200,
// 		UnrealizedPnL:           1.749,
// 	}

// 	shouldExit, _ := ps.ShouldBreakEven(0, 20)

// 	if shouldExit {
// 		t.Errorf("Expected shouldExit=false when CurrentRoundRealizedPnL >= 0, got true")
// 	}
// }

// // TestShouldBreakEven_NegativeRealizedPnL_ButStillLosing 測試虧損但 expectedProfit 還是負數，不觸發
// func TestShouldBreakEven_NegativeRealizedPnL_ButStillLosing(t *testing.T) {
// 	// 計算 unrealizedPnL (currentPrice=2505, avgPrice=2500, totalSize=500)
// 	// closedCoins = 500 / 2500 = 0.2
// 	// priceChange = 2505 - 2500 = 5
// 	// profitBeforeFee = 0.2 * 5 = 1.0
// 	// closeValue = 500 + 1.0 = 501.0
// 	// closeFee = 501.0 * 0.0005 = 0.2505
// 	// unrealizedPnL = 1.0 - 0.2505 = 0.7495
// 	ps := PositionSummary{
// 		Count:                   5,
// 		TotalSize:               500,
// 		AvgPrice:                2500,
// 		FeesPaid:                1.0,
// 		CurrentRoundRealizedPnL: -10.0,  // ⭐ 虧損 -10 USDT
// 		CurrentRoundClosedValue: 200,
// 		UnrealizedPnL:           0.7495, // ⭐ 已包含 closeFee
// 	}

// 	// expectedProfit = -10 + 0.7495 = -9.25 USDT ❌ (還是負數)
// 	shouldExit, expectedProfit := ps.ShouldBreakEven(0, 20)

// 	if shouldExit {
// 		t.Errorf("Expected shouldExit=false when expectedProfit < 0, got true")
// 	}

// 	t.Logf("CurrentRoundRealizedPnL=%.2f, ExpectedProfit=%.2f", ps.CurrentRoundRealizedPnL, expectedProfit)
// }

// // TestShouldBreakEven_NegativeRealizedPnL_BreakEvenRange 測試虧損但 expectedProfit 在 [0, 20]，觸發
// func TestShouldBreakEven_NegativeRealizedPnL_BreakEvenRange(t *testing.T) {
// 	// 計算 unrealizedPnL (currentPrice=2530, avgPrice=2500, totalSize=500)
// 	// closedCoins = 500 / 2500 = 0.2
// 	// priceChange = 2530 - 2500 = 30
// 	// profitBeforeFee = 0.2 * 30 = 6.0
// 	// closeValue = 500 + 6.0 = 506.0
// 	// closeFee = 506.0 * 0.0005 = 0.253
// 	// unrealizedPnL = 6.0 - 0.253 = 5.747
// 	ps := PositionSummary{
// 		Count:                   5,
// 		TotalSize:               500,
// 		AvgPrice:                2500,
// 		FeesPaid:                1.0,
// 		CurrentRoundRealizedPnL: -5.0,   // ⭐ 虧損 -5 USDT
// 		CurrentRoundClosedValue: 200,
// 		UnrealizedPnL:           5.747, // ⭐ 已包含 closeFee
// 	}

// 	// expectedProfit = -5 + 5.747 = +0.747 USDT ✅ (觸發打平機制)
// 	shouldExit, expectedProfit := ps.ShouldBreakEven(0, 20)

// 	if !shouldExit {
// 		t.Errorf("Expected shouldExit=true when expectedProfit in [0, 20], got false")
// 	}

// 	if expectedProfit < 0 || expectedProfit > 20 {
// 		t.Errorf("Expected expectedProfit in [0, 20], got %.2f", expectedProfit)
// 	}

// 	t.Logf("✅ Triggered break-even: CurrentRoundRealizedPnL=%.2f, ExpectedProfit=%.2f",
// 		ps.CurrentRoundRealizedPnL, expectedProfit)
// }

// // TestShouldBreakEven_ExpectedProfitHigh 測試 expectedProfit > 20 也會觸發（移除上限）
// func TestShouldBreakEven_ExpectedProfitHigh(t *testing.T) {
// 	// 計算 unrealizedPnL (currentPrice=2650, avgPrice=2500, totalSize=500)
// 	// closedCoins = 500 / 2500 = 0.2
// 	// priceChange = 2650 - 2500 = 150
// 	// profitBeforeFee = 0.2 * 150 = 30.0
// 	// closeValue = 500 + 30.0 = 530.0
// 	// closeFee = 530.0 * 0.0005 = 0.265
// 	// unrealizedPnL = 30.0 - 0.265 = 29.735
// 	ps := PositionSummary{
// 		Count:                   5,
// 		TotalSize:               500,
// 		AvgPrice:                2500,
// 		FeesPaid:                1.0,
// 		CurrentRoundRealizedPnL: -5.0,
// 		CurrentRoundClosedValue: 200,
// 		UnrealizedPnL:           29.735, // ⭐ 已包含 closeFee
// 	}

// 	// expectedProfit = -5 + 29.735 = 24.735 USDT ✅ (> 0 觸發打平機制)
// 	shouldExit, expectedProfit := ps.ShouldBreakEven(0, 20)

// 	if !shouldExit {
// 		t.Errorf("Expected shouldExit=true when expectedProfit > 0 (no upper limit), got false")
// 	}

// 	if expectedProfit <= 0 {
// 		t.Errorf("Expected expectedProfit > 0, got %.2f", expectedProfit)
// 	}

// 	t.Logf("✅ Triggered break-even: ExpectedProfit=%.2f (> 20, no upper limit)", expectedProfit)
// }

// // TestShouldBreakEven_EdgeCase_SlightlyPositive 測試邊界情況：expectedProfit 稍微 > 0
// func TestShouldBreakEven_EdgeCase_SlightlyPositive(t *testing.T) {
// 	// 計算 unrealizedPnL (currentPrice=2531.75, avgPrice=2500, totalSize=500)
// 	// closedCoins = 500 / 2500 = 0.2
// 	// priceChange = 2531.75 - 2500 = 31.75
// 	// profitBeforeFee = 0.2 * 31.75 = 6.35
// 	// closeValue = 500 + 6.35 = 506.35
// 	// closeFee = 506.35 * 0.0005 = 0.253175
// 	// unrealizedPnL = 6.35 - 0.253175 = 6.096825
// 	ps := PositionSummary{
// 		Count:                   5,
// 		TotalSize:               500,
// 		AvgPrice:                2500,
// 		FeesPaid:                1.0,
// 		CurrentRoundRealizedPnL: -6.0,
// 		CurrentRoundClosedValue: 200,
// 		UnrealizedPnL:           6.096825, // ⭐ 已包含 closeFee
// 	}

// 	// expectedProfit = -6 + 6.096825 = 0.096825 USDT ✅ (稍微 > 0，觸發打平)
// 	shouldExit, expectedProfit := ps.ShouldBreakEven(0, 20)

// 	if !shouldExit {
// 		t.Errorf("Expected shouldExit=true when expectedProfit slightly > 0, got false")
// 	}

// 	if expectedProfit <= 0 {
// 		t.Errorf("Expected expectedProfit > 0, got %.2f", expectedProfit)
// 	}

// 	t.Logf("✅ Edge case: ExpectedProfit=%.2f (slightly > 0, should trigger)", expectedProfit)
// }

// // TestShouldBreakEven_EdgeCase_ExactlyTwenty 測試邊界情況：expectedProfit = 20
// func TestShouldBreakEven_EdgeCase_ExactlyTwenty(t *testing.T) {
// 	// 計算 unrealizedPnL (currentPrice=2631.25, avgPrice=2500, totalSize=500)
// 	// closedCoins = 500 / 2500 = 0.2
// 	// priceChange = 2631.25 - 2500 = 131.25
// 	// profitBeforeFee = 0.2 * 131.25 = 26.25
// 	// closeValue = 500 + 26.25 = 526.25
// 	// closeFee = 526.25 * 0.0005 = 0.263125
// 	// unrealizedPnL = 26.25 - 0.263125 = 25.986875
// 	ps := PositionSummary{
// 		Count:                   5,
// 		TotalSize:               500,
// 		AvgPrice:                2500,
// 		FeesPaid:                1.0,
// 		CurrentRoundRealizedPnL: -6.0,
// 		CurrentRoundClosedValue: 200,
// 		UnrealizedPnL:           25.986875, // ⭐ 已包含 closeFee
// 	}

// 	// expectedProfit = -6 + 25.986875 = 19.986875 USDT ✅ (≈ 20)
// 	shouldExit, expectedProfit := ps.ShouldBreakEven(0, 20)

// 	if !shouldExit {
// 		t.Errorf("Expected shouldExit=true when expectedProfit ≈ 20, got false")
// 	}

// 	if expectedProfit < 19.5 || expectedProfit > 20.5 {
// 		t.Errorf("Expected expectedProfit ≈ 20, got %.2f", expectedProfit)
// 	}

// 	t.Logf("✅ Edge case: ExpectedProfit=%.2f (≈ 20)", expectedProfit)
// }
