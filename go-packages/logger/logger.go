package logger

import (
	"context"
	"time"

	"dizzycode.xyz/logger/level"
	"dizzycode.xyz/logger/strategies"

	"go.uber.org/zap"
)

// Logger is the main logger interface that supports multiple strategies
type Logger struct {
	strategies  []strategies.Strategy
	serviceName string
	baseFields  []zap.Field // Fields that will be added to every log entry
}

// NewLogger creates a new logger with the given strategies
// If no strategies are provided, defaults to Console strategy
func NewLogger(serviceName string, strats []strategies.Strategy, opts ...Option) *Logger {
	// Default to Console strategy if none provided
	if len(strats) == 0 {
		strats = []strategies.Strategy{strategies.NewConsole()}
	}

	l := &Logger{
		strategies:  strats,
		serviceName: serviceName,
		baseFields:  []zap.Field{},
	}

	// Apply options
	for _, opt := range opts {
		opt(l)
	}

	return l
}

// NewNopLogger creates a logger that discards all output
// Useful for testing
func NewNopLogger() *Logger {
	return &Logger{
		strategies:  []strategies.Strategy{strategies.NewNop()},
		serviceName: "nop",
		baseFields:  []zap.Field{},
	}
}

// Option is a function that configures a Logger
type Option func(*Logger)

// WithFields adds permanent fields to the logger
func WithFields(fields ...zap.Field) Option {
	return func(l *Logger) {
		l.baseFields = append(l.baseFields, fields...)
	}
}

// log is the internal method that dispatches to all strategies
func (l *Logger) log(lvl level.Level, message string, fields []zap.Field) {
	// Combine base fields with provided fields
	allFields := make([]zap.Field, 0, len(l.baseFields)+len(fields))
	allFields = append(allFields, l.baseFields...)
	allFields = append(allFields, fields...)

	entry := strategies.Entry{
		Level:       lvl,
		Message:     message,
		Fields:      allFields,
		Time:        time.Now(),
		ServiceName: l.serviceName,
	}

	// Dispatch to all strategies
	for _, strategy := range l.strategies {
		if err := strategy.Log(entry); err != nil {
			// If logging fails, silently continue
			// In production, this should rarely happen
		}
	}
}

// Debug logs a debug-level message
func (l *Logger) Debug(message string, fields ...zap.Field) {
	l.log(level.Debug, message, fields)
}

// Info logs an info-level message
func (l *Logger) Info(message string, fields ...zap.Field) {
	l.log(level.Info, message, fields)
}

// Warn logs a warn-level message
func (l *Logger) Warn(message string, fields ...zap.Field) {
	l.log(level.Warn, message, fields)
}

// Error logs an error-level message
func (l *Logger) Error(message string, err error, fields ...zap.Field) {
	allFields := make([]zap.Field, 0, len(fields)+1)
	if err != nil {
		allFields = append(allFields, zap.Error(err))
	}
	allFields = append(allFields, fields...)
	l.log(level.Error, message, allFields)
}

// Fatal logs a fatal-level message and then calls os.Exit(1)
func (l *Logger) Fatal(message string, err error, fields ...zap.Field) {
	allFields := make([]zap.Field, 0, len(fields)+1)
	if err != nil {
		allFields = append(allFields, zap.Error(err))
	}
	allFields = append(allFields, fields...)
	l.log(level.Fatal, message, allFields)
}

// With creates a child logger with additional fields
// This is useful for adding context-specific fields that will be included in all subsequent log entries
func (l *Logger) With(fields ...zap.Field) *Logger {
	newFields := make([]zap.Field, len(l.baseFields)+len(fields))
	copy(newFields, l.baseFields)
	copy(newFields[len(l.baseFields):], fields)

	return &Logger{
		strategies:  l.strategies,
		serviceName: l.serviceName,
		baseFields:  newFields,
	}
}

// WithContext extracts fields from context and creates a child logger
// This is useful for extracting trace IDs, request IDs, etc. from context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract common fields from context
	fields := make([]zap.Field, 0, 2)

	// Example: Extract trace ID if present
	// You can customize this based on your context keys
	if traceID, ok := ctx.Value("traceID").(string); ok && traceID != "" {
		fields = append(fields, zap.String("traceID", traceID))
	}

	if requestID, ok := ctx.Value("requestID").(string); ok && requestID != "" {
		fields = append(fields, zap.String("requestID", requestID))
	}

	if len(fields) == 0 {
		return l
	}

	return l.With(fields...)
}

// Sync flushes any buffered log entries
// This should be called before the application exits
func (l *Logger) Sync() error {
	var lastErr error
	for _, strategy := range l.strategies {
		if err := strategy.Sync(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
