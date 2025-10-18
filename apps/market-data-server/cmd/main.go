package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dizzycoder.xyz/market-data-service/internal/config"
	"dizzycoder.xyz/market-data-service/internal/handler"
	"dizzycoder.xyz/market-data-service/internal/logger"
	"dizzycoder.xyz/market-data-service/internal/redis"
	"dizzycoder.xyz/market-data-service/internal/storage"
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

	// 3. 創建 Redis 客戶端（直接返回 *redis.Client）
	redisClient, err := redis.NewClient(redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
		Logger:   log,
	})
	if err != nil {
		log.Error("Failed to connect to Redis", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}
	defer redisClient.Close()

	// 4. 創建 Storage 實現（可替換！）
	// 這裡使用 Redis，未來可以輕鬆替換為 Kafka, RabbitMQ 等
	marketStorage := storage.NewRedisStorage(redisClient, log)

	// 5. 創建數據保留策略
	retention := config.DefaultRetentionPolicy()

	// 6. 創建 Handlers（注入 storage）
	// 將依賴注入放在 main.go，讓依賴關係更清晰
	tickerHandler := handler.NewTickerHandler(marketStorage, log)
	candleHandler := handler.NewCandleHandler(marketStorage, retention, log)

	// 7. 設置 WebSocket 管理器（注入 handlers）
	// 返回 Managers 結構，包含 Ticker 和 Candle 兩個 Manager
	wsManagers, err := websocket.Setup(cfg, log, tickerHandler, candleHandler)
	if err != nil {
		log.Error("Failed to setup WebSocket managers", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}
	defer wsManagers.Close()

	log.Info("Market Data Service started successfully", map[string]any{
		"instruments": cfg.OKX.Instruments,
		"ticker":      cfg.OKX.Subscription.Ticker,
		"candles":     len(cfg.OKX.Subscription.Candles) > 0,
	})

	// 8. 等待退出信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Market Data Service...")

	// 9. 關閉前清理 Redis 中的市場數據
	// 防止策略服務讀到過時的價格數據
	log.Info("Cleaning up market data...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := marketStorage.Cleanup(ctx); err != nil {
		log.Error("Failed to cleanup market data", map[string]any{
			"error": err,
		})
		// 繼續關閉流程，不中斷
	}

	log.Info("Market data cleanup completed")
}
