package value_objects

import (
	"encoding/json"
	"time"
)

// SignalAction 信號動作類型
type SignalAction string

const (
	ActionBuy  SignalAction = "BUY"
	ActionSell SignalAction = "SELL"
)

// Signal 交易信號值對象
// 特點：不可變、包含完整信號信息
type Signal struct {
	action       SignalAction
	instID       string
	price        Price   // 開倉價格
	positionSize float64 // 名義倉位大小（美元）
	takeProfit   float64 // 停利百分比（例如 0.002 = 0.2%）
	timestamp    time.Time
	reason       string
}

// NewSignal 創建信號（工廠方法）
func NewSignal(action SignalAction, instID string, price Price, positionSize float64, takeProfit float64, reason string) Signal {
	return Signal{
		action:       action,
		instID:       instID,
		price:        price,
		positionSize: positionSize,
		takeProfit:   takeProfit,
		timestamp:    time.Now(),
		reason:       reason,
	}
}

// Getters
func (s Signal) Action() SignalAction  { return s.action }
func (s Signal) InstID() string        { return s.instID }
func (s Signal) Price() Price          { return s.price }
func (s Signal) PositionSize() float64 { return s.positionSize }
func (s Signal) TakeProfit() float64   { return s.takeProfit }
func (s Signal) Timestamp() time.Time  { return s.timestamp }
func (s Signal) Reason() string        { return s.reason }

// MarshalJSON 自定義 JSON 序列化（用於發布到 Redis）
func (s Signal) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"action":       string(s.action),
		"instId":       s.instID,
		"price":        s.price.Value(),
		"positionSize": s.positionSize,
		"takeProfit":   s.takeProfit,
		"timestamp":    s.timestamp.Format(time.RFC3339),
		"reason":       s.reason,
	})
}
