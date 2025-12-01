package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dizzycode.xyz/trading-strategy-server/internal/application"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/strategies/grid"
	"dizzycode.xyz/trading-strategy-server/internal/infrastructure/config"
	"dizzycode.xyz/trading-strategy-server/internal/infrastructure/logger"
	"dizzycode.xyz/trading-strategy-server/internal/infrastructure/messaging"
)

const (
	positionSize = 200.0 // 固定單次開倉大小為 200 美元
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

	// 4. 創建基礎設施層 - Market Data Reader ⭐
	dataReader := messaging.NewMarketDataReader(redisClient, log)

	// 5. 創建領域層 - GridAggregate
	if len(cfg.Strategy.Instruments) == 0 {
		log.Error("No instruments configured", map[string]any{})
		os.Exit(1)
	}

	instID := cfg.Strategy.Instruments[0]
	gridAggregate, err := grid.NewGridAggregate(
		grid.GridConfig{
			InstID:             instID,
			PositionSize:       positionSize, // 固定單次開倉大小為 200 美元
			FeeRate:            0.0005,       // 0.05% OKX Taker fee
			TakeProfitRateMin:  cfg.Strategy.Grid.TakeProfitMin,
			TakeProfitRateMax:  cfg.Strategy.Grid.TakeProfitMax,
			BreakEvenProfitMin: 0,
			BreakEvenProfitMax: 20,
		})
	if err != nil {
		log.Error("Failed to create grid aggregate", map[string]any{"error": err})
		os.Exit(1)
	}

	log.Info("Grid aggregate created", map[string]any{
		"instId":             gridAggregate.InstID,
		"positionSize":       gridAggregate.PositionSize,
		"TakeProfitRateMin":  gridAggregate.TakeProfitRateMin,
		"TakeProfitRateMax":  gridAggregate.TakeProfitRateMax,
		"BreakEvenProfitMin": gridAggregate.BreakEvenProfitMin,
		"BreakEvenProfitMax": gridAggregate.BreakEvenProfitMax,
	})

	// 6. 創建應用層 - StrategyService ⭐
	strategyService := application.NewStrategyService(gridAggregate, dataReader, log)

	log.Info("Trading Strategy Server started successfully", map[string]any{
		"mode":        "passive_advisory", // 被動諮詢模式
		"instId":      instID,
		"description": "Waiting for Order Service requests",
	})

	// 7. 模擬 Order Service 請求循環 ⭐
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(5 * time.Second) // 每 5 秒詢問一次
		defer ticker.Stop()

		log.Info("Order Service simulation: Started", map[string]any{
			"interval": "5 seconds",
		})

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 模擬：從 Redis 讀取當前價格
				currentPrice, err := dataReader.GetLatestPrice(ctx, instID)
				if err != nil {
					log.Warn("Failed to get current price", map[string]any{
						"error": err,
					})
					continue
				}

				log.Info("🔍 Order Service: Querying open advice", map[string]any{
					"currentPrice": currentPrice.String(),
				})

				// 調用策略服務獲取建議
				advice, err := strategyService.GetOpenAdvice(ctx, instID)
				if err != nil {
					log.Error("Failed to get open advice", map[string]any{
						"error": err,
					})
					continue
				}

				// 輸出建議結果
				if advice.ShouldOpen {
					log.Info("✅ Order Service: SHOULD OPEN POSITION", map[string]any{
						"advice": advice,
					})
				} else {
					log.Debug("❌ Order Service: Should not open", map[string]any{
						"reason": advice.Reason,
					})
				}
			}
		}
	}()

	// 9. 等待退出信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Trading Strategy Server...")
	cancel() // Cancel context to stop subscriptions
}
