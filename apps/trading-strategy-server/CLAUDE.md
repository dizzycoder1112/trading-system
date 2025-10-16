# Trading Strategy Server - 開發進度與計劃

## 服務概述

Trading Strategy Server 是交易系統的**策略引擎**,負責:
- 從 Redis 訂閱即時市場數據
- 計算網格交易策略
- 生成交易信號（BUY/SELL）
- 管理網格狀態和持倉
- **不執行實際交易**（交易由 Order Manager 負責）

## 架構設計

**採用 DDD (Domain-Driven Design) 架構 + 策略實例模式（方案 A）⭐**

```
trading-strategy-server/
├── cmd/
│   └── main.go                           # 應用入口（組裝依賴）
├── internal/
│   ├── domain/                           # 🎯 領域層（核心業務邏輯）
│   │   └── strategy/
│   │       ├── value_objects/            # ⭐ 共用值對象
│   │       │   ├── price.go             # 價格值對象
│   │       │   ├── candle.go            # K線值對象
│   │       │   └── signal.go            # 信號值對象
│   │       │
│   │       ├── strategies/               # ⭐ 各種策略實現
│   │       │   ├── strategy.go          # 策略介面
│   │       │   ├── grid/                # 網格策略
│   │       │   │   ├── grid.go          # GridAggregate
│   │       │   │   ├── calculator.go    # GridCalculator
│   │       │   │   └── trend_analyzer.go # ⭐ 趨勢判斷器
│   │       │   ├── dca/                 # DCA 策略（未來）
│   │       │   └── trend/               # 趨勢策略（未來）
│   │       │
│   │       └── instance/                 # ⭐ 策略實例管理
│   │           ├── instance.go          # 策略實例定義
│   │           └── manager.go           # 策略實例管理器
│   │
│   ├── application/                      # 📋 應用層（用例編排）
│   │   ├── strategy_service.go          # 策略應用服務
│   │   └── risk_advisor.go              # ⭐ 風險管理顧問（gRPC，未來）
│   │
│   └── infrastructure/                   # 🔧 基礎設施層（技術實現）
│       ├── config/
│       │   └── config.go                 # 配置管理
│       ├── logger/
│       │   └── factory.go                # Logger 工廠
│       ├── messaging/
│       │   ├── redis_client.go           # Redis 客戶端
│       │   ├── candle_subscriber.go      # 訂閱 Candle
│       │   └── signal_publisher.go       # 發布 Signal（支援方向頻道）
│       └── grpc/                         # ⭐ gRPC 服務（未來）
│           └── server.go                 # gRPC Server
├── docs/
│   └── strategy-improvements.md          # 策略改進文檔
├── .env                                  # 環境變量配置
└── go.mod

外部依賴（通用包）:
├── go-packages/logger/                   # 統一 Logger 系統
└── shared/proto/strategy/                # ⭐ gRPC Protocol Buffers（未來）
    └── strategy.proto
```

### **DDD 分層說明**

#### 🎯 **領域層 (Domain Layer)**
- **職責**：封裝核心業務邏輯和業務規則
- **特點**：
  - 完全獨立，不依賴任何技術框架
  - 可以不用 Redis/DB 測試
  - 包含聚合根、值對象、領域服務
- **範例**：`GridAggregate.ProcessPriceUpdate()` - 純業務邏輯

#### 📋 **應用層 (Application Layer)**
- **職責**：編排領域對象，處理用例流程
- **特點**：
  - 定義端口介面（Port）
  - 協調基礎設施
  - 薄薄的一層，不包含業務邏輯
- **範例**：`StrategyService.HandlePriceUpdate()` - 編排流程

#### 🔧 **基礎設施層 (Infrastructure Layer)**
- **職責**：提供技術實現（適配器 Adapter）
- **特點**：
  - 實現應用層定義的介面
  - 包含 Redis、Config、Logger
  - 可替換（Redis → Kafka）
