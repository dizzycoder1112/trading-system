package engine

import (
	"fmt"
	"os"
	"time"

	"dizzycode.xyz/shared/domain/value_objects"
	"dizzycode.xyz/trading-strategy-server/backtesting/loader"
	"dizzycode.xyz/trading-strategy-server/backtesting/metrics"
	"dizzycode.xyz/trading-strategy-server/backtesting/simulator"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/strategies/grid"
)

// BacktestConfig 回測配置
type BacktestConfig struct {
	InitialBalance     float64 // 初始資金
	FeeRate            float64 // 手續費率（默認: 0.0005 = 0.05%）
	Slippage           float64 // 滑點（默認: 0）
	InstID             string  // 交易對 (e.g., "ETH-USDT-SWAP")
	TakeProfitMin      float64 // 最小停利百分比
	TakeProfitMax      float64 // 最大停利百分比
	PositionSize       float64 // 單次開倉大小 (USDT)
	BreakEvenProfitMin float64 // 打平最小目標盈利（USDT）⭐
	BreakEvenProfitMax float64 // 打平最大目標盈利（USDT）⭐
	EnableTrendFilter  bool    // 是否啟用趨勢過濾（默認: true）⭐
}

// BacktestEngine 回測引擎核心
type BacktestEngine struct {
	strategy          *grid.GridAggregate        // 真實的 Grid 策略 ⭐
	simulator         *simulator.OrderSimulator  // 成交模擬器
	positionTracker   *simulator.PositionTracker // 倉位追蹤器
	calculator        *metrics.MetricsCalculator // 指標計算器
	config            BacktestConfig             // 配置
	tradeLog          []TradeLog                 // 交易日誌 ⭐ DEBUG
	breakEvenRounds   []BreakEvenRound           // 打平輪次記錄 ⭐
	currentRoundStats RoundStats                 // 當前輪次統計 ⭐
}

// BreakEvenRound 打平輪次記錄
type BreakEvenRound struct {
	RoundID              int       // 輪次編號
	StartTime            time.Time // 輪次開始時間
	EndTime              time.Time // 輪次結束時間（打平觸發時間）
	Duration             string    // 持續時長
	TotalOpenCount       int       // 本輪總開倉次數
	TotalCloseCount      int       // 本輪總關倉次數
	RealizedPnL          float64   // 本輪已實現盈虧（扣除手續費）
	UnrealizedPnL        float64   // 觸發時的未實現盈虧
	ExpectedProfit       float64   // 預期總盈利（實現+未實現）
	TotalFees            float64   // 本輪總手續費
	TriggerPrice         float64   // 觸發打平時的價格
	AvgCost              float64   // 平均成本
	PositionsClosedCount int       // 打平時平掉的倉位數
}

// RoundStats 當前輪次統計
type RoundStats struct {
	RoundID          int       // 當前輪次編號
	StartTime        time.Time // 輪次開始時間
	OpenCount        int       // 本輪開倉次數
	CloseCount       int       // 本輪關倉次數
	TotalFeesInRound float64   // 本輪累積手續費
}

// TradeLog 交易日誌（用於 debug）
type TradeLog struct {
	TradeID                 int       // 交易序號
	Time                    time.Time // 時間
	Action                  string    // OPEN / CLOSE
	Price                   float64   // 價格
	PositionSize            float64   // 倉位大小
	Balance                 float64   // 當前餘額
	OpenPositionValue       float64   // 累計持倉總價值（USDT）⭐
	PnLPercent              float64   // 盈虧百分比（基於單筆開倉價）⭐
	PnL                     float64   // 盈虧金額（基於單筆開倉價，未扣手續費）⭐
	AvgCost                 float64   // 平倉時的平均成本（所有未平倉的加權平均）⭐
	PnLPercent_Avg          float64   // 基於平均成本的盈虧百分比 ⭐
	PnL_Avg                 float64   // 基於平均成本的盈虧金額（未扣手續費）⭐
	Fee                     float64   // 手續費 ⭐
	RoundClosedValue        float64   // 本輪累積關倉總價值（本金 + 盈虧）⭐
	CurrentRoundRealizedPnL float64   // 本輪已實現盈虧（基於平均成本，扣除手續費）⭐
	TotalRealizedPnL        float64   // 累計已實現盈虧（從回測開始到現在的所有已實現盈虧總和）⭐
	UnrealizedPnL           float64   // 浮動盈虧（所有未平倉倉位的未實現盈虧）⭐
	Reason                  string    // 原因
	PositionID              string    // 倉位ID（關聯開倉和平倉）⭐
}

// ⭐ 已刪除：calculateUnrealizedPnL - 統一使用 PositionTracker.CalculateUnrealizedPnL()

