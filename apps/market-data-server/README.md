# Market Data Server

OKX 即時行情接收服務，作為交易系統的 **Price Oracle**。

## 功能

- 通過 OKX WebSocket 接收即時價格和 K 線數據
- 將數據存儲到 Redis，供其他服務讀取
- 自動清理過期數據，防止讀到髒數據
- 雙 WebSocket 連接（Ticker / Candle 獨立運行）

## 快速開始

```bash
# 複製環境變數
cp .env.example .env

# 編輯配置
vim .env

# 啟動服務
go run ./cmd/main.go
```

## 環境變數

參考 `.env.example`：

```bash
ENVIRONMENT=development
LOG_LEVEL=info

# OKX 配置
OKX_INSTRUMENTS=ETH-USDT-SWAP
OKX_SUBSCRIBE_TICKER=true
OKX_SUBSCRIBE_CANDLES=1m,5m

# Redis 配置
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10
```

## Redis 存儲

| Key Pattern | 類型 | 說明 |
|-------------|------|-----|
| `price.latest.{instId}` | String | 即時價格 (TTL 60s) |
| `candle.latest.{bar}.{instId}` | String | 最新 K 線 (動態 TTL) |
| `candle.history.{bar}.{instId}` | List | 歷史 K 線 (LPUSH, 保留 N 根) |

### 查看數據

```bash
# Ticker
redis-cli GET price.latest.ETH-USDT-SWAP

# 最新 K 線
redis-cli GET candle.latest.5m.ETH-USDT-SWAP

# 歷史 K 線
redis-cli LRANGE candle.history.5m.ETH-USDT-SWAP 0 4
```

## 架構

```
┌─────────────────────────────────────────────────┐
│                  main.go                         │
│           (依賴注入 & 生命週期管理)               │
└────────────┬────────────────────────────────────┘
             │
    ┌────────┼────────┬────────────┬─────────────┐
    ▼        ▼        ▼            ▼             ▼
┌────────┐ ┌────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐
│Handler │ │WS  │ │Storage  │ │ Config  │ │  Redis   │
│ Layer  │ │Mgr │ │Interface│ │         │ │  Client  │
└────────┘ └────┘ └─────────┘ └─────────┘ └──────────┘
```

### 目錄結構

```
market-data-server/
├── cmd/
│   └── main.go              # 服務入口
├── internal/
│   ├── config/              # 配置管理
│   ├── handler/             # 業務邏輯層
│   ├── storage/             # 存儲層（可替換）
│   ├── websocket/           # WebSocket 管理
│   ├── redis/               # Redis 客戶端
│   ├── okx/                 # OKX 數據結構
│   └── logger/              # Logger 工廠
├── .env.example
└── go.mod
```

## 架構演變

**原設計**：Redis Pub/Sub 推送模式
- Market Data 主動推送到 `market.ticker.{instId}` 頻道

**現行設計**：KV 存儲 + 主動讀取（Pull 模式）
- Market Data 寫入 Redis KV（SET/LPUSH）
- Order Service 主動讀取（GET/LRANGE）

**變更原因**：讓 Order Service 掌握主動權，控制讀取時機

**擴展性**：Storage 接口保留 `PublishPrice()` / `PublishCandle()` 方法，可按需啟用 Pub/Sub

## OKX 參考

- [Candlesticks Channel](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-ws-candlesticks-channel)
- [WebSocket Subscribe](https://www.okx.com/docs-v5/en/#overview-websocket-subscribe)

### 交易對格式

| 類型 | 格式 | 示例 |
|------|------|------|
| 現貨 | `{BASE}-{QUOTE}` | `BTC-USDT` |
| 永續合約 | `{BASE}-{QUOTE}-SWAP` | `ETH-USDT-SWAP` |
| 交割合約 | `{BASE}-{QUOTE}-{DATE}` | `BTC-USDT-250328` |


###  Redis 存储策略 ⭐

#### 1 数据结构

**SET（最新数据）**:
```redis
# Ticker
price.latest.BTC-USDT-SWAP      # TTL: 60s
price.latest.ETH-USDT-SWAP

# Candle（包括未确认的）
candle.latest.1m.BTC-USDT-SWAP  # TTL: 120s
candle.latest.5m.BTC-USDT-SWAP  # TTL: 600s
```

**List（历史数据，仅已确认）**:
```redis
# 最新的在前（index 0）
candle.history.1m.BTC-USDT-SWAP  # 保留最近 200 根
candle.history.5m.BTC-USDT-SWAP  # 保留最近 200 根
```

#### 2 数据保留策略

```go
// internal/config/retention.go
func DefaultRetentionPolicy() *RetentionPolicy {
    return &RetentionPolicy{
        CandleHistoryLength: map[string]int{
            "1m":  200,  // 3 小时
            "5m":  200,  // 16.6 小时
            "1H":  200,  // 8.3 天
            "1D":  365,  // 1 年
        },
    }
}
```

#### 3 Key 管理

所有 Redis key 定义在 `internal/storage/keys.go`:

```go
const (
    KeyPatternTickerLatest  = "price.latest.%s"        // %s = instId
    KeyPatternCandleLatest  = "candle.latest.%s.%s"    // bar, instId
    KeyPatternCandleHistory = "candle.history.%s.%s"   // bar, instId

    KeyPatternTickerAll        = "price.latest.*"      // 用于清理
    KeyPatternCandleLatestAll  = "candle.latest.*"     // 用于清理
    KeyPatternCandleHistoryAll = "candle.history.*"    // 用于清理
)
```

**优势**: 集中管理，易于修改

#### 4 自动清理 ⭐

```go
// internal/storage/redis_storage.go
func (s *RedisStorage) Cleanup(ctx context.Context) error {
    patterns := CleanupPatterns()

    for _, pattern := range patterns {
        // 使用 SCAN 获取所有匹配的 key
        iter := s.client.Scan(ctx, 0, pattern, 0).Iterator()

        // 批量删除
        if len(keys) > 0 {
            s.client.Del(ctx, keys...)
        }
    }
}
```

**调用时机**: 服务关闭前（`main.go` 的 `defer`）

**目的**: 防止策略服务读到过期的价格数据

---