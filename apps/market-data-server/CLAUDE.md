# Market Data Service - 開發進度與計劃

## 服務概述

Market Data Service 是交易系統的核心服務，負責：
- 連接 OKX WebSocket 接收即時價格數據和 K線數據
- 作為整個系統的**價格預言機（Price Oracle）**
- 將價格數據發布到 Redis Pub/Sub
- 提供 REST API 查詢最新價格

## 架構設計

```
market-data-server/
├── cmd/
│   └── main.go                    # 服務入口，依賴注入
├── internal/
│   ├── config/
│   │   └── config.go              # 配置管理（支援依賴注入）
│   ├── logger/
│   │   └── factory.go             # Logger 工廠
│   ├── okx/
│   │   └── types.go               # OKX 特定類型定義（Ticker + Candle）
│   └── websocket/
│       └── manager.go             # WebSocket 業務邏輯層
├── .env                           # 環境變量配置
└── go.mod

外部依賴（通用包）:
├── go-packages/websocket/         # 通用 WebSocket 客戶端（完全獨立）
└── go-packages/logger/            # 統一 Logger 系統（Console + Zap + Multi）
```

---

## ✅ 已完成的功能

### Phase 1: WebSocket 基礎架構 (2025-10-14)

#### 1. **通用 WebSocket 客戶端** (`go-packages/websocket/`)
- ✅ 設計通用 WebSocket 客戶端，不綁定特定業務邏輯
- ✅ **完全獨立**：無外部 logger 依賴，內建 defaultLogger
- ✅ 實作 Ping/Pong 機制（20秒 ping interval）
- ✅ 支援消息處理器（MessageHandler）
- ✅ 優雅關閉連接

**檔案位置**: `go-packages/websocket/client.go`, `go-packages/websocket/logger.go`

**設計亮點**:
```go
// WebSocket 定義自己的 Logger 介面，完全獨立
type Logger interface {
    Info(msg string, context ...any)
    Error(msg string, context ...any)
    Debug(msg string, context ...any)
    Warn(msg string, context ...any)
}

// 內建 defaultLogger 作為 fallback
var defaultLog Logger = &defaultLogger{}
```

#### 2. **統一 Logger 系統** (`go-packages/logger/`)
- ✅ 設計類似 TypeScript 的 Logger 架構
- ✅ Console Logger（默認 fallback，帶顏色）
- ✅ Zap Logger（支援 Pretty 和 JSON 模式）
- ✅ Multi Logger（多目標輸出）
- ✅ 支援 `map[string]any` 格式的 context 參數
- ✅ 依賴注入模式

**檔案位置**:
- `go-packages/logger/logger.go` - 核心介面
- `go-packages/logger/console.go` - 默認實現
- `go-packages/logger/zap.go` - Zap 包裝
- `go-packages/logger/multi.go` - 多目標輸出
- `go-packages/logger/utils.go` - 工具函數

**使用範例**:
```go
// 依賴注入
log := logger.Must(cfg)

// 支援 map[string]any
log.Info("message", map[string]any{"key": "value"})
```

#### 3. **OKX 類型定義** (`internal/okx/`)
- ✅ 定義 OKX WebSocket 請求/響應結構
- ✅ 定義 Ticker 數據結構（含所有欄位）
- ✅ **定義 Candle K線數據結構**（支援數組格式解析）
- ✅ 提供輔助函數（NewSubscribeRequest, NewCandleSubscribeRequest）
- ✅ **多 WebSocket URL 支援**：
  - `PublicWSURL` - Ticker 數據 (`/ws/v5/public`)
  - `BusinessWSURL` - Candle 數據 (`/ws/v5/business`)
  - `PrivateWSURL` - 私有交易數據 (`/ws/v5/private`)

**檔案位置**: `internal/okx/types.go`

**主要類型**:
```go
// Ticker 數據
type Ticker struct {
    InstID    string `json:"instId"`
    Last      string `json:"last"`
    Vol24h    string `json:"vol24h"`
    // ... 更多欄位
}

// Candle K線數據（數組格式）
type CandleRaw []string // [ts, o, h, l, c, vol, volCcy, volCcyQuote, confirm]

type Candle struct {
    Ts, Open, High, Low, Close string
    Vol, VolCcy, VolCcyQuote string
    Confirm string  // "0" = 未完成, "1" = 已完成
    InstID, Bar string
}

func ParseCandle(raw CandleRaw, instID, bar string) (*Candle, error)
```

