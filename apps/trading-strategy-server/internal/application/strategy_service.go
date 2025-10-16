package application

import (
	"context"

	"dizzycode.xyz/logger"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/strategies"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/value_objects"
)

// SignalPublisher 信號發布器介面（端口）
// 應用層定義介面，基礎設施層實現
type SignalPublisher interface {
	Publish(ctx context.Context, signal value_objects.Signal) error
}

// StrategyService 策略應用服務
// 職責：
// 1. 編排領域對象
// 2. 處理用例流程
// 3. 協調基礎設施（通過介面）
type StrategyService struct {
	strategy  strategies.Strategy  // ⭐ 使用 Strategy 介面，支援多種策略
	publisher SignalPublisher
	logger    logger.Logger
}

// NewStrategyService 創建策略服務
func NewStrategyService(
	strategy strategies.Strategy,  // ⭐ 接受任何實現 Strategy 介面的策略
	publisher SignalPublisher,
	logger logger.Logger,
) *StrategyService {
	return &StrategyService{
		strategy:  strategy,
		publisher: publisher,
		logger:    logger,
	}
}

// HandleCandleUpdate 處理 K線更新用例
// 這是應用層的入口方法
func (s *StrategyService) HandleCandleUpdate(ctx context.Context, candle value_objects.Candle) error {
	// 1. 調用領域邏輯（Candle 已經過驗證）
	signal, err := s.strategy.ProcessCandle(candle)
	if err != nil {
		s.logger.Warn("Candle processing failed", map[string]any{
			"error": err,
			"close": candle.Close().Value(),
		})
		return err
	}

	// 2. 如果沒有信號，直接返回
	if signal == nil {
		s.logger.Debug("No signal generated", map[string]any{
			"close": candle.Close().Value(),
			"low":   candle.Low().Value(),
			"high":  candle.High().Value(),
		})
		return nil
	}

	// 3. 有信號，記錄日誌
	s.logger.Info("Signal generated", map[string]any{
		"action":       signal.Action(),
		"price":        signal.Price().Value(),
		"positionSize": signal.PositionSize(),
		"takeProfit":   signal.TakeProfit(),
		"reason":       signal.Reason(),
	})

	// 4. 發布信號到基礎設施
	if err := s.publisher.Publish(ctx, *signal); err != nil {
		s.logger.Error("Failed to publish signal", map[string]any{
			"error":  err,
			"signal": signal,
		})
		return err
	}

	return nil
}

// GetStrategyState 獲取策略狀態（查詢用例）
func (s *StrategyService) GetStrategyState() map[string]any {
	return s.strategy.GetState()
}
