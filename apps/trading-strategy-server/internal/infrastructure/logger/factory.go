package logger

import (
	"dizzycode.xyz/logger"
	"dizzycode.xyz/trading-strategy-server/internal/infrastructure/config"
)

// New creates a logger instance based on configuration
func New(cfg *config.Config) (logger.Logger, error) {
	return logger.NewZap(logger.ZapOptions{
		ServiceName: "trading-strategy-server",
		IsPretty:    cfg.Environment == "development", // ✅ 启用美化输出
		Level:       parseLogLevel(cfg.LogLevel),
	})
}

// Must creates a logger and panics on error
func Must(cfg *config.Config) logger.Logger {
	log, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return log
}

func parseLogLevel(level string) logger.Level {
	switch level {
	case "debug":
		return logger.DebugLevel
	case "info":
		return logger.InfoLevel
	case "warn":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}
