package engine

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"dizzycode.xyz/shared/domain/value_objects"
	"dizzycode.xyz/trading-strategy-server/backtesting/loader"
	"dizzycode.xyz/trading-strategy-server/backtesting/metrics"
	"dizzycode.xyz/trading-strategy-server/backtesting/simulator"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/strategies/grid"
	"github.com/shopspring/decimal"
)

// BacktestConfig 回測配置
type BacktestConfig struct {
	InitialBalance        float64 // 初始資金
	FeeRate               float64 // 手續費率（默認: 0.0005 = 0.05%）
	Slippage              float64 // 滑點（默認: 0）
	InstID                string  // 交易對 (e.g., "ETH-USDT-SWAP")
	TakeProfitMin         float64 // 最小停利百分比
	TakeProfitMax         float64 // 最大停利百分比
	PositionSize          float64 // 單次開倉大小 (USDT)
	BreakEvenProfitMin    float64 // 打平最小目標盈利（USDT）⭐
	BreakEvenProfitMax    float64 // 打平最大目標盈利（USDT）⭐
	EnableTrendFilter     bool    // 是否啟用趨勢過濾（默認: true）⭐
	EnableRedCandleFilter bool    // 是否啟用紅K過濾（虧損時只在紅K開倉）⭐
	// 自動注資機制 ⭐
	EnableAutoFunding bool    // 是否啟用自動注資（默認: false）
	AutoFundingAmount float64 // 自動注資金額（USDT，默認: 5000）
	AutoFundingIdle   int     // 觸發注資的閒置K線數（默認: 288）
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
	// 自動注資追蹤 ⭐
	fundingHistory    []FundingRecord // 注資記錄
	idleCandles       int             // 當前閒置K線計數
	pendingFunding    float64         // 待回收的注資金額（累計未回收的注資）⭐
	maxPendingFunding float64         // 最大待回收注資峰值 ⭐⭐
}

// BreakEvenRound 打平輪次記錄
type BreakEvenRound struct {
	RoundID             int       // 輪次編號
	StartTime           time.Time // 輪次開始時間
	EndTime             time.Time // 輪次結束時間（打平觸發時間）
	Duration            string    // 持續時長
	TotalOpenCount      int       // 本輪總開倉次數
	NormalCloseCount    int       // 正常止盈關倉次數 ⭐
	BreakEvenCloseCount int       // 打平強制關倉次數 ⭐
	TotalCloseCount     int       // 總關倉次數（正常 + 打平）⭐
	RealizedPnL         float64   // 本輪已實現盈虧（扣除手續費）
	UnrealizedPnL       float64   // 觸發時的未實現盈虧
	ExpectedProfit      float64   // 預期總盈利（實現+未實現）
	TotalFees           float64   // 本輪總手續費
	TriggerPrice        float64   // 觸發打平時的價格
	AvgCost             float64   // 平均成本
}

// FundingRecord 自動注資記錄 ⭐
type FundingRecord struct {
	Time          time.Time // 注資時間
	Amount        float64   // 注資金額
	IdleCandles   int       // 觸發時的閒置K線數
	BalanceBefore float64   // 注資前餘額
	BalanceAfter  float64   // 注資後餘額
	Price         float64   // 當時價格
	CandleIndex   int       // K線索引
	Recovered     bool      // 是否已回收 ⭐
	RecoveredAt   time.Time // 回收時間 ⭐
}

// RoundStats 當前輪次統計
type RoundStats struct {
	RoundID             int       // 當前輪次編號
	StartTime           time.Time // 輪次開始時間
	OpenCount           int       // 本輪開倉次數
	NormalCloseCount    int       // 正常止盈關倉次數 ⭐
	BreakEvenCloseCount int       // 打平強制關倉次數 ⭐
	TotalFeesInRound    float64   // 本輪累積手續費
}

