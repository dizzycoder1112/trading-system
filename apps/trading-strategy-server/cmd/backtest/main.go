package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dizzycode.xyz/trading-strategy-server/backtesting/engine"
	"dizzycode.xyz/trading-strategy-server/backtesting/metrics"
)

func main() {
	// 解析命令行參數
	dataFile := flag.String("data", "", "歷史數據文件路徑 (必填)")
	initialBalance := flag.Float64("initial-balance", 10000.0, "初始資金 (USDT)")
	feeRate := flag.Float64("fee-rate", 0.0005, "手續費率 (默認: 0.0005 = 0.05%)")
	positionSize := flag.Float64("position-size", 300.0, "單次開倉大小 (USDT)")
	slippage := flag.Float64("slippage", 0.0, "滑點 (默認: 0)")
	instID := flag.String("inst-id", "ETH-USDT-SWAP", "交易對")
	takeProfitMin := flag.Float64("take-profit-min", 0.0015, "最小止盈百分比 (默認: 0.0015 = 0.15%)")
	takeProfitMax := flag.Float64("take-profit-max", 0.0020, "最大止盈百分比 (默認: 0.0020 = 0.20%)")
	breakEvenProfitMin := flag.Float64("break-even-profit-min", 0.0, "打平最小目標盈利 (USDT, 默認: 0)")
	breakEvenProfitMax := flag.Float64("break-even-profit-max", 20.0, "打平最大目標盈利 (USDT, 默認: 20)")

	flag.Parse()

	// 驗證必填參數
	if *dataFile == "" {
		fmt.Println("錯誤: 必須指定歷史數據文件路徑")
		fmt.Println()
		fmt.Println("使用方式:")
		fmt.Println("  go run cmd/backtest.go --data=data/20240930-20241001-5m-ETH-USDT-SWAP.json")
		fmt.Println()
		fmt.Println("參數說明:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 檢查文件是否存在
	if _, err := os.Stat(*dataFile); os.IsNotExist(err) {
		fmt.Printf("錯誤: 文件不存在: %s\n", *dataFile)
		os.Exit(1)
	}

	// 打印配置信息
	fmt.Println("========================================")
	fmt.Println("回測引擎 - 配置信息")
	fmt.Println("========================================")
	fmt.Printf("數據文件: %s\n", *dataFile)
	fmt.Printf("交易對: %s\n", *instID)
	fmt.Printf("初始資金: $%.2f USDT\n", *initialBalance)
	fmt.Printf("倉位大小: $%.2f USDT\n", *positionSize)
	fmt.Printf("手續費率: %.4f%% (%.6f)\n", *feeRate*100, *feeRate)
	fmt.Printf("滑點: %.4f%%\n", *slippage*100)
	fmt.Printf("止盈範圍: %.2f%% ~ %.2f%%\n", *takeProfitMin*100, *takeProfitMax*100)
	fmt.Printf("打平目標: $%.2f ~ $%.2f USDT\n", *breakEvenProfitMin, *breakEvenProfitMax)
	fmt.Println("========================================")
	fmt.Println()

	// 創建回測引擎配置
	config := engine.BacktestConfig{
		InitialBalance:     *initialBalance,
		FeeRate:            *feeRate,
		Slippage:           *slippage,
		InstID:             *instID,
		TakeProfitMin:      *takeProfitMin,
		TakeProfitMax:      *takeProfitMax,
		PositionSize:       *positionSize,
		BreakEvenProfitMin: *breakEvenProfitMin,
		BreakEvenProfitMax: *breakEvenProfitMax,
	}

	// 創建回測引擎
	fmt.Println("正在初始化回測引擎...")
	backtestEngine, err := engine.NewBacktestEngine(config)
	if err != nil {
		fmt.Printf("錯誤: 創建回測引擎失敗: %v\n", err)
		os.Exit(1)
	}

	// 運行回測
	fmt.Printf("正在載入歷史數據: %s\n", *dataFile)
	startTime := time.Now()
	result, err := backtestEngine.RunFromFile(*dataFile)
	if err != nil {
		fmt.Printf("錯誤: 回測執行失敗: %v\n", err)
		os.Exit(1)
	}
	duration := time.Since(startTime)

	// 打印回測結果
	printBacktestResult(result, *dataFile, duration)

	// ⭐ 導出回測結果到文件夾
	exportResults(backtestEngine, result, *dataFile, *positionSize, duration, config)
}

// printBacktestResult 格式化輸出回測結果
func printBacktestResult(result metrics.BacktestResult, dataFile string, duration time.Duration) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("回測結果: %s\n", dataFile)
	fmt.Println("========================================")
	fmt.Printf("執行時間: %v\n", duration)
	fmt.Println()

	// 資金狀況
	fmt.Println("📊 資金狀況")
	fmt.Println("----------------------------------------")
	fmt.Printf("初始資金:     $%.2f USDT\n", result.InitialBalance)
	fmt.Printf("可用餘額:     $%.2f USDT\n", result.FinalBalance)
	fmt.Printf("總權益:       $%.2f USDT (餘額 + 未平倉價值 + 浮盈虧)\n", result.TotalEquity)
	fmt.Println()

	// 倉位分析
	fmt.Println("⭐ 倉位分析")
	fmt.Println("----------------------------------------")
	fmt.Printf("總開倉數量:   %d 筆\n", result.TotalOpenedTrades)
	fmt.Printf("總關倉數量:   %d 筆\n", result.TotalClosedTrades)
	fmt.Printf("未平倉數量:   %d 筆\n", result.OpenPositionCount)
	fmt.Printf("未平倉價值:   $%.2f USDT\n", result.OpenPositionValue)
	fmt.Println()

	// 交易統計
	fmt.Println("📈 交易統計")
	fmt.Println("----------------------------------------")
	fmt.Printf("總利潤:       $%.2f USDT 💸 (未扣手續費)\n", result.TotalProfitGross)
	fmt.Printf("總手續費:     $%.2f USDT 💸 (開倉: $%.2f, 關倉: $%.2f)\n",
		result.TotalFeesPaid, result.TotalFeesOpen, result.TotalFeesClose)
	fmt.Printf("未實現盈虧:   $%.2f USDT", result.UnrealizedPnL)
	if result.UnrealizedPnL > 0 {
		fmt.Printf(" 📈 (含預估關倉手續費)\n")
	} else if result.UnrealizedPnL < 0 {
		fmt.Printf(" 📉 (含預估關倉手續費)\n")
	} else {
		fmt.Printf(" ➡️ (含預估關倉手續費)\n")
	}
	fmt.Printf("淨利潤:       $%.2f USDT", result.NetProfit)
	if result.NetProfit > 0 {
		fmt.Printf(" ✅\n")
	} else if result.NetProfit < 0 {
		fmt.Printf(" ❌\n")
	} else {
		fmt.Printf(" ⚠️\n")
	}
	fmt.Printf("總收益率:     %.2f%%", result.TotalReturn)
	if result.TotalReturn > 0 {
		fmt.Printf(" 📈\n")
	} else if result.TotalReturn < 0 {
		fmt.Printf(" 📉\n")
	} else {
		fmt.Printf(" ➡️\n")
	}
	fmt.Printf("盈虧比:       %.2f", result.ProfitFactor)
	if result.ProfitFactor >= 2.0 {
		fmt.Printf(" ✅ (優秀)\n")
	} else if result.ProfitFactor >= 1.5 {
		fmt.Printf(" ✅ (良好)\n")
	} else if result.ProfitFactor >= 1.0 {
		fmt.Printf(" ⚠️ (一般)\n")
	} else {
		fmt.Printf(" ❌ (需改進)\n")
	}
	fmt.Printf("平均持倉時長: %s\n", formatDuration(result.AvgHoldDuration))
	fmt.Printf("勝率:         %.2f%%", result.WinRate)
	if result.WinRate >= 60 {
		fmt.Printf(" ✅\n")
	} else if result.WinRate >= 40 {
		fmt.Printf(" ⚠️\n")
	} else {
		fmt.Printf(" ❌\n")
	}
	fmt.Printf("最大回撤:     %.2f%%", result.MaxDrawdown)
	if result.MaxDrawdown < 5 {
		fmt.Printf(" ✅\n")
	} else if result.MaxDrawdown < 20 {
		fmt.Printf(" ⚠️\n")
	} else {
		fmt.Printf(" ❌\n")
	}
	fmt.Println()

	// 策略評估
	fmt.Println("🎯 策略評估")
	fmt.Println("----------------------------------------")
	evaluateStrategy(result)
	fmt.Println("========================================")
}

// evaluateStrategy 根據結果評估策略表現
func evaluateStrategy(result metrics.BacktestResult) {
	score := 0

	// 評分標準
	if result.TotalReturn > 0 {
		score += 2
	}
	if result.MaxDrawdown < 10 {
		score += 2
	} else if result.MaxDrawdown < 20 {
		score += 1
	}
	if result.WinRate >= 60 {
		score += 2
	} else if result.WinRate >= 50 {
		score += 1
	}
	if result.ProfitFactor >= 1.5 {
		score += 2
	} else if result.ProfitFactor >= 1.0 {
		score += 1
	}
	if result.TotalTrades >= 10 {
		score += 1
	}

	// 評級
	var rating string
	var emoji string
	if score >= 8 {
		rating = "優秀"
		emoji = "🌟"
	} else if score >= 6 {
		rating = "良好"
		emoji = "✅"
	} else if score >= 4 {
		rating = "一般"
		emoji = "⚠️"
	} else {
		rating = "需改進"
		emoji = "❌"
	}

	fmt.Printf("綜合評分: %d/9\n", score)
	fmt.Printf("策略評級: %s %s\n", rating, emoji)
	fmt.Println()

	// 建議
	fmt.Println("改進建議:")
	if result.TotalReturn <= 0 {
		fmt.Println("  • 總收益為負，建議調整策略參數或入場邏輯")
	}
	if result.MaxDrawdown > 20 {
		fmt.Println("  • 最大回撤過高，建議加強風險控制和止損機制")
	}
	if result.WinRate < 50 {
		fmt.Println("  • 勝率偏低，建議優化入場信號的準確性")
	}
	if result.ProfitFactor < 1.0 {
		fmt.Println("  • 盈虧比小於1，虧損金額大於盈利金額，需要調整止盈止損比例")
	} else if result.ProfitFactor < 1.5 {
		fmt.Println("  • 盈虧比偏低，建議擴大止盈目標或縮小止損範圍")
	}
	if result.TotalTrades < 10 {
		fmt.Println("  • 交易次數過少，可能數據量不足或策略過於保守")
	}
	if score >= 8 {
		fmt.Println("  • 策略表現優秀，建議進行實盤小額測試！")
	}
}

// formatDuration 格式化時間長度
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f秒", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1f分鐘", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f小時", d.Hours())
	} else {
		days := d.Hours() / 24
		return fmt.Sprintf("%.1f天", days)
	}
}

