package okx

import (
	"fmt"
	"time"
)

const (
	// OKX WebSocket URLs
	PublicWSURL   = "wss://ws.okx.com:8443/ws/v5/public"   // Ticker, OrderBook 等公共數據
	BusinessWSURL = "wss://ws.okx.com:8443/ws/v5/business" // Candle K線數據
	PrivateWSURL  = "wss://ws.okx.com:8443/ws/v5/private"  // 私有交易數據
)

// ============ WebSocket 請求/響應結構 ============

// SubscribeRequest WebSocket 訂閱請求
type SubscribeRequest struct {
	Op   string         `json:"op"`
	Args []SubscribeArg `json:"args"`
}

// SubscribeArg 訂閱參數
type SubscribeArg struct {
	Channel string `json:"channel"`
	InstID  string `json:"instId"`
	Bar     string `json:"bar,omitempty"` // K線週期，如 1m, 5m, 1H, 1D
}

// WSResponse WebSocket 響應
type WSResponse struct {
	Event string      `json:"event,omitempty"` // subscribe, unsubscribe, error
	Code  string      `json:"code,omitempty"`  // 0 = success
	Msg   string      `json:"msg,omitempty"`
	Arg   *ChannelArg `json:"arg,omitempty"`
	Data  []Ticker    `json:"data,omitempty"`
}

// ChannelArg 頻道參數
type ChannelArg struct {
	Channel string `json:"channel"`
	InstID  string `json:"instId"`
	Bar     string `json:"bar,omitempty"` // K線週期
}

// ============ Ticker 數據結構 ============

// Ticker 價格數據
// 文檔: https://www.okx.com/docs-v5/en/#public-data-websocket-tickers-channel
type Ticker struct {
	InstType  string `json:"instType"`  // 產品類型: SPOT, SWAP, FUTURES, OPTION
	InstID    string `json:"instId"`    // 產品ID，如 BTC-USDT
	Last      string `json:"last"`      // 最新成交價
	LastSz    string `json:"lastSz"`    // 最新成交數量
	AskPx     string `json:"askPx"`     // 賣一價
	AskSz     string `json:"askSz"`     // 賣一數量
	BidPx     string `json:"bidPx"`     // 買一價
	BidSz     string `json:"bidSz"`     // 買一數量
	Open24h   string `json:"open24h"`   // 24小時開盤價
	High24h   string `json:"high24h"`   // 24小時最高價
	Low24h    string `json:"low24h"`    // 24小時最低價
	VolCcy24h string `json:"volCcy24h"` // 24小時成交量（計價幣）
	Vol24h    string `json:"vol24h"`    // 24小時成交量（交易幣）
	Ts        string `json:"ts"`        // Ticker 數據產生時間（毫秒時間戳）
	SodUtc0   string `json:"sodUtc0"`   // UTC 0 時開盤價
	SodUtc8   string `json:"sodUtc8"`   // UTC+8 時開盤價
}

// GetTimestamp 將 ts 字符串轉換為 time.Time
func (t *Ticker) GetTimestamp() (time.Time, error) {
	var ts int64
	_, err := fmt.Sscanf(t.Ts, "%d", &ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	return time.UnixMilli(ts), nil
}

// ============ Candle K線數據結構 ============

// CandleRaw K線原始數據（數組格式）
// 文檔: https://www.okx.com/docs-v5/en/#order-book-trading-market-data-ws-candlesticks-channel
// 數據格式為數組: [ts, open, high, low, close, vol, volCcy, volCcyQuote, confirm]
type CandleRaw []string

// Candle K線數據（解析後的結構）
type Candle struct {
	Ts          string // K線開始時間（毫秒時間戳）
	Open        string // 開盤價
	High        string // 最高價
	Low         string // 最低價
	Close       string // 收盤價
	Vol         string // 交易量（交易幣）
	VolCcy      string // 交易量（計價幣）
	VolCcyQuote string // 交易量（以USD計價）
	Confirm     string // K線狀態：0-未完成，1-已完成
	InstID      string // 產品ID（從 arg 中獲取，用於識別）
	Bar         string // K線週期（從 arg 中獲取）
}

// ParseCandle 將原始數組數據解析為 Candle 結構
func ParseCandle(raw CandleRaw, instID, bar string) (*Candle, error) {
	if len(raw) < 9 {
		return nil, fmt.Errorf("invalid candle data length: %d", len(raw))
	}
	return &Candle{
		Ts:          raw[0],
		Open:        raw[1],
		High:        raw[2],
		Low:         raw[3],
		Close:       raw[4],
		Vol:         raw[5],
		VolCcy:      raw[6],
		VolCcyQuote: raw[7],
		Confirm:     raw[8],
		InstID:      instID,
		Bar:         bar,
	}, nil
}

// GetTimestamp 將 ts 字符串轉換為 time.Time
func (c *Candle) GetTimestamp() (time.Time, error) {
	var ts int64
	_, err := fmt.Sscanf(c.Ts, "%d", &ts)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	return time.UnixMilli(ts), nil
}

// IsConfirmed 返回K線是否已完成
func (c *Candle) IsConfirmed() bool {
	return c.Confirm == "1"
}

// ============ 輔助函數 ============

// NewSubscribeRequest 創建訂閱請求
func NewSubscribeRequest(channel, instID string) SubscribeRequest {
	return SubscribeRequest{
		Op: "subscribe",
		Args: []SubscribeArg{
			{
				Channel: channel,
				InstID:  instID,
			},
		},
	}
}

// NewCandleSubscribeRequest 創建K線訂閱請求
func NewCandleSubscribeRequest(instID, bar string) SubscribeRequest {
	return SubscribeRequest{
		Op: "subscribe",
		Args: []SubscribeArg{
			{
				Channel: "candle" + bar, // 例如: candle1m, candle5m
				InstID:  instID,
			},
		},
	}
}

// NewUnsubscribeRequest 創建取消訂閱請求
func NewUnsubscribeRequest(channel, instID string) SubscribeRequest {
	return SubscribeRequest{
		Op: "unsubscribe",
		Args: []SubscribeArg{
			{
				Channel: channel,
				InstID:  instID,
			},
		},
	}
}

// NewCandleUnsubscribeRequest 創建取消K線訂閱請求
func NewCandleUnsubscribeRequest(instID, bar string) SubscribeRequest {
	return SubscribeRequest{
		Op: "unsubscribe",
		Args: []SubscribeArg{
			{
				Channel: "candle" + bar,
				InstID:  instID,
			},
		},
	}
}
