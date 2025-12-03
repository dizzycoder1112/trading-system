package simulator

import (
	"testing"
	"time"
)

func TestPositionTracker_AddPosition(t *testing.T) {
	tracker := NewPositionTracker()

	// 添加第一個持倉
	pos1 := tracker.AddPosition(2500, 200, time.Now(), 2510)

	if pos1.ID != "pos_1" {
		t.Errorf("Expected ID pos_1, got %s", pos1.ID)
	}

	if tracker.GetOpenPositionCount() != 1 {
		t.Errorf("Expected 1 open position, got %d", tracker.GetOpenPositionCount())
	}

	// 添加第二個持倉
	pos2 := tracker.AddPosition(2505, 200, time.Now(), 2515)

	if pos2.ID != "pos_2" {
		t.Errorf("Expected ID pos_2, got %s", pos2.ID)
	}

	if tracker.GetOpenPositionCount() != 2 {
		t.Errorf("Expected 2 open positions, got %d", tracker.GetOpenPositionCount())
	}

	t.Logf("✅ Added 2 positions successfully")
}

func TestPositionTracker_CalculateAverageCost(t *testing.T) {
	tracker := NewPositionTracker()

	// 添加兩個持倉
	// 倉位 #1: $200 @ $2500 → 持有 200/2500 = 0.08 幣
	// 倉位 #2: $200 @ $2600 → 持有 200/2600 = 0.0769 幣
	tracker.AddPosition(2500, 200, time.Now(), 2510)
	tracker.AddPosition(2600, 200, time.Now(), 2615)

	avgCost := tracker.CalculateAverageCost()

	// 正確計算方式：
	// 總花費 = 200 + 200 = 400 USDT
	// 總持倉量 = (200/2500) + (200/2600) = 0.08 + 0.0769 = 0.1569 幣
	// 平均成本 = 400 / 0.1569 ≈ 2549.02
	expected := 2549.02

	// 允許浮點誤差
	if diff := avgCost - expected; diff > 0.01 || diff < -0.01 {
		t.Errorf("Expected average cost %.2f, got %.2f", expected, avgCost)
	}

	t.Logf("✅ Average cost: %.2f (expected: %.2f)", avgCost, expected)
}

func TestPositionTracker_CalculateUnrealizedPnL(t *testing.T) {
	tracker := NewPositionTracker()
	feeRate := 0.0005 // 0.05%

	// 開倉：2500，倉位大小 200
	tracker.AddPosition(2500, 200, time.Now(), 2510)

	// 當前價格：2510（上漲 10，漲幅 0.4%）
	currentPrice := 2510.0

	// 計算未實現盈虧
	unrealizedPnL := tracker.CalculateUnrealizedPnL(currentPrice, feeRate)

	// 預期：
	// 幣數: 200 / 2500 = 0.08 BTC
	// 價格變化: 2510 - 2500 = 10
	// 浮動盈虧: 0.08 * 10 = 0.8 USDT
	// 平倉價值: 200 + 0.8 = 200.8 USDT
	// 平倉手續費: 200.8 * 0.0005 = 0.1004 USDT ⭐ 只計算平倉費
	// 未實現盈虧: 0.8 - 0.1004 = 0.6996 ≈ 0.70 USDT
	// 註：開倉手續費已經在開倉時從餘額中扣除，不應該在這裡再扣 ⭐
	expected := 0.70

	// 允許浮點誤差
	if diff := unrealizedPnL - expected; diff > 0.01 || diff < -0.01 {
		t.Errorf("Expected unrealized PnL %.2f, got %.2f", expected, unrealizedPnL)
	}

	t.Logf("✅ Unrealized PnL: %.2f USDT (expected: %.2f)", unrealizedPnL, expected)
}

func TestPositionTracker_ClosePosition(t *testing.T) {
	tracker := NewPositionTracker()

	// 開倉
	pos := tracker.AddPosition(2500, 200, time.Now(), 2510)

	if tracker.GetOpenPositionCount() != 1 {
		t.Fatal("Expected 1 open position")
	}

	// 平倉
	closeTime := time.Now().Add(5 * time.Minute)
	realizedPnL := 0.56 // 從上一個測試計算出來的

	err := tracker.ClosePosition(pos.ID, 2510, closeTime, realizedPnL)
	if err != nil {
		t.Fatalf("Failed to close position: %v", err)
	}

	if tracker.GetOpenPositionCount() != 0 {
		t.Errorf("Expected 0 open positions, got %d", tracker.GetOpenPositionCount())
	}

	if len(tracker.GetClosedPositions()) != 1 {
		t.Errorf("Expected 1 closed position, got %d", len(tracker.GetClosedPositions()))
	}

	// 驗證已平倉記錄
	closed := tracker.GetClosedPositions()[0]
	if closed.RealizedPnL != realizedPnL {
		t.Errorf("Expected realized PnL %.2f, got %.2f", realizedPnL, closed.RealizedPnL)
	}

	t.Logf("✅ Position closed successfully")
	t.Logf("   Hold duration: %v", closed.HoldDuration)
	t.Logf("   Realized PnL: %.2f USDT", closed.RealizedPnL)
}

