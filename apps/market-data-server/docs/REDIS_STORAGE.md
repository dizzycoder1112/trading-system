# Redis 数据存储设计

## 概述

Market Data Service 使用 **双重存储策略** 来平衡实时性和历史数据需求：

1. **SET**：存储最新数据（快速查询）
2. **List**：存储历史数据（策略计算）

---

## 数据结构

### 1. Ticker 数据（最新价格）

```redis
# Key 格式
price:latest:{instId}

# 示例
price:latest:BTC-USDT
price:latest:ETH-USDT

# 数据类型：String (JSON)
# TTL: 60秒
```

**数据格式**：
```json
{
  "instId": "BTC-USDT",
  "last": "115225.1",
  "vol24h": "7705.86942617",
  "high24h": "116000.0",
  "low24h": "114000.0",
  "ts": "1729012137281"
}
```

**使用方式**：
```bash
# 获取最新价格
GET price:latest:BTC-USDT
```

---

### 2. Candle 数据（K线）

#### 2.1 最新K线（包括未确认的）

```redis
# Key 格式
candle:latest:{bar}:{instId}

# 示例
candle:latest:1m:BTC-USDT
candle:latest:5m:BTC-USDT
candle:latest:1H:BTC-USDT

# 数据类型：String (JSON)
# TTL: 动态设置（1m=120s, 5m=600s, 1H=7200s）
```

**数据格式**：
```json
{
  "instId": "BTC-USDT",
  "bar": "1m",
  "ts": "1729012080000",
  "open": "115225.1",
  "high": "115300.0",
  "low": "115100.0",
  "close": "115250.0",
  "vol": "10.5",
  "volCcy": "1210312.5",
  "volCcyQuote": "1210312.5",
  "confirm": "0"  // 0=未确认, 1=已确认
}
```

**使用方式**：
```bash
# 获取最新K线（包括正在形成的）
GET candle:latest:1m:BTC-USDT
```

#### 2.2 历史K线（仅已确认的）⭐

```redis
# Key 格式
candle:history:{bar}:{instId}

# 示例
candle:history:1m:BTC-USDT
candle:history:5m:BTC-USDT
candle:history:1H:BTC-USDT

# 数据类型：List (JSON 数组)
# TTL: 无（通过 LTRIM 限制长度）
# 顺序：最新的在前（index 0 = 最新）
```

**数据格式**：
```json
// List 的每个元素都是一个 JSON 对象
[
  {
    "instId": "BTC-USDT",
    "bar": "1m",
    "ts": "1729012140000",  // 最新K线
    "open": "115250.0",
    "high": "115280.0",
    "low": "115220.0",
    "close": "115270.0",
    "vol": "12.3",
    "confirm": "1"
  },
  {
    "instId": "BTC-USDT",
    "bar": "1m",
    "ts": "1729012080000",  // 上一根K线
    "open": "115225.1",
    "high": "115300.0",
    "low": "115100.0",
    "close": "115250.0",
    "vol": "10.5",
    "confirm": "1"
  },
  // ... 更多历史K线
]
```

**使用方式**：
```bash
# 获取最近 10 根K线
LRANGE candle:history:1m:BTC-USDT 0 9

# 获取最近 50 根K线
LRANGE candle:history:1m:BTC-USDT 0 49

# 获取所有历史K线
LRANGE candle:history:1m:BTC-USDT 0 -1

# 获取K线数量
LLEN candle:history:1m:BTC-USDT
```

---

## 数据保留策略

### 历史K线保留数量

| 周期 | 保留数量 | 覆盖时间范围 |
|------|---------|-------------|
| 1s   | 60根    | 1分钟       |
| 1m   | 200根   | 3.3小时     |
| 3m   | 200根   | 10小时      |
| 5m   | 200根   | 16.6小时    |
| 15m  | 200根   | 2.08天      |
| 30m  | 200根   | 4.16天      |
| 1H   | 200根   | 8.3天       |
| 2H   | 200根   | 16.6天      |
| 4H   | 200根   | 33.3天      |
| 1D   | 365根   | 1年         |
| 1W   | 104根   | 2年         |
| 1M   | 60根    | 5年         |

### 自动清理机制

- **最新K线（SET）**：自动过期（TTL）
- **历史K线（List）**：通过 `LTRIM` 限制长度，超出部分自动删除

---

## 数据流

### 1. Ticker 数据流

```
OKX WebSocket (Ticker)
  ↓
Market Data Service
  ↓
Redis SET
  key: price:latest:BTC-USDT
  TTL: 60秒
```

### 2. Candle 数据流

```
OKX WebSocket (Candle)
  ↓
Market Data Service
  ↓
1. 始终更新最新K线（包括未确认的）
   Redis SET: candle:latest:1m:BTC-USDT
   TTL: 动态设置

2. 如果K线已确认（confirm=1）
   Redis List: candle:history:1m:BTC-USDT
   LPUSH + LTRIM（保留最近 N 根）
```

---

## 使用示例

### Strategy Service 如何使用

#### 1. 获取最新价格（实时）

