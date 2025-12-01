package handler

import (
	"context"
	"time"

	"dizzycode.xyz/logger"
	"dizzycoder.xyz/market-data-service/internal/okx"
	"dizzycoder.xyz/market-data-service/internal/storage"
)

// TickerHandler Ticker 數據處理器
//
// 職責：
// - 接收 OKX Ticker 數據
// - 調用 storage 保存最新價格
// - 不包含存儲實現細節（依賴抽象接口）
type TickerHandler struct {
	storage storage.MarketDataStorage // 依賴抽象接口
	logger  logger.Logger
}

// NewTickerHandler 創建 Ticker 處理器
func NewTickerHandler(storage storage.MarketDataStorage, logger logger.Logger) *TickerHandler {
	return &TickerHandler{
		storage: storage,
		logger:  logger,
	}
}

// Handle 處理 Ticker 數據
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