func TestPositionTracker_GetWinRate(t *testing.T) {
	tracker := NewPositionTracker()

	// 添加 5 個持倉並平倉
	// 3 個盈利，2 個虧損
	now := time.Now()

	pos1 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos1.ID, 2510, now.Add(1*time.Minute), 0.56) // 盈利

	pos2 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos2.ID, 2490, now.Add(2*time.Minute), -0.80) // 虧損

	pos3 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos3.ID, 2515, now.Add(3*time.Minute), 1.20) // 盈利

	pos4 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos4.ID, 2505, now.Add(4*time.Minute), 0.40) // 盈利

	pos5 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos5.ID, 2485, now.Add(5*time.Minute), -1.20) // 虧損

	winRate := tracker.GetWinRate()
	expected := 3.0 / 5.0 // 60%

	if winRate != expected {
		t.Errorf("Expected win rate %.2f%%, got %.2f%%", expected*100, winRate*100)
	}

	t.Logf("✅ Win rate: %.2f%% (3 wins out of 5 trades)", winRate*100)
}

func TestPositionTracker_GetTotalRealizedPnL(t *testing.T) {
	tracker := NewPositionTracker()
	now := time.Now()

	// 添加多個交易
	pos1 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos1.ID, 2510, now.Add(1*time.Minute), 0.56)

	pos2 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos2.ID, 2490, now.Add(2*time.Minute), -0.80)

	pos3 := tracker.AddPosition(2500, 200, now, 2510)
	tracker.ClosePosition(pos3.ID, 2515, now.Add(3*time.Minute), 1.20)

	totalPnL := tracker.CalculateTotalRealizedPnL()
	expected := 0.56 - 0.80 + 1.20 // = 0.96

	// 允許浮點誤差
	if diff := totalPnL - expected; diff > 0.01 || diff < -0.01 {
		t.Errorf("Expected total PnL %.2f, got %.2f", expected, totalPnL)
	}

	t.Logf("✅ Total realized PnL: %.2f USDT", totalPnL)
}

// ========== 多倉位場景測試（從 position_fix_test.go 合併）==========

