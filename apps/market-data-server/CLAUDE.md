# Market Data Service - 開發進度與計劃

## 服務概述

Market Data Service 是交易系統的核心服務，負責：
- 連接 OKX WebSocket 接收即時價格數據
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
│   │   └── config.go              # 配置管理
│   ├── logger/
│   │   └── logger.go              # Logger 工廠
│   ├── okx/
│   │   └── types.go               # OKX 特定類型定義
│   └── websocket/
│       ├── manager.go             # WebSocket 業務邏輯層
│       └── logger_adapter.go      # Logger 適配器
├── .env                           # 環境變量配置
└── go.mod

外部依賴（通用包）:
├── go-packages/websocket/         # 通用 WebSocket 客戶端
└── go-packages/logger/            # 自定義 Logger
```

---

## ✅ 已完成的功能

### Phase 1: WebSocket 基礎架構 (2025-10-14)

#### 1. **通用 WebSocket 客戶端** (`go-packages/websocket/`)
- ✅ 設計通用 WebSocket 客戶端，不綁定特定業務邏輯
- ✅ 使用 `interface{}` 的 Logger 介面保持靈活性
- ✅ 實作 Ping/Pong 機制（20秒 ping interval）
- ✅ 支援消息處理器（MessageHandler）
- ✅ 優雅關閉連接

**檔案位置**: `go-packages/websocket/client.go`

**設計亮點**:
```go
// 通用、不綁定特定日誌庫
type Logger interface {
    Info(msg string, fields ...any)
    Error(msg string, fields ...any)
    Debug(msg string, fields ...any)
    Warn(msg string, fields ...any)
}
```

#### 2. **OKX 類型定義** (`internal/okx/`)
- ✅ 定義 OKX WebSocket 請求/響應結構
- ✅ 定義 Ticker 數據結構（含所有欄位）
- ✅ 提供輔助函數（NewSubscribeRequest, NewUnsubscribeRequest）

**檔案位置**: `internal/okx/types.go`

**主要類型**:
```go
type Ticker struct {
    InstID    string `json:"instId"`    // BTC-USDT
    Last      string `json:"last"`      // 最新成交價
    Vol24h    string `json:"vol24h"`    // 24小時成交量
    High24h   string `json:"high24h"`   // 24小時最高價
    Low24h    string `json:"low24h"`    // 24小時最低價
    // ... 更多欄位
}
```

#### 3. **WebSocket 管理器** (`internal/websocket/`)
- ✅ 封裝業務邏輯層
- ✅ 實作 Logger Adapter（將自定義 logger 適配為 websocket.Logger）
- ✅ 支援訂閱/取消訂閱
- ✅ 處理 OKX 特定的消息格式
- ✅ 支援多個 Ticker 處理器

**檔案位置**:
- `internal/websocket/manager.go`
- `internal/websocket/logger_adapter.go`

**責任分離**:
```
通用包（websocket）   ← 不依賴業務邏輯
      ↓
業務層（manager）     ← OKX 特定邏輯 + Adapter
      ↓