```go
// 获取最新 Ticker
func (s *StrategyService) GetLatestPrice(instID string) (float64, error) {
    key := fmt.Sprintf("price:latest:%s", instID)

    data, err := s.redis.Get(ctx, key).Result()
    if err != nil {
        return 0, err
    }

    var ticker Ticker
    if err := json.Unmarshal([]byte(data), &ticker); err != nil {
        return 0, err
    }

    price, _ := strconv.ParseFloat(ticker.Last, 64)
    return price, nil
}
```

#### 2. 获取最新K线（包括未确认的）

```go
// 用于实时监控当前K线
func (s *StrategyService) GetCurrentCandle(instID, bar string) (*Candle, error) {
    key := fmt.Sprintf("candle:latest:%s:%s", bar, instID)

    data, err := s.redis.Get(ctx, key).Result()
    if err != nil {
        return nil, err
    }

    var candle Candle
    if err := json.Unmarshal([]byte(data), &candle); err != nil {
        return nil, err
    }

    return &candle, nil
}
```

#### 3. 获取历史K线（计算指标）⭐ 最常用

```go
// 获取最近 N 根已确认的K线
func (s *StrategyService) GetHistoryCandles(instID, bar string, count int) ([]*Candle, error) {
    key := fmt.Sprintf("candle:history:%s:%s", bar, instID)

    // 获取最近 count 根K线（index 0 到 count-1）
    data, err := s.redis.LRange(ctx, key, 0, int64(count-1)).Result()
    if err != nil {
        return nil, err
    }

    candles := make([]*Candle, 0, len(data))
    for _, item := range data {
        var candle Candle
        if err := json.Unmarshal([]byte(item), &candle); err != nil {
            continue
        }
        candles = append(candles, &candle)
    }

    return candles, nil
}

// 示例：计算移动平均线
func (s *StrategyService) CalculateMA(instID string, period int) (float64, error) {
    // 获取最近 period 根K线
    candles, err := s.GetHistoryCandles(instID, "5m", period)
    if err != nil {
        return 0, err
    }

    if len(candles) < period {
        return 0, fmt.Errorf("not enough data")
    }

    // 计算平均值
    sum := 0.0
    for _, candle := range candles {
        close, _ := strconv.ParseFloat(candle.Close, 64)
        sum += close
    }

    return sum / float64(period), nil
}
```

---

## 监控与调试

### 查看所有相关 Key

```bash
# 查看所有 Ticker Key
redis-cli KEYS "price:latest:*"

# 查看所有最新K线 Key
redis-cli KEYS "candle:latest:*"

# 查看所有历史K线 Key
redis-cli KEYS "candle:history:*"
```

### 查看数据

```bash
# 查看 BTC-USDT 最新价格
redis-cli GET price:latest:BTC-USDT

# 查看 BTC-USDT 1分钟K线
redis-cli GET candle:latest:1m:BTC-USDT

# 查看 BTC-USDT 历史K线数量
redis-cli LLEN candle:history:1m:BTC-USDT

# 查看最近 5 根历史K线
redis-cli LRANGE candle:history:1m:BTC-USDT 0 4
```

### 内存占用估算

假设订阅 2 个交易对（BTC-USDT, ETH-USDT），3 个周期（1m, 5m, 1H）：

```
Ticker 数据：
  2 个交易对 × 500 bytes ≈ 1 KB

最新K线（SET）：
  2 个交易对 × 3 个周期 × 600 bytes ≈ 3.6 KB

历史K线（List）：
  - 1m: 2 × 200 × 600 bytes ≈ 234 KB
  - 5m: 2 × 200 × 600 bytes ≈ 234 KB
  - 1H: 2 × 200 × 600 bytes ≈ 234 KB

总计：≈ 705 KB
```

完全可以接受！

---

## 设计优势

### ✅ 优点

1. **实时性**：最新K线（SET）即时更新，包括未确认的
2. **历史数据**：历史K线（List）保存已确认的，用于指标计算
3. **自动清理**：通过 TTL 和 LTRIM 自动管理数据生命周期
4. **内存高效**：只保留必要的历史数据，不会无限增长
5. **简单可靠**：使用 Redis 原生命令，不需要额外模块
6. **查询快速**：List 的 LRANGE 命令非常高效

### 📊 性能

- **写入**：LPUSH + LTRIM（Pipeline），单次操作 < 1ms
- **读取最新**：GET，单次操作 < 1ms
- **读取历史**：LRANGE，100 根K线 < 5ms

---

## 未来优化（可选）

如果数据量增大，可以考虑：

1. **Redis Streams**：更适合时间序列数据
2. **Redis TimeSeries Module**：专为时间序列优化，支持压缩
3. **时序数据库**：InfluxDB / TimescaleDB（长期存储）
4. **分层存储**：
   - Redis：最近 1 天（热数据）
   - PostgreSQL：最近 30 天（温数据）
   - S3：历史归档（冷数据）

但对于当前需求（网格交易策略），**双重存储（SET + List）完全足够**。

---

*最后更新: 2025-10-18*
