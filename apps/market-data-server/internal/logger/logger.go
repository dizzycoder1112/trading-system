package logger

import (
	"dizzycode.xyz/logger"
	"dizzycode.xyz/logger/level"
	"dizzycode.xyz/logger/strategies"
)

func CreateLogger(lv string) *logger.Logger {
	logLevel := level.Parse(lv)

	// 根據環境決定輸出格式

	zapStrategy := strategies.NewZapMust(strategies.ZapOptions{
		IsPretty: false,
		Level:    logLevel,
	})

	return logger.NewLogger("market-data-service", []strategies.Strategy{zapStrategy})
}
