package strategy

import (
	"errors"
	"fmt"
)

// Price 價格值對象
// 特點：不可變、帶業務規則、值比較
type Price struct {
	value float64
}

// NewPrice 創建價格值對象（工廠方法）
func NewPrice(value float64) (Price, error) {
	if value <= 0 {
		return Price{}, errors.New("price must be positive")
	}
	return Price{value: value}, nil
}

// Value 獲取價格值
func (p Price) Value() float64 {
	return p.value
}

// IsAbove 判斷是否高於另一個價格
func (p Price) IsAbove(other Price) bool {
	return p.value > other.value
}

// IsBelow 判斷是否低於另一個價格
func (p Price) IsBelow(other Price) bool {
	return p.value < other.value
}

// Equals 判斷是否等於另一個價格
func (p Price) Equals(other Price) bool {
	return p.value == other.value
}

// String 字符串表示
func (p Price) String() string {
	return fmt.Sprintf("%.2f", p.value)
}
