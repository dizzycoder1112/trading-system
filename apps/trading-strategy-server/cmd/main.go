package main

import (
	"os"
	"os/signal"
	"syscall"

	"dizzycoder.xyz/trading-strategy-server/internal/infrastructure/config"
	"dizzycoder.xyz/trading-strategy-server/internal/infrastructure/logger"
)

func main() {
	// 1. 載入配置
	cfg := config.Load()

	// 2. 創建 logger
	log := logger.Must(cfg)

	log.Info("Starting Trading Strategy Server", map[string]any{
		"environment": cfg.Environment,
		"port":        cfg.Port,
		"strategy":    cfg.Strategy.Type,
	})

	// 3. TODO: 創建 Redis 客戶端
	// redisClient, err := redis.NewClient(...)

	// 4. TODO: 創建 Redis 訂閱器
	// subscriber := redis.NewSubscriber(...)

	// 5. TODO: 創建策略引擎
	// strategyEngine := strategy.NewEngine(...)

	// 6. TODO: 啟動策略引擎
	// strategyEngine.Start()

	log.Info("Trading Strategy Server started successfully", map[string]any{
		"instruments": cfg.Strategy.Instruments,
	})

	// 7. 等待退出信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Trading Strategy Server...")
}
