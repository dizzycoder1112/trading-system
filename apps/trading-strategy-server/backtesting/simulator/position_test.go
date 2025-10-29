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
	// 價格變化: 2510 - 2500 = 10
	// 收益: (10 / 2500) * 200 = 0.8 USDT
	// 手續費: 200 * 0.0005 * 2 = 0.20 USDT
	// 淨盈虧: 0.8 - 0.20 = 0.60 USDT
	expected := 0.60

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
