package websocket

import (
	"context"
	"fmt"
	"time"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/config"
	"dizzycoder.xyz/market-data-service/internal/okx"
	"dizzycoder.xyz/market-data-service/internal/redis"
)

// Setup 設置並返回配置好的 WebSocket Manager
func Setup(cfg *config.Config, log logger.Logger, publisher *redis.Publisher) (*Manager, error) {
	// 1. 創建 WebSocket 管理器
	wsManager := NewManager(Config{
		URL:    okx.BusinessWSURL,
		Logger: log,
	})

	// 2. 根據配置添加 Ticker 處理器
	if cfg.OKX.Subscription.Ticker {
		setupTickerHandler(wsManager, log, publisher)
		log.Info("Ticker handler registered")
	}

	// 3. 根據配置添加 Candle 處理器
	if len(cfg.OKX.Subscription.Candles) > 0 {
		setupCandleHandler(wsManager, log, publisher)
		log.Info("Candle handler registered", map[string]any{
			"periods": getEnabledCandles(cfg.OKX.Subscription.Candles),
		})
	}

	// 4. 連接到 OKX WebSocket
	if err := wsManager.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to OKX WebSocket: %w", err)
	}

	// 5. 根據配置訂閱交易對
	if err := subscribeInstruments(wsManager, cfg, log); err != nil {
		wsManager.Close()
		return nil, err
	}

	return wsManager, nil
}

// setupTickerHandler 設置 Ticker 數據處理器
func setupTickerHandler(wsManager *Manager, log logger.Logger, publisher *redis.Publisher) {
	wsManager.AddTickerHandler(func(ticker okx.Ticker) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 發布到 Redis Pub/Sub
		if err := publisher.PublishTicker(ctx, ticker); err != nil {
			return err
		}

		// 快取最新價格
		if err := publisher.CacheLatestTicker(ctx, ticker); err != nil {
			// 快取失敗不影響 Pub/Sub，只記錄錯誤
			log.Error("Failed to cache ticker", map[string]any{
				"error":  err,
				"instId": ticker.InstID,
			})
		}

		return nil
	})
}

// setupCandleHandler 設置 Candle K線數據處理器
func setupCandleHandler(wsManager *Manager, log logger.Logger, publisher *redis.Publisher) {
	wsManager.AddCandleHandler(func(candle okx.Candle) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 發布到 Redis Pub/Sub
		if err := publisher.PublishCandle(ctx, candle); err != nil {
			return err
		}

		// 快取最新 K線（只快取已完成的 K線）
		if candle.IsConfirmed() {
			if err := publisher.CacheLatestCandle(ctx, candle); err != nil {
				// 快取失敗不影響 Pub/Sub，只記錄錯誤
				log.Error("Failed to cache candle", map[string]any{
					"error":  err,
					"instId": candle.InstID,
					"bar":    candle.Bar,
				})
			}
		}

		return nil
	})
}

// subscribeInstruments 根據配置訂閱交易對
func subscribeInstruments(wsManager *Manager, cfg *config.Config, log logger.Logger) error {
	for _, instID := range cfg.OKX.Instruments {
		// 訂閱 Ticker（如果啟用）
		if cfg.OKX.Subscription.Ticker {
			if err := wsManager.SubscribeTicker(instID); err != nil {
				log.Error("Failed to subscribe to ticker", map[string]any{
					"error":  err,
					"instId": instID,
				})
				// 繼續訂閱其他項目，不中斷
			}
		}

		// 訂閱 K線（根據配置的週期）
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

	return nil
}

// getEnabledCandles 獲取已啟用的 K線週期列表
func getEnabledCandles(candles map[string]bool) []string {
	result := []string{}
	for bar := range candles {
		result = append(result, bar)
	}
	return result
}
