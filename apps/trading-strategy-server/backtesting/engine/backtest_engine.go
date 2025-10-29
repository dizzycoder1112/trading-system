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
}

// BacktestEngine 回測引擎核心
type BacktestEngine struct {
	strategy        *grid.GridAggregate        // 真實的 Grid 策略 ⭐
	simulator       *simulator.OrderSimulator  // 成交模擬器
	positionTracker *simulator.PositionTracker // 倉位追蹤器
	calculator      *metrics.MetricsCalculator // 指標計算器
	config          BacktestConfig             // 配置
	tradeLog        []TradeLog                 // 交易日誌 ⭐ DEBUG
}

// TradeLog 交易日誌（用於 debug）
type TradeLog struct {
	TradeID            int       // 交易序號
	Time               time.Time // 時間
	Action             string    // OPEN / CLOSE
	Price              float64   // 價格
	PositionSize       float64   // 倉位大小
	Balance            float64   // 當前餘額
	OpenPositionValue  float64   // 累計持倉總價值（USDT）⭐ 新增
	PnLPercent         float64   // 盈虧百分比（基於單筆開倉價）⭐
	PnL                float64   // 盈虧金額（基於單筆開倉價，未扣手續費）⭐
	AvgCost            float64   // 平倉時的平均成本（所有未平倉的加權平均）⭐
	PnLPercent_Avg     float64   // 基於平均成本的盈虧百分比 ⭐
	PnL_Avg            float64   // 基於平均成本的盈虧金額（未扣手續費）⭐
	Fee                float64   // 手續費 ⭐
	Reason             string    // 原因
	PositionID         string    // 倉位ID（關聯開倉和平倉）⭐
}

