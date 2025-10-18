# Market Data Service - 启动流程详解

## 完整启动流程 🚀

假设 `.env` 文件配置如下：
```bash
PORT=50051
ENVIRONMENT=development
LOG_LEVEL=info
OKX_INSTRUMENTS=BTC-USDT,ETH-USDT
OKX_SUBSCRIBE_TICKER=false
OKX_SUBSCRIBE_CANDLES=1m,5m
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10
```

---

## Step 1: 加载配置 📋

**文件**: `cmd/main.go:16`

```go
cfg := config.Load()
```

**发生了什么**：

1. 读取 `.env` 文件
2. 解析环境变量
3. 创建配置对象：

```go
cfg = &Config{
    Port: "50051",
    Environment: "development",
    LogLevel: "info",
    OKX: {
        Instruments: ["BTC-USDT", "ETH-USDT"],
        Subscription: {
            Ticker: false,           // ← 不订阅 Ticker
            Candles: {               // ← 订阅 1m 和 5m K线
                "1m": true,
                "5m": true,
            },
        },
    },
    Redis: {
        Addr: "localhost:6379",
        Password: "",
        DB: 0,
        PoolSize: 10,
    },
}
```

---

## Step 2: 创建 Logger 🪵

**文件**: `cmd/main.go:19`

```go
log := logger.Must(cfg)
```

**发生了什么**：

1. 根据 `LOG_LEVEL=info` 创建 Zap Logger
2. 设置日志格式（Pretty Mode）
3. 输出启动日志：

```
2025-10-18T10:30:00 INFO: Starting Market Data Service environment=development port=50051
```

---

## Step 3: 连接 Redis 🔌

**文件**: `cmd/main.go:27-41`

```go
redisClient, err := redis.NewClient(redis.Config{
    Addr:     cfg.Redis.Addr,
    Password: cfg.Redis.Password,
    DB:       cfg.Redis.DB,
    PoolSize: cfg.Redis.PoolSize,
    Logger:   log,
})
```

**发生了什么**：

1. 创建 Redis 客户端（`*redis.Client`）
2. 测试连接（执行 `PING` 命令）
3. 如果成功，输出日志：

```
2025-10-18T10:30:00 INFO: Connected to Redis successfully host=localhost:6379 db=0
```

4. 如果失败，程序退出：

```
2025-10-18T10:30:00 ERROR: Failed to connect to Redis error="dial tcp [::1]:6379: connect: connection refused"
[程序退出]
```

---

## Step 4: 创建 Storage 实现 💾

**文件**: `cmd/main.go:43-45`

```go
marketStorage := storage.NewRedisStorage(redisClient, log)
```

**发生了什么**：

创建 `RedisStorage` 实例：

```go
marketStorage = &RedisStorage{
    client: redisClient,  // ← *redis.Client
    logger: log,
}
```

**注意**：这时候还没有任何数据操作，只是创建了对象。

---

## Step 5: 设置 WebSocket Manager 🌐

**文件**: `cmd/main.go:47-53`

```go
wsManager, err := websocket.Setup(cfg, log, marketStorage)
```

这是最复杂的一步，让我们深入展开：

### Step 5.1: 创建 Retention Policy

**文件**: `internal/websocket/setup.go:21-22`

```go
retention := config.DefaultRetentionPolicy()
```

创建数据保留策略：

```go
retention = &RetentionPolicy{
    CandleHistoryLength: {
        "1m": 200,   // 保留 200 根 1分钟 K线
        "5m": 200,   // 保留 200 根 5分钟 K线
        "1H": 200,
        "1D": 365,
        // ...
    },
}
```

### Step 5.2: 创建 Handler 实例

**文件**: `internal/websocket/setup.go:24-26`

```go
tickerHandler := handler.NewTickerHandler(marketStorage, log)
candleHandler := handler.NewCandleHandler(marketStorage, retention, log)
```

创建两个 Handler：

```go
tickerHandler = &TickerHandler{
    storage: marketStorage,  // ← storage.MarketDataStorage 接口
    logger:  log,
}

candleHandler = &CandleHandler{
    storage:   marketStorage,  // ← storage.MarketDataStorage 接口
    retention: retention,      // ← 保留策略
    logger:    log,
}
```

**关键**：这里发生了**隐式类型转换**：
```
*storage.RedisStorage → storage.MarketDataStorage (interface)
```

### Step 5.3: 创建 WebSocket Manager

