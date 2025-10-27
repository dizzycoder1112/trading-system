package engine

import (
	"fmt"

	"dizzycode.xyz/shared/domain/value_objects"
	"dizzycode.xyz/trading-strategy-server/backtesting/loader"
	"dizzycode.xyz/trading-strategy-server/backtesting/metrics"
	"dizzycode.xyz/trading-strategy-server/backtesting/simulator"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/strategies/grid"
)

// BacktestConfig 回測配置
type BacktestConfig struct {
	InitialBalance float64 // 初始資金
	FeeRate        float64 // 手續費率（默認: 0.0005 = 0.05%）
	Slippage       float64 // 滑點（默認: 0）
	InstID         string  // 交易對 (e.g., "ETH-USDT-SWAP")
	TakeProfitMin  float64 // 最小停利百分比
	TakeProfitMax  float64 // 最大停利百分比
}

// BacktestEngine 回測引擎核心
type BacktestEngine struct {
	strategy        *grid.GridAggregate        // 真實的 Grid 策略 ⭐
	simulator       *simulator.OrderSimulator  // 成交模擬器
	positionTracker *simulator.PositionTracker // 倉位追蹤器
	calculator      *metrics.MetricsCalculator // 指標計算器
	config          BacktestConfig             // 配置
}

// NewBacktestEngine 創建回測引擎
func NewBacktestEngine(config BacktestConfig) (*BacktestEngine, error) {
	// 1. 創建真實的 Grid 策略 ⭐
	strategy, err := grid.NewGridAggregate(grid.GridConfig{
		InstID:        config.InstID,
		PositionSize:  200, // 固定單次開倉大小為 200 美元
		TakeProfitMin: config.TakeProfitMin,
		TakeProfitMax: config.TakeProfitMax,
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

	// 記錄初始資金
	e.calculator.RecordBalance(candles[0].Timestamp(), balance)

	// 遍歷所有K線
	for i := 0; i < len(candles); i++ {
		currentCandle := candles[i]
		currentPrice := currentCandle.Close()
		currentTime := currentCandle.Timestamp()

		// ========== 步驟 1: 檢查是否需要平倉 ==========
		// 注意：先檢查平倉，再考慮開倉（避免資金不足）
		for _, pos := range e.positionTracker.GetOpenPositions() {
			// 檢查是否觸及目標平倉價格
			if currentPrice.Value() >= pos.TargetClosePrice {
				// 模擬平倉
				closedPos, revenue, err := e.simulator.SimulateClose(pos, currentPrice.Value(), currentTime)
				if err != nil {
					// 平倉失敗，記錄錯誤但繼續
					continue
				}

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

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)
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

		// 獲取開倉建議（grid.OpenAdvice）
		gridAdvice := e.strategy.GetOpenAdvice(currentPrice, lastCandle, histories)

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
					TakeProfit:   gridAdvice.TakeProfit,
					Reason:       gridAdvice.Reason,
				}

				// 模擬開倉
				position, cost, err := e.simulator.SimulateOpen(advice, balance, currentTime)
				if err != nil {
					// 開倉失敗，跳過
					continue
				}

				// 更新倉位追蹤器
				e.positionTracker.AddPosition(
					position.EntryPrice,
					position.Size,
					position.OpenTime,
					position.TargetClosePrice,
				)

				// 更新餘額
				balance -= cost

				// 記錄資金快照
				e.calculator.RecordBalance(currentTime, balance)
			}
		}
	}

	// ========== 步驟 4: 強制平倉所有未平倉 ==========
	if e.positionTracker.HasOpenPositions() {
		lastCandle := candles[len(candles)-1]
		lastPrice := lastCandle.Close()
		lastTime := lastCandle.Timestamp()

		// 平倉所有持倉
		for _, pos := range e.positionTracker.GetOpenPositions() {
			closedPos, revenue, err := e.simulator.SimulateClose(pos, lastPrice.Value(), lastTime)
			if err != nil {
				continue
			}

			e.positionTracker.ClosePosition(
				pos.ID,
				closedPos.ClosePrice,
				closedPos.CloseTime,
				closedPos.RealizedPnL,
			)

			balance += revenue
		}

		// 記錄最終資金
		e.calculator.RecordBalance(lastTime, balance)
	}

	// ========== 步驟 5: 計算回測指標 ==========
	result := e.calculator.Calculate(e.positionTracker, balance)

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