// NewBacktestEngine 創建回測引擎
func NewBacktestEngine(config BacktestConfig) (*BacktestEngine, error) {
	// 1. 創建真實的 Grid 策略 ⭐
	strategy, err := grid.NewGridAggregate(grid.GridConfig{
		InstID:             config.InstID,
		PositionSize:       config.PositionSize,
		TakeProfitRateMin:  config.TakeProfitMin,
		TakeProfitRateMax:  config.TakeProfitMax,
		BreakEvenProfitMin: config.BreakEvenProfitMin, // ⭐ 從配置讀取
		BreakEvenProfitMax: config.BreakEvenProfitMax, // ⭐ 從配置讀取
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create grid strategy: %w", err)
	}

	// 2. 創建模擬器和追蹤器
	orderSimulator := simulator.NewOrderSimulator(config.FeeRate, config.Slippage)
	positionTracker := simulator.NewPositionTracker()
	calculator := metrics.NewMetricsCalculator(config.InitialBalance)

	return &BacktestEngine{
		strategy:        strategy,
		simulator:       orderSimulator,
		positionTracker: positionTracker,
		calculator:      calculator,
		config:          config,
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
	totalOpenedTrades := 0   // 總開倉數量
	totalProfitGross := 0.0  // 總利潤（未扣手續費）
	totalFeesOpen := 0.0     // 開倉總手續費
	totalFeesClose := 0.0    // 關倉總手續費

	// ⭐ 追蹤當前交易輪次數據（用於打平機制）
	openPositionValue := 0.0          // 累計持倉總價值（USDT）
	currentRoundRealizedPnL := 0.0    // 當前輪次已實現盈虧（未扣手續費）

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
			// 檢查是否觸及目標平倉價格
			if currentPrice.Value() >= pos.TargetClosePrice {

				// 模擬平倉
				closedPos, revenue, err := e.simulator.SimulateClose(pos, currentPrice.Value(), currentTime)
				if err != nil {
					// 平倉失敗，記錄錯誤但繼續
					continue
				}

				// ⭐ 計算基於單筆開倉價的盈虧（原有邏輯）
				priceChange := closedPos.ClosePrice - pos.EntryPrice
				pnlPercent := (priceChange / pos.EntryPrice) * 100     // 百分比
				pnlAmount := pos.Size * (priceChange / pos.EntryPrice) // 金額（未扣手續費）

				// ⭐ 計算基於平均成本的盈虧（使用這個時刻的平均成本）
				priceChange_Avg := closedPos.ClosePrice - avgCostAtThisTime
				pnlPercent_Avg := 0.0
				pnlAmount_Avg := 0.0
				if avgCostAtThisTime > 0 {
					pnlPercent_Avg = (priceChange_Avg / avgCostAtThisTime) * 100
					pnlAmount_Avg = pos.Size * (priceChange_Avg / avgCostAtThisTime)
				}

				// ⭐ 計算平倉時的實際價值和手續費
				closeValue := pos.Size + pnlAmount        // 平倉時的總價值（本金 + 盈虧）
				closeFee := closeValue * e.config.FeeRate // 平倉手續費基於總價值

				// 更新倉位追蹤器
				err = e.positionTracker.ClosePosition(
					pos.ID,
					closedPos.ClosePrice,
					closedPos.CloseTime,
					closedPos.RealizedPnL,
				)
				if err != nil {
					continue
				}

				// 更新餘額
				balance += revenue

				// ⭐ 累加統計數據
				totalProfitGross += pnlAmount  // 累加未扣費盈虧
				totalFeesClose += closeFee     // 累加關倉手續費

				// ⭐ 更新當前交易輪次數據
				openPositionValue -= pos.Size                // 減少累計持倉價值
				currentRoundRealizedPnL += pnlAmount         // 累加當前輪次已實現盈虧

				// ⭐ 檢查是否所有倉位被關閉（交易輪次結束）
				if openPositionValue <= 0.01 { // 使用小值避免浮點誤差
					openPositionValue = 0
					currentRoundRealizedPnL = 0 // 重置，開始新的交易輪次
				}

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)

				// ⭐ 記錄平倉日誌（使用這個時刻的平均成本，所有同時平倉的倉位都使用相同值）
				tradeCounter++
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:           tradeCounter,
					Time:              currentTime,
					Action:            "CLOSE",
					Price:             closedPos.ClosePrice,
					PositionSize:      closeValue,         // ⭐ 平倉時的實際收回金額（含盈虧）
					Balance:           balance,
					OpenPositionValue: openPositionValue,  // ⭐ 平倉後的累計持倉價值
					PnLPercent:        pnlPercent,         // ⭐ 基於單筆開倉價的盈虧百分比
					PnL:               pnlAmount,          // ⭐ 基於單筆開倉價的盈虧金額（未扣手續費）
					AvgCost:           avgCostAtThisTime,  // ⭐ 這個時刻的平均成本（平倉前的狀態）
					PnLPercent_Avg:    pnlPercent_Avg,    // ⭐ 基於平均成本的盈虧百分比
					PnL_Avg:           pnlAmount_Avg,     // ⭐ 基於平均成本的盈虧金額（未扣手續費）
					Fee:               closeFee,           // ⭐ 平倉手續費（基於實際價值）
					Reason:            fmt.Sprintf("hit_target_%.2f", pos.TargetClosePrice),
					PositionID:        pos.ID, // ⭐ 記錄倉位ID
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

		// 創建倉位摘要（包含當前輪次已實現盈虧）⭐
		positionSummary := value_objects.NewPositionSummary(
			openCount,
			totalSize,
			avgCost,
			totalFeesPaid,
			currentRoundRealizedPnL, // ⭐ 傳入當前輪次已實現盈虧
		)

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

			for _, pos := range positionsToClose {
				// 以當前價格平倉
				closedPos, revenue, err := e.simulator.SimulateClose(pos, currentPrice.Value(), currentTime)
				if err != nil {
					continue
				}

				// ⭐ 計算基於單筆開倉價的盈虧（原有邏輯）
				priceChange := closedPos.ClosePrice - pos.EntryPrice
				pnlPercent := (priceChange / pos.EntryPrice) * 100
				pnlAmount := pos.Size * (priceChange / pos.EntryPrice)

				// ⭐ 計算基於平均成本的盈虧（使用這個時刻的平均成本）
				priceChange_Avg := closedPos.ClosePrice - avgCostAtThisTime
				pnlPercent_Avg := 0.0
				pnlAmount_Avg := 0.0
				if avgCostAtThisTime > 0 {
					pnlPercent_Avg = (priceChange_Avg / avgCostAtThisTime) * 100
					pnlAmount_Avg = pos.Size * (priceChange_Avg / avgCostAtThisTime)
				}

				// ⭐ 計算平倉時的實際價值和手續費
				closeValue := pos.Size + pnlAmount
				closeFee := closeValue * e.config.FeeRate

				// 更新倉位追蹤器
				err = e.positionTracker.ClosePosition(
					pos.ID,
					closedPos.ClosePrice,
					closedPos.CloseTime,
					closedPos.RealizedPnL,
				)
				if err != nil {
					continue
				}

				// 更新餘額
				balance += revenue

				// ⭐ 累加統計數據
				totalProfitGross += pnlAmount
				totalFeesClose += closeFee

				// ⭐ 更新當前交易輪次數據
				openPositionValue -= pos.Size                // 減少累計持倉價值
				currentRoundRealizedPnL += pnlAmount         // 累加當前輪次已實現盈虧

				// ⭐ 檢查是否所有倉位被關閉（交易輪次結束）
				if openPositionValue <= 0.01 { // 使用小值避免浮點誤差
					openPositionValue = 0
					currentRoundRealizedPnL = 0 // 重置，開始新的交易輪次
				}

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)

				// ⭐ 記錄平倉日誌（使用這個時刻的平均成本）
				tradeCounter++
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:           tradeCounter,
					Time:              currentTime,
					Action:            "CLOSE",
					Price:             closedPos.ClosePrice,
					PositionSize:      closeValue,
					Balance:           balance,
					OpenPositionValue: openPositionValue,  // ⭐ 平倉後的累計持倉價值
					PnLPercent:        pnlPercent,         // ⭐ 基於單筆開倉價的盈虧百分比
					PnL:               pnlAmount,          // ⭐ 基於單筆開倉價的盈虧金額
					AvgCost:           avgCostAtThisTime,  // ⭐ 這個時刻的平均成本（打平前的狀態）
					PnLPercent_Avg:    pnlPercent_Avg,    // ⭐ 基於平均成本的盈虧百分比
					PnL_Avg:           pnlAmount_Avg,     // ⭐ 基於平均成本的盈虧金額
					Fee:               closeFee,
					Reason:            gridAdvice.Reason, // ⭐ 記錄打平退出原因
					PositionID:        pos.ID,
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
				totalOpenedTrades++         // 累加開倉數量
				totalFeesOpen += openFee    // 累加開倉手續費

				// ⭐ 更新當前交易輪次數據
				openPositionValue += position.Size  // 增加累計持倉價值

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)

				// ⭐ 計算開倉後的平均成本
				avgCostAfterOpen := e.positionTracker.CalculateAverageCost()

				// ⭐ 記錄開倉日誌
				tradeCounter++
				e.tradeLog = append(e.tradeLog, TradeLog{
					TradeID:           tradeCounter,
					Time:              currentTime,
					Action:            "OPEN",
					Price:             position.EntryPrice,
					PositionSize:      position.Size,
					Balance:           balance,
					OpenPositionValue: openPositionValue, // ⭐ 開倉後的累計持倉價值
					AvgCost:           avgCostAfterOpen,  // ⭐ 開倉後的平均成本
					PnL:               0,
					Fee:               openFee, // ⭐ 記錄開倉手續費
					Reason:            gridAdvice.Reason,
					PositionID:        newPosition.ID, // ⭐ 記錄倉位ID
				})
			}
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
	content := "TradeID,Time,Action,Price,PositionSize,Balance,OpenPositionValue,PnL%,PnL,AvgCost,PnL%_Avg,PnL_Avg,Fee,Reason,PositionID\n"

	for _, log := range e.tradeLog {
		line := fmt.Sprintf("%d,%s,%s,%.2f,%.2f,%.2f,%.2f,%.4f,%.2f,%.2f,%.4f,%.2f,%.8f,%s,%s\n",
			log.TradeID,
			log.Time.UTC().Format("2006-01-02 15:04:05"), // ⭐ 使用 UTC 時間（GMT+0）
			log.Action,
			log.Price,             // 價格：2位小數
			log.PositionSize,      // 倉位大小：2位小數
			log.Balance,           // 餘額：2位小數
			log.OpenPositionValue, // ⭐ 累計持倉總價值：2位小數
			log.PnLPercent,        // ⭐ 盈虧百分比（基於單筆）：4位小數
			log.PnL,               // ⭐ 盈虧金額（基於單筆）：2位小數
			log.AvgCost,           // ⭐ 平均成本：2位小數
			log.PnLPercent_Avg,    // ⭐ 盈虧百分比（基於平均）：4位小數
			log.PnL_Avg,           // ⭐ 盈虧金額（基於平均）：2位小數
			log.Fee,               // ⭐ 手續費：8位小數
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