- **範例**：`RedisSignalPublisher` - 實現 SignalPublisher 介面

---

## 系統職責

### **Trading Strategy Server 的職責** ✅

1. **訂閱市場數據**
   - 從 Redis 訂閱 `market:candle:1m:ETH-USDT`
   - 解析 Candle 數據

2. **計算網格策略**
   - 初始化網格線（上界、下界、網格數）
   - 監聽價格變化
   - 判斷觸發條件（價格穿越網格線）

3. **生成交易信號**
   - 向上穿越 → SELL 信號
   - 向下穿越 → BUY 信號
   - 輸出: `Signal{Action, InstID, Price, Quantity, Time}`

4. **管理網格狀態**
   - 記錄當前持倉
   - 追蹤已觸發/未觸發的網格線
   - 計算 P&L

### **Trading Strategy Server 不做的事** ❌

- ❌ 不執行實際交易（由 Order Manager 負責）
- ❌ 不直接調用交易所 API
- ❌ 不管理訂單狀態

---

## ✅ 已完成的功能

### Phase 1: DDD 領域層實作 (2025-10-14) ⭐

#### 1. **價格值對象** (`internal/domain/strategy/price.go`)
- ✅ 封裝價格業務規則（必須為正數）
- ✅ 不可變設計
- ✅ 提供比較方法（IsAbove, IsBelow, Equals）

**特點**：
```go
// 值對象帶有業務規則
price, err := strategy.NewPrice(2500.0)  // 驗證 > 0
if price.IsAbove(otherPrice) { ... }
```

#### 2. **信號值對象** (`internal/domain/strategy/signal.go`)
- ✅ 不可變交易信號
- ✅ 包含完整信號信息（Action, Price, Quantity, Reason）
- ✅ 自定義 JSON 序列化

**特點**：
```go
signal := strategy.NewSignal(
    strategy.ActionBuy,
    "ETH-USDT",
    price,
    0.01,
    "grid_cross_down",
)
```

#### 3. **領域服務** (`internal/domain/strategy/calculator.go`)
- ✅ 網格線計算（等差數列）
- ✅ 穿越檢測（DetectCrossedLine）
- ✅ 倉位大小計算
- ✅ 純函數設計，易於測試

**特點**：
```go
// 完全無狀態，可獨立測試
calculator := strategy.NewGridCalculator()
gridLines := calculator.CalculateGridLines(3000, 2000, 10)
```

#### 4. **網格聚合根** (`internal/domain/strategy/grid.go`) ⭐ 核心
- ✅ 封裝網格業務邏輯
- ✅ 保證不變性（Invariants）
- ✅ 價格穿越檢測
- ✅ 信號生成邏輯
- ✅ 完全獨立於技術實現

**特點**：
```go
// 創建時驗證業務規則
grid, err := strategy.NewGridAggregate("ETH-USDT", 3000, 2000, 10)

// 純業務邏輯，不依賴 Redis
signal, err := grid.ProcessPriceUpdate(newPrice)
```

**業務規則**：
- 上界必須大於下界
- 至少 2 個網格層級
- 價格必須在網格範圍內
- 向上穿越 → SELL，向下穿越 → BUY

#### 5. **應用服務** (`internal/application/strategy_service.go`)
- ✅ 編排領域對象
- ✅ 定義 SignalPublisher 介面（端口）
- ✅ 處理價格更新用例
- ✅ 依賴介面，不依賴具體實現

**特點**：
```go
// 應用層定義介面，基礎設施層實現
type SignalPublisher interface {
    Publish(ctx context.Context, signal strategy.Signal) error
}

// 編排領域邏輯 + 基礎設施
func (s *StrategyService) HandlePriceUpdate(ctx context.Context, price float64) error
```

#### 6. **基礎設施層** (`internal/infrastructure/`)
- ✅ 配置管理（`config/`）
- ✅ Logger 工廠（`logger/`）
- ✅ 移動到正確的 DDD 位置