**文件**: `internal/websocket/setup.go:28-32`

```go
wsManager := NewManager(Config{
    URL:    okx.BusinessWSURL,
    Logger: log,
})
```

创建 WebSocket 管理器：

```go
wsManager = &Manager{
    client:         ws.NewClient(...),  // ← 通用 WebSocket 客户端
    logger:         log,
    tickerHandlers: [],
    candleHandlers: [],
    subscriptions:  {},
}
```

**重要**：
- `URL` = `wss://ws.okx.com:8443/ws/v5/business` (OKX Business WebSocket)
- 这时候**还没有连接**，只是创建了对象

### Step 5.4: 注册 Handler

**文件**: `internal/websocket/setup.go:34-44`

```go
// 根据配置决定是否注册 Ticker Handler
if cfg.OKX.Subscription.Ticker {  // false，跳过
    wsManager.AddTickerHandler(tickerHandler.Handle)
}

// 根据配置注册 Candle Handler
if len(cfg.OKX.Subscription.Candles) > 0 {  // true，有 1m 和 5m
    wsManager.AddCandleHandler(candleHandler.Handle)
    log.Info("Candle handler registered", {"periods": ["1m", "5m"]})
}
```

现在 `wsManager` 的状态：

```go
wsManager = &Manager{
    client:         ws.Client{...},
    logger:         log,
    tickerHandlers: [],              // ← 空，因为 Ticker=false
    candleHandlers: [                // ← 有一个 handler
        candleHandler.Handle,        //    这是一个函数引用
    ],
    subscriptions:  {},
}
```

输出日志：
```
2025-10-18T10:30:00 INFO: Candle handler registered periods=[1m 5m]
```

### Step 5.5: 连接到 OKX WebSocket 🔌

**文件**: `internal/websocket/setup.go:46-50`

```go
if err := wsManager.Connect(); err != nil {
    return nil, fmt.Errorf("failed to connect to OKX WebSocket: %w", err)
}
```

**发生了什么**：

1. 建立 WebSocket 连接到 `wss://ws.okx.com:8443/ws/v5/business`
2. 启动 Ping/Pong 心跳机制（每 20 秒）
3. 启动消息接收 goroutine（在后台运行）

**通用 WebSocket 客户端** (`go-packages/websocket/client.go`) 现在在后台运行：

```go
// 后台 goroutine 1: 接收消息
go func() {
    for {
        _, message, err := conn.ReadMessage()
        if err != nil {
            // 处理错误
            break
        }
        // 调用 message handler（稍后会设置）
        c.handleMessage(message)
    }
}()

// 后台 goroutine 2: Ping/Pong 心跳
go func() {
    ticker := time.NewTicker(20 * time.Second)
    for {
        <-ticker.C
        conn.WriteMessage(websocket.PingMessage, []byte{})
    }
}()
```

输出日志（来自 WebSocket 客户端）：
```
2025-10-18T10:30:00 INFO: WebSocket connected url=wss://ws.okx.com:8443/ws/v5/business
2025-10-18T10:30:00 INFO: Starting ping loop interval=20s
```

### Step 5.6: 订阅交易对 📡

**文件**: `internal/websocket/setup.go:52-58`

```go
if err := subscribeInstruments(wsManager, cfg, log); err != nil {
    wsManager.Close()
    return nil, err
}
```

**发生了什么**：

遍历交易对列表 `["BTC-USDT", "ETH-USDT"]` 和 K线周期 `["1m", "5m"]`，发送订阅请求：

```go
// 订阅 BTC-USDT 的 1m K线
wsManager.SubscribeCandle("BTC-USDT", "1m")
// → 发送 WebSocket 消息:
{
  "op": "subscribe",
  "args": [{
    "channel": "candle1m",
    "instId": "BTC-USDT"
  }]
}

// 订阅 BTC-USDT 的 5m K线
wsManager.SubscribeCandle("BTC-USDT", "5m")
// → 发送 WebSocket 消息:
{
  "op": "subscribe",
  "args": [{
    "channel": "candle5m",
    "instId": "BTC-USDT"
  }]
}

// 订阅 ETH-USDT 的 1m K线
wsManager.SubscribeCandle("ETH-USDT", "1m")
// → 发送 WebSocket 消息...

// 订阅 ETH-USDT 的 5m K线
wsManager.SubscribeCandle("ETH-USDT", "5m")
// → 发送 WebSocket 消息...
```

