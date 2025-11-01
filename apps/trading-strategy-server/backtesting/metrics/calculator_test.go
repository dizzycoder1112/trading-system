package metrics

import (
	"testing"
	"time"

	"dizzycode.xyz/trading-strategy-server/backtesting/simulator"
)

// TestNetProfitCalculation_VerifyFeeAccounting 驗證淨利潤計算的費用會計邏輯
//
// 測試場景：
// 1. 初始資金：10000 USDT
// 2. 開倉 3 筆，每筆 100 USDT
// 3. 平倉 2 筆（1盈1虧）
// 4. 剩餘 1 筆未平倉（有浮虧）
//
// 驗證：NetProfit = TotalProfitGross + UnrealizedPnL - TotalFeesOpen - TotalFeesClose
func TestNetProfitCalculation_VerifyFeeAccounting(t *testing.T) {
	feeRate := 0.0005 // 0.05%

	// ===== 步驟 1: 初始化 =====
	initialBalance := 10000.0
	calculator := NewMetricsCalculator(initialBalance)
	positionTracker := simulator.NewPositionTracker()

	balance := initialBalance
	now := time.Now()

	// ===== 步驟 2: 開倉 3 筆 =====
	// 每筆開倉：
	// - positionSize = 100 USDT
	// - openFee = 100 * 0.0005 = 0.05 USDT
	// - cost = 100 + 0.05 = 100.05 USDT

	totalFeesOpen := 0.0
	totalFeesClose := 0.0
	totalProfitGross := 0.0

	// 開倉 #1: 價格 2500
	pos1 := positionTracker.AddPosition(2500, 100, now, 2510)
	openFee1 := 100 * feeRate // 0.05
	balance -= (100 + openFee1)
	totalFeesOpen += openFee1
	t.Logf("開倉 #1: price=2500, size=100, openFee=%.4f, balance=%.2f", openFee1, balance)

	// 開倉 #2: 價格 2520
	pos2 := positionTracker.AddPosition(2520, 100, now, 2530)
	openFee2 := 100 * feeRate // 0.05
	balance -= (100 + openFee2)
	totalFeesOpen += openFee2
	t.Logf("開倉 #2: price=2520, size=100, openFee=%.4f, balance=%.2f", openFee2, balance)

	// 開倉 #3: 價格 2510 (未平倉)
	_ = positionTracker.AddPosition(2510, 100, now, 2520)
	openFee3 := 100 * feeRate // 0.05
	balance -= (100 + openFee3)
	totalFeesOpen += openFee3
	t.Logf("開倉 #3: price=2510, size=100, openFee=%.4f, balance=%.2f", openFee3, balance)

	t.Logf("\n開倉後狀態：")
	t.Logf("  balance = %.2f USDT", balance)
	t.Logf("  totalFeesOpen = %.4f USDT", totalFeesOpen)

	// ===== 步驟 3: 平倉 #1 (盈利) =====
	// 開倉: 2500, 平倉: 2510
	// coins = 100 / 2500 = 0.04
	// profit = 0.04 * (2510 - 2500) = 0.4 USDT
	// closeValue = 100 + 0.4 = 100.4
	// closeFee = 100.4 * 0.0005 = 0.0502
	// revenue = 100.4 - 0.0502 = 100.3498
	// realizedPnL = 0.4 - 0.05 - 0.0502 = 0.2998

	closePrice1 := 2510.0
	coins1 := 100.0 / 2500.0
	profit1 := coins1 * (closePrice1 - 2500)
	closeValue1 := 100 + profit1
	closeFee1 := closeValue1 * feeRate
	revenue1 := closeValue1 - closeFee1
	realizedPnL1 := profit1 - openFee1 - closeFee1

	positionTracker.ClosePosition(pos1.ID, closePrice1, now.Add(5*time.Minute), realizedPnL1)
	balance += revenue1
	totalProfitGross += profit1
	totalFeesClose += closeFee1

	t.Logf("\n平倉 #1 (盈利)：")
	t.Logf("  profit = %.4f USDT", profit1)
	t.Logf("  closeFee = %.4f USDT", closeFee1)
	t.Logf("  revenue = %.4f USDT", revenue1)
	t.Logf("  realizedPnL = %.4f USDT", realizedPnL1)
	t.Logf("  balance = %.2f USDT", balance)

	// ===== 步驟 4: 平倉 #2 (虧損) =====
	// 開倉: 2520, 平倉: 2510
	// coins = 100 / 2520 = 0.03968
	// profit = 0.03968 * (2510 - 2520) = -0.3968
	// closeValue = 100 - 0.3968 = 99.6032
	// closeFee = 99.6032 * 0.0005 = 0.0498
	// revenue = 99.6032 - 0.0498 = 99.5534
	// realizedPnL = -0.3968 - 0.05 - 0.0498 = -0.4966

	closePrice2 := 2510.0
	coins2 := 100.0 / 2520.0
	profit2 := coins2 * (closePrice2 - 2520)
	closeValue2 := 100 + profit2
	closeFee2 := closeValue2 * feeRate
	revenue2 := closeValue2 - closeFee2
	realizedPnL2 := profit2 - openFee2 - closeFee2

	positionTracker.ClosePosition(pos2.ID, closePrice2, now.Add(10*time.Minute), realizedPnL2)
	balance += revenue2
	totalProfitGross += profit2
	totalFeesClose += closeFee2

	t.Logf("\n平倉 #2 (虧損)：")
	t.Logf("  profit = %.4f USDT", profit2)
	t.Logf("  closeFee = %.4f USDT", closeFee2)
	t.Logf("  revenue = %.4f USDT", revenue2)
	t.Logf("  realizedPnL = %.4f USDT", realizedPnL2)
	t.Logf("  balance = %.2f USDT", balance)

	// ===== 步驟 5: 計算未平倉 #3 的未實現盈虧 =====
	// 開倉: 2510, 當前價: 2500
	// coins = 100 / 2510 = 0.03984
	// profit = 0.03984 * (2500 - 2510) = -0.3984
	// estimatedCloseValue = 100 - 0.3984 = 99.6016
	// estimatedCloseFee = 99.6016 * 0.0005 = 0.0498
	// unrealizedPnL = -0.3984 - 0.0498 = -0.4482 (注意：不包含開倉費)

	currentPrice := 2500.0
	unrealizedPnL := positionTracker.CalculateUnrealizedPnL(currentPrice, feeRate)

	t.Logf("\n未平倉 #3 狀態：")
	t.Logf("  openPrice = 2510")
	t.Logf("  currentPrice = %.2f", currentPrice)
	t.Logf("  unrealizedPnL = %.4f USDT (已扣預估平倉費，未扣開倉費)", unrealizedPnL)

	// ===== 步驟 6: 驗證淨利潤計算 =====
	t.Logf("\n========== 費用統計 ==========")
	t.Logf("totalProfitGross = %.4f USDT (已平倉毛利)", totalProfitGross)
	t.Logf("totalFeesOpen    = %.4f USDT (3筆開倉費)", totalFeesOpen)
	t.Logf("totalFeesClose   = %.4f USDT (2筆平倉費)", totalFeesClose)
	t.Logf("unrealizedPnL    = %.4f USDT (1筆未實現盈虧，已扣預估平倉費)", unrealizedPnL)

	// 計算淨利潤（使用正確的公式）
	netProfit := totalProfitGross + unrealizedPnL - totalFeesOpen - totalFeesClose

	t.Logf("\n========== 淨利潤計算 ==========")
	t.Logf("公式: NetProfit = TotalProfitGross + UnrealizedPnL - TotalFeesOpen - TotalFeesClose")
	t.Logf("NetProfit = %.4f + %.4f - %.4f - %.4f", totalProfitGross, unrealizedPnL, totalFeesOpen, totalFeesClose)
	t.Logf("NetProfit = %.4f USDT", netProfit)

	// 驗證：淨利潤應該等於 (最終總權益 - 初始資金)
	// 總權益 = balance + 未平倉價值 + unrealizedPnL
	openPositionValue := positionTracker.GetTotalSize()
	totalEquity := balance + openPositionValue + unrealizedPnL

	expectedNetProfit := totalEquity - initialBalance

	t.Logf("\n========== 驗證 ==========")
	t.Logf("balance              = %.4f USDT", balance)
	t.Logf("openPositionValue    = %.4f USDT", openPositionValue)
	t.Logf("unrealizedPnL        = %.4f USDT", unrealizedPnL)
	t.Logf("totalEquity          = %.4f USDT", totalEquity)
	t.Logf("initialBalance       = %.4f USDT", initialBalance)
	t.Logf("expectedNetProfit    = %.4f USDT", expectedNetProfit)

	// 驗證兩種計算方式結果一致
	if diff := netProfit - expectedNetProfit; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("淨利潤計算不一致！")
		t.Errorf("  公式計算: %.4f USDT", netProfit)
		t.Errorf("  權益計算: %.4f USDT", expectedNetProfit)
		t.Errorf("  差異: %.6f USDT", diff)
	} else {
		t.Logf("\n✅ 淨利潤計算正確：%.4f USDT", netProfit)
	}

	// ===== 步驟 7: 測試 MetricsCalculator =====
	result := calculator.Calculate(
		positionTracker,
		balance,
		currentPrice,
		3, // totalOpenedTrades
		totalProfitGross,
		totalFeesOpen,
		totalFeesClose,
	)

	t.Logf("\n========== MetricsCalculator 結果 ==========")
	t.Logf("NetProfit (calculator) = %.4f USDT", result.NetProfit)
	t.Logf("NetProfit (expected)   = %.4f USDT", expectedNetProfit)

	if diff := result.NetProfit - expectedNetProfit; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("MetricsCalculator 計算錯誤！")
		t.Errorf("  實際: %.4f USDT", result.NetProfit)
		t.Errorf("  預期: %.4f USDT", expectedNetProfit)
		t.Errorf("  差異: %.6f USDT", diff)
	} else {
		t.Logf("\n✅ MetricsCalculator 計算正確")
	}
}