// TestPositionTracker_ClosePosition_MultiPosition 測試多倉位場景
// 驗證平倉時使用開倉價而不是平均成本計算幣數
func TestPositionTracker_ClosePosition_MultiPosition(t *testing.T) {
	tracker := NewPositionTracker()

	// 開倉1: 100 USDT @ 2500 → 應該買入 0.04 BTC
	pos1 := tracker.AddPosition(2500, 100, time.Now(), 2600)
	expectedCoins1 := 100.0 / 2500.0 // 0.04 BTC

	if tracker.totalCoins != expectedCoins1 {
		t.Errorf("開倉1後，totalCoins 應該是 %.6f，實際是 %.6f", expectedCoins1, tracker.totalCoins)
	}

	if tracker.avgCost != 2500 {
		t.Errorf("開倉1後，avgCost 應該是 2500，實際是 %.2f", tracker.avgCost)
	}

	// 開倉2: 100 USDT @ 2600 → 應該買入 0.0385 BTC
	tracker.AddPosition(2600, 100, time.Now(), 2700)
	expectedCoins2 := 100.0 / 2600.0 // 0.0385 BTC
	expectedTotalCoins := expectedCoins1 + expectedCoins2

	if tracker.totalCoins < expectedTotalCoins-0.00001 || tracker.totalCoins > expectedTotalCoins+0.00001 {
		t.Errorf("開倉2後，totalCoins 應該是 %.6f，實際是 %.6f", expectedTotalCoins, tracker.totalCoins)
	}

	// 平均成本應該是加權平均
	// avgCost = (2500*0.04 + 2600*0.038462) / (0.04 + 0.038462)
	//         = (100 + 100) / 0.078462
	//         = 200 / 0.078462 ≈ 2549.02
	expectedAvgCost := 200.0 / expectedTotalCoins

	if tracker.avgCost < expectedAvgCost-1 || tracker.avgCost > expectedAvgCost+1 {
		t.Errorf("開倉2後，avgCost 應該約為 %.2f，實際是 %.2f", expectedAvgCost, tracker.avgCost)
	}

	// ⭐ 關鍵測試：平倉1，在 2600 平掉第一筆開倉
	// 應該平掉 0.04 BTC（第一筆實際買入的幣數）
	// 而不是 100 / 2549 ≈ 0.0392 BTC（用平均成本計算的幣數）
	err := tracker.ClosePosition(pos1.ID, 2600, time.Now(), 4.0)
	if err != nil {
		t.Fatalf("平倉失敗: %v", err)
	}

	// 平倉後，剩餘幣數應該是 0.0385 BTC（只剩第二筆）
	expectedRemainingCoins := expectedCoins2

	if tracker.totalCoins < expectedRemainingCoins-0.00001 || tracker.totalCoins > expectedRemainingCoins+0.00001 {
		t.Errorf("平倉1後，剩餘 totalCoins 應該是 %.6f，實際是 %.6f", expectedRemainingCoins, tracker.totalCoins)
	}

	// ⭐ 平倉後，平均成本應該保持不變（關鍵特性）
	// 平倉只減少幣數，不改變平均成本
	if tracker.avgCost < expectedAvgCost-1 || tracker.avgCost > expectedAvgCost+1 {
		t.Errorf("平倉1後，avgCost 應該保持約 %.2f，實際是 %.2f", expectedAvgCost, tracker.avgCost)
	}

	// 驗證盈虧計算
	// 第一筆：100 USDT @ 2500 → 2600 平倉
	// 收入 = 0.04 * 2600 = 104 USDT
	// 成本 = 100 USDT
	// 盈虧 = 4 USDT ✅
	expectedPnL := 4.0
	actualPnL := tracker.CalculateTotalRealizedPnL()

	if actualPnL != expectedPnL {
		t.Errorf("已實現盈虧應該是 %.2f USDT，實際是 %.2f USDT", expectedPnL, actualPnL)
	}

	t.Logf("✅ 多倉位場景驗證通過:")
	t.Logf("   開倉1: 100 USDT @ 2500 → 0.04 BTC")
	t.Logf("   開倉2: 100 USDT @ 2600 → %.6f BTC", expectedCoins2)
	t.Logf("   總持倉: %.6f BTC，平均成本: %.2f", expectedTotalCoins, expectedAvgCost)
	t.Logf("   平倉1: 0.04 BTC @ 2600，盈虧: %.2f USDT", expectedPnL)
	t.Logf("   剩餘持倉: %.6f BTC，平均成本: %.2f", tracker.totalCoins, tracker.avgCost)
}

// TestPositionTracker_ClosePosition_CoinsCalculation 對比正確/錯誤的幣數計算
func TestPositionTracker_ClosePosition_CoinsCalculation(t *testing.T) {
	tracker := NewPositionTracker()
	pos1 := tracker.AddPosition(2500, 100, time.Now(), 2600)
	tracker.AddPosition(2600, 100, time.Now(), 2700)

	// 第一筆開倉買入的實際幣數
	correctCoins := 100.0 / 2500.0 // 0.04 BTC

	// 錯誤計算（如果用平均成本）
	avgCost := tracker.avgCost  // ≈ 2549
	wrongCoins := 100.0 / avgCost // ≈ 0.0392 BTC

	t.Logf("第一筆開倉實際買入: %.6f BTC @ 2500", correctCoins)
	t.Logf("如果用平均成本計算: %.6f BTC @ %.2f (錯誤！)", wrongCoins, avgCost)
	t.Logf("差異: %.6f BTC", correctCoins-wrongCoins)

	// 驗證：平倉後 totalCoins 應該準確減少
	totalBefore := tracker.totalCoins
	tracker.ClosePosition(pos1.ID, 2600, time.Now(), 4.0)
	totalAfter := tracker.totalCoins

	actualReduction := totalBefore - totalAfter

	if actualReduction < correctCoins-0.00001 || actualReduction > correctCoins+0.00001 {
		t.Errorf("平倉應該減少 %.6f BTC，實際減少 %.6f BTC", correctCoins, actualReduction)
	}

	t.Logf("✅ 正確減少了 %.6f BTC", actualReduction)
}
