package application

import (
	"context"

	"dizzycode.xyz/logger"
	"dizzycode.xyz/shared/domain/value_objects"
	"dizzycode.xyz/trading-strategy-server/internal/domain/strategy/strategies/grid"
)

// MarketDataReader 介面（端口）⭐
// 應用層定義介面，基礎設施層實現
type MarketDataReader interface {
	// GetLastestCandle 從 Redis 讀取最新的已確認 Candle（歷史第一根）
	// Key format: candle.history.{bar}.{instId}
	// 用於策略計算：開倉點位 = 上一根已確認 K 線的 MidLow
	// 如果不存在則返回 error
	GetLatestCandle(ctx context.Context, instID string, bar string) (value_objects.Candle, error)
	GetLatestPrice(ctx context.Context, instID string) (value_objects.Price, error)

	// GetCandleHistories 從 Redis 讀取歷史 Candle 列表
	// Key format: candle.history.{bar}.{instId}
	// 返回最近 N 根已確認的 K 線（用於趨勢分析）
	GetCandleHistories(ctx context.Context, instID string, bar string) ([]value_objects.Candle, error)
}

// StrategyService 策略應用服務（被動諮詢模式）⭐
// 職責：
// 1. 編排領域對象
// 2. 處理用例流程（GetOpenAdvice）
// 3. 協調基礎設施（通過介面）
type StrategyService struct {
	grid       *grid.GridAggregate // ⭐ 直接使用 GridAggregate
	dataReader MarketDataReader    // ⭐ 新增：從 Redis 讀取市場數據
	logger     logger.Logger
}

// NewStrategyService 創建策略服務
func NewStrategyService(
	grid *grid.GridAggregate, // ⭐ 接受 GridAggregate
	dataReader MarketDataReader, // ⭐ 新增參數
	logger logger.Logger,
) *StrategyService {
	return &StrategyService{
		grid:       grid,
		dataReader: dataReader,
		logger:     logger,
	}
}

// GetOpenAdvice 獲取開倉建議（被動諮詢用例）⭐
// 這是應用層的入口方法
func (s *StrategyService) GetOpenAdvice(
	ctx context.Context,
	instID string,
) (*grid.OpenAdvice, error) {
	// 1. 從 Redis 讀取最新的已確認 Candle（歷史第一根）⭐
	lastCandle, err := s.dataReader.GetLatestCandle(ctx, instID, "5m")
	if err != nil {
		s.logger.Error("Failed to get last confirmed candle", map[string]any{
			"error":  err,
			"instId": instID,
		})
		return nil, err
	}

	candlehistories, err := s.dataReader.GetCandleHistories(ctx, instID, "5m")
	if err != nil {
		s.logger.Error("Failed to get last confirmed candle", map[string]any{
			"error":  err,
			"instId": instID,
		})
		return nil, err
	}

	s.logger.Debug("Retrieved last confirmed candle", map[string]any{
		"close": lastCandle.Close().Value(),
		"low":   lastCandle.Low().Value(),
		"high":  lastCandle.High().Value(),
	})

	currentPrice, err := s.dataReader.GetLatestPrice(ctx, instID)

	if err != nil {
		s.logger.Warn("Failed to get current price", map[string]any{
			"error": err,
		})
		return nil, err
	}

	// 2. 創建價格值對象

	// 3. 創建空的倉位摘要（Strategy Service 無狀態，倉位由 Order Service 管理）⭐
	// TODO: 未來可能需要從 Order Service 獲取倉位摘要
	emptyPositionSummary := value_objects.NewPositionSummary(0, 0, 0, 0, 0, 0, 0) // ⭐ 包含 currentRoundRealizedPnL 和 currentRoundClosedValue

	// 4. 調用領域邏輯獲取建議 ⭐ 傳入倉位摘要
	advice := s.grid.GetOpenAdvice(currentPrice, lastCandle, candlehistories, emptyPositionSummary)

	// 4. 記錄日誌
	// if advice.ShouldOpen {
	// 	s.logger.Info("Open advice: SHOULD OPEN", map[string]any{
	// 		"currentPrice": currentPrice,
	// 		"openPrice":    advice.OpenPrice,
	// 		"positionSize": advice.PositionSize,
	// 		"takeProfit":   advice.TakeProfit,
	// 		"reason":       advice.Reason,
	// 	})
	// } else {
	// 	s.logger.Debug("Open advice: SHOULD NOT OPEN", map[string]any{
	// 		"currentPrice": currentPrice,
	// 		"reason":       advice.Reason,
	// 	})
	// }

	return &advice, nil
}

// GetStrategyState 獲取策略狀態（查詢用例）
func (s *StrategyService) GetStrategyState() map[string]any {
	return s.grid.GetState()
}
