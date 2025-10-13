package strategies

import (
	"time"

	"dizzycode.xyz/logger/level"

	"go.uber.org/zap"
)

// Entry represents a single log entry
type Entry struct {
	Level       level.Level
	Message     string
	Fields      []zap.Field
	Time        time.Time
	ServiceName string
}

// Strategy defines the interface for different logging strategies
type Strategy interface {
	Log(entry Entry) error
	Sync() error
}
