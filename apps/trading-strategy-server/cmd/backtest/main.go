package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"dizzycode.xyz/trading-strategy-server/backtesting/engine"
	"dizzycode.xyz/trading-strategy-server/backtesting/metrics"
)

func main() {
	// 解析命令行參數
	dataFile := flag.String("data", "", "歷史數據文件路徑 (必填)")
	initialBalance := flag.Float64("initial-balance", 10000.0, "初始資金 (USDT)")
	feeRate := flag.Float64("fee-rate", 0.0005, "手續費率 (默認: 0.0005 = 0.05%)")
	slippage := flag.Float64("slippage", 0.0, "滑點 (默認: 0)")
	instID := flag.String("inst-id", "ETH-USDT-SWAP", "交易對")
	takeProfitMin := flag.Float64("take-profit-min", 0.0015, "最小止盈百分比 (默認: 0.0015 = 0.15%)")
	takeProfitMax := flag.Float64("take-profit-max", 0.0020, "最大止盈百分比 (默認: 0.0020 = 0.20%)")

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
	fmt.Printf("手續費率: %.4f%% (%.6f)\n", *feeRate*100, *feeRate)
	fmt.Printf("滑點: %.4f%%\n", *slippage*100)
	fmt.Printf("止盈範圍: %.2f%% ~ %.2f%%\n", *takeProfitMin*100, *takeProfitMax*100)
	fmt.Println("========================================")
	fmt.Println()

	// 創建回測引擎配置
	config := engine.BacktestConfig{
		InitialBalance: *initialBalance,
		FeeRate:        *feeRate,
		Slippage:       *slippage,
		InstID:         *instID,
		TakeProfitMin:  *takeProfitMin,
		TakeProfitMax:  *takeProfitMax,
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
	fmt.Printf("初始資金: $%.2f USDT\n", result.InitialBalance)
	fmt.Printf("最終資金: $%.2f USDT\n", result.FinalBalance)
	fmt.Printf("淨利潤:   $%.2f USDT", result.NetProfit)
	if result.NetProfit > 0 {
		fmt.Printf(" ✅\n")
	} else if result.NetProfit < 0 {
		fmt.Printf(" ❌\n")
	} else {
		fmt.Printf(" ⚠️\n")
	}
	fmt.Printf("總收益率: %.2f%%", result.TotalReturn)
	if result.TotalReturn > 0 {
		fmt.Printf(" 📈\n")
	} else if result.TotalReturn < 0 {
		fmt.Printf(" 📉\n")
	} else {
		fmt.Printf(" ➡️\n")
	}
	fmt.Printf("最大回撤: %.2f%%", result.MaxDrawdown)
	if result.MaxDrawdown < 5 {
		fmt.Printf(" ✅\n")
	} else if result.MaxDrawdown < 20 {
		fmt.Printf(" ⚠️\n")
	} else {
		fmt.Printf(" ❌\n")
	}
	fmt.Println()

	// 交易統計
	fmt.Println("📈 交易統計")
	fmt.Println("----------------------------------------")
	fmt.Printf("總交易次數: %d\n", result.TotalTrades)
	fmt.Printf("盈利交易:   %d\n", result.WinningTrades)
	fmt.Printf("虧損交易:   %d\n", result.LosingTrades)
	fmt.Printf("勝率:       %.2f%%", result.WinRate)
	if result.WinRate >= 60 {
		fmt.Printf(" ✅\n")
	} else if result.WinRate >= 40 {
		fmt.Printf(" ⚠️\n")
	} else {
		fmt.Printf(" ❌\n")
	}
	fmt.Println()

	// 盈虧分析
	fmt.Println("💰 盈虧分析")
	fmt.Println("----------------------------------------")
	fmt.Printf("總盈利金額: $%.2f USDT\n", result.TotalProfit)
	fmt.Printf("總虧損金額: $%.2f USDT\n", result.TotalLoss)
	fmt.Printf("盈虧比:     %.2f", result.ProfitFactor)
	if result.ProfitFactor >= 2.0 {
		fmt.Printf(" ✅ (優秀)\n")
	} else if result.ProfitFactor >= 1.5 {
		fmt.Printf(" ✅ (良好)\n")
	} else if result.ProfitFactor >= 1.0 {
		fmt.Printf(" ⚠️ (一般)\n")
	} else {
		fmt.Printf(" ❌ (需改進)\n")
	}
	fmt.Printf("平均持倉時長: %v\n", formatDuration(result.AvgHoldDuration))
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
