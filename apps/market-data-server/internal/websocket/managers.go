package websocket

// Managers WebSocket 管理器集合
//
// 因为 OKX 的 Ticker 和 Candle 使用不同的 WebSocket URL：
// - Ticker: wss://ws.okx.com:8443/ws/v5/public
// - Candle: wss://ws.okx.com:8443/ws/v5/business
//
// 所以需要两个独立的 Manager 实例
type Managers struct {
	Ticker *Manager // Ticker 数据管理器（可能为 nil）
	Candle *Manager // Candle 数据管理器（可能为 nil）
}

// Close 关闭所有 WebSocket 连接
func (m *Managers) Close() error {
	var err error

	if m.Ticker != nil {
		if e := m.Ticker.Close(); e != nil {
			err = e
		}
	}

	if m.Candle != nil {
		if e := m.Candle.Close(); e != nil {
			err = e
		}
	}

	return err
}

// Wait 等待所有连接关闭
func (m *Managers) Wait() {
	if m.Ticker != nil {
		m.Ticker.Wait()
	}

	if m.Candle != nil {
		m.Candle.Wait()
	}
}
