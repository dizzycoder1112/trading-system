package simulator

import (
	"testing"
	"time"
)

// TestPositionTracker_ClosePosition_UserExample 测试用户提出的例子
// 验证平仓时使用开仓价而不是平均成本计算币数
func TestPositionTracker_ClosePosition_UserExample(t *testing.T) {
	tracker := NewPositionTracker()

	// 开仓1: 100 USDT @ 2500 → 应该买入 0.04 BTC
	pos1 := tracker.AddPosition(2500, 100, time.Now(), 2600)
	expectedCoins1 := 100.0 / 2500.0 // 0.04 BTC

	if tracker.totalCoins != expectedCoins1 {
		t.Errorf("开仓1后，totalCoins 应该是 %.6f，实际是 %.6f", expectedCoins1, tracker.totalCoins)
	}

	if tracker.avgCost != 2500 {
		t.Errorf("开仓1后，avgCost 应该是 2500，实际是 %.2f", tracker.avgCost)
	}

	// 开仓2: 100 USDT @ 2600 → 应该买入 0.0385 BTC
	tracker.AddPosition(2600, 100, time.Now(), 2700)
	expectedCoins2 := 100.0 / 2600.0 // 0.0385 BTC
	expectedTotalCoins := expectedCoins1 + expectedCoins2

	if tracker.totalCoins < expectedTotalCoins-0.00001 || tracker.totalCoins > expectedTotalCoins+0.00001 {
		t.Errorf("开仓2后，totalCoins 应该是 %.6f，实际是 %.6f", expectedTotalCoins, tracker.totalCoins)
	}

	// 平均成本应该是加权平均
	// avgCost = (2500*0.04 + 2600*0.038462) / (0.04 + 0.038462)
	//         = (100 + 100) / 0.078462
	//         = 200 / 0.078462 ≈ 2549.02
	expectedAvgCost := 200.0 / expectedTotalCoins

	if tracker.avgCost < expectedAvgCost-1 || tracker.avgCost > expectedAvgCost+1 {
		t.Errorf("开仓2后，avgCost 应该约为 %.2f，实际是 %.2f", expectedAvgCost, tracker.avgCost)
	}

	// ⭐ 关键测试：平仓1，在 2600 平掉第一笔开仓
	// 应该平掉 0.04 BTC（第一笔实际买入的币数）
	// 而不是 100 / 2549 ≈ 0.0392 BTC（用平均成本计算的币数）
	err := tracker.ClosePosition(pos1.ID, 2600, time.Now(), 4.0)
	if err != nil {
		t.Fatalf("平仓失败: %v", err)
	}

	// 平仓后，剩余币数应该是 0.0385 BTC（只剩第二笔）
	expectedRemainingCoins := expectedCoins2

	if tracker.totalCoins < expectedRemainingCoins-0.00001 || tracker.totalCoins > expectedRemainingCoins+0.00001 {
		t.Errorf("平仓1后，剩余 totalCoins 应该是 %.6f，实际是 %.6f", expectedRemainingCoins, tracker.totalCoins)
	}

	// ⭐ 平仓后，平均成本应该保持不变（关键特性）
	// 平仓只减少币数，不改变平均成本
	if tracker.avgCost < expectedAvgCost-1 || tracker.avgCost > expectedAvgCost+1 {
		t.Errorf("平仓1后，avgCost 应该保持约 %.2f，实际是 %.2f", expectedAvgCost, tracker.avgCost)
	}

	// 验证盈亏计算
	// 第一笔：100 USDT @ 2500 → 2600 平仓
	// 收入 = 0.04 * 2600 = 104 USDT
	// 成本 = 100 USDT
	// 盈亏 = 4 USDT ✅
	expectedPnL := 4.0
	actualPnL := tracker.CalculateTotalRealizedPnL()

	if actualPnL != expectedPnL {
		t.Errorf("已实现盈亏应该是 %.2f USDT，实际是 %.2f USDT", expectedPnL, actualPnL)
	}

	t.Logf("✅ 用户例子验证通过:")
	t.Logf("   开仓1: 100 USDT @ 2500 → 0.04 BTC")
	t.Logf("   开仓2: 100 USDT @ 2600 → %.6f BTC", expectedCoins2)
	t.Logf("   总持仓: %.6f BTC，平均成本: %.2f", expectedTotalCoins, expectedAvgCost)
	t.Logf("   平仓1: 0.04 BTC @ 2600，盈亏: %.2f USDT", expectedPnL)
	t.Logf("   剩余持仓: %.6f BTC，平均成本: %.2f", tracker.totalCoins, tracker.avgCost)
}

// TestPositionTracker_ClosePosition_WrongCalculation 演示错误计算的后果
func TestPositionTracker_ClosePosition_CompareCalculations(t *testing.T) {
	// ===== 正确计算 =====
	trackerCorrect := NewPositionTracker()
	pos1 := trackerCorrect.AddPosition(2500, 100, time.Now(), 2600)
	trackerCorrect.AddPosition(2600, 100, time.Now(), 2700)

	// 第一笔开仓买入的实际币数
	correctCoins := 100.0 / 2500.0 // 0.04 BTC

	// ===== 错误计算（如果用平均成本）=====
	avgCost := trackerCorrect.avgCost // ≈ 2549
	wrongCoins := 100.0 / avgCost     // ≈ 0.0392 BTC

	t.Logf("第一笔开仓实际买入: %.6f BTC @ 2500", correctCoins)
	t.Logf("如果用平均成本计算: %.6f BTC @ %.2f (错误！)", wrongCoins, avgCost)
	t.Logf("差异: %.6f BTC", correctCoins-wrongCoins)

	// 验证：平仓后 totalCoins 应该准确减少
	totalBefore := trackerCorrect.totalCoins
	trackerCorrect.ClosePosition(pos1.ID, 2600, time.Now(), 4.0)
	totalAfter := trackerCorrect.totalCoins

	actualReduction := totalBefore - totalAfter

	if actualReduction < correctCoins-0.00001 || actualReduction > correctCoins+0.00001 {
		t.Errorf("平仓应该减少 %.6f BTC，实际减少 %.6f BTC", correctCoins, actualReduction)
	}

	t.Logf("✅ 正确减少了 %.6f BTC", actualReduction)
}
