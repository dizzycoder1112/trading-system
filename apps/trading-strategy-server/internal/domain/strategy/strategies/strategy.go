package strategies

import "dizzycode.xyz/shared/domain/value_objects"

// Strategy 策略介面（多態）
// 所有策略必須實現此介面
type Strategy interface {
	// ProcessCandle 處理 K 線，返回信號
	// 如果沒有觸發條件，返回 nil
	ProcessCandle(candle value_objects.Candle) (*value_objects.Signal, error)

	// GetState 獲取策略狀態（用於監控和日誌）
	GetState() map[string]any

	// GetName 獲取策略名稱
	GetName() string
}
