package application

import (
	"context"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/trading-strategy-server/internal/domain/strategy"
)

// SignalPublisher 信號發布器介面（端口）
// 應用層定義介面，基礎設施層實現
type SignalPublisher interface {
	Publish(ctx context.Context, signal strategy.Signal) error
}

// StrategyService 策略應用服務
// 職責：
// 1. 編排領域對象
// 2. 處理用例流程
// 3. 協調基礎設施（通過介面）
type StrategyService struct {
	grid      *strategy.GridAggregate
	publisher SignalPublisher
	logger    logger.Logger
}

// NewStrategyService 創建策略服務
func NewStrategyService(
	grid *strategy.GridAggregate,
	publisher SignalPublisher,
	logger logger.Logger,
) *StrategyService {
	return &StrategyService{
		grid:      grid,
		publisher: publisher,
		logger:    logger,
	}
}

// HandlePriceUpdate 處理價格更新用例
// 這是應用層的入口方法
func (s *StrategyService) HandlePriceUpdate(ctx context.Context, priceValue float64) error {
	// 1. 創建價格值對象
	price, err := strategy.NewPrice(priceValue)
	if err != nil {
		s.logger.Error("Invalid price", map[string]any{
			"error": err,
			"price": priceValue,
		})
		return err
	}

	// 2. 調用領域邏輯
	signal, err := s.grid.ProcessPriceUpdate(price)
	if err != nil {
		s.logger.Warn("Price update failed", map[string]any{
			"error": err,
			"price": priceValue,
		})
		return err
	}

	// 3. 如果沒有信號，直接返回
	if signal == nil {
		s.logger.Debug("No signal generated", map[string]any{
			"price": priceValue,
		})
		return nil
	}

	// 4. 有信號，記錄日誌
	s.logger.Info("Signal generated", map[string]any{
		"action":   signal.Action(),
		"price":    signal.Price().Value(),
		"quantity": signal.Quantity(),
		"reason":   signal.Reason(),
	})

	// 5. 發布信號到基礎設施
	if err := s.publisher.Publish(ctx, *signal); err != nil {
		s.logger.Error("Failed to publish signal", map[string]any{
			"error":  err,
			"signal": signal,
		})
		return err
	}

	return nil
}

// GetGridState 獲取網格狀態（查詢用例）
func (s *StrategyService) GetGridState() map[string]interface{} {
	return s.grid.GetState()
}
