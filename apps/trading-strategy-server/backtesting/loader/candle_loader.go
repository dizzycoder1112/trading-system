package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"dizzycode.xyz/shared/domain/value_objects"
)

// OKXResponse OKX API 返回格式
type OKXResponse struct {
	Code string     `json:"code"`
	Msg  string     `json:"msg"`
	Data [][]string `json:"data"` // OKX 返回的是字符串數組
}

// OKXCandle OKX K線數據格式
// 數組索引：[ts, o, h, l, c, vol, volCcy, volCcyQuote, confirm]
type OKXCandle struct {
	Timestamp string // [0] 時間戳（毫秒）
	Open      string // [1] 開盤價
	High      string // [2] 最高價
	Low       string // [3] 最低價
	Close     string // [4] 收盤價
	// 其他字段暫時不需要
}

// CandleLoader K線數據加載器
type CandleLoader struct {
	filepath string
}

// NewCandleLoader 創建加載器
func NewCandleLoader(filepath string) *CandleLoader {
	return &CandleLoader{
		filepath: filepath,
	}
}

// Load 載入歷史K線數據
// 返回：Candle切片（從舊到新排序）
func (l *CandleLoader) Load() ([]value_objects.Candle, error) {
	// 1. 讀取文件
	data, err := os.ReadFile(l.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 2. 解析 JSON
	var response OKXResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 3. 檢查響應
	if response.Code != "0" {
		return nil, fmt.Errorf("OKX error: %s", response.Msg)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no data in file")
	}

	// 4. 轉換為 Candle 對象
	candles := make([]value_objects.Candle, 0, len(response.Data))

	for i, row := range response.Data {
		// 驗證數組長度
		if len(row) < 5 {
			return nil, fmt.Errorf("invalid candle at index %d: insufficient fields", i)
		}

		candle, err := l.parseOKXCandle(row)
		if err != nil {
			return nil, fmt.Errorf("failed to parse candle at index %d: %w", i, err)
		}

		candles = append(candles, candle)
	}

	// 5. 反轉順序（OKX 是從新到舊，我們需要從舊到新）
	reversed := make([]value_objects.Candle, len(candles))
	for i := range candles {
		reversed[i] = candles[len(candles)-1-i]
	}

	return reversed, nil
}

// parseOKXCandle 解析 OKX K線數據為 Candle 對象
func (l *CandleLoader) parseOKXCandle(row []string) (value_objects.Candle, error) {
	// 解析時間戳（毫秒）
	tsMs, err := strconv.ParseInt(row[0], 10, 64)
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	timestamp := time.UnixMilli(tsMs)

	// 解析價格
	open, err := strconv.ParseFloat(row[1], 64)
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("invalid open price: %w", err)
	}

	high, err := strconv.ParseFloat(row[2], 64)
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("invalid high price: %w", err)
	}

	low, err := strconv.ParseFloat(row[3], 64)
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("invalid low price: %w", err)
	}

	close, err := strconv.ParseFloat(row[4], 64)
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("invalid close price: %w", err)
	}

	// 創建 Candle 值對象
	candle, err := value_objects.NewCandle(open, high, low, close, timestamp)
	if err != nil {
		return value_objects.Candle{}, fmt.Errorf("failed to create candle: %w", err)
	}

	return candle, nil
}

// LoadFromJSON 便捷函數：從 JSON 文件加載 K 線數據
func LoadFromJSON(filepath string) ([]value_objects.Candle, error) {
	loader := NewCandleLoader(filepath)
	return loader.Load()
}
