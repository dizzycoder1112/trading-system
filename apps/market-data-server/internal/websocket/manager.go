package websocket

import (
	"encoding/json"

	"dizzycode.xyz/logger"
	ws "dizzycode.xyz/websocket"
	"dizzycoder.xyz/market-data-service/internal/okx"
)

// TickerHandler 處理 Ticker 數據的回調
type TickerHandler func(ticker okx.Ticker) error

// CandleHandler 處理 Candle K線數據的回調
type CandleHandler func(candle okx.Candle) error

// Manager WebSocket 管理器，封裝業務邏輯
type Manager struct {
	client         *ws.Client
	logger         logger.Logger
	tickerHandlers []TickerHandler
	candleHandlers []CandleHandler
	subscriptions  map[string]bool // 記錄已訂閱的交易對
}

// Config 管理器配置
type Config struct {
	URL    string
	Logger logger.Logger
}

// NewManager 創建新的 WebSocket 管理器
func NewManager(config Config) *Manager {
	// 直接傳入 logger，不需要 adapter！
	// WebSocket client 會自動 fallback 到 console 如果 logger 是 nil
	wsClient := ws.NewClient(ws.Config{
		URL: config.URL,
	}, config.Logger)

	manager := &Manager{
		client:         wsClient,
		logger:         config.Logger,
		tickerHandlers: make([]TickerHandler, 0),
		candleHandlers: make([]CandleHandler, 0),
		subscriptions:  make(map[string]bool),
	}

	// 設置消息處理器
	wsClient.SetMessageHandler(manager.handleMessage)

	return manager
}

// AddTickerHandler 添加 Ticker 數據處理器
func (m *Manager) AddTickerHandler(handler TickerHandler) {
	m.tickerHandlers = append(m.tickerHandlers, handler)
}

// AddCandleHandler 添加 Candle K線數據處理器
func (m *Manager) AddCandleHandler(handler CandleHandler) {
	m.candleHandlers = append(m.candleHandlers, handler)
}

// Connect 連接到 OKX WebSocket
func (m *Manager) Connect() error {
	return m.client.Connect()
}

// SubscribeTicker 訂閱 Ticker 頻道
func (m *Manager) SubscribeTicker(instID string) error {
	req := okx.NewSubscribeRequest("tickers", instID)

	if err := m.client.SendJSON(req); err != nil {
		return err
	}

	m.subscriptions[instID] = true
	m.logger.Info("Subscribed to ticker", "instId", instID)

	return nil
}

// UnsubscribeTicker 取消訂閱 Ticker
func (m *Manager) UnsubscribeTicker(instID string) error {
	req := okx.NewUnsubscribeRequest("tickers", instID)

	if err := m.client.SendJSON(req); err != nil {
		return err
	}

	delete(m.subscriptions, instID)
	m.logger.Info("Unsubscribed from ticker", "instId", instID)

	return nil
}

// SubscribeCandle 訂閱 K線頻道
// bar: 時間週期，例如 "1m", "5m", "1H", "1D"
func (m *Manager) SubscribeCandle(instID, bar string) error {
	req := okx.NewCandleSubscribeRequest(instID, bar)

	if err := m.client.SendJSON(req); err != nil {
		return err
	}

	key := instID + ":" + bar
	m.subscriptions[key] = true
	m.logger.Info("Subscribed to candle", "instId", instID, "bar", bar)

	return nil
}

// UnsubscribeCandle 取消訂閱 K線
func (m *Manager) UnsubscribeCandle(instID, bar string) error {
	req := okx.NewCandleUnsubscribeRequest(instID, bar)

	if err := m.client.SendJSON(req); err != nil {
		return err
	}

	key := instID + ":" + bar
	delete(m.subscriptions, key)
	m.logger.Info("Unsubscribed from candle", "instId", instID, "bar", bar)

	return nil
}

