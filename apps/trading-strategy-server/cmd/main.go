package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"dizzycode.xyz/trading-strategy-server/internal/application"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/strategies/grid"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/value_objects"
	"dizzycode.xyz/trading-strategy-server/internal/infrastructure/config"
	"dizzycode.xyz/trading-strategy-server/internal/infrastructure/logger"
	"dizzycode.xyz/trading-strategy-server/internal/infrastructure/messaging"
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

	// 3. 創建 Redis 客戶端
	redisClient, err := messaging.NewRedisClient(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		log,
	)
	if err != nil {
		log.Error("Failed to connect to Redis", map[string]any{"error": err})
		os.Exit(1)
	}
	defer redisClient.Close()

	log.Info("Connected to Redis", map[string]any{"addr": cfg.Redis.Addr})

	// 4. 創建基礎設施層 - Signal 發布器
	signalPublisher := messaging.NewRedisSignalPublisher(redisClient, log)

	// 5. 創建領域層 - GridAggregate (one per instrument)
	// For now, handle first instrument only
	if len(cfg.Strategy.Instruments) == 0 {
		log.Error("No instruments configured", map[string]any{})
		os.Exit(1)
	}

	instID := cfg.Strategy.Instruments[0]
	grid, err := grid.NewGridAggregate(
		instID,
		cfg.Strategy.Grid.PositionSize,
		cfg.Strategy.Grid.TakeProfitMin,
		cfg.Strategy.Grid.TakeProfitMax,
	)
	if err != nil {
		log.Error("Failed to create grid aggregate", map[string]any{"error": err})
		os.Exit(1)
	}

	log.Info("Grid aggregate created", map[string]any{
		"instId":        instID,
		"positionSize":  cfg.Strategy.Grid.PositionSize,
		"takeProfitMin": cfg.Strategy.Grid.TakeProfitMin,
		"takeProfitMax": cfg.Strategy.Grid.TakeProfitMax,
	})

	// 6. 創建應用層 - StrategyService
	strategyService := application.NewStrategyService(grid, signalPublisher, log)

	// 7. 創建基礎設施層 - Candle 訂閱器
	candleSubscriber := messaging.NewCandleSubscriber(redisClient, log)

	// 8. 啟動訂閱循環
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := candleSubscriber.Subscribe(
			ctx,
			instID,
			"5m", // 5-minute candles as per strategy document
			func(candle value_objects.Candle) error {
				// Call application layer use case
				return strategyService.HandleCandleUpdate(ctx, candle)
			},
		); err != nil && err != context.Canceled {
			log.Error("Candle subscription failed", map[string]any{"error": err})
		}
	}()

	log.Info("Trading Strategy Server started successfully", map[string]any{
		"instruments": cfg.Strategy.Instruments,
		"listening":   "market.candle.5m." + instID,
	})

	// 9. 等待退出信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Trading Strategy Server...")
	cancel() // Cancel context to stop subscriptions
}