// NewBacktestEngine 創建回測引擎
func NewBacktestEngine(config BacktestConfig) (*BacktestEngine, error) {
	// 1. 創建真實的 Grid 策略 ⭐ 直接寫死參數（POC）
	strategy, err := grid.NewGridAggregate(grid.GridConfig{
		InstID:             config.InstID,
		PositionSize:       config.PositionSize,
		FeeRate:            config.FeeRate,
		TakeProfitRateMin:  config.TakeProfitMin,
		TakeProfitRateMax:  config.TakeProfitMax,
		BreakEvenProfitMin: config.BreakEvenProfitMin,
		BreakEvenProfitMax: config.BreakEvenProfitMax,
		EnableTrendFilter:  config.EnableTrendFilter, // ⭐ 直接寫死啟用
		TrendFilterConfig: grid.TrendAnalyzerConfig{
			EMAThreshold:    0.003, // 0.3%
			CandleThreshold: 0.004, // 0.4%
			EMAShortPeriod:  20,
			EMALongPeriod:   50,
			// 以下參數由 TrendAnalyzer 內部默認值處理：
			// PriceDropThreshold: 0.008 (0.8%)
			// ConsecutivePeriod:  5
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create grid strategy: %w", err)
	}

	// 2. 創建模擬器和追蹤器
	orderSimulator := simulator.NewOrderSimulator(config.FeeRate, config.Slippage)
	positionTracker := simulator.NewPositionTracker()
	calculator := metrics.NewMetricsCalculator(config.InitialBalance)

	return &BacktestEngine{
		strategy:          strategy,
		simulator:         orderSimulator,
		positionTracker:   positionTracker,
		calculator:        calculator,
		config:            config,
		breakEvenRounds:   []BreakEvenRound{},
		currentRoundStats: RoundStats{RoundID: 1}, // 從第1輪開始
	}, nil
}

// Run 執行回測
//
// 回測流程：
//  1. 遍歷歷史K線數據
//  2. 對每根K線，調用策略獲取開倉建議
//  3. 如果建議開倉，模擬開倉交易
//  4. 檢查所有未平倉是否觸及止盈
//  5. 如果觸及，模擬平倉交易
//  6. 記錄資金曲線（用於計算最大回撤）
//  7. 回測結束後，強制平倉所有未平倉
//  8. 計算回測指標
//
// 參數：
//   - candles: 歷史K線數據（從舊到新排序）
//
// 返回：
//   - BacktestResult: 回測結果
func (e *BacktestEngine) Run(candles []value_objects.Candle) (metrics.BacktestResult, error) {
	if len(candles) == 0 {
		return metrics.BacktestResult{}, fmt.Errorf("no candles provided")
	}

	balance := e.config.InitialBalance
	tradeCounter := 0 // 交易計數器

	// ⭐ 追蹤統計數據
	totalOpenedTrades := 0  // 總開倉數量
	totalProfitGross := 0.0 // 總利潤（未扣手續費）
	totalFeesOpen := 0.0    // 開倉總手續費
	totalFeesClose := 0.0   // 關倉總手續費

	// ⭐ 追蹤當前交易輪次數據（用於打平機制）
	openPositionValue := 0.0       // 累計持倉總價值（USDT）
	currentRoundRealizedPnL := 0.0 // 當前輪次已實現盈虧（扣除手續費）
	currentRoundClosedValue := 0.0 // 當前輪次累積關倉價值（本金 + 盈虧）⭐
	totalRealizedPnL := 0.0        // 累計已實現盈虧（從回測開始的所有已實現盈虧總和）⭐

	// ⭐ 追蹤持倉全滿天數（定義：可用餘額 < 單次開倉成本）
	fullPositionDays := make(map[string]bool) // 記錄哪些天達到持倉全滿（key: YYYY-MM-DD）
	maxOpenPositionValue := 0.0               // 追蹤最大持倉價值（USDT）⭐

	// 記錄初始資金
	e.calculator.RecordBalance(candles[0].Timestamp(), balance)

	// 遍歷所有K線
	for i := 0; i < len(candles); i++ {
		currentCandle := candles[i]
		currentPrice := currentCandle.Close()
		currentTime := currentCandle.Timestamp()

		// ========== 步驟 1: 檢查是否需要平倉 ==========
		// ⭐ 在平倉循環開始前，先計算當前時刻的平均成本（所有同一時間的平倉都使用這個值）
		avgCostAtThisTime := e.positionTracker.CalculateAverageCost()

		// ⭐ 重要：先複製倉位列表，避免在循環中修改導致跳過某些倉位
		positionsToCheck := make([]simulator.Position, len(e.positionTracker.GetOpenPositions()))
		copy(positionsToCheck, e.positionTracker.GetOpenPositions())

		// 注意：先檢查平倉，再考慮開倉（避免資金不足）
		for _, pos := range positionsToCheck {
			// ⭐ 檢查是否觸及目標平倉價格
			if currentPrice.Value() >= pos.TargetClosePrice {

				// ⭐ 模擬平倉（統一計算所有盈虧指標）
				closeResult, err := e.simulator.SimulateClose(pos, currentPrice.Value(), currentTime, avgCostAtThisTime)
				if err != nil {
					// 平倉失敗，記錄錯誤但繼續
					continue
				}

				// ⭐ 直接使用 OrderSimulator 計算的結果（無需重複計算）
				pnlAmount := closeResult.PnL                 // 基於開倉價的盈虧
				pnlPercent := closeResult.PnLPercent         // 基於開倉價的盈虧百分比
				pnlAmount_Avg := closeResult.PnL_Avg         // 基於平均成本的盈虧
				pnlPercent_Avg := closeResult.PnLPercent_Avg // 基於平均成本的盈虧百分比
				closeValue := closeResult.CloseValue         // 平倉總價值
				closeFee := closeResult.CloseFee             // 平倉手續費
				revenue := closeResult.Revenue               // 實際收入

				// 更新倉位追蹤器（傳入基於平均成本的盈虧，用於勝率計算）⭐
				err = e.positionTracker.ClosePosition(
					pos.ID,
					closeResult.ClosedPosition.ClosePrice,
					closeResult.ClosedPosition.CloseTime,
					closeResult.ClosedPosition.RealizedPnL, // 基於平均成本的已實現盈虧
				)
				if err != nil {
					continue
				}

				// 更新餘額
				balance += revenue

				// ⭐ 累加統計數據（使用基於單筆開倉價的盈虧）
				totalProfitGross += pnlAmount // 累加未扣費盈虧（基於單筆開倉價）⭐
				totalFeesClose += closeFee    // 累加關倉手續費

				// ⭐ 更新當前交易輪次數據
				openPositionValue -= pos.Size                                     // 減少累計持倉價值
				currentRoundRealizedPnL += closeResult.ClosedPosition.RealizedPnL // 累加當前輪次已實現盈虧（基於平均成本）⭐
				currentRoundClosedValue += closeValue                             // 累加當前輪次關倉價值⭐
				totalRealizedPnL += closeResult.ClosedPosition.RealizedPnL        // 累加總已實現盈虧⭐

				// ⭐ 在重置前保存當前值（用於日誌記錄）
				roundClosedValueForLog := currentRoundClosedValue
				roundRealizedPnLForLog := currentRoundRealizedPnL

				// ⭐ 檢查是否所有倉位被關閉（交易輪次結束）
				if openPositionValue <= 0.01 { // 使用小值避免浮點誤差
					openPositionValue = 0
					currentRoundRealizedPnL = 0 // 重置，開始新的交易輪次
					currentRoundClosedValue = 0 // 重置關倉價值⭐
				}

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)

				// ⭐ 記錄平倉日誌（使用這個時刻的平均成本，所有同時平倉的倉位都使用相同值）
				tradeCounter++
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:                 tradeCounter,
					Time:                    currentTime,
					Action:                  "CLOSE",
					Price:                   closeResult.ClosedPosition.ClosePrice,
					PositionSize:            closeValue, // ⭐ 平倉時的實際收回金額（含盈虧）
					Balance:                 balance,
					OpenPositionValue:       openPositionValue,                                                                // ⭐ 平倉後的累計持倉價值
					PnLPercent:              pnlPercent,                                                                       // ⭐ 基於單筆開倉價的盈虧百分比
					PnL:                     pnlAmount,                                                                        // ⭐ 基於單筆開倉價的盈虧金額（未扣手續費）
					AvgCost:                 avgCostAtThisTime,                                                                // ⭐ 這個時刻的平均成本（平倉前的狀態）
					PnLPercent_Avg:          pnlPercent_Avg,                                                                   // ⭐ 基於平均成本的盈虧百分比
					PnL_Avg:                 pnlAmount_Avg,                                                                    // ⭐ 基於平均成本的盈虧金額（未扣手續費）
					Fee:                     closeFee,                                                                         // ⭐ 平倉手續費（基於實際價值）
					RoundClosedValue:        roundClosedValueForLog,                                                           // ⭐ 本輪累積關倉總價值（重置前的值）
					CurrentRoundRealizedPnL: roundRealizedPnLForLog,                                                           // ⭐ 本輪已實現盈虧（重置前的值）
					TotalRealizedPnL:        totalRealizedPnL,                                                                 // ⭐ 累計已實現盈虧
					UnrealizedPnL:           e.positionTracker.CalculateUnrealizedPnL(currentPrice.Value(), e.config.FeeRate), // ⭐ 統一使用 PositionTracker
					Reason:                  fmt.Sprintf("hit_target_%.2f", pos.TargetClosePrice),
					PositionID:              pos.ID, // ⭐ 記錄倉位ID
				})
			}
		}

		// ========== 步驟 2: 調用策略獲取開倉建議 ==========
		// 使用當前價格和歷史K線（currentPrice 已經是 Price 對象）

		// 構建歷史K線（最多100根）
		startIdx := 0
		if i > 100 {
			startIdx = i - 100
		}
		histories := candles[startIdx:i]

		// 獲取上一根K線（如果存在）
		var lastCandle value_objects.Candle
		if i > 0 {
			lastCandle = candles[i-1]
		} else {
			lastCandle = currentCandle
		}

		// ========== 步驟 2.5: 計算當前倉位摘要 ⭐ ==========
		// 計算已支付的總手續費（從交易日誌）
		totalFeesPaid := e.GetTotalFees()

		// 獲取當前未平倉信息
		openPositions := e.positionTracker.GetOpenPositions()
		openCount := len(openPositions)
		totalSize := e.positionTracker.GetTotalSize()
		avgCost := e.positionTracker.CalculateAverageCost()

		// ⭐ 計算未實現盈虧（通過 PositionTracker，已包含預估平倉費）
		unrealizedPnL := e.positionTracker.CalculateUnrealizedPnL(currentPrice.Value(), e.config.FeeRate)

		// 創建倉位摘要（包含當前輪次已實現盈虧和關倉價值）⭐
		positionSummary := value_objects.NewPositionSummary(
			openCount,
			totalSize,
			avgCost,
			totalFeesPaid,
			currentRoundRealizedPnL, // ⭐ 傳入當前輪次已實現盈虧
			currentRoundClosedValue, // ⭐ 傳入當前輪次累積關倉價值
			unrealizedPnL,           // ⭐ 傳入外部計算的未實現盈虧
		)

		// ========== 🔍 驗證：對比兩種 ShouldBreakEven 方法 ⭐ ==========
		// if !positionSummary.IsEmpty() {
		// 方法1：內部計算 unrealizedPnL（簡化版，用平均價格）
		// shouldExit1, expectedProfit1 := positionSummary.ShouldBreakEven(
		// 	currentPrice.Value(),
		// 	e.config.FeeRate,
		// 	e.config.BreakEvenProfitMin,
		// 	e.config.BreakEvenProfitMax,
		// )

		// // 方法2：使用外部計算的 unrealizedPnL（精確版，逐倉位計算）
		// shouldExit2, expectedProfit2 := positionSummary.ShouldBreakEven2(
		// 	e.config.BreakEvenProfitMin,
		// 	e.config.BreakEvenProfitMax,
		// )

		// 記錄差異（只在結果不同時輸出）
		// if shouldExit1 != shouldExit2 {
		// 	fmt.Printf("⚠️ [K線 %d] ShouldBreakEven 差異檢測:\n", i)
		// 	fmt.Printf("   方法1 (內部計算): shouldExit=%v, expectedProfit=%.4f USDT\n", shouldExit1, expectedProfit1)
		// 	fmt.Printf("   方法2 (外部計算): shouldExit=%v, expectedProfit=%.4f USDT\n", shouldExit2, expectedProfit2)
		// 	fmt.Printf("   差值: %.4f USDT, 倉位數=%d, 平均成本=%.2f, 當前價格=%.2f\n\n",
		// 		expectedProfit2-expectedProfit1, positionSummary.Count, positionSummary.AvgPrice, currentPrice.Value())
		// }
		// }

		// 獲取開倉建議（grid.OpenAdvice）⭐ 傳入倉位摘要
		gridAdvice := e.strategy.GetOpenAdvice(currentPrice, lastCandle, histories, positionSummary)

		// ========== 步驟 2.8: 檢查是否觸發打平機制 ⭐ ==========
		// 即使不應該開倉，也要檢查是否因為打平退出
		if !gridAdvice.ShouldOpen && len(gridAdvice.Reason) >= 16 &&
			gridAdvice.Reason[:16] == "break_even_exit:" {
			// ⭐ 觸發打平機制：平掉所有未平倉位
			// ⭐ 重要：先複製倉位列表，避免在循環中修改導致跳過某些倉位
			positionsToClose := make([]simulator.Position, len(e.positionTracker.GetOpenPositions()))
			copy(positionsToClose, e.positionTracker.GetOpenPositions())

			// ⭐ 記錄本輪打平前的狀態
			beforeClosePositionCount := len(positionsToClose)
			beforeCloseRealizedPnL := currentRoundRealizedPnL
			beforeCloseUnrealizedPnL := unrealizedPnL

			for _, pos := range positionsToClose {
				// ⭐ 模擬平倉（統一計算所有盈虧指標）
				closeResult, err := e.simulator.SimulateClose(pos, currentPrice.Value(), currentTime, avgCostAtThisTime)
				if err != nil {
					continue
				}

				// ⭐ 直接使用 OrderSimulator 計算的結果（無需重複計算）
				pnlAmount := closeResult.PnL                 // 基於開倉價的盈虧
				pnlPercent := closeResult.PnLPercent         // 基於開倉價的盈虧百分比
				pnlAmount_Avg := closeResult.PnL_Avg         // 基於平均成本的盈虧
				pnlPercent_Avg := closeResult.PnLPercent_Avg // 基於平均成本的盈虧百分比
				closeValue := closeResult.CloseValue         // 平倉總價值
				closeFee := closeResult.CloseFee             // 平倉手續費
				revenue := closeResult.Revenue               // 實際收入

				// 更新倉位追蹤器（傳入基於平均成本的盈虧，用於勝率計算）⭐
				err = e.positionTracker.ClosePosition(
					pos.ID,
					closeResult.ClosedPosition.ClosePrice,
					closeResult.ClosedPosition.CloseTime,
					closeResult.ClosedPosition.RealizedPnL, // 基於平均成本的已實現盈虧
				)
				if err != nil {
					continue
				}

				// 更新餘額
				balance += revenue

				// ⭐ 累加統計數據（使用基於單筆開倉價的盈虧）
				totalProfitGross += pnlAmount // 累加未扣費盈虧（基於單筆開倉價）⭐
				totalFeesClose += closeFee    // 累加關倉手續費

				// ⭐ 更新當前交易輪次數據
				openPositionValue -= pos.Size                                     // 減少累計持倉價值
				currentRoundRealizedPnL += closeResult.ClosedPosition.RealizedPnL // 累加當前輪次已實現盈虧（基於平均成本）⭐
				currentRoundClosedValue += closeValue                             // 累加當前輪次關倉價值⭐
				totalRealizedPnL += closeResult.ClosedPosition.RealizedPnL        // 累加總已實現盈虧⭐

				// 更新當前輪次統計
				e.currentRoundStats.CloseCount++
				e.currentRoundStats.TotalFeesInRound += closeFee

				// ⭐ 檢查是否所有倉位被關閉（交易輪次結束）
				if openPositionValue <= 0.01 { // 使用小值避免浮點誤差
					openPositionValue = 0

					// ⭐ 記錄打平輪次（在重置前）
					if e.currentRoundStats.StartTime.IsZero() {
						e.currentRoundStats.StartTime = currentTime // 首次設置開始時間
					}

					round := BreakEvenRound{
						RoundID:              e.currentRoundStats.RoundID,
						StartTime:            e.currentRoundStats.StartTime,
						EndTime:              currentTime,
						Duration:             currentTime.Sub(e.currentRoundStats.StartTime).String(),
						TotalOpenCount:       e.currentRoundStats.OpenCount,
						TotalCloseCount:      e.currentRoundStats.CloseCount,
						RealizedPnL:          beforeCloseRealizedPnL,   // 打平前的已實現盈虧
						UnrealizedPnL:        beforeCloseUnrealizedPnL, // 打平前的未實現盈虧
						ExpectedProfit:       beforeCloseRealizedPnL + beforeCloseUnrealizedPnL,
						TotalFees:            e.currentRoundStats.TotalFeesInRound,
						TriggerPrice:         currentPrice.Value(),
						AvgCost:              avgCostAtThisTime,
						PositionsClosedCount: beforeClosePositionCount,
					}
					e.breakEvenRounds = append(e.breakEvenRounds, round)

					// 重置輪次數據
					currentRoundRealizedPnL = 0 // 重置，開始新的交易輪次
					currentRoundClosedValue = 0 // 重置關倉價值⭐
					e.currentRoundStats = RoundStats{
						RoundID:   e.currentRoundStats.RoundID + 1,
						StartTime: time.Time{}, // 重置，下次開倉時會設置
					}
				}

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)

				// ⭐ 記錄平倉日誌（使用這個時刻的平均成本）
				tradeCounter++
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:                 tradeCounter,
					Time:                    currentTime,
					Action:                  "CLOSE",
					Price:                   closeResult.ClosedPosition.ClosePrice,
					PositionSize:            closeValue,
					Balance:                 balance,
					OpenPositionValue:       openPositionValue, // ⭐ 平倉後的累計持倉價值
					PnLPercent:              pnlPercent,        // ⭐ 基於單筆開倉價的盈虧百分比
					PnL:                     pnlAmount,         // ⭐ 基於單筆開倉價的盈虧金額
					AvgCost:                 avgCostAtThisTime, // ⭐ 這個時刻的平均成本（打平前的狀態）
					PnLPercent_Avg:          pnlPercent_Avg,    // ⭐ 基於平均成本的盈虧百分比
					PnL_Avg:                 pnlAmount_Avg,     // ⭐ 基於平均成本的盈虧金額
					Fee:                     closeFee,
					RoundClosedValue:        currentRoundClosedValue,                                                          // ⭐ 本輪累積關倉總價值
					CurrentRoundRealizedPnL: currentRoundRealizedPnL,                                                          // ⭐ 本輪已實現盈虧
					UnrealizedPnL:           e.positionTracker.CalculateUnrealizedPnL(currentPrice.Value(), e.config.FeeRate), // ⭐ 統一使用 PositionTracker
					Reason:                  gridAdvice.Reason,                                                                // ⭐ 記錄打平退出原因
					PositionID:              pos.ID,
				})
			}
		}

		// ========== 步驟 3: 如果建議開倉，模擬開倉 ==========
		if gridAdvice.ShouldOpen {
			// 檢查餘額是否充足
			estimatedCost := gridAdvice.PositionSize * (1 + e.config.FeeRate) // 倉位大小 + 手續費

			if balance >= estimatedCost {
				// 轉換為 simulator.OpenAdvice
				advice := simulator.OpenAdvice{
					ShouldOpen:   gridAdvice.ShouldOpen,
					CurrentPrice: gridAdvice.CurrentPrice,
					OpenPrice:    gridAdvice.OpenPrice,
					ClosePrice:   gridAdvice.ClosePrice,
					PositionSize: gridAdvice.PositionSize,
					TakeProfit:   gridAdvice.TakeProfitRate,
					Reason:       gridAdvice.Reason,
				}

				// 模擬開倉
				position, cost, err := e.simulator.SimulateOpen(advice, balance, currentTime)
				if err != nil {
					// 開倉失敗，跳過
					continue
				}

				// 計算開倉手續費
				openFee := position.Size * e.config.FeeRate

				// 更新倉位追蹤器
				newPosition := e.positionTracker.AddPosition(
					position.EntryPrice,
					position.Size,
					position.OpenTime,
					position.TargetClosePrice,
				)

				// 更新餘額
				balance -= cost

				// ⭐ 累加統計數據
				totalOpenedTrades++      // 累加開倉數量
				totalFeesOpen += openFee // 累加開倉手續費

				// ⭐ 更新當前交易輪次數據
				openPositionValue += position.Size // 增加累計持倉價值

				// ⭐ 更新當前輪次統計
				if e.currentRoundStats.StartTime.IsZero() {
					e.currentRoundStats.StartTime = currentTime // 首次開倉，設置開始時間
				}
				e.currentRoundStats.OpenCount++
				e.currentRoundStats.TotalFeesInRound += openFee

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)

				// ⭐ 計算開倉後的平均成本
				avgCostAfterOpen := e.positionTracker.CalculateAverageCost()

				// ⭐ 記錄開倉日誌
				tradeCounter++
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:                 tradeCounter,
					Time:                    currentTime,
					Action:                  "OPEN",
					Price:                   position.EntryPrice,
					PositionSize:            position.Size,
					Balance:                 balance,
					OpenPositionValue:       openPositionValue, // ⭐ 開倉後的累計持倉價值
					AvgCost:                 avgCostAfterOpen,  // ⭐ 開倉後的平均成本
					PnL:                     0,
					Fee:                     openFee,                                                                          // ⭐ 記錄開倉手續費
					RoundClosedValue:        currentRoundClosedValue,                                                          // ⭐ 本輪累積關倉總價值
					CurrentRoundRealizedPnL: currentRoundRealizedPnL,                                                          // ⭐ 本輪已實現盈虧
					TotalRealizedPnL:        totalRealizedPnL,                                                                 // ⭐ 累計已實現盈虧
					UnrealizedPnL:           e.positionTracker.CalculateUnrealizedPnL(currentPrice.Value(), e.config.FeeRate), // ⭐ 統一使用 PositionTracker
					Reason:                  gridAdvice.Reason,
					PositionID:              newPosition.ID, // ⭐ 記錄倉位ID
				})
			}
		}

		// ⭐ 檢查當前是否為持倉全滿狀態（每根 K 線結束時檢查）
		if openPositionValue > maxOpenPositionValue {
			maxOpenPositionValue = openPositionValue // 更新最大持倉價值 ⭐
		}

		if balance < e.config.PositionSize {
			// 可用餘額不足以開下一個倉位 = 持倉全滿
			dateKey := currentTime.Format("2006-01-02") // YYYY-MM-DD
			fullPositionDays[dateKey] = true
		}
	}

	// ========== 步驟 4: 計算未實現盈虧（不強制平倉）==========
	lastCandle := candles[len(candles)-1]
	lastPrice := lastCandle.Close().Value()
	lastTime := lastCandle.Timestamp()

	// 記錄最終資金快照（不包含未平倉）
	e.calculator.RecordBalance(lastTime, balance)

	// ========== 步驟 5: 計算回測指標（包含未實現盈虧）==========
	result := e.calculator.Calculate(
		e.positionTracker,
		balance,
		lastPrice,
		totalOpenedTrades,
		totalProfitGross,
		totalFeesOpen,
		totalFeesClose,
	)

	// ⭐ 加入持倉全滿天數統計
	result.FullPositionDays = len(fullPositionDays)
	result.MaxOpenPositionValue = maxOpenPositionValue // ⭐ 加入最大持倉價值

	// ⭐ 輸出打平輪次統計報告
	e.printBreakEvenRoundsReport()

	return result, nil
}

