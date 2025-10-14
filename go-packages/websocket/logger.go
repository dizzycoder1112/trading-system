package websocket

import (
	"fmt"
	"time"
)

// Logger is the logging interface required by the WebSocket client
// Users can provide any logger implementation that satisfies this interface
// Similar to TypeScript's Logger interface pattern
type Logger interface {
	Info(msg string, context ...any)
	Error(msg string, context ...any)
	Debug(msg string, context ...any)
	Warn(msg string, context ...any)
}

// defaultLogger is a simple console-based logger used as fallback
// Similar to TypeScript's console object
type defaultLogger struct{}

func (d *defaultLogger) Info(msg string, context ...any) {
	d.log("INFO", msg, context...)
}

func (d *defaultLogger) Error(msg string, context ...any) {
	d.log("ERROR", msg, context...)
}

func (d *defaultLogger) Debug(msg string, context ...any) {
	d.log("DEBUG", msg, context...)
}

func (d *defaultLogger) Warn(msg string, context ...any) {
	d.log("WARN", msg, context...)
}

func (d *defaultLogger) log(level, msg string, context ...any) {
	timestamp := time.Now().Format(time.RFC3339)
	fmt.Printf("%s [%s] [websocket] %s", timestamp, level, msg)

	if len(context) > 0 {
		fmt.Printf(" %v", context)
	}

	fmt.Println()
}

// defaultLog is the package-level default logger instance
// Similar to TypeScript's console global object
var defaultLog Logger = &defaultLogger{}