**環境變數**:
```bash
PORT=50052
ENVIRONMENT=development
LOG_LEVEL=debug
STRATEGY_TYPE=grid
STRATEGY_INSTRUMENTS=ETH-USDT
REDIS_ADDR=db.redis.orb.local:6379
```

---

## 📋 待完成的功能

### Phase 2: 基礎設施層實作（優先級：高）⭐ 下一步

**說明**：領域層和應用層已完成，現在需要實作基礎設施層的適配器（Adapters）

**Note**: Redis channel 使用 `.` 作為分隔符（例如：`market.ticker.ETH-USDT`）

#### 1. **Redis 客戶端** (`internal/infrastructure/messaging/redis_client.go`)
- [ ] 創建 Redis 客戶端封裝
- [ ] 支援連接池
- [ ] 健康檢查
- [ ] 提供統一的 Pub/Sub 和 Cache 介面

**實作範例**:
```go
package messaging

type RedisClient struct {
    rdb    *redis.Client
    logger logger.Logger
}

func NewRedisClient(addr, password string, db int, logger logger.Logger) (*RedisClient, error) {
    rdb := redis.NewClient(&redis.Options{
        Addr: addr, Password: password, DB: db, PoolSize: 10,
    })
    if err := rdb.Ping(context.Background()).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Redis: %w", err)
    }
    return &RedisClient{rdb: rdb, logger: logger}, nil
}
```

#### 2. **Candle 訂閱器** (`internal/infrastructure/messaging/candle_subscriber.go`)
- [ ] 實作 Redis Pub/Sub 訂閱器
- [ ] 訂閱 `market.candle.1m.{instId}` 頻道
- [ ] 解析 JSON 為 Candle 結構
- [ ] 提取價格並傳遞給應用層

**實作範例**:
```go
package messaging

type CandleSubscriber struct {
    client  *RedisClient
    logger  logger.Logger
}

// Subscribe 訂閱 Candle 數據並調用回調
func (s *CandleSubscriber) Subscribe(
    ctx context.Context,
    instID string,
    bar string,
    onCandle func(price float64) error,
) error {
    channel := fmt.Sprintf("market.candle.%s.%s", bar, instID)

    pubsub := s.client.rdb.Subscribe(ctx, channel)
    defer pubsub.Close()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case msg := <-pubsub.Channel():
            var candle struct {
                Close string `json:"close"`
            }
            if err := json.Unmarshal([]byte(msg.Payload), &candle); err != nil {
                s.logger.Error("Failed to parse candle", map[string]any{"error": err})
                continue
            }

            price, err := strconv.ParseFloat(candle.Close, 64)
            if err != nil {
                s.logger.Error("Invalid price", map[string]any{"error": err})
                continue
            }

            // 傳給應用層
            if err := onCandle(price); err != nil {
                s.logger.Error("Handler failed", map[string]any{"error": err})
            }
        }
    }
}
```

#### 3. **Signal 發布器** (`internal/infrastructure/messaging/signal_publisher.go`)
- [ ] 實作 `SignalPublisher` 介面（應用層定義的端口）
- [ ] 將 Signal 發布到 Redis Pub/Sub
- [ ] 頻道命名: `strategy.signals.{instId}`
- [ ] JSON 序列化

**實作範例**:
```go
package messaging

// RedisSignalPublisher 實作應用層的 SignalPublisher 介面
type RedisSignalPublisher struct {
    client *RedisClient
    logger logger.Logger
}

func NewRedisSignalPublisher(client *RedisClient, logger logger.Logger) *RedisSignalPublisher {
    return &RedisSignalPublisher{client: client, logger: logger}
}

// Publish 實作 application.SignalPublisher 介面
func (p *RedisSignalPublisher) Publish(ctx context.Context, signal strategy.Signal) error {
    channel := fmt.Sprintf("strategy.signals.%s", signal.InstID())

    // Signal 已實作 MarshalJSON，直接序列化
    data, err := json.Marshal(signal)
    if err != nil {
        return fmt.Errorf("failed to marshal signal: %w", err)
    }

    if err := p.client.rdb.Publish(ctx, channel, data).Err(); err != nil {
        return fmt.Errorf("failed to publish signal: %w", err)
    }

    p.logger.Debug("Signal published", map[string]any{"channel": channel})
    return nil
}
```

