# Market Data Service - 重构总结

## ✅ 重构完成

**时间**: 2025-10-18
**重构类型**: Layered Architecture（分层架构）
**原因**: 解耦业务逻辑与基础设施，便于替换存储后端

---

## 重构前后对比

### 重构前（耦合设计）❌

```
internal/
├── redis/
│   └── publisher.go          # 包含所有业务逻辑 + Redis 实现
├── websocket/
│   └── setup.go              # 直接依赖 redis.Publisher
└── okx/
    └── types.go
```

**问题**：
- ❌ 业务逻辑（保留多少 K 线）与 Redis 实现耦合
- ❌ 无法替换为 Kafka/RabbitMQ
- ❌ 违反依赖倒置原则
- ❌ 难以测试（无法 mock storage）

### 重构后（分层架构）✅

```
internal/
├── handler/                   # Application Layer（业务逻辑）
│   ├── ticker_handler.go     # Ticker 处理逻辑
│   └── candle_handler.go     # Candle 处理逻辑（含历史策略）
│
├── storage/                   # Infrastructure Layer（可替换）
│   ├── storage.go            # 接口定义（抽象层）⭐
│   └── redis_storage.go      # Redis 实现
│
├── config/
│   ├── config.go
│   └── retention.go          # 数据保留策略
│
├── websocket/                 # Presentation Layer
│   ├── manager.go
│   └── setup.go              # 依赖 storage 接口
│
├── okx/                       # 适配层
│   └── types.go
│
├── redis/                     # 基础设施（Redis 客户端）
│   ├── client.go
│   └── publisher.go          # 保留用于向后兼容
│
└── logger/
    └── factory.go
```

---

## 核心改动

### 1. 定义 Storage 接口（抽象层）⭐

**文件**: `internal/storage/storage.go`

```go
type MarketDataStorage interface {
    SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error
    SaveLatestCandle(ctx context.Context, candle okx.Candle) error
    AppendCandleHistory(ctx context.Context, candle okx.Candle, maxLength int) error
}
```

**作用**：
- ✅ 定义抽象接口，业务逻辑依赖接口而不是具体实现
- ✅ 遵循依赖倒置原则（DIP）
- ✅ 易于替换实现（Redis → Kafka → RabbitMQ）

---

### 2. 实现 Redis Storage

**文件**: `internal/storage/redis_storage.go`

```go
type RedisStorage struct {
    client *redis.Client
    logger logger.Logger
}

func (s *RedisStorage) SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error {
    // Redis 实现细节
}
```

**特点**：
- ✅ 实现 `MarketDataStorage` 接口
- ✅ 封装所有 Redis 操作细节
- ✅ 可以轻松替换为其他实现

---

### 3. 创建 Handler 层（业务逻辑）

**文件**:
- `internal/handler/ticker_handler.go`
- `internal/handler/candle_handler.go`

```go
type TickerHandler struct {
    storage storage.MarketDataStorage  // 依赖抽象接口
    logger  logger.Logger
}

func (h *TickerHandler) Handle(ticker okx.Ticker) error {
    // 业务逻辑：调用 storage 保存数据
    return h.storage.SaveLatestPrice(ctx, ticker)
}
```

**职责**：
- ✅ 接收 OKX 数据
- ✅ 应用业务规则（如保留策略）
- ✅ 调用 storage 接口保存数据
- ✅ 不关心存储实现细节

---

### 4. 数据保留策略配置

**文件**: `internal/config/retention.go`

```go
type RetentionPolicy struct {
    CandleHistoryLength map[string]int // bar -> 保留数量
}

func DefaultRetentionPolicy() *RetentionPolicy {
    return &RetentionPolicy{
        CandleHistoryLength: map[string]int{
            "1m": 200,  // 3.3小时
            "5m": 200,  // 16.6小时
            "1H": 200,  // 8.3天
            "1D": 365,  // 1年
        },
    }
}
```

**作用**：
- ✅ 将配置与实现分离
- ✅ 易于调整保留策略
- ✅ 可根据不同环境使用不同策略

---

### 5. 更新 Setup（依赖注入）

**文件**: `internal/websocket/setup.go`

```go
// 重构前
func Setup(cfg *config.Config, log logger.Logger, publisher *redis.Publisher) (*Manager, error)

// 重构后
func Setup(
    cfg *config.Config,
    log logger.Logger,
    marketStorage storage.MarketDataStorage,  // 注入接口
) (*Manager, error) {
    // 创建 handlers
    tickerHandler := handler.NewTickerHandler(marketStorage, log)
    candleHandler := handler.NewCandleHandler(marketStorage, retention, log)

    // 注册 handlers
    wsManager.AddTickerHandler(tickerHandler.Handle)
    wsManager.AddCandleHandler(candleHandler.Handle)
}
```

**改进**：
- ✅ 依赖接口而不是具体实现
- ✅ 支持依赖注入
- ✅ 易于测试（可注入 mock storage）

---

### 6. 更新 Main（可替换实现）

**文件**: `cmd/main.go`

```go
// 创建 Storage 实现（可替换！）
marketStorage := storage.NewRedisStorage(redisClient.GetClient(), log)

// 注入到 Setup
wsManager, err := websocket.Setup(cfg, log, marketStorage)
```

