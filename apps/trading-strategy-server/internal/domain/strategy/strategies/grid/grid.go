package grid

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"dizzycode.xyz/shared/domain/value_objects"
)

// GridConfig 網格策略配置
type GridConfig struct {
	InstID                string              // 交易對
	PositionSize          float64             // 單次開倉大小（美元）
	FeeRate               float64             // 手續費率（例: 0.0005 = 0.05%）
	TakeProfitRateMin     float64             // 最小停利比例（例: 0.0015 = 0.15%）
	TakeProfitRateMax     float64             // 最大停利比例（例: 0.002 = 0.2%）
	BreakEvenProfitMin    float64             // 盈虧平衡最小目標盈利（USDT）
	BreakEvenProfitMax    float64             // 盈虧平衡最大目標盈利（USDT）
	TrendFilterConfig     TrendAnalyzerConfig // 趨勢過濾配置 ⭐
	EnableTrendFilter     bool                // 是否啟用趨勢過濾 ⭐
	EnableRedCandleFilter bool                // 是否啟用紅K過濾（虧損時只在紅K開倉）⭐
}

// OpenAdvice 開倉建議（領域值對象）
type OpenAdvice struct {
	ShouldOpen     bool    // 是否應該開倉
	CurrentPrice   string  // 當前價格
	OpenPrice      string  // 建議開倉價格（精確字符串，例: "3889.94"）
	ClosePrice     string  // 建議平倉價格（精確字符串，例: "3895.78"）
	PositionSize   float64 // 建議倉位大小（美元）
	TakeProfitRate float64 // 建議停利比例（例: 0.0015 = 0.15%）
	Reason         string  // 原因
}

// GridAggregate 網格聚合根（無狀態設計）⭐
// 特點：
// 1. 封裝業務規則
// 2. 保證不變性（Invariants）
// 3. 無狀態：不記錄 lastCandle（改為參數傳入）
// 4. 不依賴任何技術實現
type GridAggregate struct {
	InstID                string
	PositionSize          float64 // 單次開倉大小（美元）
	FeeRate               float64 // 手續費率（例: 0.0005 = 0.05%）
	TakeProfitRateMin     float64 // 最小停利比例（例: 0.0015 = 0.15%）
	TakeProfitRateMax     float64 // 最大停利比例（例: 0.002 = 0.2%）
	BreakEvenProfitMin    float64 // 盈虧平衡最小目標盈利（USDT）
	BreakEvenProfitMax    float64 // 盈虧平衡最大目標盈利（USDT）
	Calculator            *GridCalculator
	TrendAnalyzer         *TrendAnalyzer // 趨勢分析器 ⭐
	EnableTrendFilter     bool           // 是否啟用趨勢過濾 ⭐
	EnableRedCandleFilter bool           // 是否啟用紅K過濾（虧損時只在紅K開倉）⭐
	// ❌ 移除 lastCandle（改為參數傳入，無狀態設計）
}

// NewGridAggregate 創建網格聚合根（工廠方法）
func NewGridAggregate(config GridConfig) (*GridAggregate, error) {
	// 驗證業務規則
	if config.TakeProfitRateMin <= 0 || config.TakeProfitRateMax <= 0 {
		return nil, errors.New("take profit rate must be positive")
	}

	if config.TakeProfitRateMin > config.TakeProfitRateMax {
		return nil, errors.New("take profit rate min must be <= max")
	}

	// if config.BreakEvenProfitMin < 0 || config.BreakEvenProfitMax < 0 {
	// 	return nil, errors.New("break even profit must be non-negative")
	// }

	if config.BreakEvenProfitMin > config.BreakEvenProfitMax {
		return nil, errors.New("break even profit min must be <= max")
	}

	return &GridAggregate{
		InstID:                config.InstID,
		PositionSize:          config.PositionSize,
		FeeRate:               config.FeeRate,
		TakeProfitRateMin:     config.TakeProfitRateMin,
		TakeProfitRateMax:     config.TakeProfitRateMax,
		BreakEvenProfitMin:    config.BreakEvenProfitMin,
		BreakEvenProfitMax:    config.BreakEvenProfitMax,
		Calculator:            NewGridCalculator(),
		TrendAnalyzer:         NewTrendAnalyzer(config.TrendFilterConfig), // ⭐ 初始化趨勢分析器
		EnableTrendFilter:     config.EnableTrendFilter,                   // ⭐ 是否啟用趨勢過濾
		EnableRedCandleFilter: config.EnableRedCandleFilter,               // ⭐ 是否啟用紅K過濾
	}, nil
}

