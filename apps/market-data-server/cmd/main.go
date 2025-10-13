package main

import (
	"os"
	"os/signal"
	"syscall"

	"dizzycoder.xyz/market-data-service/internal/config"
	"dizzycoder.xyz/market-data-service/internal/logger"
	"dizzycoder.xyz/market-data-service/internal/okx"
	"dizzycoder.xyz/market-data-service/internal/websocket"

	"go.uber.org/zap"
)

func main() {
	// 1. 載入配置
	config.Load()
	log := logger.CreateLogger(config.AppConfig.LogLevel)
	defer log.Sync()

	log.Info("Starting Market Data Service",
		zap.String("environment", config.AppConfig.Environment),
		zap.String("port", config.AppConfig.Port))

	// 2. 創建 WebSocket 管理器
	wsManager := websocket.NewManager(websocket.Config{
		URL:    okx.PublicWSURL,
		Logger: log,
	})

	// 3. 添加 Ticker 數據處理器
	wsManager.AddTickerHandler(func(ticker okx.Ticker) error {
		log.Info("Received ticker",
			zap.String("instId", ticker.InstID),
			zap.String("last", ticker.Last),
			zap.String("volume24h", ticker.Vol24h))
		// TODO: 將數據發布到 Redis
		return nil
	})

	// 4. 連接到 OKX WebSocket
	if err := wsManager.Connect(); err != nil {
		log.Fatal("Failed to connect to OKX WebSocket", err)
	}
	defer wsManager.Close()

	// 5. 訂閱配置中的交易對
	for _, instID := range config.AppConfig.OKX.Instruments {
		if err := wsManager.SubscribeTicker(instID); err != nil {
			log.Error("Failed to subscribe to ticker", err, zap.String("instId", instID))
		}
	}

	log.Info("Market Data Service started successfully",
		zap.Strings("instruments", config.AppConfig.OKX.Instruments))

	// 6. 等待退出信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Market Data Service...")
}
