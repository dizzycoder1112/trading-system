package websocket

import (
	"dizzycoder.xyz/market-data-service/internal/okx"
	"dizzycode.xyz/logger"
	ws "dizzycode.xyz/websocket"
	"encoding/json"

	"go.uber.org/zap"
)

// TickerHandler 處理 Ticker 數據的回調
type TickerHandler func(ticker okx.Ticker) error

// Manager WebSocket 管理器，封裝業務邏輯
type Manager struct {
	client         *ws.Client
	logger         *logger.Logger
	tickerHandlers []TickerHandler
	subscriptions  map[string]bool // 記錄已訂閱的交易對
}

// Config 管理器配置
type Config struct {
	URL    string
	Logger *logger.Logger
}

// NewManager 創建新的 WebSocket 管理器
func NewManager(config Config) *Manager {
	// 創建 logger adapter（將自定義 logger 適配為通用 websocket.Logger 介面）
	logAdapter := newLoggerAdapter(config.Logger)

	// 創建通用 WebSocket 客戶端
	wsClient := ws.NewClient(ws.Config{
		URL: config.URL,
	}, logAdapter)

	manager := &Manager{
		client:         wsClient,
		logger:         config.Logger,
		tickerHandlers: make([]TickerHandler, 0),
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
	m.logger.Info("Subscribed to ticker", zap.String("instId", instID))

	return nil
}

// UnsubscribeTicker 取消訂閱 Ticker
func (m *Manager) UnsubscribeTicker(instID string) error {
	req := okx.NewUnsubscribeRequest("tickers", instID)

	if err := m.client.SendJSON(req); err != nil {
		return err
	}

	delete(m.subscriptions, instID)
	m.logger.Info("Unsubscribed from ticker", zap.String("instId", instID))

	return nil
}

// handleMessage 處理接收到的 WebSocket 消息
func (m *Manager) handleMessage(messageType int, data []byte) error {
	var resp okx.WSResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		m.logger.Error("Failed to unmarshal message", err, zap.ByteString("data", data))
		return err
	}

	// 處理訂閱響應
	if resp.Event == "subscribe" {
		if resp.Code == "0" {
			m.logger.Info("Subscription confirmed",
				zap.String("channel", resp.Arg.Channel),
				zap.String("instId", resp.Arg.InstID))
		} else {
			m.logger.Error("Subscription failed", nil,
				zap.String("code", resp.Code),
				zap.String("msg", resp.Msg))
		}
		return nil
	}

	// 處理取消訂閱響應
	if resp.Event == "unsubscribe" {
		if resp.Code == "0" {
			m.logger.Info("Unsubscription confirmed",
				zap.String("channel", resp.Arg.Channel),
				zap.String("instId", resp.Arg.InstID))
		}
		return nil
	}

	// 處理 Ticker 數據
	if len(resp.Data) > 0 {
		for _, ticker := range resp.Data {
			for _, handler := range m.tickerHandlers {
				if err := handler(ticker); err != nil {
					m.logger.Error("Ticker handler error", err,
						zap.String("instId", ticker.InstID))
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