// GetTotalCloseCount 獲取總關倉次數（正常 + 打平）
func (rs *RoundStats) GetTotalCloseCount() int {
	return rs.NormalCloseCount + rs.BreakEvenCloseCount
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

// ExecuteCloseResult 平倉執行結果（用於 decimal 累加）⭐
type ExecuteCloseResult struct {
	Revenue           decimal.Decimal // 實際收入（返回餘額）
	ProfitGross       decimal.Decimal // 基於平均成本的盈虧
	ProfitGross_Entry decimal.Decimal // 基於單筆開倉價的盈虧
	CloseFee          decimal.Decimal // 平倉手續費
	RealizedPnL       decimal.Decimal // 已實現盈虧（基於平均成本，扣除手續費）
	ClosedValue       decimal.Decimal // 平倉總價值
	PositionSize      decimal.Decimal // 倉位大小（用於更新 openPositionValue）
	// 用於交易日誌的 float64 值
	PnLPercent     float64 // 基於開倉價的盈虧百分比
	PnL            float64 // 基於開倉價的盈虧金額
	PnLPercent_Avg float64 // 基於平均成本的盈虧百分比
	PnL_Avg        float64 // 基於平均成本的盈虧金額
	ClosePrice     float64 // 平倉價格
}

// NewBacktestEngine 創建回測引擎
func NewBacktestEngine(config BacktestConfig) (*BacktestEngine, error) {
	// 1. 創建真實的 Grid 策略 ⭐ 直接寫死參數（POC）
	strategy, err := grid.NewGridAggregate(grid.GridConfig{
		InstID:                config.InstID,
		PositionSize:          config.PositionSize,
		FeeRate:               config.FeeRate,
		TakeProfitRateMin:     config.TakeProfitMin,
		TakeProfitRateMax:     config.TakeProfitMax,
		BreakEvenProfitMin:    config.BreakEvenProfitMin,
		BreakEvenProfitMax:    config.BreakEvenProfitMax,
		EnableTrendFilter:     config.EnableTrendFilter,     // ⭐ 是否啟用趨勢過濾
		EnableRedCandleFilter: config.EnableRedCandleFilter, // ⭐ 是否啟用紅K過濾
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
		fundingHistory:    []FundingRecord{},      // 初始化注資記錄 ⭐
		idleCandles:       0,                      // 初始化閒置計數 ⭐
		pendingFunding:    0,                      // 初始化待回收注資 ⭐
		maxPendingFunding: 0,                      // 初始化最大待回收峰值 ⭐⭐
	}, nil
}

// executeClose 执行平仓操作（返回需要累加的結果）⭐ 重構版
//
// 这个辅助函数封装了平仓的核心流程：
//  1. 调用 OrderSimulator.SimulateClose() 计算平仓结果
//  2. 更新 PositionTracker 状态
//  3. 返回需要累加的数据（由调用方用 decimal 累加）
//
// 参数：
//   - pos: 要平仓的仓位
//   - closePrice: 平仓价格
//   - closeTime: 平仓时间
//   - avgCost: 平均成本
//
// 返回：
//   - ExecuteCloseResult: 平仓結果（用於 decimal 累加）
//   - error: 如果平仓失败则返回错误
func (e *BacktestEngine) executeClose(
	pos simulator.Position,
	closePrice float64,
	closeTime time.Time,
	avgCost float64,
) (ExecuteCloseResult, error) {
	// 1. 模拟平仓（统一计算所有盈亏指标）
	closeResult, err := e.simulator.SimulateClose(pos, closePrice, closeTime, avgCost)
	if err != nil {
		return ExecuteCloseResult{}, err
	}

	// 2. 更新仓位追踪器
	err = e.positionTracker.ClosePosition(
		pos.ID,
		closeResult.ClosedPosition.ClosePrice,
		closeResult.ClosedPosition.CloseTime,
		closeResult.ClosedPosition.RealizedPnL, // 基于平均成本的已实现盈亏
	)
	if err != nil {
		return ExecuteCloseResult{}, err
	}

	// 3. 構建返回結果（使用 decimal 類型）
	return ExecuteCloseResult{
		Revenue:           decimal.NewFromFloat(closeResult.Revenue),
		ProfitGross:       decimal.NewFromFloat(closeResult.PnL_Avg), // 基於平均成本
		ProfitGross_Entry: decimal.NewFromFloat(closeResult.PnL),     // 基於開倉價
		CloseFee:          decimal.NewFromFloat(closeResult.CloseFee),
		RealizedPnL:       decimal.NewFromFloat(closeResult.ClosedPosition.RealizedPnL),
		ClosedValue:       decimal.NewFromFloat(closeResult.CloseValue),
		PositionSize:      decimal.NewFromFloat(pos.Size),
		// 用於交易日誌的 float64 值
		PnLPercent:     closeResult.PnLPercent,
		PnL:            closeResult.PnL,
		PnLPercent_Avg: closeResult.PnLPercent_Avg,
		PnL_Avg:        closeResult.PnL_Avg,
		ClosePrice:     closeResult.ClosedPosition.ClosePrice,
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

	// ⭐ 使用 decimal 計算，避免浮點誤差
	balanceD := decimal.NewFromFloat(e.config.InitialBalance)
	tradeCounter := 0 // 交易計數器

	// ⭐ 追蹤統計數據（使用 decimal）
	totalOpenedTrades := 0                      // 總開倉數量
	totalProfitGrossD := decimal.Zero           // 總利潤（基於平均成本，未扣手續費）
	totalProfitGross_EntryD := decimal.Zero     // 總利潤（基於單筆開倉價，未扣手續費）⭐ 新增
	totalFeesOpenD := decimal.Zero              // 開倉總手續費
	totalFeesCloseD := decimal.Zero             // 關倉總手續費

	// ⭐ 追蹤當前交易輪次數據（用於打平機制）
	openPositionValueD := decimal.Zero       // 累計持倉總價值（USDT）
	currentRoundRealizedPnLD := decimal.Zero // 當前輪次已實現盈虧（扣除手續費）
	currentRoundClosedValueD := decimal.Zero // 當前輪次累積關倉價值（本金 + 盈虧）⭐
	totalRealizedPnLD := decimal.Zero        // 累計已實現盈虧（從回測開始的所有已實現盈虧總和）⭐

	// ⭐ 追蹤持倉全滿天數（定義：可用餘額 < 單次開倉成本）
	fullPositionDays := make(map[string]bool) // 記錄哪些天達到持倉全滿（key: YYYY-MM-DD）
	maxOpenPositionValueD := decimal.Zero     // 追蹤最大持倉價值（USDT）⭐

	// 記錄初始資金
	e.calculator.RecordBalance(candles[0].Timestamp(), balanceD.InexactFloat64())

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
			// ⭐ 檢查是否觸及目標平倉價格（使用 High 價格）
			if currentCandle.High().Value() >= pos.TargetClosePrice {
				// ⭐ 使用提取的辅助函数执行平仓（使用止盈價）
				closeResult, err := e.executeClose(
					pos,
					pos.TargetClosePrice, // ⭐ 修正：使用止盈價而不是收盤價
					currentTime,
					avgCostAtThisTime,
				)
				if err != nil {
					// 平倉失敗，記錄錯誤但繼續
					continue
				}

				// ⭐ 使用 decimal 累加（避免浮點精度問題）
				balanceD = balanceD.Add(closeResult.Revenue)
				totalProfitGrossD = totalProfitGrossD.Add(closeResult.ProfitGross)
				totalProfitGross_EntryD = totalProfitGross_EntryD.Add(closeResult.ProfitGross_Entry)
				totalFeesCloseD = totalFeesCloseD.Add(closeResult.CloseFee)
				openPositionValueD = openPositionValueD.Sub(closeResult.PositionSize)
				currentRoundRealizedPnLD = currentRoundRealizedPnLD.Add(closeResult.RealizedPnL)
				currentRoundClosedValueD = currentRoundClosedValueD.Add(closeResult.ClosedValue)
				totalRealizedPnLD = totalRealizedPnLD.Add(closeResult.RealizedPnL)

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balanceD.InexactFloat64())

				// 記錄交易日誌
				tradeCounter++
				reason := fmt.Sprintf("hit_target_%.2f", pos.TargetClosePrice)
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:                 tradeCounter,
					Time:                    currentTime,
					Action:                  "CLOSE",
					Price:                   closeResult.ClosePrice,
					PositionSize:            closeResult.ClosedValue.InexactFloat64(),
					Balance:                 balanceD.InexactFloat64(),
					OpenPositionValue:       openPositionValueD.InexactFloat64(),
					PnLPercent:              closeResult.PnLPercent,
					PnL:                     closeResult.PnL,
					AvgCost:                 avgCostAtThisTime,
					PnLPercent_Avg:          closeResult.PnLPercent_Avg,
					PnL_Avg:                 closeResult.PnL_Avg,
					Fee:                     closeResult.CloseFee.InexactFloat64(),
					RoundClosedValue:        currentRoundClosedValueD.InexactFloat64(),
					CurrentRoundRealizedPnL: currentRoundRealizedPnLD.InexactFloat64(),
					TotalRealizedPnL:        totalRealizedPnLD.InexactFloat64(),
					UnrealizedPnL:           e.positionTracker.CalculateUnrealizedPnL(pos.TargetClosePrice, e.config.FeeRate),
					Reason:                  reason,
					PositionID:              pos.ID,
				})

				// ⭐ 更新正常關倉計數
				e.currentRoundStats.NormalCloseCount++
				e.currentRoundStats.TotalFeesInRound += closeResult.CloseFee.InexactFloat64()

				// ⭐ 檢查是否所有倉位被關閉（交易輪次結束）
				if openPositionValueD.LessThanOrEqual(decimal.NewFromFloat(0.01)) {
					openPositionValueD = decimal.Zero
					currentRoundRealizedPnLD = decimal.Zero // 重置，開始新的交易輪次
					currentRoundClosedValueD = decimal.Zero // 重置關倉價值⭐
				}
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
			currentRoundRealizedPnLD.InexactFloat64(), // ⭐ 傳入當前輪次已實現盈虧
			currentRoundClosedValueD.InexactFloat64(), // ⭐ 傳入當前輪次累積關倉價值
			unrealizedPnL,                             // ⭐ 傳入外部計算的未實現盈虧
		)

		// 獲取開倉建議（grid.OpenAdvice）⭐ 傳入倉位摘要和當前K線
		gridAdvice := e.strategy.GetOpenAdvice(currentPrice, currentCandle, lastCandle, histories, positionSummary)

		// ========== 步驟 2.8: 檢查是否觸發打平機制 ⭐ ==========
		// 即使不應該開倉，也要檢查是否因為打平退出
		if !gridAdvice.ShouldOpen && len(gridAdvice.Reason) >= 16 &&
			gridAdvice.Reason[:16] == "break_even_exit:" {
			// ⭐ 觸發打平機制：平掉所有未平倉位
			// ⭐ 重要：先複製倉位列表，避免在循環中修改導致跳過某些倉位
			positionsToClose := make([]simulator.Position, len(e.positionTracker.GetOpenPositions()))
			copy(positionsToClose, e.positionTracker.GetOpenPositions())

			// ⭐ 記錄本輪打平前的狀態
			beforeCloseRealizedPnL := currentRoundRealizedPnLD.InexactFloat64()
			beforeCloseUnrealizedPnL := unrealizedPnL

			for _, pos := range positionsToClose {
				// ⭐ 使用提取的辅助函数执行平仓
				closeResult, err := e.executeClose(
					pos,
					currentPrice.Value(),
					currentTime,
					avgCostAtThisTime,
				)
				if err != nil {
					continue
				}

				// ⭐ 使用 decimal 累加（避免浮點精度問題）
				balanceD = balanceD.Add(closeResult.Revenue)
				totalProfitGrossD = totalProfitGrossD.Add(closeResult.ProfitGross)
				totalProfitGross_EntryD = totalProfitGross_EntryD.Add(closeResult.ProfitGross_Entry)
				totalFeesCloseD = totalFeesCloseD.Add(closeResult.CloseFee)
				openPositionValueD = openPositionValueD.Sub(closeResult.PositionSize)
				currentRoundRealizedPnLD = currentRoundRealizedPnLD.Add(closeResult.RealizedPnL)
				currentRoundClosedValueD = currentRoundClosedValueD.Add(closeResult.ClosedValue)
				totalRealizedPnLD = totalRealizedPnLD.Add(closeResult.RealizedPnL)

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balanceD.InexactFloat64())

				// 記錄交易日誌
				tradeCounter++
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:                 tradeCounter,
					Time:                    currentTime,
					Action:                  "CLOSE",
					Price:                   closeResult.ClosePrice,
					PositionSize:            closeResult.ClosedValue.InexactFloat64(),
					Balance:                 balanceD.InexactFloat64(),
					OpenPositionValue:       openPositionValueD.InexactFloat64(),
					PnLPercent:              closeResult.PnLPercent,
					PnL:                     closeResult.PnL,
					AvgCost:                 avgCostAtThisTime,
					PnLPercent_Avg:          closeResult.PnLPercent_Avg,
					PnL_Avg:                 closeResult.PnL_Avg,
					Fee:                     closeResult.CloseFee.InexactFloat64(),
					RoundClosedValue:        currentRoundClosedValueD.InexactFloat64(),
					CurrentRoundRealizedPnL: currentRoundRealizedPnLD.InexactFloat64(),
					TotalRealizedPnL:        totalRealizedPnLD.InexactFloat64(),
					UnrealizedPnL:           e.positionTracker.CalculateUnrealizedPnL(currentPrice.Value(), e.config.FeeRate),
					Reason:                  gridAdvice.Reason,
					PositionID:              pos.ID,
				})

				// ⭐ 打平机制特有：更新当前轮次统计
				e.currentRoundStats.BreakEvenCloseCount++
				e.currentRoundStats.TotalFeesInRound += closeResult.CloseFee.InexactFloat64()

				// ⭐ 檢查是否所有倉位被關閉（交易輪次結束）
				if openPositionValueD.LessThanOrEqual(decimal.NewFromFloat(0.01)) {
					openPositionValueD = decimal.Zero

					// ⭐ 記錄打平輪次（在重置前）
					if e.currentRoundStats.StartTime.IsZero() {
						e.currentRoundStats.StartTime = currentTime // 首次設置開始時間
					}

					round := BreakEvenRound{
						RoundID:             e.currentRoundStats.RoundID,
						StartTime:           e.currentRoundStats.StartTime,
						EndTime:             currentTime,
						Duration:            currentTime.Sub(e.currentRoundStats.StartTime).String(),
						TotalOpenCount:      e.currentRoundStats.OpenCount,
						NormalCloseCount:    e.currentRoundStats.NormalCloseCount,     // 正常止盈關倉數 ⭐
						BreakEvenCloseCount: e.currentRoundStats.BreakEvenCloseCount,  // 打平強制關倉數 ⭐
						TotalCloseCount:     e.currentRoundStats.GetTotalCloseCount(), // 總關倉數 ⭐
						RealizedPnL:         beforeCloseRealizedPnL,                   // 打平前的已實現盈虧
						UnrealizedPnL:       beforeCloseUnrealizedPnL,                 // 打平前的未實現盈虧
						ExpectedProfit:      beforeCloseRealizedPnL + beforeCloseUnrealizedPnL,
						TotalFees:           e.currentRoundStats.TotalFeesInRound,
						TriggerPrice:        currentPrice.Value(),
						AvgCost:             avgCostAtThisTime,
					}
					e.breakEvenRounds = append(e.breakEvenRounds, round)

					// ⭐ 打平退出時回收注資（如果有待回收的注資）
					if e.pendingFunding > 0 {
						recoveryAmountD := decimal.NewFromFloat(e.pendingFunding)
						balanceD = balanceD.Sub(recoveryAmountD) // 扣除注資金額（相當於取回）

						// 更新注資記錄狀態
						for i := len(e.fundingHistory) - 1; i >= 0; i-- {
							if !e.fundingHistory[i].Recovered {
								e.fundingHistory[i].Recovered = true
								e.fundingHistory[i].RecoveredAt = currentTime
							}
						}

						// 清空待回收注資
						e.pendingFunding = 0

						// 記錄資金快照（重要：讓計算器知道資金減少了）
						e.calculator.RecordBalance(currentTime, balanceD.InexactFloat64())
					}

					// 重置輪次數據
					currentRoundRealizedPnLD = decimal.Zero // 重置，開始新的交易輪次
					currentRoundClosedValueD = decimal.Zero // 重置關倉價值⭐
					e.currentRoundStats = RoundStats{
						RoundID:   e.currentRoundStats.RoundID + 1,
						StartTime: time.Time{}, // 重置，下次開倉時會設置
					}
				}
			}
		}

		// ========== 步驟 3: 如果建議開倉，模擬開倉 ==========
		if gridAdvice.ShouldOpen {
			// 檢查餘額是否充足
			estimatedCostD := decimal.NewFromFloat(gridAdvice.PositionSize).Mul(decimal.NewFromFloat(1 + e.config.FeeRate))

			if balanceD.GreaterThanOrEqual(estimatedCostD) {
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
				position, cost, err := e.simulator.SimulateOpen(advice, balanceD.InexactFloat64(), currentTime)
				if err != nil {
					// 開倉失敗，跳過
					continue
				}

				// 計算開倉手續費（使用 decimal）
				openFeeD := decimal.NewFromFloat(position.Size).Mul(decimal.NewFromFloat(e.config.FeeRate))

				// 更新倉位追蹤器
				newPosition := e.positionTracker.AddPosition(
					position.EntryPrice,
					position.Size,
					position.OpenTime,
					position.TargetClosePrice,
				)

				// 更新餘額（使用 decimal）
				costD := decimal.NewFromFloat(cost)
				balanceD = balanceD.Sub(costD)

				// ⭐ 累加統計數據（使用 decimal）
				totalOpenedTrades++                    // 累加開倉數量
				totalFeesOpenD = totalFeesOpenD.Add(openFeeD) // 累加開倉手續費

				// ⭐ 更新當前交易輪次數據（使用 decimal）
				positionSizeD := decimal.NewFromFloat(position.Size)
				openPositionValueD = openPositionValueD.Add(positionSizeD) // 增加累計持倉價值

				// ⭐ 更新當前輪次統計
				if e.currentRoundStats.StartTime.IsZero() {
					e.currentRoundStats.StartTime = currentTime // 首次開倉，設置開始時間
				}
				e.currentRoundStats.OpenCount++
				e.currentRoundStats.TotalFeesInRound += openFeeD.InexactFloat64()

				// ⭐ 重置閒置計數器（成功開倉後）
				e.idleCandles = 0

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balanceD.InexactFloat64())

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
					Balance:                 balanceD.InexactFloat64(),
					OpenPositionValue:       openPositionValueD.InexactFloat64(), // ⭐ 開倉後的累計持倉價值
					AvgCost:                 avgCostAfterOpen,                    // ⭐ 開倉後的平均成本
					PnL:                     0,
					Fee:                     openFeeD.InexactFloat64(),                                                        // ⭐ 記錄開倉手續費
					RoundClosedValue:        currentRoundClosedValueD.InexactFloat64(),                                        // ⭐ 本輪累積關倉總價值
					CurrentRoundRealizedPnL: currentRoundRealizedPnLD.InexactFloat64(),                                        // ⭐ 本輪已實現盈虧
					TotalRealizedPnL:        totalRealizedPnLD.InexactFloat64(),                                               // ⭐ 累計已實現盈虧
					UnrealizedPnL:           e.positionTracker.CalculateUnrealizedPnL(currentPrice.Value(), e.config.FeeRate), // ⭐ 統一使用 PositionTracker
					Reason:                  gridAdvice.Reason,
					PositionID:              newPosition.ID, // ⭐ 記錄倉位ID
				})
			}
		}

		// ⭐ 檢查當前是否為持倉全滿狀態（每根 K 線結束時檢查）
		if openPositionValueD.GreaterThan(maxOpenPositionValueD) {
			maxOpenPositionValueD = openPositionValueD // 更新最大持倉價值 ⭐
		}

		if balanceD.LessThan(decimal.NewFromFloat(e.config.PositionSize)) {
			// 可用餘額不足以開下一個倉位 = 持倉全滿
			dateKey := currentTime.Format("2006-01-02") // YYYY-MM-DD
			fullPositionDays[dateKey] = true
		}

		// ⭐ 自動注資機制檢查（每根K線結束時）
		if e.config.EnableAutoFunding {
			// 增加閒置計數（無論是否開倉）
			e.idleCandles++

			// 檢查是否達到注資閾值
			if e.idleCandles >= e.config.AutoFundingIdle {
				// 記錄注資前狀態
				balanceBefore := balanceD.InexactFloat64()

				// 執行注資（使用 decimal）
				fundingAmountD := decimal.NewFromFloat(e.config.AutoFundingAmount)
				balanceD = balanceD.Add(fundingAmountD)

				// ⭐ 增加待回收注資金額
				e.pendingFunding += e.config.AutoFundingAmount

				// ⭐⭐ 更新最大待回收注資峰值
				if e.pendingFunding > e.maxPendingFunding {
					e.maxPendingFunding = e.pendingFunding
				}

				// 記錄注資事件
				fundingRecord := FundingRecord{
					Time:          currentTime,
					Amount:        e.config.AutoFundingAmount,
					IdleCandles:   e.idleCandles,
					BalanceBefore: balanceBefore,
					BalanceAfter:  balanceD.InexactFloat64(),
					Price:         currentPrice.Value(),
					CandleIndex:   i,
					Recovered:     false, // 初始未回收 ⭐
				}
				e.fundingHistory = append(e.fundingHistory, fundingRecord)

				// 重置閒置計數器
				e.idleCandles = 0

				// 記錄資金快照（重要：讓計算器知道資金增加了）
				e.calculator.RecordBalance(currentTime, balanceD.InexactFloat64())
			}
		}
	}

	// ========== 步驟 4: 回測結束，強制平倉所有未平倉位 ⭐ ==========
	lastCandle := candles[len(candles)-1]
	lastPrice := lastCandle.Close().Value()
	lastTime := lastCandle.Timestamp()

	// 強制平倉所有未平倉位（使用最後收盤價）
	// finalAvgCost := e.positionTracker.CalculateAverageCost()
	// finalPositionsToClose := make([]simulator.Position, len(e.positionTracker.GetOpenPositions()))
	// copy(finalPositionsToClose, e.positionTracker.GetOpenPositions())

	// for _, pos := range finalPositionsToClose {
	// 	err := e.executeClose(
	// 		pos,
	// 		lastPrice, // 使用最後收盤價平倉
	// 		lastTime,
	// 		finalAvgCost,
	// 		"backtest_end_force_close", // 回測結束強制平倉
	// 		&balance,
	// 		&totalProfitGross,
	// 		&totalProfitGross_Entry,
	// 		&totalFeesClose,
	// 		&openPositionValue,
	// 		&currentRoundRealizedPnL,
	// 		&currentRoundClosedValue,
	// 		&totalRealizedPnL,
	// 		&tradeCounter,
	// 	)
	// 	if err != nil {
	// 		continue
	// 	}
	// 	// 標記為強制平倉（不計入正常止盈）
	// 	e.currentRoundStats.BreakEvenCloseCount++
	// }

	// 記錄最終資金快照
	e.calculator.RecordBalance(lastTime, balanceD.InexactFloat64())

	// ========== 步驟 5: 計算回測指標（包含未實現盈虧）==========
	result := e.calculator.Calculate(
		e.positionTracker,
		balanceD.InexactFloat64(),
		lastPrice,
		totalOpenedTrades,
		totalProfitGrossD.InexactFloat64(),
		totalProfitGross_EntryD.InexactFloat64(), // ⭐ 新增：基于单笔开仓价的总利润
		totalFeesOpenD.InexactFloat64(),
		totalFeesCloseD.InexactFloat64(),
	)

	// ⭐ 加入持倉全滿天數統計
	result.FullPositionDays = len(fullPositionDays)
	result.MaxOpenPositionValue = maxOpenPositionValueD.InexactFloat64() // ⭐ 加入最大持倉價值

	// ⭐ 輸出打平輪次統計報告
	e.printBreakEvenRoundsReport()

	// ⭐ 輸出自動注資統計報告
	e.printFundingReport()

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