// exportResults 導出回測結果到文件夾
func exportResults(
	backtestEngine *engine.BacktestEngine,
	result metrics.BacktestResult,
	dataFile string,
	positionSize float64,
	duration time.Duration,
	config engine.BacktestConfig,
) {
	// 獲取數據文件所在目錄
	dataDir := filepath.Dir(dataFile)

	// 創建文件夾名稱：backtest_trades_pos{size}
	folderName := fmt.Sprintf("backtest_trades_pos%.0f", positionSize)

	// 完整路徑：與數據文件在同一目錄
	fullPath := filepath.Join(dataDir, folderName)

	// 創建文件夾
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		fmt.Printf("\n❌ 無法創建文件夾 %s: %v\n", fullPath, err)
		return
	}

	// 1. 導出 CSV 文件
	csvPath := filepath.Join(fullPath, "trades.csv")
	if err := backtestEngine.ExportTradeLogCSV(csvPath); err != nil {
		fmt.Printf("\n❌ 無法導出 CSV: %v\n", err)
	} else {
		fmt.Printf("\n✅ 交易日誌已導出: %s\n", csvPath)
	}

	// 2. 生成報告內容
	reportContent := generateReport(result, dataFile, positionSize, duration, config)

	// 3. 導出報告文件 (Markdown)
	reportPath := filepath.Join(fullPath, "report.md")
	if err := os.WriteFile(reportPath, []byte(reportContent), 0644); err != nil {
		fmt.Printf("❌ 無法導出報告: %v\n", err)
	} else {
		fmt.Printf("✅ 回測報告已導出: %s\n", reportPath)
	}

	fmt.Printf("\n📁 所有文件已保存到文件夾: %s/\n", fullPath)
}