// RunFromFile 從文件執行回測
//
// 便捷方法：載入歷史數據並執行回測
//
// 參數：
//   - filepath: 歷史數據文件路徑
//
// 返回：
//   - BacktestResult: 回測結果
func (e *BacktestEngine) RunFromFile(filepath string) (metrics.BacktestResult, error) {
	// 1. 載入歷史數據
	candles, err := loader.LoadFromJSON(filepath)
	if err != nil {
		return metrics.BacktestResult{}, fmt.Errorf("failed to load candles: %w", err)
	}

	// 2. 執行回測
	return e.Run(candles)
}

// GetPositionTracker 獲取倉位追蹤器（用於調試）
func (e *BacktestEngine) GetPositionTracker() *simulator.PositionTracker {
	return e.positionTracker
}

// GetMetricsCalculator 獲取指標計算器（用於調試）
func (e *BacktestEngine) GetMetricsCalculator() *metrics.MetricsCalculator {
	return e.calculator
}

// GetTradeLog 獲取交易日誌（用於 debug）
func (e *BacktestEngine) GetTradeLog() []TradeLog {
	return e.tradeLog
}

// GetTotalFees 計算總手續費
func (e *BacktestEngine) GetTotalFees() float64 {
	totalFees := 0.0
	for _, log := range e.tradeLog {
		totalFees += log.Fee
	}
	return totalFees
}

