package handler

import (
	"context"
	"time"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/okx"
	"dizzycoder.xyz/market-data-service/internal/storage"
)

// TickerHandler Ticker 数据处理器
//
// 职责：
// - 接收 OKX Ticker 数据
// - 调用 storage 保存最新价格
// - 不包含存储实现细节（依赖抽象接口）
type TickerHandler struct {
	storage storage.MarketDataStorage // 依赖抽象接口
	logger  logger.Logger
}

// NewTickerHandler 创建 Ticker 处理器
func NewTickerHandler(storage storage.MarketDataStorage, logger logger.Logger) *TickerHandler {
	return &TickerHandler{
		storage: storage,
		logger:  logger,
	}
}

// Handle 处理 Ticker 数据
func (h *TickerHandler) Handle(ticker okx.Ticker) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 保存最新价格（委托给 storage）
	if err := h.storage.SaveLatestPrice(ctx, ticker); err != nil {
		h.logger.Error("Failed to save ticker", map[string]any{
			"error":  err,
			"instId": ticker.InstID,
		})
		return err
	}

	return nil
}