---

### Phase 3: 組裝與整合（優先級：高）

#### 1. **在 main.go 中組裝依賴**
- [ ] 創建 Redis 客戶端
- [ ] 創建 Candle 訂閱器
- [ ] 創建 Signal 發布器
- [ ] 創建 GridAggregate（領域層）
- [ ] 創建 StrategyService（應用層）
- [ ] 啟動訂閱循環

**實作範例**:
```go
func main() {
    cfg := config.Load()
    log := logger.Must(cfg)

    // 1. 基礎設施層 - Redis 客戶端
    redisClient, err := messaging.NewRedisClient(
        cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, log)
    if err != nil {
        log.Error("Failed to connect to Redis", map[string]any{"error": err})
        os.Exit(1)
    }
    defer redisClient.Close()

    // 2. 基礎設施層 - Signal 發布器（實作端口介面）
    signalPublisher := messaging.NewRedisSignalPublisher(redisClient, log)

    // 3. 領域層 - 創建網格聚合根
    grid, err := strategy.NewGridAggregate("ETH-USDT", 3000, 2000, 10)
    if err != nil {
        log.Error("Failed to create grid", map[string]any{"error": err})
        os.Exit(1)
    }

    // 4. 應用層 - 創建策略服務
    strategyService := application.NewStrategyService(grid, signalPublisher, log)

    // 5. 基礎設施層 - 訂閱 Candle 數據
    subscriber := messaging.NewCandleSubscriber(redisClient, log)
    go func() {
        if err := subscriber.Subscribe(
            context.Background(),
            "ETH-USDT",
            "1m",
            func(price float64) error {
                // 調用應用層用例
                return strategyService.HandlePriceUpdate(context.Background(), price)
            },
        ); err != nil {
            log.Error("Subscription failed", map[string]any{"error": err})
        }
    }()

    log.Info("Trading Strategy Server started", map[string]any{
        "grid": grid.GetState(),
    })

    // 等待退出信號
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Info("Shutting down...")
}
```

---

### Phase 4: 測試與優化（優先級：低）

#### 1. **領域層單元測試**
- [ ] 測試 Price 值對象（正數驗證）
- [ ] 測試 GridCalculator（網格線計算、穿越檢測）
- [ ] 測試 GridAggregate（業務規則、信號生成）
- [ ] 測試 Signal 值對象（序列化）

**測試範例**:
```go
func TestGridAggregate_ProcessPriceUpdate(t *testing.T) {
    grid, _ := strategy.NewGridAggregate("ETH-USDT", 3000, 2000, 5)

    // 測試向上穿越
    price, _ := strategy.NewPrice(2500)
    signal, err := grid.ProcessPriceUpdate(price)

    assert.NoError(t, err)
    assert.Nil(t, signal) // 第一次更新，沒有穿越

    // 測試穿越網格線
    price2, _ := strategy.NewPrice(2750)
    signal2, err := grid.ProcessPriceUpdate(price2)

    assert.NoError(t, err)
    assert.NotNil(t, signal2)
    assert.Equal(t, strategy.ActionSell, signal2.Action())
}
```

#### 2. **整合測試**
- [ ] 測試完整數據流（Redis → 策略服務 → Signal 發布）
- [ ] 使用 Mock Redis 測試訂閱器
- [ ] 測試錯誤處理（無效價格、斷線重連）

#### 3. **回測功能**（可選）
- [ ] 使用歷史 Candle 數據測試策略
- [ ] 計算歷史 P&L
- [ ] 優化網格參數