**易于替换**：
```go
// 替换为 Kafka（只需改一行！）
marketStorage := storage.NewKafkaStorage(kafkaProducer, log)

// 替换为 RabbitMQ
marketStorage := storage.NewRabbitMQStorage(rabbitmqConn, log)

// Setup 不需要改变！
wsManager, err := websocket.Setup(cfg, log, marketStorage)
```

---

## 重构优势

### 1. 解耦（Decoupling）✅

```
重构前：
Handler → Redis Implementation（紧耦合）

重构后：
Handler → Storage Interface → Redis Implementation（松耦合）
```

### 2. 易于替换（Swappable）✅

```go
// 只需实现接口，无需修改业务逻辑
type KafkaStorage struct {
    producer *kafka.Producer
}

func (k *KafkaStorage) SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error {
    // Kafka 实现
    return k.producer.Produce(...)
}

// 在 main.go 中替换
marketStorage := storage.NewKafkaStorage(...)  // ← 只改这一行
wsManager, _ := websocket.Setup(cfg, log, marketStorage)
```

### 3. 易于测试（Testable）✅

```go
// Mock Storage 进行单元测试
type MockStorage struct{}

func (m *MockStorage) SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error {
    // 测试逻辑
    return nil
}

// 测试 Handler（不需要真实 Redis）
func TestTickerHandler(t *testing.T) {
    mockStorage := &MockStorage{}
    handler := handler.NewTickerHandler(mockStorage, log)

    // 测试业务逻辑
    err := handler.Handle(testTicker)
    assert.NoError(t, err)
}
```

### 4. 清晰的职责分离✅

| 层级 | 职责 | 依赖 |
|------|------|------|
| **Handler** | 业务逻辑 | Storage 接口 |
| **Storage Interface** | 定义抽象 | 无依赖 |
| **Redis Storage** | 实现细节 | Storage 接口 |
| **WebSocket Manager** | 数据接收 | Handler |

---

## 数据流

```
OKX WebSocket
  ↓
WebSocket Manager
  ↓
Handler (业务逻辑)
  - TickerHandler.Handle()
  - CandleHandler.Handle()
  ↓
Storage Interface (抽象)
  - SaveLatestPrice()
  - SaveLatestCandle()
  - AppendCandleHistory()
  ↓
Redis Storage (实现)
  - 写入 Redis SET
  - 写入 Redis List
```

---

## 编译验证

```bash
$ cd apps/market-data-server
$ go build -o bin/market-data-server ./cmd/main.go
✅ 编译成功，无错误
```

---

## 文件清单

### 新增文件

- ✅ `internal/storage/storage.go` - Storage 接口定义
- ✅ `internal/storage/redis_storage.go` - Redis 实现
- ✅ `internal/handler/ticker_handler.go` - Ticker 处理器
- ✅ `internal/handler/candle_handler.go` - Candle 处理器
- ✅ `internal/config/retention.go` - 数据保留策略

### 修改文件

- ✅ `internal/websocket/setup.go` - 使用 storage 接口
- ✅ `cmd/main.go` - 创建 storage 实现并注入

### 保留文件

- ✅ `internal/redis/client.go` - Redis 客户端
- ✅ `internal/redis/publisher.go` - 保留用于向后兼容（未来可删除）

---

## 未来扩展

### 添加 Kafka Storage

```go
// internal/storage/kafka_storage.go
type KafkaStorage struct {
    producer *kafka.Producer
    logger   logger.Logger
}

func NewKafkaStorage(producer *kafka.Producer, logger logger.Logger) *KafkaStorage {
    return &KafkaStorage{producer: producer, logger: logger}
}

func (k *KafkaStorage) SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error {
    data, _ := json.Marshal(ticker)
    return k.producer.Produce(&kafka.Message{
        Topic: "market.ticker",
        Key:   []byte(ticker.InstID),
        Value: data,
    })
}

// 其他方法实现...
```

### 添加 RabbitMQ Storage

```go
// internal/storage/rabbitmq_storage.go
type RabbitMQStorage struct {
    channel *amqp.Channel
    logger  logger.Logger
}

func (r *RabbitMQStorage) SaveLatestPrice(ctx context.Context, ticker okx.Ticker) error {
    data, _ := json.Marshal(ticker)
    return r.channel.Publish(
        "market.exchange",
        "ticker." + ticker.InstID,
        false,
        false,
        amqp.Publishing{
            ContentType: "application/json",
            Body:        data,
        },
    )
}
```

---

## 架构原则

本次重构遵循以下软件工程原则：

1. **依赖倒置原则（DIP）**: Handler 依赖 Storage 接口，而不是具体实现
2. **单一职责原则（SRP）**: Handler 只负责业务逻辑，Storage 只负责存储
3. **开闭原则（OCP）**: 对扩展开放（添加新 Storage），对修改封闭（Handler 不需要改）
4. **里氏替换原则（LSP）**: 所有 Storage 实现可以互换使用
5. **接口隔离原则（ISP）**: Storage 接口只定义必要的方法

---

## 总结

✅ **重构成功**
- 编译通过，无错误
- 架构清晰，职责分明
- 易于扩展和维护
- 为未来多存储后端支持打下基础

🎯 **下一步**
- 运行服务，验证功能
- 添加单元测试
- 考虑添加 Kafka/RabbitMQ Storage（按需）

---

*重构完成时间: 2025-10-18*
*重构耗时: ~40分钟*
*架构模式: Layered Architecture*
