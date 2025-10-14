package main

import (
	"os"
	"os/signal"
	"syscall"

	"dizzycoder.xyz/market-data-service/internal/config"
	"dizzycoder.xyz/market-data-service/internal/logger"
	"dizzycoder.xyz/market-data-service/internal/redis"
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

	// 3. 創建 Redis 客戶端
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

	// 4. 創建 Redis 發布器
	publisher := redis.NewPublisher(redisClient, log)

	// 5. 設置 WebSocket 管理器（包含連接、註冊 handler、訂閱）
	wsManager, err := websocket.Setup(cfg, log, publisher)
	if err != nil {
		log.Error("Failed to setup WebSocket manager", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}
	defer wsManager.Close()

	log.Info("Market Data Service started successfully", map[string]any{
		"instruments": cfg.OKX.Instruments,
	})

	// 6. 等待退出信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Market Data Service...")
}
