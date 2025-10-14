package logger

import (
	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/config"
)

// New creates a logger instance based on configuration
// Similar to TypeScript's createLogger pattern
func New(cfg *config.Config) (logger.Logger, error) {
	return logger.NewZap(logger.ZapOptions{
		ServiceName: "market-data-service",
		IsPretty:    cfg.Environment != "production",
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