#### 4. **WebSocket 管理器** (`internal/websocket/`)
- ✅ 封裝業務邏輯層
- ✅ **支援 Ticker 和 Candle 雙頻道訂閱**
- ✅ 自動處理 OKX 特定的消息格式（JSON 對象 vs 數組）
- ✅ **Manager 自動打印日誌**（符合依賴注入原則）
- ✅ Handler 只處理業務邏輯（如 Redis 發布）
- ✅ **完整錯誤處理**：
  - OKX 錯誤事件處理 (`event: "error"`)
  - 訂閱成功/失敗處理
  - Debug 日誌（可透過 LOG_LEVEL 控制）

**檔案位置**: `internal/websocket/manager.go`

**責任分離**:
```
通用包（websocket）   ← 完全獨立，無外部依賴
      ↓
業務層（manager）     ← OKX 特定邏輯，自動打印日誌
      ↓
應用層（main.go）     ← 依賴注入，Handler 只處理業務
```

**支援的時間週期**:
- 秒級：`1s`
- 分鐘級：`1m`, `3m`, `5m`, `15m`, `30m`
- 小時級：`1H`, `2H`, `4H`, `6H`, `12H`
- 天級：`1D`, `2D`, `3D`, `5D`
- 週/月級：`1W`, `1M`, `3M`

#### 5. **配置管理** (`internal/config/`)
- ✅ 支援 .env 檔案
- ✅ 支援多個交易對配置（OKX_INSTRUMENTS）
- ✅ **依賴注入模式**：`Load()` 返回 `*Config`
- ✅ 提供預設值

**環境變量**:
```bash
PORT=50051
ENVIRONMENT=development
LOG_LEVEL=debug            # debug, info, warn, error
OKX_INSTRUMENTS=BTC-USDT,ETH-USDT
```

#### 6. **主程式與依賴注入** (`cmd/main.go`)
- ✅ 完整依賴注入架構
- ✅ 信號處理（SIGINT, SIGTERM）
- ✅ 優雅關閉
- ✅ **Ticker 和 Candle 雙處理器**
- ✅ **Handler 只處理業務邏輯，不打印日誌**

**執行流程**:
1. 載入配置（返回 `*Config`）
2. 創建 Logger（注入 Config）
3. 創建 WebSocket Manager（注入 Logger）
4. 添加 Ticker/Candle Handler（業務邏輯）
5. 連接 OKX WebSocket
6. 訂閱交易對
7. 等待退出信號

**設計原則**:
```go
// ✅ 正確：Manager 自動打印日誌
wsManager := websocket.NewManager(websocket.Config{
    URL:    okx.BusinessWSURL,
    Logger: log,  // ← 注入一次
})

wsManager.AddCandleHandler(func(candle okx.Candle) error {
    // Handler 只處理業務邏輯，不用再打印日誌
    // TODO: 發布到 Redis
    return nil
})
```

#### 7. **測試驗證**
- ✅ 成功連接到 OKX WebSocket（雙 URL）
- ✅ 成功訂閱 BTC-USDT, ETH-USDT（Ticker + Candle）
- ✅ 持續接收即時價格數據和 K線數據
- ✅ 錯誤處理正常（訂閱失敗會顯示 ERROR）
- ✅ 日誌級別控制正常（debug/info 可切換）
- ✅ 優雅關閉正常

**測試結果**:
```
2025-10-14T16:15:30 INFO: Subscription confirmed channel=candle1m instId=ETH-USDT
2025-10-14T16:15:30 INFO: Received candle instId=ETH-USDT bar=1m open=3987.91 high=3989.67 low=3986.22 close=3987.09 volume=50.309607 confirm=0
2025-10-14T16:15:31 INFO: Received candle instId=ETH-USDT bar=1m ... confirm=0
```

---

## 🔄 進行中的任務

**下一步：實作 Redis 整合** ⭐

---

## 📋 待完成的功能

### Phase 2: Redis 整合（優先級：高）⭐ 下一步

#### 1. **Redis 連接管理**
- [ ] 創建 Redis 客戶端封裝（`internal/redis/client.go`）
- [ ] 支援連接池配置
- [ ] 健康檢查與重連機制

**配置項**:
```bash
REDIS_HOST=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10
```

#### 2. **價格數據發布**
- [ ] 實作 Redis Pub/Sub 發布器
- [ ] 定義 Pub/Sub 頻道命名規則：
  - Ticker: `market:ticker:BTC-USDT`
  - Candle: `market:candle:1m:BTC-USDT`
- [ ] 將 Ticker/Candle 數據序列化為 JSON 並發布
- [ ] 添加發布失敗重試機制

**數據流**:
```
OKX WebSocket → Manager (自動打印日誌) → Handler → Redis Publisher
```

