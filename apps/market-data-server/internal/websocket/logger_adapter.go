package websocket

import (
	"dizzycode.xyz/logger"
	"go.uber.org/zap"
)

// loggerAdapter 將自定義 logger 適配為 websocket.Logger 介面
// 負責將 interface{} 類型的 key-value pairs 轉換為 zap.Field
type loggerAdapter struct {
	logger *logger.Logger
}

func newLoggerAdapter(l *logger.Logger) *loggerAdapter {
	return &loggerAdapter{logger: l}
}

func (l *loggerAdapter) Info(msg string, fields ...any) {
	zapFields := convertToZapFields(fields)
	l.logger.Info(msg, zapFields...)
}

func (l *loggerAdapter) Error(msg string, fields ...any) {
	zapFields := convertToZapFields(fields)
	// 從 fields 中提取 error
	var err error
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			if fields[i] == "error" {
				if e, ok := fields[i+1].(error); ok {
					err = e
					break
				}
			}
		}
	}
	l.logger.Error(msg, err, zapFields...)
}

func (l *loggerAdapter) Debug(msg string, fields ...any) {
	zapFields := convertToZapFields(fields)
	l.logger.Debug(msg, zapFields...)
}

func (l *loggerAdapter) Warn(msg string, fields ...any) {
	zapFields := convertToZapFields(fields)
	l.logger.Warn(msg, zapFields...)
}

// convertToZapFields 將 key-value pairs 轉換為 zap.Field
// 預期格式: "key1", value1, "key2", value2, ...
func convertToZapFields(fields []any) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			break
		}

		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		value := fields[i+1]

		// 根據值類型創建對應的 zap.Field
		switch v := value.(type) {
		case string:
			zapFields = append(zapFields, zap.String(key, v))
		case int:
			zapFields = append(zapFields, zap.Int(key, v))
		case int64:
			zapFields = append(zapFields, zap.Int64(key, v))
		case error:
			if key == "error" {
				zapFields = append(zapFields, zap.Error(v))
			} else {
				zapFields = append(zapFields, zap.NamedError(key, v))
			}
		case bool:
			zapFields = append(zapFields, zap.Bool(key, v))
		case float64:
			zapFields = append(zapFields, zap.Float64(key, v))
		default:
			zapFields = append(zapFields, zap.Any(key, v))
		}
	}

	return zapFields
}