---

## 🎯 數據流

```
Market Data Service
    ↓ 發布 Candle 數據
Redis Pub/Sub (market.candle.1m.ETH-USDT)
    ↓ 訂閱 (CandleSubscriber - Infrastructure)
Trading Strategy Server
    ↓ 提取價格
Application Layer (StrategyService)
    ↓ 調用領域邏輯
Domain Layer (GridAggregate.ProcessPriceUpdate)
    ↓ 生成信號
Application Layer
    ↓ 通過端口發布
Infrastructure Layer (RedisSignalPublisher)
    ↓ 發布信號
Redis Pub/Sub (strategy.signals.ETH-USDT)
    ↓ 訂閱
Order Service (未實作)
    ↓ 驗證並執行交易
OKX API
```

**DDD 數據流說明**：
1. **Infrastructure → Application**: Candle 訂閱器接收數據，提取價格，調用應用層用例
2. **Application → Domain**: 應用層創建 Price 值對象，調用領域邏輯
3. **Domain**: 純業務邏輯計算，返回 Signal 值對象（或 nil）
4. **Application → Infrastructure**: 通過端口介面發布信號到 Redis

---

## 📚 相關文檔

- [項目整體架構](../../CLAUDE.md)
- [Market Data Service](../market-data-server/CLAUDE.md)

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
- **領域層完全獨立，不依賴基礎設施**
- **應用層定義端口，基礎設施層實現適配器**

---

## 🏆 設計原則

1. **單一職責** - 只負責策略計算，不執行交易
2. **關注點分離** - 數據訂閱、策略邏輯、信號發布分離
3. **可測試性** - 策略邏輯可獨立測試（不需要 Redis）
4. **可擴展性** - 易於添加新策略類型（DCA, Martingale, etc.）
5. **依賴反轉** - 應用層定義介面，基礎設施層實現

---

## 💡 DDD vs Layered Architecture

### 為什麼選擇 DDD？

**Trading Strategy Server 使用 DDD 的原因**：
- ✅ 網格策略是**複雜的業務邏輯**（網格線計算、穿越檢測、信號生成）
- ✅ 需要**高度可測試性**（獨立測試業務邏輯，無需 Redis）
- ✅ 策略算法會**頻繁變化**（優化、回測、參數調整）
- ✅ 將來要添加**多種策略**（Grid, DCA, Martingale）

**對比 Market Data Service 的 Layered Architecture**：
- Market Data Service 主要是**數據轉發**（OKX → Redis），業務邏輯簡單
- Trading Strategy Server 是**業務核心**（策略計算），需要 DDD 保護業務邏輯純粹性

### 架構對比

| 特性 | Layered Architecture | DDD |
|------|---------------------|-----|
| 業務邏輯複雜度 | 低（Market Data Server） | 高（Trading Strategy Server） |
| 可測試性 | 需要 Mock 基礎設施 | 領域層完全獨立測試 |
| 業務邏輯保護 | 可能洩漏到各層 | 完全封裝在領域層 |
| 擴展性 | 適合簡單場景 | 適合複雜業務場景 |
| 學習曲線 | 低 | 中等 |

---

## 🔮 Phase 5: gRPC 風險管理服務（未來實作）⭐

**說明**：目前專注於開倉策略（Redis Pub/Sub），風險管理（打平出場、止損）將來通過 gRPC 實現

### 架構：混合通信模式

| 通信方式 | 使用場景 | 方向 |
|---------|---------|------|
| **Redis Pub/Sub** | 開倉信號 | Strategy → Order（推送）|
| **gRPC** | 風險管理諮詢 | Order → Strategy（拉取）⭐ |

### 為什麼需要 gRPC？

**問題**：風險管理需要持倉信息，但 Strategy Service 是無狀態的

```
Order Service 持有：
- 31 筆未平倉多單
- 平均成本：2450
- 當前損益：-$500

Strategy Service 需要：
- 計算打平價格：2450 * 1.001 = 2452.45
- 判斷是否需要平倉
```