**总共发送 4 个订阅请求**：
- BTC-USDT: 1m, 5m
- ETH-USDT: 1m, 5m

输出日志：
```
2025-10-18T10:30:00 INFO: Subscribed to candle instId=BTC-USDT bar=1m
2025-10-18T10:30:00 INFO: Subscribed to candle instId=BTC-USDT bar=5m
2025-10-18T10:30:00 INFO: Subscribed to candle instId=ETH-USDT bar=1m
2025-10-18T10:30:00 INFO: Subscribed to candle instId=ETH-USDT bar=5m
```

### Step 5.7: OKX 响应订阅确认 ✅

OKX WebSocket 服务器返回订阅确认消息：

```json
{
  "event": "subscribe",
  "arg": {
    "channel": "candle1m",
    "instId": "BTC-USDT"
  },
  "code": "0"
}
```

WebSocket Manager 接收到消息，处理并输出日志：

```
2025-10-18T10:30:01 INFO: Subscription confirmed channel=candle1m instId=BTC-USDT
2025-10-18T10:30:01 INFO: Subscription confirmed channel=candle5m instId=BTC-USDT
2025-10-18T10:30:01 INFO: Subscription confirmed channel=candle1m instId=ETH-USDT
2025-10-18T10:30:01 INFO: Subscription confirmed channel=candle5m instId=ETH-USDT
```

---

## Step 6: 启动完成 ✅

**文件**: `cmd/main.go:55-57`

```go
log.Info("Market Data Service started successfully", map[string]any{
    "instruments": cfg.OKX.Instruments,
})
```

输出日志：
```
2025-10-18T10:30:01 INFO: Market Data Service started successfully instruments=[BTC-USDT ETH-USDT]
```

---

## Step 7: 等待信号 ⏸️

**文件**: `cmd/main.go:59-62`

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit  // ← 阻塞在这里，等待 Ctrl+C
```

程序进入等待状态，同时：
- WebSocket 客户端在后台持续接收消息
- Ping/Pong 心跳每 20 秒执行一次
- 接收到 K线数据时会触发 Handler

---

## 运行时：接收 K线数据 📊

### 数据流

```
OKX WebSocket 服务器
  ↓ 推送 K线数据
{
  "arg": {
    "channel": "candle1m",
    "instId": "BTC-USDT"
  },
  "data": [[
    "1729234560000",  // timestamp
    "67000.0",        // open
    "67100.0",        // high
    "66900.0",        // low
    "67050.0",        // close
    "100.5",          // volume
    "6725000.0",      // volCcy
    "6725000.0",      // volCcyQuote
    "0"               // confirm (0=未确认, 1=已确认)
  ]]
}
  ↓
WebSocket Client (接收消息)
  ↓
WebSocket Manager (解析 JSON)
  ↓ 调用 handleMessage()
  ↓ 识别 channel="candle1m"
  ↓ 解析为 okx.Candle 对象
  ↓ 调用所有注册的 candleHandlers
  ↓
CandleHandler.Handle(candle)
  ↓ 1. 调用 storage.SaveLatestCandle()
  ↓
RedisStorage.SaveLatestCandle()
  ↓ 序列化为 JSON
  ↓ Redis SET 命令
Redis
  key: candle:latest:1m:BTC-USDT
  value: {"instId":"BTC-USDT","bar":"1m","open":"67000.0",...}
  TTL: 120秒

  ↓ 2. 如果 K线已确认 (confirm=1)
CandleHandler 检查: candle.IsConfirmed() == true
  ↓
  ↓ 调用 storage.AppendCandleHistory()
  ↓
RedisStorage.AppendCandleHistory()
  ↓ 获取保留策略: retention.GetMaxLength("1m") = 200
  ↓ Redis Pipeline
  ↓   LPUSH candle:history:1m:BTC-USDT "{...}"
  ↓   LTRIM candle:history:1m:BTC-USDT 0 199
Redis
  key: candle:history:1m:BTC-USDT
  type: List
  length: 最多 200
  [0]: 最新 K线
  [1]: 第二新
  ...
  [199]: 第 200 根 K线
```

### 日志输出

```
# Manager 自动打印
2025-10-18T10:30:05 INFO: Received candle instId=BTC-USDT bar=1m open=67000.0 high=67100.0 low=66900.0 close=67050.0 volume=100.5 confirm=0

