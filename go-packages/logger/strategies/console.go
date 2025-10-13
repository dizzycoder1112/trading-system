package strategies

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"dizzycode.xyz/logger/level"
)

// Console implements the Strategy interface using standard output
// This is a lightweight strategy suitable for testing and simple applications
type Console struct {
	colored    bool
	prettyJSON bool
}

// ConsoleOptions configures the Console strategy
type ConsoleOptions struct {
	// Colored enables colored output (default: true if TTY)
	Colored bool
	// PrettyJSON enables pretty-printed JSON for fields (default: false)
	PrettyJSON bool
}

// NewConsole creates a new Console strategy
func NewConsole(opts ...ConsoleOptions) *Console {
	colored := true    // default to colored
	prettyJSON := true // default to pretty JSON

	if len(opts) > 0 {
		colored = opts[0].Colored
		prettyJSON = opts[0].PrettyJSON
	}

	return &Console{
		colored:    colored,
		prettyJSON: prettyJSON,
	}
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

func (c *Console) levelColor(lvl level.Level) string {
	if !c.colored {
		return ""
	}

	switch lvl {
	case level.Debug:
		return colorGray
	case level.Info:
		return colorBlue
	case level.Warn:
		return colorYellow
	case level.Error, level.Fatal:
		return colorRed
	default:
		return ""
	}
}

func (c *Console) levelString(lvl level.Level) string {
	s := lvl.String()
	if c.colored {
		return c.levelColor(lvl) + s + colorReset
	}
	return s
}

// Log implements the Strategy interface
func (c *Console) Log(entry Entry) error {
	timestamp := entry.Time.Format(time.RFC3339)
	levelStr := c.levelString(entry.Level)

	// First line: timestamp LEVEL: message
	fmt.Fprintf(os.Stdout, "%s %s: %s\n",
		timestamp,
		levelStr,
		entry.Message,
	)

	// Print fields as JSON if present
	if len(entry.Fields) > 0 {
		c.printFieldsJSON(entry)
	}

	return nil
}

// printFieldsJSON prints fields as JSON
func (c *Console) printFieldsJSON(entry Entry) {
	// Convert zap fields to map
	fieldsMap := make(map[string]interface{})

	// Add service name
	fieldsMap["service"] = entry.ServiceName

	// Add all fields
	for _, field := range entry.Fields {
		fieldsMap[field.Key] = field.Interface
	}

	// Marshal to JSON
	var jsonBytes []byte
	var err error

	if c.prettyJSON {
		jsonBytes, err = json.MarshalIndent(fieldsMap, "", "  ")
	} else {
		jsonBytes, err = json.Marshal(fieldsMap)
	}

	if err != nil {
		fmt.Fprintf(os.Stdout, "  (failed to marshal fields: %v)\n", err)
		return
	}

	fmt.Fprintf(os.Stdout, "%s\n", string(jsonBytes))
}

// Sync implements the Strategy interface
func (c *Console) Sync() error {
	// os.Stdout doesn't need explicit syncing
	return nil
}
