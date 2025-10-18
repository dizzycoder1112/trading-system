package websocket

import (
	"fmt"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/config"
	"dizzycoder.xyz/market-data-service/internal/handler"
	"dizzycoder.xyz/market-data-service/internal/okx"
)

// Setup 設置並返回配置好的 WebSocket Managers
//
// 因為 OKX 的 Ticker 和 Candle 使用不同的 WebSocket URL，
// 需要創建兩個獨立的 Manager 實例：
// - Ticker: wss://ws.okx.com:8443/ws/v5/public
// - Candle: wss://ws.okx.com:8443/ws/v5/business
func Setup(
	cfg *config.Config,
	log logger.Logger,
	tickerHandler *handler.TickerHandler, // 注入 Ticker Handler
	candleHandler *handler.CandleHandler, // 注入 Candle Handler
) (*Managers, error) {
	managers := &Managers{}

	// 1. 根據配置創建並設置 Ticker Manager
	if cfg.OKX.Subscription.Ticker {
		tickerManager, err := setupTickerManager(cfg, log, tickerHandler)
		if err != nil {
			return nil, fmt.Errorf("failed to setup ticker manager: %w", err)
		}
		managers.Ticker = tickerManager
	}

	// 2. 根據配置創建並設置 Candle Manager
	if len(cfg.OKX.Subscription.Candles) > 0 {
		candleManager, err := setupCandleManager(cfg, log, candleHandler)
		if err != nil {
			// 如果 Candle Manager 創建失敗，關閉已創建的 Ticker Manager
			if managers.Ticker != nil {
				managers.Ticker.Close()
			}
			return nil, fmt.Errorf("failed to setup candle manager: %w", err)
		}
		managers.Candle = candleManager
	}

	return managers, nil
}

// setupTickerManager 設置 Ticker WebSocket Manager
func setupTickerManager(
	cfg *config.Config,
	log logger.Logger,
	tickerHandler *handler.TickerHandler,
) (*Manager, error) {
	// 1. 創建 WebSocket Manager（使用 Public URL）
	wsManager := NewManager(Config{
		URL:    okx.PublicWSURL, // Ticker 使用 Public WebSocket
		Logger: log,
	})

	// 2. 註冊 Ticker Handler
	wsManager.AddTickerHandler(tickerHandler.Handle)
	log.Info("Ticker handler registered")

	// 3. 連接到 OKX WebSocket
	if err := wsManager.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to OKX Public WebSocket: %w", err)
	}

	// 4. 訂閱 Ticker
	for _, instID := range cfg.OKX.Instruments {
		if err := wsManager.SubscribeTicker(instID); err != nil {
			log.Error("Failed to subscribe to ticker", map[string]any{
				"error":  err,
				"instId": instID,
			})
			// 繼續訂閱其他項目，不中斷
		}
	}

	return wsManager, nil
}

// setupCandleManager 設置 Candle WebSocket Manager
func setupCandleManager(
	cfg *config.Config,
	log logger.Logger,
	candleHandler *handler.CandleHandler,
) (*Manager, error) {
	// 1. 創建 WebSocket Manager（使用 Business URL）
	wsManager := NewManager(Config{
		URL:    okx.BusinessWSURL, // Candle 使用 Business WebSocket
		Logger: log,
	})

	// 2. 註冊 Candle Handler
	wsManager.AddCandleHandler(candleHandler.Handle)
	log.Info("Candle handler registered", map[string]any{
		"periods": getEnabledCandles(cfg.OKX.Subscription.Candles),
	})

	// 3. 連接到 OKX WebSocket
	if err := wsManager.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to OKX Business WebSocket: %w", err)
	}

	// 4. 訂閱 Candle
	for _, instID := range cfg.OKX.Instruments {
		for bar := range cfg.OKX.Subscription.Candles {
			if err := wsManager.SubscribeCandle(instID, bar); err != nil {
				log.Error("Failed to subscribe to candle", map[string]any{
					"error":  err,
					"instId": instID,
					"bar":    bar,
				})
				// 繼續訂閱其他項目，不中斷
			}
		}
	}

	return wsManager, nil
}

// getEnabledCandles 獲取已啟用的 K線週期列表
func getEnabledCandles(candles map[string]bool) []string {
	result := []string{}
	for bar := range candles {
		result = append(result, bar)
	}
	return result
}