**解決方案**：Order Service 主動諮詢 Strategy Service

### 待實作任務

#### 1. **定義 Protocol Buffers** ⭐ 優先級：中

**檔案位置**: `shared/proto/strategy/strategy.proto`

```protobuf
syntax = "proto3";

package strategy;
option go_package = "dizzycoder.xyz/trading-system/shared/proto/strategy";

import "google/protobuf/timestamp.proto";

service StrategyService {
  // 風險管理諮詢（Order Service 呼叫）
  rpc GetRiskAdvice(RiskAdviceRequest) returns (RiskAdviceResponse);

  // 健康檢查
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

// ========== 風險管理請求 ==========
message RiskAdviceRequest {
  string inst_id = 1;                      // 交易對
  double current_price = 2;                // 當前價格
  Direction direction = 3;                 // 持倉方向（LONG/SHORT）

  // 持倉信息（Order Service 提供）
  repeated Position positions = 4;

  // 策略配置
  string strategy_type = 5;
  map<string, string> strategy_config = 6;
}

message Position {
  string position_id = 1;
  double entry_price = 2;                  // 開倉價格
  double size = 3;                         // 倉位大小（$）
  double take_profit = 4;                  // 停利百分比
  google.protobuf.Timestamp open_time = 5;
}

enum Direction {
  LONG = 0;
  SHORT = 1;
}

// ========== 風險管理響應 ==========
message RiskAdviceResponse {
  RiskAction action = 1;                   // 建議動作
  string reason = 2;                       // 原因

  double break_even_price = 3;             // 打平價格（含手續費）
  double average_cost = 4;                 // 平均成本
  double stop_loss_price = 5;              // 止損價格
}

enum RiskAction {
  HOLD = 0;                                // 持有
  CLOSE_ALL = 1;                           // 全部平倉（打平出場）
  STOP_LOSS = 2;                           // 止損出場
  PARTIAL_CLOSE = 3;                       // 部分平倉
}

message HealthCheckRequest {}
message HealthCheckResponse {
  bool healthy = 1;
}
```

**任務清單**：
- [ ] 創建 `shared/proto/strategy/` 目錄
- [ ] 編寫 `strategy.proto` 定義
- [ ] 編寫 Makefile 生成 Go 代碼
- [ ] 生成 `strategy.pb.go` 和 `strategy_grpc.pb.go`

---

#### 2. **實作 Risk Advisor（gRPC Server）** ⭐ 優先級：中

**檔案位置**: `internal/application/risk_advisor.go`

**功能**：
- 接收 Order Service 的持倉信息
- 計算平均成本
- 計算打平價格（含手續費 0.1%）
- 判斷是否需要平倉
- 返回風險建議

**核心邏輯**：

