package okx

import (
	"fmt"
	"time"
)

const (
	// OKX WebSocket URLs
	PublicWSURL  = "wss://ws.okx.com:8443/ws/v5/public"
	PrivateWSURL = "wss://ws.okx.com:8443/ws/v5/private"
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