# 如果是已确认的 K线 (confirm=1)
2025-10-18T10:31:00 INFO: Received candle instId=BTC-USDT bar=1m open=67000.0 high=67100.0 low=66900.0 close=67050.0 volume=100.5 confirm=1
2025-10-18T10:31:00 DEBUG: Appended candle to history key=candle:history:1m:BTC-USDT instId=BTC-USDT bar=1m maxLength=200
```

---

## Step 8: 优雅关闭 🛑

当用户按下 `Ctrl+C`：

```go
// 接收到 SIGINT 信号
<-quit  // ← 解除阻塞

log.Info("Shutting down Market Data Service...")

// defer 语句按倒序执行
defer wsManager.Close()       // 关闭 WebSocket 连接
defer redisClient.Close()     // 关闭 Redis 连接
```

输出日志：
```
^C
2025-10-18T10:35:00 INFO: Shutting down Market Data Service...
2025-10-18T10:35:00 INFO: WebSocket connection closed
2025-10-18T10:35:00 INFO: Redis connection closed
[程序退出]
```

---

## 关键对象的生命周期

| 对象 | 创建时机 | 作用域 | 销毁时机 |
|------|---------|--------|---------|
| `cfg` | Step 1 | main 函数 | 程序退出 |
| `log` | Step 2 | main 函数 | 程序退出 |
| `redisClient` | Step 3 | main 函数 | defer 关闭 |
| `marketStorage` | Step 4 | main 函数 | 随 redisClient 关闭 |
| `retention` | Step 5.1 | Setup 函数 | 被 candleHandler 持有 |
| `tickerHandler` | Step 5.2 | Setup 函数 | 被 wsManager 持有 |
| `candleHandler` | Step 5.2 | Setup 函数 | 被 wsManager 持有 |
| `wsManager` | Step 5.3 | main 函数 | defer 关闭 |

---

## 依赖注入流程图

```
main.go
  │
  ├─ cfg = config.Load()
  │
  ├─ log = logger.Must(cfg)
  │
  ├─ redisClient = redis.NewClient(...)
  │
  ├─ marketStorage = storage.NewRedisStorage(redisClient, log)
  │                     ↓
  │              实现 MarketDataStorage 接口
  │
  └─ wsManager = websocket.Setup(cfg, log, marketStorage)
       │                                        ↓
       ├─ retention = config.DefaultRetentionPolicy()
       │
       ├─ tickerHandler = handler.NewTickerHandler(marketStorage, log)
       │                                              ↑
       │                                     注入 storage 接口
       │
       ├─ candleHandler = handler.NewCandleHandler(marketStorage, retention, log)
       │                                              ↑            ↑
       │                                     注入 storage       注入策略
       │
       ├─ wsManager = NewManager(...)
       │
       ├─ wsManager.AddCandleHandler(candleHandler.Handle)
       │                                ↑
       │                          函数引用（闭包）
       │
       ├─ wsManager.Connect()
       │      ↓
       │   连接 OKX WebSocket
       │
       └─ subscribeInstruments(...)
              ↓
           发送订阅请求
```

---

## 总结

启动后系统进入以下状态：

✅ **已连接**：
- Redis (localhost:6379)
- OKX WebSocket (wss://ws.okx.com:8443/ws/v5/business)

✅ **已订阅**：
- BTC-USDT: 1m, 5m K线
- ETH-USDT: 1m, 5m K线

✅ **后台运行**：
- WebSocket 消息接收 goroutine
- Ping/Pong 心跳 goroutine (20秒间隔)

✅ **数据处理链**：
```
OKX → WebSocket → Manager → CandleHandler → RedisStorage → Redis
```

✅ **Redis 数据**：
- `candle:latest:1m:BTC-USDT` (SET, 最新 K线)
- `candle:latest:5m:BTC-USDT` (SET, 最新 K线)
- `candle:latest:1m:ETH-USDT` (SET, 最新 K线)
- `candle:latest:5m:ETH-USDT` (SET, 最新 K线)
- `candle:history:1m:BTC-USDT` (List, 最多 200 根已确认 K线)
- `candle:history:5m:BTC-USDT` (List, 最多 200 根已确认 K线)
- `candle:history:1m:ETH-USDT` (List, 最多 200 根已确认 K线)
- `candle:history:5m:ETH-USDT` (List, 最多 200 根已确认 K线)

---

*完整启动流程说明 - 2025-10-18*
