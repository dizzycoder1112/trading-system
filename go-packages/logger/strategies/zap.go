package strategies

import (
	"dizzycode.xyz/logger/level"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Zap implements the Strategy interface using Uber's Zap logger
type Zap struct {
	logger *zap.Logger
}

// ZapOptions configures the Zap strategy
type ZapOptions struct {
	// IsPretty enables human-readable console output (for development)
	// If false, outputs JSON format (for production)
	IsPretty bool

	// Level sets the minimum log level
	Level level.Level
}

// NewZap creates a new Zap strategy with the given options
func NewZap(opts ZapOptions) (*Zap, error) {
	var zapLogger *zap.Logger
	var err error

	if opts.IsPretty {
		// Development mode: pretty console output
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(opts.Level.ToZapLevel())
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		zapLogger, err = config.Build(
			zap.AddCallerSkip(3), // Skip: zap.go -> logger.log() -> logger.Info()
		)
	} else {
		// Production mode: JSON output
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(opts.Level.ToZapLevel())
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		zapLogger, err = config.Build(
			zap.AddCallerSkip(3), // Skip: zap.go -> logger.log() -> logger.Info()
		)
	}

	if err != nil {
		return nil, err
	}

	return &Zap{
		logger: zapLogger,
	}, nil
}

// NewZapMust creates a new Zap strategy and panics on error
// This is useful for initialization in main() where errors should be fatal
func NewZapMust(opts ZapOptions) *Zap {
	strategy, err := NewZap(opts)
	if err != nil {
		panic(err)
	}
	return strategy
}

// Log implements the Strategy interface
func (z *Zap) Log(entry Entry) error {
	// Add service name to fields
	fields := make([]zap.Field, len(entry.Fields)+1)
	fields[0] = zap.String("service", entry.ServiceName)
	copy(fields[1:], entry.Fields)

	// Dispatch to the appropriate log level
	switch entry.Level {
	case level.Debug:
		z.logger.Debug(entry.Message, fields...)
	case level.Info:
		z.logger.Info(entry.Message, fields...)
	case level.Warn:
		z.logger.Warn(entry.Message, fields...)
	case level.Error:
		z.logger.Error(entry.Message, fields...)
	case level.Fatal:
		z.logger.Fatal(entry.Message, fields...)
	default:
		z.logger.Info(entry.Message, fields...)
	}

	return nil
}

// Sync implements the Strategy interface
func (z *Zap) Sync() error {
	return z.logger.Sync()
}

// GetZapLogger returns the underlying zap logger
// This is useful if you need direct access to zap's features
func (z *Zap) GetZapLogger() *zap.Logger {
	return z.logger
}