#### 3. **價格快取**
- [ ] 在 Redis 中快取最新價格（使用 SET）
- [ ] 設置 Key 命名規則：
  - Ticker: `price:latest:BTC-USDT`
  - Candle: `candle:latest:1m:BTC-USDT`
- [ ] 設置合理的 TTL（例如: 60秒）

**快取結構**:
```json
// Ticker
{
  "instId": "BTC-USDT",
  "last": "115225.1",
  "timestamp": "2025-10-14T02:28:57.281+0800",
  "high24h": "116000.0",
  "low24h": "114000.0",
  "vol24h": "7705.86942617"
}

// Candle
{
  "instId": "BTC-USDT",
  "bar": "1m",
  "open": "115225.1",
  "high": "115300.0",
  "low": "115100.0",
  "close": "115250.0",
  "volume": "10.5",
  "confirm": "0"
}
```

---

### Phase 3: 錯誤處理與監控（優先級：中）

#### 1. **斷線重連機制**
- [ ] 實作 WebSocket 斷線檢測
- [ ] 實作 Exponential Backoff 重連策略
- [ ] 重連後自動重新訂閱交易對
- [ ] 記錄重連事件

**重連配置**:
```go
maxReconnectAttempts = 5
reconnectDelay       = 5 * time.Second
maxReconnectDelay    = 5 * time.Minute
```

#### 2. **Metrics 收集**
- [ ] WebSocket 連接狀態
- [ ] 接收到的消息數量
- [ ] 發布到 Redis 的成功/失敗次數
- [ ] API 請求統計

#### 3. **告警機制**
- [ ] WebSocket 斷線超過 N 次
- [ ] Redis 連接失敗
- [ ] 價格數據超過 N 秒未更新

---

### Phase 4: 優化與擴展（優先級：低）

#### 1. **性能優化**
- [ ] 批量發布到 Redis（減少網絡開銷）
- [ ] 限流控制（避免過度日誌輸出）
- [ ] Goroutine Pool 管理

#### 2. **多交易所支援**
- [ ] 抽象交易所介面
- [ ] 支援 Binance WebSocket
- [ ] 支援 Bybit WebSocket

#### 3. **數據聚合**
- [ ] K線數據聚合（1分鐘、5分鐘、1小時）
- [ ] 存儲到時序資料庫（InfluxDB / TimescaleDB）

#### 4. **測試**
- [ ] 單元測試（各 package）
- [ ] 整合測試（WebSocket + Redis）
- [ ] 壓力測試

---

## 🔧 技術債務

### 已解決

1. ✅ **訂閱響應錯誤處理** - 已加入 `event: "error"` 處理
2. ✅ **Logger 依賴注入混亂** - 已重構為 Manager 自動打印日誌
3. ✅ **WebSocket 包外部依賴** - 已完全獨立，無需外部 logger

### 待處理

1. **缺少單元測試**
   - 所有 package 都缺少測試覆蓋
   - 建議先為核心邏輯添加測試

2. **配置驗證不完整**
   - 沒有驗證 OKX_INSTRUMENTS 格式
   - 沒有驗證端口號範圍

---

## 🎯 下次開發建議

### 優先順序排序

1. **實作 Redis 整合** (1-2小時) ⭐ **最重要**
   - 創建 Redis 客戶端
   - 實作 Ticker/Candle 數據發布到 Redis Pub/Sub
   - 實作價格快取
   - 這是 Market Data Service 成為「價格預言機」的關鍵

2. **實作斷線重連** (1小時)
   - 這對生產環境很重要

### 建議的開發流程

```bash
# 1. 啟動 Redis（用於測試）
docker run -d --name redis -p 6379:6379 redis:latest

# 2. 運行服務
go run cmd/main.go

# 3. 測試 Redis Pub/Sub（另一個終端）
redis-cli
> SUBSCRIBE market:ticker:BTC-USDT
> SUBSCRIBE market:candle:1m:BTC-USDT
```

---

## 📚 相關文檔