// generateReport 生成 Markdown 格式的回測報告
func generateReport(
	result metrics.BacktestResult,
	dataFile string,
	positionSize float64,
	duration time.Duration,
	config engine.BacktestConfig,
) string {
	var report string

	// 標題
	report += "# 回測報告\n\n"
	report += fmt.Sprintf("生成時間: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// 配置信息
	report += "## 回測配置\n\n"
	report += fmt.Sprintf("- **數據文件**: %s\n", dataFile)
	report += fmt.Sprintf("- **交易對**: %s\n", config.InstID)
	report += fmt.Sprintf("- **初始資金**: $%.2f USDT\n", config.InitialBalance)
	report += fmt.Sprintf("- **倉位大小**: $%.2f USDT\n", positionSize)
	report += fmt.Sprintf("- **手續費率**: %.4f%% (%.6f)\n", config.FeeRate*100, config.FeeRate)
	report += fmt.Sprintf("- **滑點**: %.4f%%\n", config.Slippage*100)
	report += fmt.Sprintf("- **止盈範圍**: %.2f%% ~ %.2f%%\n", config.TakeProfitMin*100, config.TakeProfitMax*100)
	report += fmt.Sprintf("- **執行時間**: %v\n", duration)
	report += "\n"

	// 資金狀況
	report += "## 📊 資金狀況\n\n"
	report += fmt.Sprintf("- **初始資金**: $%.2f USDT\n", result.InitialBalance)
	report += fmt.Sprintf("- **可用餘額**: $%.2f USDT\n", result.FinalBalance)
	report += "\n"

	// 倉位分析
	report += "### ⭐ 倉位分析\n\n"
	report += fmt.Sprintf("- **總開倉數量**: %d 筆\n", result.TotalOpenedTrades)
	report += fmt.Sprintf("- **總關倉數量**: %d 筆\n", result.TotalClosedTrades)
	report += fmt.Sprintf("- **未平倉數量**: %d 筆\n", result.OpenPositionCount)
	report += fmt.Sprintf("- **未平倉價值**: $%.2f USDT\n", result.OpenPositionValue)
	report += "\n"

	// 交易統計
	report += "## 📈 交易統計\n\n"
	report += fmt.Sprintf("- **總利潤**: $%.2f USDT 💸 (未扣手續費)\n", result.TotalProfitGross)
	report += fmt.Sprintf("- **總手續費**: $%.2f USDT 💸 (開倉: $%.2f, 關倉: $%.2f)\n",
		result.TotalFeesPaid, result.TotalFeesOpen, result.TotalFeesClose)
	report += fmt.Sprintf("- **未實現盈虧**: $%.2f USDT", result.UnrealizedPnL)
	if result.UnrealizedPnL > 0 {
		report += " 📈 (含預估關倉手續費)\n"
	} else if result.UnrealizedPnL < 0 {
		report += " 📉 (含預估關倉手續費)\n"
	} else {
		report += " ➡️ (含預估關倉手續費)\n"
	}
	report += fmt.Sprintf("- **淨利潤**: $%.2f USDT", result.NetProfit)
	if result.NetProfit > 0 {
		report += " ✅\n"
	} else {
		report += " ❌\n"
	}
	report += fmt.Sprintf("- **總收益率**: %.2f%%", result.TotalReturn)
	if result.TotalReturn > 0 {
		report += " 📈\n"
	} else {
		report += " 📉\n"
	}
	report += fmt.Sprintf("- **盈虧比**: %.2f", result.ProfitFactor)
	if result.ProfitFactor >= 2.0 {
		report += " ✅ (優秀)\n"
	} else if result.ProfitFactor >= 1.5 {
		report += " ✅ (良好)\n"
	} else if result.ProfitFactor >= 1.0 {
		report += " ⚠️ (一般)\n"
	} else {
		report += " ❌ (需改進)\n"
	}
	report += fmt.Sprintf("- **平均持倉時長**: %s\n", formatDuration(result.AvgHoldDuration))
	report += fmt.Sprintf("- **勝率**: %.2f%%\n", result.WinRate)
	report += fmt.Sprintf("- **最大回撤**: %.2f%%\n", result.MaxDrawdown)
	report += "\n"

	// 策略評估
	report += "## 🎯 策略評估\n\n"
	score := 0
	if result.TotalReturn > 0 {
		score += 2
	}
	if result.MaxDrawdown < 10 {
		score += 2
	} else if result.MaxDrawdown < 20 {
		score += 1
	}
	if result.WinRate >= 60 {
		score += 2
	} else if result.WinRate >= 50 {
		score += 1
	}
	if result.ProfitFactor >= 1.5 {
		score += 2
	} else if result.ProfitFactor >= 1.0 {
		score += 1
	}
	if result.TotalTrades >= 10 {
		score += 1
	}

	var rating string
	if score >= 8 {
		rating = "優秀 🌟"
	} else if score >= 6 {
		rating = "良好 ✅"
	} else if score >= 4 {
		rating = "一般 ⚠️"
	} else {
		rating = "需改進 ❌"
	}

	report += fmt.Sprintf("- **綜合評分**: %d/9\n", score)
	report += fmt.Sprintf("- **策略評級**: %s\n", rating)
	report += "\n"

	// 改進建議
	report += "### 改進建議\n\n"
	if result.TotalReturn <= 0 {
		report += "- 總收益為負，建議調整策略參數或入場邏輯\n"
	}
	if result.MaxDrawdown > 20 {
		report += "- 最大回撤過高，建議加強風險控制和止損機制\n"
	}
	if result.WinRate < 50 {
		report += "- 勝率偏低，建議優化入場信號的準確性\n"
	}
	if result.ProfitFactor < 1.0 {
		report += "- 盈虧比小於1，虧損金額大於盈利金額，需要調整止盈止損比例\n"
	} else if result.ProfitFactor < 1.5 {
		report += "- 盈虧比偏低，建議擴大止盈目標或縮小止損範圍\n"
	}
	if result.TotalTrades < 10 {
		report += "- 交易次數過少，可能數據量不足或策略過於保守\n"
	}
	if score >= 8 {
		report += "- 策略表現優秀，建議進行實盤小額測試！\n"
	}

	return report
}

// printTradeLog 打印交易日誌（DEBUG用）
func printTradeLog(backtestEngine *engine.BacktestEngine) {
	logs := backtestEngine.GetTradeLog()
	if len(logs) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("📋 交易日誌 (DEBUG)")
	fmt.Println("========================================")
	fmt.Printf("總交易日誌數: %d\n", len(logs))
	fmt.Println()

	// 打印前 10 筆
	fmt.Println("前 10 筆交易:")
	fmt.Println("----------------------------------------")
	printCount := 10
	if len(logs) < 10 {
		printCount = len(logs)
	}
	for i := 0; i < printCount; i++ {
		log := logs[i]
		fmt.Printf("#%d [%s] %s | Price: %.2f | Size: %.2f | Balance: %.2f | PnL: %.2f | %s\n",
			log.TradeID,
			log.Time.Format("15:04:05"),
			log.Action,
			log.Price,
			log.PositionSize,
			log.Balance,
			log.PnL,
			log.Reason,
		)
	}

	// 打印後 10 筆
	if len(logs) > 10 {
		fmt.Println()
		fmt.Println("後 10 筆交易:")
		fmt.Println("----------------------------------------")
		startIdx := len(logs) - 10
		for i := startIdx; i < len(logs); i++ {
			log := logs[i]
			fmt.Printf("#%d [%s] %s | Price: %.2f | Size: %.2f | Balance: %.2f | PnL: %.2f | %s\n",
				log.TradeID,
				log.Time.Format("15:04:05"),
				log.Action,
				log.Price,
				log.PositionSize,
				log.Balance,
				log.PnL,
				log.Reason,
			)
		}
	}
	fmt.Println("========================================")
}
