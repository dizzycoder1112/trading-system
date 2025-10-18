package handler

import (
	"context"
	"time"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/config"
	"dizzycoder.xyz/market-data-service/internal/okx"
	"dizzycoder.xyz/market-data-service/internal/storage"
)

// CandleHandler Candle 数据处理器
//
// 职责：
// - 接收 OKX Candle 数据
// - 保存最新 K 线（包括未确认的）
// - 如果已确认，追加到历史列表
// - 应用 RetentionPolicy 决定保留多少历史数据
type CandleHandler struct {
	storage   storage.MarketDataStorage // 依赖抽象接口
	retention *config.RetentionPolicy    // 数据保留策略
	logger    logger.Logger
}

// NewCandleHandler 创建 Candle 处理器
func NewCandleHandler(
	storage storage.MarketDataStorage,
	retention *config.RetentionPolicy,
	logger logger.Logger,
) *CandleHandler {
	return &CandleHandler{
		storage:   storage,
		retention: retention,
		logger:    logger,
	}
}

// Handle 处理 Candle 数据
func (h *CandleHandler) Handle(candle okx.Candle) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. 始终保存最新 K 线（包括未确认的）
	if err := h.storage.SaveLatestCandle(ctx, candle); err != nil {
		h.logger.Error("Failed to save latest candle", map[string]any{
			"error":  err,
			"instId": candle.InstID,
			"bar":    candle.Bar,
		})
		return err
	}

	// 2. 如果 K 线已确认，追加到历史列表
	if candle.IsConfirmed() {
		maxLength := h.retention.GetMaxLength(candle.Bar)
		if err := h.storage.AppendCandleHistory(ctx, candle, maxLength); err != nil {
			// 历史数据保存失败不影响最新数据，只记录错误
			h.logger.Error("Failed to append candle history", map[string]any{
				"error":  err,
				"instId": candle.InstID,
				"bar":    candle.Bar,
			})
			// 不返回错误，继续处理
		}
	}

	return nil
}