```go
package application

import (
    "context"
    pb "dizzycoder.xyz/trading-system/shared/proto/strategy"
    "dizzycode.xyz/logger"
)

type RiskAdvisor struct {
    pb.UnimplementedStrategyServiceServer
    logger logger.Logger
}

func NewRiskAdvisor(log logger.Logger) *RiskAdvisor {
    return &RiskAdvisor{logger: log}
}

func (r *RiskAdvisor) GetRiskAdvice(
    ctx context.Context,
    req *pb.RiskAdviceRequest,
) (*pb.RiskAdviceResponse, error) {

    // 1. 計算平均成本
    avgCost := r.calculateAverageCost(req.Positions)

    // 2. 計算打平價格（含手續費 0.1%）
    feeRate := 0.001
    var breakEvenPrice float64

    if req.Direction == pb.Direction_LONG {
        breakEvenPrice = avgCost * (1 + feeRate)
    } else {
        breakEvenPrice = avgCost * (1 - feeRate)
    }

    // 3. 計算當前損益
    totalSize := r.calculateTotalSize(req.Positions)
    var pnl float64
    if req.Direction == pb.Direction_LONG {
        pnl = (req.CurrentPrice - avgCost) * totalSize
    } else {
        pnl = (avgCost - req.CurrentPrice) * totalSize
    }

    // 4. 風險決策邏輯

    // 規則 1: 打平出場（損益接近 0）
    if pnl < 0 {
        if req.Direction == pb.Direction_LONG && req.CurrentPrice >= breakEvenPrice {
            return &pb.RiskAdviceResponse{
                Action:         pb.RiskAction_CLOSE_ALL,
                Reason:         "break_even_exit",
                BreakEvenPrice: breakEvenPrice,
                AverageCost:    avgCost,
            }, nil
        }

        if req.Direction == pb.Direction_SHORT && req.CurrentPrice <= breakEvenPrice {
            return &pb.RiskAdviceResponse{
                Action:         pb.RiskAction_CLOSE_ALL,
                Reason:         "break_even_exit",
                BreakEvenPrice: breakEvenPrice,
                AverageCost:    avgCost,
            }, nil
        }
    }

    // 規則 2: 止損（虧損超過 5%）
    maxLossRate := 0.05
    if pnl < -(totalSize * maxLossRate) {
        return &pb.RiskAdviceResponse{
            Action:        pb.RiskAction_STOP_LOSS,
            Reason:        "stop_loss_triggered",
            AverageCost:   avgCost,
            StopLossPrice: req.CurrentPrice,
        }, nil
    }

    // 規則 3: 持有
    return &pb.RiskAdviceResponse{
        Action:         pb.RiskAction_HOLD,
        Reason:         "within_acceptable_range",
        BreakEvenPrice: breakEvenPrice,
        AverageCost:    avgCost,
    }, nil
}

func (r *RiskAdvisor) calculateAverageCost(positions []*pb.Position) float64 {
    if len(positions) == 0 {
        return 0
    }

    totalValue := 0.0
    totalSize := 0.0

    for _, pos := range positions {
        totalValue += pos.EntryPrice * pos.Size
        totalSize += pos.Size
    }

    return totalValue / totalSize
}

func (r *RiskAdvisor) calculateTotalSize(positions []*pb.Position) float64 {
    total := 0.0
    for _, pos := range positions {
        total += pos.Size
    }
    return total
}

func (r *RiskAdvisor) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
    return &pb.HealthCheckResponse{Healthy: true}, nil
}
```

**任務清單**：
- [ ] 創建 `risk_advisor.go`
- [ ] 實作 `GetRiskAdvice()` 方法
- [ ] 實作平均成本計算
- [ ] 實作打平價格計算
- [ ] 實作風險判斷邏輯
- [ ] 添加單元測試

---

#### 3. **實作 gRPC Server（Infrastructure）** ⭐ 優先級：中

**檔案位置**: `internal/infrastructure/grpc/server.go`

**功能**：
- 啟動 gRPC Server
- 註冊 Risk Advisor 服務
- 處理連接和錯誤

```go
package grpc

import (
    "fmt"
    "net"

    "google.golang.org/grpc"
    pb "dizzycoder.xyz/trading-system/shared/proto/strategy"
    "dizzycode.xyz/logger"
    "dizzycode.xyz/trading-strategy-server/internal/application"
)

type Server struct {
    grpcServer  *grpc.Server
    riskAdvisor *application.RiskAdvisor
    logger      logger.Logger
    port        string
}

func NewServer(riskAdvisor *application.RiskAdvisor, port string, log logger.Logger) *Server {
    return &Server{
        riskAdvisor: riskAdvisor,
        logger:      log,
        port:        port,
    }
}

func (s *Server) Start() error {
    lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
    if err != nil {
        return fmt.Errorf("failed to listen: %w", err)
    }

    s.grpcServer = grpc.NewServer()
    pb.RegisterStrategyServiceServer(s.grpcServer, s.riskAdvisor)

    s.logger.Info("gRPC server starting", map[string]any{"port": s.port})

    return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
    if s.grpcServer != nil {
        s.logger.Info("Stopping gRPC server")
        s.grpcServer.GracefulStop()
    }
}
```