// handleMessage 處理接收到的 WebSocket 消息
func (m *Manager) handleMessage(messageType int, data []byte) error {
	// Debug: 打印所有原始消息（只在 LOG_LEVEL=debug 時顯示）
	m.logger.Debug("Raw WebSocket message", "data", string(data))

	// 先嘗試解析基本響應結構
	var baseResp struct {
		Event string          `json:"event,omitempty"`
		Code  string          `json:"code,omitempty"`
		Msg   string          `json:"msg,omitempty"`
		Arg   *okx.ChannelArg `json:"arg,omitempty"`
		Data  json.RawMessage `json:"data,omitempty"`
	}

	if err := json.Unmarshal(data, &baseResp); err != nil {
		m.logger.Error("Failed to unmarshal message", "error", err, "data", string(data))
		return err
	}

	// 處理錯誤事件
	if baseResp.Event == "error" {
		m.logger.Error("WebSocket error from OKX",
			"code", baseResp.Code,
			"msg", baseResp.Msg)
		return nil // 不中斷連接，繼續處理其他消息
	}

	// 處理訂閱響應
	if baseResp.Event == "subscribe" {
		// OKX 訂閱成功時 Code 為空或 "0"
		if baseResp.Code == "" || baseResp.Code == "0" {
			m.logger.Info("Subscription confirmed",
				"channel", baseResp.Arg.Channel,
				"instId", baseResp.Arg.InstID)
		} else {
			m.logger.Error("Subscription failed",
				"code", baseResp.Code,
				"msg", baseResp.Msg)
		}
		return nil
	}

	// 處理取消訂閱響應
	if baseResp.Event == "unsubscribe" {
		if baseResp.Code == "0" || baseResp.Code == "" {
			m.logger.Info("Unsubscription confirmed",
				"channel", baseResp.Arg.Channel,
				"instId", baseResp.Arg.InstID)
		}
		return nil
	}

	// 根據 channel 類型處理數據
	if baseResp.Arg != nil && len(baseResp.Data) > 0 {
		channel := baseResp.Arg.Channel

		// 處理 Ticker 數據
		if channel == "tickers" {
			var tickers []okx.Ticker
			if err := json.Unmarshal(baseResp.Data, &tickers); err != nil {
				m.logger.Error("Failed to unmarshal ticker data", "error", err)
				return err
			}

			for _, ticker := range tickers {
				// Manager 自動打印接收到的 Ticker 數據
				m.logger.Info("Received ticker",
					"instId", ticker.InstID,
					"last", ticker.Last,
					"volume24h", ticker.Vol24h)

				// 調用用戶註冊的 handler（用於業務邏輯，如發布到 Redis）
				for _, handler := range m.tickerHandlers {
					if err := handler(ticker); err != nil {
						m.logger.Error("Ticker handler error",
							"error", err,
							"instId", ticker.InstID)
					}
				}
			}
		}

		// 處理 Candle 數據（channel 格式: candle1m, candle5m, etc）
		if len(channel) > 6 && channel[:6] == "candle" {
			// OKX 返回的 Candle 數據是數組格式: [[ts, o, h, l, c, vol, volCcy, volCcyQuote, confirm], ...]
			var candleRaws []okx.CandleRaw
			if err := json.Unmarshal(baseResp.Data, &candleRaws); err != nil {
				m.logger.Error("Failed to unmarshal candle data", "error", err)
				return err
			}

			bar := channel[6:] // 提取週期，例如 "1m", "5m"
			for _, candleRaw := range candleRaws {
				candle, err := okx.ParseCandle(candleRaw, baseResp.Arg.InstID, bar)
				if err != nil {
					m.logger.Error("Failed to parse candle", "error", err)
					continue
				}

				// Manager 自動打印接收到的 Candle 數據
				m.logger.Info("Received candle",
					"instId", candle.InstID,
					"bar", candle.Bar,
					"open", candle.Open,
					"high", candle.High,
					"low", candle.Low,
					"close", candle.Close,
					"volume", candle.Vol,
					"confirm", candle.Confirm)

				// 調用用戶註冊的 handler（用於業務邏輯，如發布到 Redis）
				for _, handler := range m.candleHandlers {
					if err := handler(*candle); err != nil {
						m.logger.Error("Candle handler error",
							"error", err,
							"instId", candle.InstID,
							"bar", candle.Bar)
					}
				}
			}
		}
	}

	return nil
}

// Close 關閉 WebSocket 連接
func (m *Manager) Close() error {
	return m.client.Close()
}

// IsConnected 檢查連接狀態
func (m *Manager) IsConnected() bool {
	return m.client.IsConnected()
}

// Wait 等待連接關閉
func (m *Manager) Wait() {
	m.client.Wait()
}