// ExportTradeLogCSV 導出交易日誌到 CSV 文件
func (e *BacktestEngine) ExportTradeLogCSV(filepath string) error {
	content := "TradeID,Time,Action,Price,PositionSize,Balance,OpenPositionValue,PnL%,PnL,AvgCost,PnL%_Avg,PnL_Avg,Fee,RoundClosedValue,CurrentRoundRealizedPnL,TotalRealizedPnL,UnrealizedPnL,Reason,PositionID\n"

	for _, log := range e.tradeLog {
		line := fmt.Sprintf("%d,%s,%s,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.8f,%.6f,%.6f,%.6f,%.6f,%s,%s\n",
			log.TradeID,
			log.Time.UTC().Format("2006-01-02 15:04:05"), // ⭐ 使用 UTC 時間（GMT+0）
			log.Action,
			log.Price,                   // 價格：6位小數
			log.PositionSize,            // 倉位大小：6位小數
			log.Balance,                 // 餘額：6位小數
			log.OpenPositionValue,       // ⭐ 累計持倉總價值：6位小數
			log.PnLPercent,              // ⭐ 盈虧百分比（基於單筆）：6位小數
			log.PnL,                     // ⭐ 盈虧金額（基於單筆）：6位小數 ⭐
			log.AvgCost,                 // ⭐ 平均成本：6位小數 ⭐
			log.PnLPercent_Avg,          // ⭐ 盈虧百分比（基於平均）：6位小數
			log.PnL_Avg,                 // ⭐ 盈虧金額（基於平均）：6位小數 ⭐
			log.Fee,                     // ⭐ 手續費：8位小數
			log.RoundClosedValue,        // ⭐ 本輪累積關倉總價值：6位小數
			log.CurrentRoundRealizedPnL, // ⭐ 本輪已實現盈虧：6位小數
			log.TotalRealizedPnL,        // ⭐ 累計已實現盈虧：6位小數
			log.UnrealizedPnL,           // ⭐ 浮動盈虧（所有未平倉倉位）：6位小數
			log.Reason,
			log.PositionID,
		)
		content += line
	}

	// 寫入文件
	err := os.WriteFile(filepath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write CSV file: %w", err)
	}

	return nil
}