應用層（main.go）     ← 依賴注入
```

#### 4. **配置管理** (`internal/config/`)
- ✅ 支援 .env 檔案
- ✅ 支援多個交易對配置（OKX_INSTRUMENTS）
- ✅ 提供預設值

**環境變量**:
```bash
PORT=50051
ENVIRONMENT=development
LOG_LEVEL=debug
OKX_INSTRUMENTS=BTC-USDT,ETH-USDT
```

#### 5. **主程式與依賴注入** (`cmd/main.go`)
- ✅ 依賴注入架構
- ✅ 信號處理（SIGINT, SIGTERM）
- ✅ 優雅關閉
- ✅ Ticker 數據處理（目前僅記錄日誌）

**執行流程**:
1. 載入配置
2. 創建 Logger
3. 創建 WebSocket Manager
4. 添加 Ticker Handler
5. 連接 OKX WebSocket
6. 訂閱交易對
7. 等待退出信號

#### 6. **測試驗證**
- ✅ 成功連接到 OKX WebSocket
- ✅ 成功訂閱 BTC-USDT, ETH-USDT
- ✅ 持續接收即時價格數據
- ✅ 日誌正常輸出
- ✅ 優雅關閉正常

**測試結果**:
```
2025-10-14T02:28:57 INFO: Successfully connected to WebSocket
2025-10-14T02:28:57 INFO: Subscribed to ticker instId=BTC-USDT
2025-10-14T02:28:57 INFO: Subscribed to ticker instId=ETH-USDT
2025-10-14T02:28:57 INFO: Received ticker instId=BTC-USDT last=115225.1 volume24h=7705.86942617
2025-10-14T02:28:57 INFO: Received ticker instId=ETH-USDT last=4227.38 volume24h=278982.548659
```

---

## 🔄 進行中的任務

目前無進行中的任務。

---

## 📋 待完成的功能

### Phase 2: Redis 整合（優先級：高）

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
- [ ] 定義 Pub/Sub 頻道命名規則（例如: `ticker:BTC-USDT`）
- [ ] 將 Ticker 數據序列化為 JSON 並發布
- [ ] 添加發布失敗重試機制

**數據流**:
```
OKX WebSocket → Manager → TickerHandler → Redis Publisher
```

#### 3. **價格快取**
- [ ] 在 Redis 中快取最新價格（使用 SET）
- [ ] 設置 Key 命名規則（例如: `price:latest:BTC-USDT`）
- [ ] 設置合理的 TTL（例如: 60秒）

**快取結構**:
```json
{
  "instId": "BTC-USDT",
  "last": "115225.1",
  "timestamp": "2025-10-14T02:28:57.281+0800",
  "high24h": "116000.0",
  "low24h": "114000.0",
  "vol24h": "7705.86942617"
}
```

---

### Phase 3: REST API（優先級：高）

#### 1. **HTTP 服務器**
- [ ] 使用 Gin 框架創建 HTTP 服務器
- [ ] 實作健康檢查端點 `GET /health`
- [ ] 實作 Metrics 端點（可選）

#### 2. **價格查詢 API**
- [ ] `GET /api/v1/ticker/:instId` - 查詢指定交易對的最新價格
- [ ] `GET /api/v1/tickers` - 查詢所有已訂閱交易對的價格
- [ ] 從 Redis 快取讀取數據
- [ ] 錯誤處理與狀態碼

**API 響應範例**:
```json
GET /api/v1/ticker/BTC-USDT

{
  "code": 0,
  "msg": "success",
  "data": {
    "instId": "BTC-USDT",
    "last": "115225.1",
    "high24h": "116000.0",
    "low24h": "114000.0",
    "vol24h": "7705.86942617",
    "timestamp": "2025-10-14T02:28:57.281+0800"
  }
}
```

#### 3. **WebSocket 訂閱端點（可選）**
- [ ] `WS /ws/v1/subscribe` - 允許客戶端訂閱價格更新
- [ ] 實作 WebSocket 服務器端邏輯
- [ ] 支援多客戶端訂閱

---

### Phase 4: 錯誤處理與監控（優先級：中）

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

### Phase 5: 優化與擴展（優先級：低）

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

### 需要處理的問題

1. **訂閱響應錯誤處理不完整**
   - 目前在日誌中看到 "Subscription failed" 但 code 和 msg 為空
   - 需要檢查 OKX 響應格式是否完全匹配
   - 位置: `internal/websocket/manager.go:105`

2. **缺少單元測試**
   - 所有 package 都缺少測試覆蓋
   - 建議先為核心邏輯添加測試

3. **配置驗證不完整**
   - 沒有驗證 OKX_INSTRUMENTS 格式
   - 沒有驗證端口號範圍

---

## 🎯 下次開發建議

### 優先順序排序

1. **修復訂閱響應錯誤** (5分鐘)
   - 檢查 OKX 響應格式
   - 改善錯誤日誌

2. **實作 Redis 整合** (1-2小時)
   - 創建 Redis 客戶端
   - 實作價格發布與快取
   - 這是最核心的功能，需要優先完成

3. **實作 REST API** (1-2小時)
   - 使用 Gin 創建 HTTP 服務器
   - 實作價格查詢端點
   - 整合 Redis 讀取

4. **實作斷線重連** (1小時)
   - 這對生產環境很重要

### 建議的開發流程

```bash
# 啟動 Redis（用於測試）
docker run -d --name redis -p 6379:6379 redis:latest

# 運行服務
go run cmd/main.go

# 測試 API（Redis 完成後）
curl http://localhost:50051/api/v1/ticker/BTC-USDT
```

---

## 📚 相關文檔

- [項目整體架構](../../CLAUDE.md)
- [OKX API 文檔](https://www.okx.com/docs-v5/en/)
- [OKX WebSocket 概覽](https://www.okx.com/docs-v5/en/#overview-websocket-overview)
- [OKX Tickers Channel](https://www.okx.com/docs-v5/en/#public-data-websocket-tickers-channel)

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

---

*最後更新: 2025-10-14*
*下次更新: 實作 Redis 整合後*