**任務清單**：
- [ ] 創建 `internal/infrastructure/grpc/` 目錄
- [ ] 實作 `server.go`
- [ ] 在 `main.go` 中啟動 gRPC Server
- [ ] 添加優雅關閉邏輯

---

#### 4. **整合到 main.go** ⭐ 優先級：中

**更新**: `cmd/main.go`

```go
func main() {
    cfg := config.Load()
    log := logger.Must(cfg)

    // ... 現有的 Redis Pub/Sub 邏輯 ...

    // ========== gRPC Server 設置 ⭐ ==========

    // 創建 Risk Advisor
    riskAdvisor := application.NewRiskAdvisor(log)

    // 創建 gRPC Server
    grpcServer := grpc.NewServer(riskAdvisor, cfg.Port, log)

    // 啟動 gRPC Server（背景運行）
    go func() {
        if err := grpcServer.Start(); err != nil {
            log.Error("gRPC server failed", map[string]any{"error": err})
        }
    }()

    log.Info("Trading Strategy Server started", map[string]any{
        "grpc_port": cfg.Port,
        "redis_subscriptions": channels,
    })

    // 等待退出信號
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Info("Shutting down...")

    // 優雅關閉 gRPC Server
    grpcServer.Stop()
}
```

**任務清單**：
- [ ] 在 main.go 中創建 Risk Advisor
- [ ] 在 main.go 中啟動 gRPC Server
- [ ] 添加優雅關閉邏輯
- [ ] 測試 gRPC Server 啟動

---

### 完整數據流（開倉 + 風險管理）

```
========== 開倉流程（Redis Pub/Sub）==========
Market Data Service
    ↓ Publish: market.candle.5m.BTC-USDT
Strategy Service (Grid Instance)
    ↓ Subscribe
    ↓ 判斷趨勢：平盤 ✅
    ↓ 檢查價格：觸及 MidLow ✅
    ↓ 生成開倉信號
    ↓ Publish: strategy.signals.long.BTC-USDT
Order Service
    ↓ Subscribe
    ↓ 執行開倉
    ↓ 記錄持倉：Position{entry: 2500, size: 200}

========== 風險管理流程（gRPC）⭐ ==========
Order Service（每次價格變化）
    ↓ 收集持倉：31 筆多單，平均成本 2450
    ↓ gRPC Call: GetRiskAdvice(positions, currentPrice)
Strategy Service (Risk Advisor)
    ↓ 計算打平價格：2450 * 1.001 = 2452.45
    ↓ 判斷：currentPrice >= breakEvenPrice?
    ↓ Yes → Return: CLOSE_ALL
Order Service
    ↓ 執行：平倉所有 31 筆
    ↓ 清空持倉
```

---

### 開發時程建議

| 階段 | 任務 | 優先級 | 預估時間 |
|------|------|--------|---------|
| **Phase 1** | 開倉策略（Redis Pub/Sub） | ⭐⭐⭐ 高 | 當前進行中 |
| **Phase 2** | Protocol Buffers 定義 | ⭐⭐ 中 | 1-2 小時 |
| **Phase 3** | Risk Advisor 實作 | ⭐⭐ 中 | 2-3 小時 |
| **Phase 4** | gRPC Server 整合 | ⭐⭐ 中 | 1-2 小時 |
| **Phase 5** | Order Service 整合 | ⭐⭐ 中 | 2-3 小時 |

**建議**：先完成開倉策略並測試，之後再實作 gRPC 風險管理

---

*最後更新: 2025-10-17*
*當前進度: Phase 1 - 實作開倉策略（方案 A 架構重構中）*
*下一步: 完成策略實例模式，加入趨勢判斷器*