// GetOpenAdvice 獲取開倉建議（被動諮詢方法）⭐
// 參數：
//   - currentPrice: 當前價格（Order Service 提供）
//   - currentCandle: 當前 K 線（用於紅K過濾）⭐ 新增
//   - lastCandle: 上一根 K 線（從 Redis 讀取）
//   - candleHistories: K線歷史數據
//   - positionSummary: 當前持倉摘要（用於盈虧平衡判斷）⭐
//
// 返回：OpenAdvice（開倉建議）
func (g *GridAggregate) GetOpenAdvice(
	currentPrice value_objects.Price,
	currentCandle value_objects.Candle,
	lastCandle value_objects.Candle,
	candleHistories []value_objects.Candle,
	positionSummary value_objects.PositionSummary,
) OpenAdvice {
	// ========== 步驟 1: 趨勢過濾檢查 ⭐ ==========
	// 如果啟用趨勢過濾，檢查是否允許開倉
	if g.EnableTrendFilter && len(candleHistories) > 0 {
		canOpenLong := g.TrendAnalyzer.CanOpenLong(candleHistories)
		if !canOpenLong {
			// 趨勢過濾：禁止開多單
			trendInfo := g.TrendAnalyzer.GetTrendInfo(candleHistories)
			return OpenAdvice{
				ShouldOpen: false,
				Reason: fmt.Sprintf(
					"trend_filter_blocked: trend=%s, ema_diff=%.2f%%, candle_change=%.2f%%",
					trendInfo.Status,
					trendInfo.EMADiffPercent,
					trendInfo.CandleChange,
				),
			}
		}
	}

	// ========== 步驟 2: 檢查盈虧平衡退出 ⭐ ==========
	// 如果有未平倉位，優先檢查是否應該盈虧平衡退出
	if !positionSummary.IsEmpty() {
		// 判斷是否應該盈虧平衡退出

		shouldExit, expectedProfit := positionSummary.ShouldBreakEven(
			g.BreakEvenProfitMin,
			g.BreakEvenProfitMax,
		)

		if shouldExit {
			// 應該盈虧平衡退出 - 不開新倉
			return OpenAdvice{
				ShouldOpen: false,
				Reason: fmt.Sprintf(
					"break_even_exit: expected_profit=%.2f USDT (target: %.0f-%.0f USDT)",
					expectedProfit,
					g.BreakEvenProfitMin,
					g.BreakEvenProfitMax,
				),
			}
		}
	}

	// ========== 步驟 3: 紅K過濾檢查（虧損時只在紅K開倉）⭐ ==========
	if g.EnableRedCandleFilter && !positionSummary.IsEmpty() {
		avgCost := positionSummary.AvgPrice
		currentPriceValue := currentPrice.Value()

		// 如果平均成本高於現價（持倉處於虧損），檢查當前K線顏色
		if avgCost > currentPriceValue {
			isRedCandle := currentCandle.Close().Value() < currentCandle.Open().Value() // 紅K = Close < Open
			if !isRedCandle {
				// 當前為綠K，虧損狀態下不允許開倉
				return OpenAdvice{
					ShouldOpen: false,
					Reason: fmt.Sprintf(
						"red_candle_filter: loss_state_green_candle (avgCost=%.2f, price=%.2f, close=%.2f, open=%.2f)",
						avgCost,
						currentPriceValue,
						currentCandle.Close().Value(),
						currentCandle.Open().Value(),
					),
				}
			}
		}
	}

	// ========== 步驟 4: 正常開倉邏輯 ⭐ ==========
	// ✅ 使用 decimal 进行精确计算
	currentPriceDecimal := decimal.NewFromFloat(currentPrice.Value())

	// 策略参数
	openDiscountRate := 0.001 // 开仓折扣比例：0.1%（在低于市价 0.1% 处挂单）

	// 计算因子
	openDiscountFactor := decimal.NewFromFloat(1 - openDiscountRate)  // 1 - 0.001 = 0.999
	takeProfitFactor := decimal.NewFromFloat(1 + g.TakeProfitRateMin) // 1 + 0.0015 = 1.0015

	// 计算开仓价格：当前价格 * 0.999，无条件舍去到小数点第 2 位
	openPriceDecimal := currentPriceDecimal.Mul(openDiscountFactor).Truncate(2)

	// 计算平仓价格：开仓价格 * 1.0015，无条件进位到小数点第 2 位
	// 实现无条件进位：先乘以 100，向上取整，再除以 100
	closePriceDecimal := openPriceDecimal.Mul(takeProfitFactor)
	shift := decimal.NewFromInt(100)
	closePriceDecimal = closePriceDecimal.Mul(shift).Ceil().Div(shift)

	return OpenAdvice{
		ShouldOpen:     true,
		CurrentPrice:   currentPriceDecimal.String(),
		OpenPrice:      openPriceDecimal.String(),  // 例: "3889.94" (舍去)
		ClosePrice:     closePriceDecimal.String(), // 例: "3895.78" (进位)
		PositionSize:   g.PositionSize,
		TakeProfitRate: g.TakeProfitRateMin, // 0.0015 (0.15%)
		Reason:         "simulated_advice",
	}
}

// ProcessCandle 處理新的K線（舊方法，保留用於向後兼容）
// ⚠️ 已棄用：請使用 GetOpenAdvice() 方法
// 根據策略文件：開倉位置 = 前一根K線的MidLow
func (g *GridAggregate) ProcessCandle(candle value_objects.Candle) (*value_objects.Signal, error) {
	// 舊方法已棄用，但保留代碼避免破壞現有依賴
	return nil, fmt.Errorf("ProcessCandle is deprecated, use GetOpenAdvice instead")
}

// GetState 獲取當前狀態（用於日誌或監控）
func (g *GridAggregate) GetState() map[string]any {
	return map[string]any{
		"instID":             g.InstID,
		"positionSize":       g.PositionSize,
		"takeProfitRateMin":  g.TakeProfitRateMin,
		"takeProfitRateMax":  g.TakeProfitRateMax,
		"breakEvenProfitMin": g.BreakEvenProfitMin,
		"breakEvenProfitMax": g.BreakEvenProfitMax,
		"enableTrendFilter":  g.EnableTrendFilter, // ⭐ 新增
	}
}

// GetTrendInfo 獲取當前趨勢信息（用於日誌調試）⭐
// 參數：
//   - candleHistories: K線歷史數據
//
// 返回：
//   - TrendInfo: 趨勢詳細信息
func (g *GridAggregate) GetTrendInfo(candleHistories []value_objects.Candle) TrendInfo {
	if !g.EnableTrendFilter {
		return TrendInfo{
			Status: "trend_filter_disabled",
		}
	}

	return g.TrendAnalyzer.GetTrendInfo(candleHistories)
}

// GetName 獲取策略名稱
func (g *GridAggregate) GetName() string {
	return "grid"
}
