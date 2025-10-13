package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// 默認配置
	DefaultPingInterval = 20 * time.Second
	DefaultPongWait     = 30 * time.Second
	DefaultWriteWait    = 10 * time.Second
)

// MessageHandler 處理接收到的消息
type MessageHandler func(messageType int, data []byte) error

// Logger 日誌接口（讓使用者注入自己的 logger）
// 使用 interface{} 保持通用性，不綁定特定日誌庫
type Logger interface {
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
	Debug(msg string, fields ...any)
	Warn(msg string, fields ...any)
}

// Config WebSocket 客戶端配置
type Config struct {
	URL          string
	PingInterval time.Duration
	PongWait     time.Duration
	WriteWait    time.Duration
}

// Client 通用 WebSocket 客戶端
type Client struct {
	config         Config
	conn           *websocket.Conn
	logger         Logger
	mu             sync.RWMutex
	messageHandler MessageHandler
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	isConnected    bool
}

// NewClient 創建新的 WebSocket 客戶端
func NewClient(config Config, logger Logger) *Client {
	if config.PingInterval == 0 {
		config.PingInterval = DefaultPingInterval
	}
	if config.PongWait == 0 {
		config.PongWait = DefaultPongWait
	}
	if config.WriteWait == 0 {
		config.WriteWait = DefaultWriteWait
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		config:      config,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
		isConnected: false,
	}
}

// SetMessageHandler 設置消息處理器
func (c *Client) SetMessageHandler(handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messageHandler = handler
}

// Connect 連接到 WebSocket 服務器
func (c *Client) Connect() error {
	c.logger.Info("Connecting to WebSocket", "url", c.config.URL)

	conn, _, err := websocket.DefaultDialer.Dial(c.config.URL, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.mu.Unlock()

	c.logger.Info("Successfully connected to WebSocket")

	// 啟動消息處理協程
	go c.readPump()
	go c.pingPump()

	return nil
}

// SendJSON 發送 JSON 消息
func (c *Client) SendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected || c.conn == nil {
		return ErrNotConnected
	}

	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
	return c.conn.WriteJSON(v)
}

// SendMessage 發送原始消息
func (c *Client) SendMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected || c.conn == nil {
		return ErrNotConnected
	}

	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
	return c.conn.WriteMessage(messageType, data)
}

// readPump 讀取 WebSocket 消息
func (c *Client) readPump() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.isConnected = false
		c.mu.Unlock()
		close(c.done)
	}()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Error("WebSocket unexpected close", "error", err)
				}
				return
			}

			c.mu.RLock()
			handler := c.messageHandler
			c.mu.RUnlock()

			if handler != nil {
				if err := handler(messageType, message); err != nil {
					c.logger.Error("Message handler error", "error", err)
				}
			}
		}
	}
}

// pingPump 定期發送 ping 保持連接
func (c *Client) pingPump() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.conn != nil && c.isConnected {
				c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
				if err := c.conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
					c.logger.Error("Failed to send ping", "error", err)
					c.mu.Unlock()
					return
				}
			}
			c.mu.Unlock()
		}
	}
}

// Close 關閉 WebSocket 連接
func (c *Client) Close() error {
	c.logger.Info("Closing WebSocket connection")
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
		c.conn = nil
	}

	c.isConnected = false
	return nil
}

// IsConnected 檢查是否已連接
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}

// Wait 等待連接關閉
func (c *Client) Wait() {
	<-c.done
}