// printBreakEvenRoundsReport 輸出打平輪次統計報告
func (e *BacktestEngine) printBreakEvenRoundsReport() {
	if len(e.breakEvenRounds) == 0 {
		fmt.Println("\n========================================")
		fmt.Println("⭐ 打平輪次統計")
		fmt.Println("========================================")
		fmt.Println("本次回測沒有觸發打平機制")
		return
	}

	fmt.Println("\n========================================")
	fmt.Println("⭐ 打平輪次統計")
	fmt.Println("========================================")
	fmt.Printf("總輪次數: %d\n\n", len(e.breakEvenRounds))

	// 統計數據
	totalProfit := 0.0
	totalFees := 0.0
	totalTrades := 0
	maxReleasePosition := 0.0

	for i, round := range e.breakEvenRounds {
		totalProfit += round.ExpectedProfit
		totalFees += round.TotalFees
		totalTrades += round.TotalOpenCount + round.TotalCloseCount
		releasePosition := float64(round.PositionsClosedCount) * e.config.PositionSize
		if releasePosition > maxReleasePosition {
			maxReleasePosition = releasePosition
		}

		fmt.Printf("【輪次 %d】\n", round.RoundID)
		fmt.Printf("  時間範圍: %s ~ %s (持續: %s)\n",
			round.StartTime.Format("2006-01-02 15:04"),
			round.EndTime.Format("2006-01-02 15:04"),
			round.Duration)
		fmt.Printf("  交易次數: 開倉 %d 筆 | 關倉 %d 筆\n",
			round.TotalOpenCount, round.TotalCloseCount)
		fmt.Printf("  盈虧狀況:\n")
		fmt.Printf("    - 已實現盈虧: %.2f USDT\n", round.RealizedPnL)
		fmt.Printf("    - 未實現盈虧: %.2f USDT\n", round.UnrealizedPnL)
		fmt.Printf("    - 預期總盈利: %.2f USDT ⭐\n", round.ExpectedProfit)
		fmt.Printf("    - 總手續費: %.2f USDT\n", round.TotalFees)
		fmt.Printf("  觸發價格: %.2f (平均成本: %.2f)\n", round.TriggerPrice, round.AvgCost)
		fmt.Printf("  平倉數量: %d 筆倉位\n", round.PositionsClosedCount)

		if round.ExpectedProfit >= 0 {
			fmt.Printf("  ✅ 保本/盈利退出\n")
		} else {
			fmt.Printf("  ❌ 虧損退出\n")
		}
		fmt.Println()

		// 只顯示前10輪，避免輸出過長
		if i >= 9 && i < len(e.breakEvenRounds)-1 {
			fmt.Printf("... (省略 %d 輪) ...\n\n", len(e.breakEvenRounds)-10)
			break
		}
	}

	// 如果有超過10輪，顯示最後一輪
	if len(e.breakEvenRounds) > 10 {
		round := e.breakEvenRounds[len(e.breakEvenRounds)-1]
		fmt.Printf("【輪次 %d】(最後一輪)\n", round.RoundID)
		fmt.Printf("  時間範圍: %s ~ %s (持續: %s)\n",
			round.StartTime.Format("2006-01-02 15:04"),
			round.EndTime.Format("2006-01-02 15:04"),
			round.Duration)
		fmt.Printf("  交易次數: 開倉 %d 筆 | 關倉 %d 筆\n",
			round.TotalOpenCount, round.TotalCloseCount)
		fmt.Printf("  預期總盈利: %.2f USDT ⭐\n", round.ExpectedProfit)
		fmt.Println()
	}

	// 彙總統計
	fmt.Println("----------------------------------------")
	fmt.Println("📊 彙總統計")
	fmt.Println("----------------------------------------")
	fmt.Printf("總輪次數: %d\n", len(e.breakEvenRounds))
	fmt.Printf("平均每輪盈利: %.2f USDT\n", totalProfit/float64(len(e.breakEvenRounds)))
	fmt.Printf("平均每輪手續費: %.2f USDT\n", totalFees/float64(len(e.breakEvenRounds)))
	// fmt.Printf("平均每輪交易數: %.1f 筆\n", float64(totalTrades)/float64(len(e.breakEvenRounds)))
	fmt.Printf("最大釋放倉位量: %.2f USDT\n", maxReleasePosition)
	fmt.Printf("觸發平攤總盈虧: %.2f USDT\n", totalProfit)

	// 盈虧分佈
	profitRounds := 0
	lossRounds := 0
	for _, round := range e.breakEvenRounds {
		if round.ExpectedProfit >= 0 {
			profitRounds++
		} else {
			lossRounds++
		}
	}
	fmt.Printf("盈利輪次: %d (%.1f%%)\n", profitRounds, float64(profitRounds)/float64(len(e.breakEvenRounds))*100)
	fmt.Printf("虧損輪次: %d (%.1f%%)\n", lossRounds, float64(lossRounds)/float64(len(e.breakEvenRounds))*100)
	fmt.Println("========================================\n")
}