- [項目整體架構](../../CLAUDE.md)
- [OKX API 文檔](https://www.okx.com/docs-v5/en/)
- [OKX WebSocket 概覽](https://www.okx.com/docs-v5/en/#overview-websocket-overview)
- [OKX Tickers Channel](https://www.okx.com/docs-v5/en/#public-data-websocket-tickers-channel)
- [OKX Candlesticks Channel](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-ws-candlesticks-channel)

---

## 🤝 開發規範

### Git Commit 規範

```
feat: 新增功能
fix: 修復 bug
refactor: 重構代碼
docs: 文檔更新
test: 測試相關
chore: 其他雜項
```

### 代碼規範

- 使用 `gofmt` 格式化代碼
- 每個 public 函數都需要註釋
- 錯誤處理不能忽略
- 使用 context 管理生命週期
- **依賴注入優先於全局變量**
- **Manager 自動處理日誌，Handler 專注業務**

---

## 🏆 設計亮點總結

1. **完全獨立的 WebSocket 包** - 無外部依賴，可復用於任何項目
2. **統一的 Logger 系統** - 類似 TypeScript，支援多種策略
3. **真正的依賴注入** - Manager 自動打印日誌，Handler 專注業務
4. **多 WebSocket URL 支援** - Ticker 和 Candle 使用不同端點
5. **完整的錯誤處理** - OKX 錯誤事件、訂閱失敗、Debug 日誌

---

## 🔮 未來擴展：多交易所支援（Adapter Pattern）

### 現狀分析

**目前架構**：Market Data Service 專注於 OKX 交易所

- `internal/okx/` - OKX 特定的數據結構
- `internal/websocket/manager.go` - 包含 OKX 特定的消息解析邏輯
- `internal/websocket/setup.go` - 使用 OKX 特定類型

**問題**：不同交易所的 WebSocket API 格式完全不同

```
OKX:
  - Ticker channel: "tickers"
  - Candle channel: "candle1m"
  - 訂閱: {"op":"subscribe","args":[{"channel":"tickers","instId":"BTC-USDT"}]}

Binance:
  - Ticker stream: "btcusdt@ticker"
  - Kline stream: "btcusdt@kline_1m"
  - 訂閱: {"method":"SUBSCRIBE","params":["btcusdt@ticker"],"id":1}

Bybit:
  - Ticker topic: "tickers.BTCUSDT"
  - Kline topic: "kline.1.BTCUSDT"
  - 訂閱: {"op":"subscribe","args":["tickers.BTCUSDT"]}
```

### 推薦方案：Adapter Pattern

當需要支援多個交易所時，採用 **Adapter Pattern** 進行重構：

```
架構圖：
┌─────────────────────────────────────────────────────┐
│          Market Data Service                         │
├─────────────────────────────────────────────────────┤
│                                                       │
│  通用 WebSocket 客戶端（go-packages/websocket）      │
│             ↓                                         │
│  ┌──────────────┬──────────────┬──────────────┐     │
│  │ OKX Adapter  │Binance Adapter│Bybit Adapter │     │
│  └──────────────┴──────────────┴──────────────┘     │
│             ↓ 輸出統一格式                            │
│  ┌────────────────────────────────────────────┐     │
│  │      統一數據模型 (internal/model/)         │     │
│  │  - model.Ticker                             │     │
│  │  - model.Candle                             │     │
│  └────────────────────────────────────────────┘     │
│             ↓                                         │
│  ┌────────────────────────────────────────────┐     │
│  │      Redis Publisher                        │     │
│  │  (接收統一格式，發布到 Redis Pub/Sub)       │     │
│  └────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────┘
```

### 實作步驟（未來）

#### 1. 定義統一數據模型

創建 `internal/model/market_data.go`：

```go
package model

import "time"

// Ticker 通用 Ticker 數據
type Ticker struct {
    Exchange   string    // 交易所名稱：okx, binance, bybit
    InstID     string    // 交易對：BTC-USDT
    Last       string    // 最新價格
    Volume24h  string    // 24小時交易量
    High24h    string    // 24小時最高價
    Low24h     string    // 24小時最低價
    Timestamp  time.Time // 時間戳
}

// Candle 通用 K線數據
type Candle struct {
    Exchange  string    // 交易所名稱
    InstID    string    // 交易對
    Bar       string    // 週期：1m, 5m, 1H
    Open      string    // 開盤價
    High      string    // 最高價
    Low       string    // 最低價
    Close     string    // 收盤價
    Volume    string    // 交易量
    Timestamp time.Time // 時間戳
    Confirmed bool      // 是否已完成
}
```

#### 2. 定義 Adapter 介面

創建 `internal/exchange/adapter.go`：

```go
package exchange

import "dizzycoder.xyz/market-data-service/internal/model"

// Adapter 交易所適配器介面
type Adapter interface {
    // GetName 返回交易所名稱
    GetName() string

    // ConvertTicker 將交易所特定的 Ticker 轉換為統一格式
    ConvertTicker(raw interface{}) (*model.Ticker, error)

    // ConvertCandle 將交易所特定的 Candle 轉換為統一格式
    ConvertCandle(raw interface{}) (*model.Candle, error)

    // GetWebSocketURL 返回 WebSocket URL
    GetWebSocketURL() string

    // BuildSubscribeRequest 構建訂閱請求
    BuildSubscribeRequest(channel string, instID string) interface{}
}
```

#### 3. 實作 OKX Adapter

創建 `internal/exchange/okx/adapter.go`：

```go
package okx

import (
    "time"
    "dizzycoder.xyz/market-data-service/internal/model"
    "dizzycoder.xyz/market-data-service/internal/okx"
)

type OKXAdapter struct{}

func NewAdapter() *OKXAdapter {
    return &OKXAdapter{}
}

func (a *OKXAdapter) GetName() string {
    return "okx"
}

func (a *OKXAdapter) ConvertTicker(raw interface{}) (*model.Ticker, error) {
    okxTicker, ok := raw.(okx.Ticker)
    if !ok {
        return nil, fmt.Errorf("invalid ticker type")
    }

    ts, _ := okxTicker.GetTimestamp()

    return &model.Ticker{
        Exchange:  "okx",
        InstID:    okxTicker.InstID,
        Last:      okxTicker.Last,
        Volume24h: okxTicker.Vol24h,
        High24h:   okxTicker.High24h,
        Low24h:    okxTicker.Low24h,
        Timestamp: ts,
    }, nil
}

func (a *OKXAdapter) ConvertCandle(raw interface{}) (*model.Candle, error) {
    okxCandle, ok := raw.(okx.Candle)
    if !ok {
        return nil, fmt.Errorf("invalid candle type")
    }

    ts, _ := okxCandle.GetTimestamp()

    return &model.Candle{
        Exchange:  "okx",
        InstID:    okxCandle.InstID,
        Bar:       okxCandle.Bar,
        Open:      okxCandle.Open,
        High:      okxCandle.High,
        Low:       okxCandle.Low,
        Close:     okxCandle.Close,
        Volume:    okxCandle.Vol,
        Timestamp: ts,
        Confirmed: okxCandle.IsConfirmed(),
    }, nil
}

// ... GetWebSocketURL, BuildSubscribeRequest 實作
```

#### 4. 修改 Redis Publisher

修改 `internal/redis/publisher.go` 接收統一格式：

```go
// 從 okx.Ticker 改為 model.Ticker
func (p *Publisher) PublishTicker(ctx context.Context, ticker model.Ticker) error {
    channel := fmt.Sprintf("market:ticker:%s:%s", ticker.Exchange, ticker.InstID)
    // ... 發布邏輯
}

// 從 okx.Candle 改為 model.Candle
func (p *Publisher) PublishCandle(ctx context.Context, candle model.Candle) error {
    channel := fmt.Sprintf("market:candle:%s:%s:%s",
        candle.Exchange, candle.Bar, candle.InstID)
    // ... 發布邏輯
}
```

#### 5. 在 Setup 中使用 Adapter

修改 `internal/websocket/setup.go`：

```go
func Setup(
    cfg *config.Config,
    log logger.Logger,
    publisher *redis.Publisher,
    adapter exchange.Adapter,  // ← 注入 Adapter
) (*Manager, error) {
    // 使用 adapter.GetWebSocketURL()
    // 使用 adapter.ConvertTicker/ConvertCandle
    // ...
}
```

### 重構時機

**建議：不要現在重構！**

理由：
- ✅ 當前架構對單一交易所（OKX）最簡單高效
- ✅ 還沒開始實作 Grid Engine，不確定實際需求
- ✅ 過早抽象可能導致設計錯誤（YAGNI 原則）

**何時重構？**
1. 確定需要支援第二個交易所時
2. Grid Engine 需要統一格式時
3. 發現當前架構難以維護時

### 替代方案：多服務架構

如果不想重構，也可以為每個交易所創建獨立服務：

```
apps/
├── market-data-okx/      # OKX 專用服務
├── market-data-binance/  # Binance 專用服務
└── market-data-bybit/    # Bybit 專用服務
    ↓ 都發布到 Redis Pub/Sub（統一格式）
Redis
    ↓
Grid Engine Service（不關心數據來源）
```

**優點**：
- 完全解耦，服務之間互不影響
- 某個交易所掛掉不影響其他
- 易於獨立部署和擴展
- 無需重構現有代碼

**缺點**：
- 代碼重複（但可共用 `go-packages/`）
- 部署複雜度增加

### 當前建議

1. **現在**：保持現有架構，專注完成 OKX 整合
2. **文檔**：在此記錄 Adapter Pattern 設計（已完成）
3. **未來**：根據實際需求選擇重構方案

---

*最後更新: 2025-10-14*