// ExportRoundsToCSV 導出打平輪次詳細記錄到 CSV 文件 ⭐
func (e *BacktestEngine) ExportRoundsToCSV(filePath string) error {
	if len(e.breakEvenRounds) == 0 {
		return nil // 沒有輪次記錄，跳過
	}

	// 創建文件
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 寫入 CSV 標題
	header := []string{
		"RoundID",
		"StartTime",
		"EndTime",
		"Duration",
		"TotalOpenCount",
		"NormalCloseCount",
		"BreakEvenCloseCount",
		"TotalCloseCount",
		"RealizedPnL",
		"UnrealizedPnL",
		"ExpectedProfit",
		"TotalFees",
		"TriggerPrice",
		"AvgCost",
		"Status",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// 寫入每輪數據
	for _, round := range e.breakEvenRounds {
		status := "profit"
		if round.ExpectedProfit < 0 {
			status = "loss"
		}

		row := []string{
			fmt.Sprintf("%d", round.RoundID),
			round.StartTime.Format("2006-01-02 15:04:05"),
			round.EndTime.Format("2006-01-02 15:04:05"),
			round.Duration,
			fmt.Sprintf("%d", round.TotalOpenCount),
			fmt.Sprintf("%d", round.NormalCloseCount),
			fmt.Sprintf("%d", round.BreakEvenCloseCount),
			fmt.Sprintf("%d", round.TotalCloseCount),
			fmt.Sprintf("%.2f", round.RealizedPnL),
			fmt.Sprintf("%.2f", round.UnrealizedPnL),
			fmt.Sprintf("%.2f", round.ExpectedProfit),
			fmt.Sprintf("%.2f", round.TotalFees),
			fmt.Sprintf("%.2f", round.TriggerPrice),
			fmt.Sprintf("%.2f", round.AvgCost),
			status,
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
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

	for _, round := range e.breakEvenRounds {
		totalProfit += round.ExpectedProfit
		totalFees += round.TotalFees
		totalTrades += round.TotalOpenCount + round.TotalCloseCount
		releasePosition := float64(round.BreakEvenCloseCount) * e.config.PositionSize
		if releasePosition > maxReleasePosition {
			maxReleasePosition = releasePosition
		}
	}

	// 彙總統計
	fmt.Println("----------------------------------------")
	fmt.Println("📊 彙總統計")
	fmt.Println("----------------------------------------")
	fmt.Printf("總輪次數: %d\n", len(e.breakEvenRounds))
	fmt.Printf("詳細內容看報告")

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
	fmt.Println("========================================")
	fmt.Println()
}

// printFundingReport 輸出自動注資統計報告 ⭐
func (e *BacktestEngine) printFundingReport() {
	if !e.config.EnableAutoFunding {
		return // 未啟用自動注資，不輸出報告
	}

	if len(e.fundingHistory) == 0 {
		fmt.Println("\n========================================")
		fmt.Println("💰 自動注資統計")
		fmt.Println("========================================")
		fmt.Println("本次回測未觸發自動注資機制")
		fmt.Printf("閒置閾值設定: %d 根K線\n", e.config.AutoFundingIdle)
		fmt.Printf("注資金額設定: %.2f USDT\n", e.config.AutoFundingAmount)
		fmt.Println("========================================")
		fmt.Println()
		return
	}

	fmt.Println("\n========================================")
	fmt.Println("💰 自動注資統計")
	fmt.Println("========================================")
	fmt.Printf("總注資次數: %d 次\n", len(e.fundingHistory))
	fmt.Printf("閒置閾值: %d 根K線 (約 %.1f 天)\n",
		e.config.AutoFundingIdle,
		float64(e.config.AutoFundingIdle)*5/60/24) // 5分鐘K線換算天數
	fmt.Printf("單次注資金額: %.2f USDT\n\n", e.config.AutoFundingAmount)

	// 計算總注資金額和已回收金額 ⭐
	totalFunding := 0.0
	totalRecovered := 0.0
	recoveredCount := 0
	for _, record := range e.fundingHistory {
		totalFunding += record.Amount
		if record.Recovered {
			totalRecovered += record.Amount
			recoveredCount++
		}
	}
	netFunding := totalFunding - totalRecovered // 淨注資金額（未回收的）

	// 標準輸出只顯示簡要信息（詳細記錄見 report.md）
	fmt.Println("----------------------------------------")
	fmt.Println("📋 注資記錄")
	fmt.Println("----------------------------------------")
	fmt.Printf("總計 %d 次注資（詳細記錄請查看 report.md）\n\n", len(e.fundingHistory))

	// 彙總統計
	fmt.Println("----------------------------------------")
	fmt.Println("📊 注資彙總")
	fmt.Println("----------------------------------------")
	fmt.Printf("總注資次數: %d 次\n", len(e.fundingHistory))
	fmt.Printf("總注資金額: $%.2f USDT ⭐\n", totalFunding)
	fmt.Printf("已回收次數: %d 次 ✅\n", recoveredCount)
	fmt.Printf("已回收金額: $%.2f USDT ✅\n", totalRecovered)
	fmt.Printf("淨注資金額: $%.2f USDT 💰 (最終未回收)\n", netFunding)
	fmt.Printf("最大注資峰值: $%.2f USDT 🔥 (最壞情況需準備的額外資金)\n", e.maxPendingFunding)
	fmt.Printf("回收率: %.1f%% ⭐\n", (totalRecovered/totalFunding)*100)
	fmt.Printf("平均注資間隔: %.1f 根K線 (約 %.1f 天)\n",
		float64(e.fundingHistory[len(e.fundingHistory)-1].CandleIndex)/float64(len(e.fundingHistory)),
		float64(e.fundingHistory[len(e.fundingHistory)-1].CandleIndex)*5/60/24/float64(len(e.fundingHistory)))

	// 如果有注資，計算對最終結果的影響
	fmt.Printf("\n💡 注資影響分析:\n")
	fmt.Printf("   初始資金: $%.2f\n", e.config.InitialBalance)
	fmt.Printf("   累積注資: $%.2f (投入 %d 次)\n", totalFunding, len(e.fundingHistory))
	fmt.Printf("   已回收: $%.2f (回收 %d 次) ✅\n", totalRecovered, recoveredCount)
	fmt.Printf("   最終未回收: $%.2f 💰\n", netFunding)
	fmt.Printf("   最大峰值: $%.2f 🔥\n", e.maxPendingFunding)
	fmt.Printf("\n   📌 結論:\n")
	fmt.Printf("      - 最壞情況需準備: $%.2f (初始 + 最大峰值)\n", e.config.InitialBalance+e.maxPendingFunding)
	fmt.Printf("      - 回測結束時佔用: $%.2f (初始 + 最終未回收)\n", e.config.InitialBalance+netFunding)
	fmt.Println("========================================")
	fmt.Println()
}

// GenerateBreakEvenReportMarkdown 生成打平輪次報告的 Markdown 內容 ⭐
// 返回值：Markdown 格式的打平輪次報告字符串，可以附加到完整報告中
func (e *BacktestEngine) GenerateBreakEvenReportMarkdown() string {
	if len(e.breakEvenRounds) == 0 {
		return "" // 沒有打平輪次，返回空字符串
	}

	// 統計數據
	totalProfit := 0.0
	totalFees := 0.0
	maxReleasePosition := 0.0

	for _, round := range e.breakEvenRounds {
		totalProfit += round.ExpectedProfit
		totalFees += round.TotalFees
		releasePosition := float64(round.BreakEvenCloseCount) * e.config.PositionSize
		if releasePosition > maxReleasePosition {
			maxReleasePosition = releasePosition
		}
	}

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

	// 構建 Markdown 內容
	var content string
	content += "## ⭐ 打平輪次統計\n\n"
	content += fmt.Sprintf("- **總輪次數**: %d\n", len(e.breakEvenRounds))
	content += fmt.Sprintf("- **平均每輪盈利**: %.2f USDT\n", totalProfit/float64(len(e.breakEvenRounds)))
	content += fmt.Sprintf("- **平均每輪手續費**: %.2f USDT\n", totalFees/float64(len(e.breakEvenRounds)))
	content += fmt.Sprintf("- **最大釋放倉位量**: %.2f USDT\n", maxReleasePosition)
	content += fmt.Sprintf("- **觸發平攤總盈虧**: %.2f USDT\n", totalProfit)
	content += fmt.Sprintf("- **盈利輪次**: %d (%.1f%%)\n", profitRounds, float64(profitRounds)/float64(len(e.breakEvenRounds))*100)
	content += fmt.Sprintf("- **虧損輪次**: %d (%.1f%%)\n\n", lossRounds, float64(lossRounds)/float64(len(e.breakEvenRounds))*100)

	// 詳細輪次記錄說明
	content += "### 詳細輪次記錄\n\n"
	content += fmt.Sprintf("⭐ **輪次詳細記錄已導出到 CSV 文件**（共 %d 輪）\n", len(e.breakEvenRounds))
	content += "- 文件名: `rounds_detail.csv`\n"
	content += "- 包含字段: 輪次編號、時間、開關倉數、盈虧、手續費等\n"
	content += "- 可使用 Excel/Numbers 打開查看和分析\n\n"

	return content
}

// GenerateFundingReportMarkdown 生成自動注資報告的 Markdown 內容 ⭐
// 返回值：Markdown 格式的注資報告字符串，可以附加到完整報告中
func (e *BacktestEngine) GenerateFundingReportMarkdown() string {
	if !e.config.EnableAutoFunding || len(e.fundingHistory) == 0 {
		return "" // 沒有注資記錄，返回空字符串
	}

	// 計算統計數據
	totalFunding := 0.0
	totalRecovered := 0.0
	recoveredCount := 0
	for _, record := range e.fundingHistory {
		totalFunding += record.Amount
		if record.Recovered {
			totalRecovered += record.Amount
			recoveredCount++
		}
	}
	netFunding := totalFunding - totalRecovered

	// 構建 Markdown 內容
	var content string
	content += "## 💰 自動注資統計\n\n"
	content += fmt.Sprintf("- **總注資次數**: %d 次\n", len(e.fundingHistory))
	content += fmt.Sprintf("- **總注資金額**: $%.2f USDT\n", totalFunding)
	content += fmt.Sprintf("- **已回收次數**: %d 次 ✅\n", recoveredCount)
	content += fmt.Sprintf("- **已回收金額**: $%.2f USDT\n", totalRecovered)
	content += fmt.Sprintf("- **淨注資金額**: $%.2f USDT 💰 (最終未回收)\n", netFunding)
	content += fmt.Sprintf("- **最大注資峰值**: $%.2f USDT 🔥\n", e.maxPendingFunding)
	content += fmt.Sprintf("- **回收率**: %.1f%%\n", (totalRecovered/totalFunding)*100)
	content += fmt.Sprintf("- **自動注資閒置閾值**: %d 根K線 (約 %.1f 天)\n",
		e.config.AutoFundingIdle,
		float64(e.config.AutoFundingIdle)*5/60/24)
	content += fmt.Sprintf("- **單次注資金額**: $%.2f USDT\n\n", e.config.AutoFundingAmount)

	// 注資影響分析
	content += "### 注資影響分析\n\n"
	content += fmt.Sprintf("- **初始資金**: $%.2f\n", e.config.InitialBalance)
	content += fmt.Sprintf("- **累積注資**: $%.2f (投入 %d 次)\n", totalFunding, len(e.fundingHistory))
	content += fmt.Sprintf("- **已回收**: $%.2f (回收 %d 次)\n", totalRecovered, recoveredCount)
	content += fmt.Sprintf("- **最終未回收**: $%.2f\n", netFunding)
	content += fmt.Sprintf("- **最大峰值**: $%.2f\n\n", e.maxPendingFunding)

	content += "**結論**:\n\n"
	content += fmt.Sprintf("- 最壞情況需準備: $%.2f (初始 + 最大峰值)\n",
		e.config.InitialBalance+e.maxPendingFunding)
	content += fmt.Sprintf("- 回測結束時佔用: $%.2f (初始 + 最終未回收)\n\n",
		e.config.InitialBalance+netFunding)

	// 詳細注資記錄
	content += "### 詳細注資記錄\n\n"
	content += "| # | 注資時間 | 回收時間 | K線索引 | 閒置時長 (K線) | 閒置天數 | 當時價格 | 注資前餘額 | 注資後餘額 | 注資金額 | 狀態 |\n"
	content += "|---|---------|---------|---------|--------------|---------|---------|-----------|-----------|---------|------|\n"

	for i, record := range e.fundingHistory {
		idleDays := float64(record.IdleCandles) * 5 / 60 / 24
		status := "⏳ 未回收"
		recoveredTime := "-"
		if record.Recovered {
			status = "✅ 已回收"
			recoveredTime = record.RecoveredAt.Format("2006-01-02 15:04")
		}

		content += fmt.Sprintf("| %d | %s | %s | %d | %d | %.1f | $%.2f | $%.2f | $%.2f | $%.2f | %s |\n",
			i+1,
			record.Time.Format("2006-01-02 15:04"),
			recoveredTime,
			record.CandleIndex,
			record.IdleCandles,
			idleDays,
			record.Price,
			record.BalanceBefore,
			record.BalanceAfter,
			record.Amount,
			status,
		)
	}

	content += "\n"
	return content
}
