package main

import (
	"os"
	"os/signal"
	"syscall"

	"dizzycoder.xyz/market-data-service/internal/config"
	"dizzycoder.xyz/market-data-service/internal/logger"
	"dizzycoder.xyz/market-data-service/internal/okx"
	"dizzycoder.xyz/market-data-service/internal/websocket"
)

func main() {
	// 1. 載入配置
	cfg := config.Load()

	// 2. 創建 logger
	log := logger.Must(cfg)

	log.Info("Starting Market Data Service", map[string]any{
		"environment": cfg.Environment,
		"port":        cfg.Port,
	})

	// 3. 創建 WebSocket 管理器
	wsManager := websocket.NewManager(websocket.Config{
		URL:    okx.BusinessWSURL,
		Logger: log,
	})

	// 4. 添加 Ticker 數據處理器
	// Manager 會自動打印日誌，Handler 只需處理業務邏輯
	wsManager.AddTickerHandler(func(ticker okx.Ticker) error {
		// TODO: 將數據發布到 Redis Pub/Sub
		return nil
	})

	// 5. 添加 Candle K線數據處理器
	// Manager 會自動打印日誌，Handler 只需處理業務邏輯
	wsManager.AddCandleHandler(func(candle okx.Candle) error {
		// TODO: 將數據發布到 Redis Pub/Sub
		return nil
	})

	// 6. 連接到 OKX WebSocket
	if err := wsManager.Connect(); err != nil {
		log.Error("Failed to connect to OKX WebSocket", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}
	defer wsManager.Close()

	// 7. 訂閱配置中的交易對
	for _, instID := range cfg.OKX.Instruments {
		// 訂閱 Ticker（即時價格）
		// if err := wsManager.SubscribeTicker(instID); err != nil {
		// 	log.Error("Failed to subscribe to ticker", map[string]any{
		// 		"error":  err,
		// 		"instId": instID,
		// 	})
		// }

		// 訂閱 1 分鐘 K線
		if err := wsManager.SubscribeCandle(instID, "1m"); err != nil {
			log.Error("Failed to subscribe to candle", map[string]any{
				"error":  err,
				"instId": instID,
				"bar":    "1m",
			})
		}
	}

	log.Info("Market Data Service started successfully", map[string]any{
		"instruments": cfg.OKX.Instruments,
	})

	// 7. 等待退出信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Market Data Service...")
}
