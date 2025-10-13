package level

import "go.uber.org/zap/zapcore"

// Level represents the severity level of a log entry
type Level int8

const (
	// Debug logs are typically voluminous, and are usually disabled in production.
	Debug Level = iota - 1
	// Info is the default logging priority.
	Info
	// Warn logs are more important than Info, but don't need individual human review.
	Warn
	// Error logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs.
	Error
	// Fatal logs a message, then calls os.Exit(1).
	Fatal
)

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	case Fatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// ToZapLevel converts our Level to zapcore.Level
func (l Level) ToZapLevel() zapcore.Level {
	switch l {
	case Debug:
		return zapcore.DebugLevel
	case Info:
		return zapcore.InfoLevel
	case Warn:
		return zapcore.WarnLevel
	case Error:
		return zapcore.ErrorLevel
	case Fatal:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Parse converts a string to Level
func Parse(s string) Level {
	switch s {
	case "debug":
		return Debug
	case "info":
		return Info
	case "warn", "warning":
		return Warn
	case "error":
		return Error
	case "fatal":
		return Fatal
	default:
		return Info
	}
}
