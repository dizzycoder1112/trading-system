# Trading System Architecture Design

## Overview

A microservices-based grid trading bot system built primarily with Golang, designed for OKX cryptocurrency exchange. The system follows a monorepo structure with clear separation of concerns across multiple services.

## System Architecture

```
┌─────────────────────────────────────────────────────┐
│                    API Gateway                       │
│            (Optional, TS for Web UI)                 │
└──────────────────┬──────────────────────────────────┘
                   │
     ┌─────────────┼─────────────┬──────────────┐
     │             │             │              │
     ▼             ▼             ▼              ▼
┌─────────┐  ┌──────────┐  ┌─────────┐  ┌──────────┐
│ Market  │  │ Trading  │  │ Order   │  │ Risk     │
│ Data    │  │ Strategy │  │ Service │  │ Manager  │
│ Service │  │ Service  │  │(Executor)│  │ Service  │
└─────────┘  └──────────┘  └─────────┘  └──────────┘
     │             ▲             │              │
     │             │ gRPC        │              │
     │ Redis       │"What should │              │
     │ Pub/Sub     │ I do now?"  │              │
     │             └─────────────┘              │
     │                                          │
     └──────────────────────────────────────────┘
                   │
              ┌────┴────┐
              │         │
         ┌────▼───┐ ┌──▼─────┐
         │ Redis  │ │Postgres│
         │(Cache) │ │  (DB)  │
         └────────┘ └────────┘
```

### Communication Patterns

- **Market Data → Strategy**: Redis Pub/Sub (broadcast, price updates)
- **Strategy → Order**: Redis Pub/Sub (signal push, trading signals) ⭐ **HYBRID MODEL**
- **Order → OKX**: REST API + Private WebSocket (external API)
- **All Services → Redis/DB**: Direct connection

### Key Design Decision ⭐

**Hybrid Model: Strategy pushes signals, Order validates and executes**:
- **Strategy Service**: Continuously monitors prices and publishes trading signals (should we trade?)
- **Order Service**: Subscribes to signals, validates feasibility, calculates quantity (can we trade? + how much?)
- **Clear separation of concerns**: Strategy = trading logic, Order = risk control + execution
- **Benefits**: Decoupled, scalable, single decision point for strategy logic

## Core Services

### 1. Market Data Service (Go) ⭐ Priority #1

**Purpose**: Acts as the price oracle for the entire system

**Responsibilities**:
- Connect to OKX WebSocket API
- Receive and process real-time price updates
- Publish price data to Redis Pub/Sub or Message Queue
- Provide REST API for querying latest prices
- Handle WebSocket reconnection logic
- Optional: Aggregate data from multiple exchanges

**Why It's Essential**:
- ✅ Decoupling: Other services don't need direct OKX connections
- ✅ Single source of truth: All services see consistent prices
- ✅ Fault tolerance: Centralized connection management
- ✅ Scalability: Easy to add more exchanges later

**API Endpoints**:
```
GET /api/v1/ticker/:instId          # Get latest price
GET /api/v1/orderbook/:instId       # Get order book
WS  /ws/v1/subscribe                # WebSocket subscription
```

**Technology Stack**:
- Go + gorilla/websocket
- Redis for pub/sub
- OKX WebSocket API: `wss://ws.okx.com:8443/ws/v5/public`

---

### 2. Trading Strategy Service (Go) - DDD Architecture ⭐ **The Signal Generator**

**Purpose**: Continuously monitor market and generate trading signals based on strategy logic

**Responsibilities**:
- **Subscribe to Market Data Service** for latest price updates (via Redis Pub/Sub)
- Calculate grid strategy logic (grid lines, trigger conditions)
- **Publish trading signals** to Redis Pub/Sub (via `strategy:signals:{instId}` channel)
- **Stateless design** - only focuses on "should we trade?" not "can we trade?"
- Generate signals based purely on price action and strategy parameters

**Architecture**: Domain-Driven Design (DDD)
- **Domain Layer**: Pure business logic (Grid Calculator, Price, Signal Generator)
- **Application Layer**: Use case orchestration (Signal Publishing Service)
- **Infrastructure Layer**: Redis price subscriber + signal publisher

**Grid Strategy Parameters**:
- Upper price bound
- Lower price bound
- Number of grid levels
- Grid spacing (arithmetic/geometric)
- Signal generation thresholds

**Communication**:
- Input: Price updates from Market Data Service (via Redis Pub/Sub)
- Output: Trading signals published to Redis Pub/Sub ⭐

**Signal Structure**:
```go
type TradingSignal struct {
    InstID    string    // Trading pair
    Action    Action    // BUY/SELL/CLOSE_LONG/CLOSE_SHORT
    Price     float64   // Trigger price
    Reason    string    // "Grid level #5 triggered at $50000"
    Timestamp time.Time // Signal generation time
    GridLevel int       // Which grid level triggered
}
```

**Why Push Model (Strategy → Order)** ⭐:
- ✅ Strategy is proactive: Continuously monitors and generates signals
- ✅ Better decoupling: Strategy doesn't need to know Order Service exists
- ✅ Scalable: Multiple services can subscribe to same signals
- ✅ Event-driven: Follows typical financial system patterns (Signal → Execution)
- ✅ Clear responsibility: Strategy = "should trade?", Order = "can trade? + how much?"

---

### 3. Order Service (Go) ⭐ **The Executor/Validator**

**Purpose**: Validate signals and execute orders - **The ONLY service with OKX API Key**

**Responsibilities**:
- **Subscribe to trading signals** from Strategy Service (via Redis Pub/Sub)
- **Validate signal feasibility** - check if order can be executed
- **Maintain position state** - track holdings, cost basis, P&L
- **Calculate order quantity** based on position size and risk limits
- **Execute orders via OKX REST API** (place buy/sell orders)
- **Subscribe to OKX Private WebSocket** (monitor order fills)
- **Monitor fills and update position state**
- Track order status (pending, filled, cancelled)
- Handle order failures and retries
- Maintain order history in PostgreSQL

**Signal Validation Logic**:
```go
// Order Service subscribes to signals
func (o *OrderService) OnSignal(signal TradingSignal) {
    // 1. Get current position
    position := o.getPosition(signal.InstID)

    // 2. Validate if signal can be executed
    if !o.canExecute(signal, position) {
        o.logger.Info("Signal ignored",
            "reason", "insufficient balance or position limit reached")
        return
    }

    // 3. Calculate order quantity (Order Service's responsibility)
    quantity := o.calculateQuantity(signal, position)

    // 4. Place order via OKX API
    o.placeOrder(signal.Action, signal.Price, quantity)
}
```

**OKX API Integration**:
```
# REST API (Private - Requires API Key)
POST /api/v5/trade/order           # Place order
GET  /api/v5/trade/order           # Get order details
GET  /api/v5/trade/orders-pending  # Get pending orders
POST /api/v5/trade/cancel-order    # Cancel order
GET  /api/v5/account/positions     # Get current positions

# WebSocket (Private - Requires Signature)
wss://ws.okx.com:8443/ws/v5/private
Subscribe: {"op":"subscribe","args":[{"channel":"orders","instType":"SPOT"}]}
```

**Trading Loop Lifecycle** ⭐:
1. **Receive signal** from Strategy Service (via Redis Pub/Sub)
2. **Get current position** (from local state or OKX API)
3. **Validate signal** (check balance, position limits, risk controls)
4. **Calculate quantity** (based on position size and risk parameters)
5. **Execute order** (place order via OKX REST API)
6. **Monitor fill** (via OKX Private WebSocket)
7. **Update position state** (recalculate cost basis, P&L)

**Validation Rules**:
- Check available balance before buy orders
- Check position size before sell orders
- Verify signal is not stale (timestamp check)
- Prevent duplicate orders (debouncing)
- Apply risk limits (max position size, daily loss limit)

**Features**:
- Event-driven signal processing
- Order retry logic with exponential backoff
- Signal validation and risk controls
- Real-time order status updates via Private WebSocket
- Position state management (holdings, cost basis, unrealized P&L)
- Order deduplication and throttling

---

### 4. Risk Manager Service (Go)

**Responsibilities**:
- Monitor positions and account balance
- Check margin ratio and liquidation risk
- Implement stop-loss/take-profit logic
- Prevent over-trading
- Alert on abnormal conditions

**Risk Controls**:
- Maximum position size limits
- Daily loss limits
- Exposure limits per trading pair
- Emergency shutdown triggers

---

### 5. API Gateway / Dashboard (TypeScript) - Optional

**Responsibilities**:
- Web UI interface
- View positions, orders, P&L
- Adjust strategy parameters
- Historical data visualization
- System health monitoring

**Technology Stack**:
- Next.js / React
- TailwindCSS
- WebSocket for real-time updates

---

## Data Flow

### 1. Price Update Flow (Redis Pub/Sub - Broadcast)
```
OKX WebSocket (Public)
  ↓
Market Data Service
  ↓ Publish
Redis Pub/Sub (market.ticker.BTC-USDT-SWAP)
  ↓ Subscribe
Trading Strategy Service (monitors price, generates signals)
```

### 2. Signal Generation Flow (Strategy → Redis) ⭐ **HYBRID MODEL**
```
Trading Strategy Service
  ↓ Receive price update: BTC-USDT-SWAP = $50000
  ↓ Calculate grid logic
Grid Calculator: "Price $50000 hits grid level #5 (sell trigger)"
  ↓ Generate signal
Signal: {Action: SELL, Price: 50000, Reason: "Grid level #5 triggered", GridLevel: 5}
  ↓ Publish to Redis
Redis Pub/Sub (strategy.signals.BTC-USDT-SWAP)
```

### 3. Signal Execution Flow (Redis → Order Service) ⭐
```
Redis Pub/Sub (strategy.signals.BTC-USDT-SWAP)
  ↓ Subscribe
Order Service - Receives Signal
  ↓ Signal: {Action: SELL, Price: 50000, Reason: "Grid level #5"}
  ↓ 1. Get current position
Position: {size: 0.5 BTC, avgCost: $48000, balance: $10000}
  ↓ 2. Validate signal
Validation: ✅ Can sell (have 0.5 BTC)
  ↓ 3. Calculate quantity
Quantity: 0.1 BTC (based on grid size + risk limits)
  ↓ 4. Execute order
OKX REST API (POST /api/v5/trade/order)
  ↓ 5. Update database
PostgreSQL (orders table)
```

### 4. Order Fill Monitoring Flow (Private WebSocket) ⭐
```
Order Service
  ↓ Subscribe (with API Key signature)
OKX Private WebSocket (orders channel)
  ↓ Push fill event
Order Update: {orderId: "123456", state: "filled", avgPx: "50000"}
  ↓ Handle fill event
Order Service - onOrderFilled()
  ↓ 1. Update position state (new cost basis, size)
Position updated: {size: 0.4 BTC, avgCost: $48500}
  ↓ 2. Update database
PostgreSQL: Save trade record
  ↓ 3. Wait for new signals
(Strategy Service continues monitoring, will generate new signals based on new price levels)
```

### 5. Complete Trading Loop ⭐
```
┌─────────────────────────────────────────┐
│ 1. Market Data Service                   │
│    OKX WebSocket → Redis Pub/Sub         │
└─────────────┬───────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│ 2. Strategy Service (Signal Generator)   │
│    - Monitor price updates               │
│    - Calculate grid logic                │
│    - Publish trading signals             │
└─────────────┬───────────────────────────┘
              ↓ Redis Pub/Sub
┌─────────────────────────────────────────┐
│ 3. Order Service (Validator + Executor)  │
│    - Receive signal                      │
│    - Validate feasibility                │
│    - Calculate quantity                  │
│    - Execute order                       │
└─────────────┬───────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│ 4. Order Execution                       │
│    Order Service → OKX API               │
└─────────────┬───────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│ 5. Fill Monitoring & State Update        │
│    OKX WebSocket → Order Service         │
│    Update position & database            │
└─────────────┬───────────────────────────┘
              ↓
          Continuous Loop
```

### Communication Pattern Summary

| Flow | Technology | Pattern | Reason |
|------|-----------|---------|--------|
| Market Data → Strategy | Redis Pub/Sub | Broadcast (1-to-many) | Strategy monitors prices for signal generation |
| **Strategy → Order** ⭐ | **Redis Pub/Sub** | **Event Push (signals)** | **Strategy publishes signals, Order subscribes** |
| Order → OKX | REST + WebSocket | External API | OKX's API design |
| OKX → Order | Private WebSocket | Push (order fills) | Real-time fill notifications |
| Order → Database | Direct | Internal | Persist order history |

---

## Technology Stack

### Languages
- **Primary**: Golang (all core services)
- **Secondary**: TypeScript (optional dashboard)

### Infrastructure
- **Database**: PostgreSQL (order history, trading records)
- **Cache**: Redis (real-time data, pub/sub, state management)
- **Message Queue**: NATS / RabbitMQ / Kafka (optional, for service communication)
- **Container**: Docker + Docker Compose
- **Orchestration**: Kubernetes (for production)

### Go Libraries
```
gorilla/websocket         # WebSocket client
go-redis/redis            # Redis client (Pub/Sub for signals) ⭐
lib/pq                    # PostgreSQL driver
gin-gonic/gin             # HTTP framework (optional REST API)
spf13/viper               # Configuration management
uber-go/zap               # Structured logging
encoding/json             # JSON serialization for signals
```

---

## Data Storage Strategy

### What NOT to Store
- ❌ Every tick/price update (86,400+ records per day)
- ❌ Raw WebSocket messages

### What TO Store
- ✅ Order execution records
- ✅ Position changes
- ✅ Key price levels (grid triggers)
- ✅ Hourly P&L snapshots
- ✅ Aggregated K-line data (candlesticks)

### Data Retention Policy
```
Real-time tick data:  Keep in memory only (Redis, 1-7 days TTL)
1-minute K-lines:     30 days
5-minute K-lines:     90 days
1-hour K-lines:       1 year
Daily K-lines:        Permanent
Order records:        Permanent
Trade history:        Permanent
```

---

## Development Roadmap

### Phase 1: Foundation (Week 1-2)
1. ✅ Set up monorepo structure
2. ✅ Create Market Data Service
   - Connect to OKX WebSocket
   - Store prices in Redis
   - Provide simple HTTP API

### Phase 2: Core Trading Logic (Week 3-4)
3. ✅ Create Grid Engine Service
   - Read prices from Redis
   - Calculate grid logic
   - Generate signals (log only first)

### Phase 3: Order Execution (Week 5-6)
4. ✅ Create Order Manager Service
   - Receive signals and place orders
   - Handle order lifecycle

### Phase 4: Risk Management (Week 7-8)
5. ✅ Create Risk Manager Service
   - Add risk control logic
   - Position monitoring

### Phase 5: Visualization (Optional)
6. ✅ Create Dashboard (TypeScript)
   - Web UI for monitoring
   - Real-time updates

---

## OKX API Reference

### Market Data (Public)
```
# REST API
GET /api/v5/market/ticker?instId=ETH-USDT-SWAP

# WebSocket
wss://ws.okx.com:8443/ws/v5/public
Subscribe: {"op":"subscribe","args":[{"channel":"tickers","instId":"ETH-USDT-SWAP"}]}
```

### Trading (Private - Requires Authentication)
```
POST /api/v5/trade/order
GET  /api/v5/account/balance
GET  /api/v5/account/positions
```

### Instrument ID Format
- Spot: `ETH-USDT`
- Perpetual Swap: `ETH-USDT-SWAP`
- Futures: `ETH-USDT-220325`

---

## Security Considerations

1. **API Key Management**
   - Store in environment variables or secret management system
   - Never commit to repository
   - Use different keys for different services if possible

2. **WebSocket Authentication**
   - Implement signature-based authentication for private channels
   - Handle token refresh

3. **Rate Limiting**
   - Respect OKX API rate limits
   - Implement exponential backoff

4. **Error Handling**
   - Graceful degradation
   - Circuit breaker pattern
   - Comprehensive logging

---

## Monitoring & Observability

### Metrics to Track
- WebSocket connection status
- Order success/failure rates
- API latency
- Grid execution performance
- P&L tracking
- System resource usage

### Logging Strategy
- Structured logging (JSON format)
- Different log levels per environment
- Centralized log aggregation (optional: ELK stack)

### Alerts
- WebSocket disconnection
- Order execution failures
- Risk limit breaches
- System errors

---

## Configuration Management

```yaml
# config/development.yaml
market_data:
  okx:
    ws_url: "wss://ws.okx.com:8443/ws/v5/public"
    rest_url: "https://www.okx.com"
  redis:
    host: "localhost:6379"

grid_engine:
  strategy:
    upper_bound: 3000
    lower_bound: 2000
    grid_levels: 20
    position_size: 0.01

order_manager:
  okx:
    api_key: "${OKX_API_KEY}"
    secret_key: "${OKX_SECRET_KEY}"
    passphrase: "${OKX_PASSPHRASE}"

risk_manager:
  max_position: 1.0
  daily_loss_limit: 100
```

---

## Project Structure

```
trading-system/
├── apps/
│   ├── market-data-server/        # Go service (WebSocket → Redis Pub/Sub)
│   ├── trading-strategy-server/   # Go service (DDD, Signal Generator)
│   ├── order-service/             # Go service (Signal Subscriber, OKX API)
│   ├── risk-manager/              # Go service (future)
│   └── dashboard/                 # TypeScript (optional, future)
├── go-packages/                   # Shared Go packages
│   ├── logger/                    # Unified logger system
│   ├── websocket/                 # Generic WebSocket client
│   └── signals/                   # Signal types and serialization ⭐
├── config/
│   ├── development.yaml
│   ├── production.yaml
│   └── testing.yaml
├── scripts/
│   ├── setup.sh
│   └── deploy.sh
├── docker-compose.yml
├── Makefile
└── CLAUDE.md                      # This file
```

---

## Questions & Decisions

### Q: Do we need a separate Oracle Service?
**A: Yes - The Market Data Service IS the Oracle**
- Single source of truth for prices
- Decouples data ingestion from trading logic
- Makes system more maintainable and testable

### Q: WebSocket vs HTTP Polling?
**A: WebSocket**
- More efficient (push vs pull)
- Lower latency
- Reduces API call count

### Q: Monorepo vs Polyrepo?
**A: Monorepo**
- Easier to share code
- Atomic changes across services
- Simplified dependency management

### Q: Should Strategy push signals or Order pull advice?
**A: Hybrid Model - Strategy pushes signals, Order validates** ⭐ **Key Design Decision**
- ✅ Strategy focuses on "should we trade?" (trading logic)
- ✅ Order focuses on "can we trade?" (risk control + execution)
- ✅ Clear separation of concerns: Strategy = signal generator, Order = validator + executor
- ✅ Better decoupling: Strategy doesn't need to know Order Service exists
- ✅ Scalable: Multiple services can subscribe to same signals
- ✅ Event-driven: Follows financial system patterns (Signal → Execution)

### Q: Why not Pull Model (Order → Strategy)?
**A: Less scalable and more coupled**
- Requires tight coupling between Order and Strategy (gRPC)
- Order Service needs to actively poll or trigger consultations
- Less flexible for multiple strategy subscribers
- More complex to implement multiple strategies

### Q: Who decides when an order is filled?
**A: Order Service** ⭐
- Order Service is the ONLY service with OKX API Key
- Order Service subscribes to OKX Private WebSocket (orders channel)
- Order Service monitors fill events and updates position state
- After fill, Strategy Service will generate new signals based on new price levels

### Q: What about Message Queue?
**A: Redis Pub/Sub is sufficient**
- Redis Pub/Sub perfect for Market Data → Strategy (price broadcasts)
- Redis Pub/Sub perfect for Strategy → Order (signal broadcasts)
- Lightweight, fast, and built-in to Redis
- Future: Consider message queue (NATS/Kafka) for analytics, audit logs, notifications

---

## Redis Pub/Sub Implementation Guide ⭐ **HYBRID MODEL**

### 1. Shared Signal Types

Create `go-packages/signals/types.go`:

```go
package signals

import "time"

// TradingSignal represents a trading signal from Strategy Service
type TradingSignal struct {
    InstID    string    `json:"inst_id"`     // Trading pair (e.g., "BTC-USDT-SWAP")
    Action    Action    `json:"action"`      // BUY/SELL/CLOSE_LONG/CLOSE_SHORT
    Price     float64   `json:"price"`       // Trigger price
    Reason    string    `json:"reason"`      // Explanation (e.g., "Grid level #5 triggered")
    GridLevel int       `json:"grid_level"`  // Which grid level triggered (optional)
    Timestamp time.Time `json:"timestamp"`   // Signal generation time
}

type Action string

const (
    ActionHold       Action = "HOLD"
    ActionBuy        Action = "BUY"
    ActionSell       Action = "SELL"
    ActionCloseLong  Action = "CLOSE_LONG"
    ActionCloseShort Action = "CLOSE_SHORT"
)

// Redis channel patterns
const (
    SignalChannelPattern = "strategy.signals.%s" // %s = instId
    PriceChannelPattern  = "market.ticker.%s"    // %s = instId
)
```

### 2. Strategy Service - Signal Publisher

```go
// apps/trading-strategy-server/internal/publisher/signal_publisher.go
type SignalPublisher struct {
    redis  *redis.Client
    logger logger.Logger
}

func (p *SignalPublisher) PublishSignal(ctx context.Context, signal signals.TradingSignal) error {
    // Serialize signal to JSON
    data, err := json.Marshal(signal)
    if err != nil {
        return fmt.Errorf("failed to marshal signal: %w", err)
    }

    // Publish to Redis channel
    channel := fmt.Sprintf(signals.SignalChannelPattern, signal.InstID)
    if err := p.redis.Publish(ctx, channel, data).Err(); err != nil {
        return fmt.Errorf("failed to publish signal: %w", err)
    }

    p.logger.Info("Signal published",
        "instId", signal.InstID,
        "action", signal.Action,
        "price", signal.Price,
        "reason", signal.Reason,
    )

    return nil
}
```

### 3. Strategy Service - Price Monitoring

```go
// apps/trading-strategy-server/internal/service/strategy_service.go
func (s *StrategyService) MonitorPrices(ctx context.Context, instID string) {
    // Subscribe to price updates
    channel := fmt.Sprintf(signals.PriceChannelPattern, instID)
    pubsub := s.redis.Subscribe(ctx, channel)
    defer pubsub.Close()

    for msg := range pubsub.Channel() {
        var price float64
        if err := json.Unmarshal([]byte(msg.Payload), &price); err != nil {
            s.logger.Error("Failed to parse price", "error", err)
            continue
        }

        // Calculate if signal should be generated
        if signal := s.gridCalculator.CheckTrigger(instID, price); signal != nil {
            s.publisher.PublishSignal(ctx, *signal)
        }
    }
}
```

### 4. Order Service - Signal Subscriber

```go
// apps/order-service/internal/subscriber/signal_subscriber.go
type SignalSubscriber struct {
    redis         *redis.Client
    orderExecutor *executor.OrderExecutor
    logger        logger.Logger
}

func (s *SignalSubscriber) Subscribe(ctx context.Context, instID string) {
    channel := fmt.Sprintf(signals.SignalChannelPattern, instID)
    pubsub := s.redis.Subscribe(ctx, channel)
    defer pubsub.Close()

    s.logger.Info("Subscribed to signals", "channel", channel)

    for msg := range pubsub.Channel() {
        var signal signals.TradingSignal
        if err := json.Unmarshal([]byte(msg.Payload), &signal); err != nil {
            s.logger.Error("Failed to parse signal", "error", err)
            continue
        }

        s.handleSignal(ctx, signal)
    }
}

func (s *SignalSubscriber) handleSignal(ctx context.Context, signal signals.TradingSignal) {
    // 1. Get current position
    position, err := s.orderExecutor.GetPosition(signal.InstID)
    if err != nil {
        s.logger.Error("Failed to get position", "error", err)
        return
    }

    // 2. Validate signal
    if !s.canExecute(signal, position) {
        s.logger.Info("Signal validation failed",
            "instId", signal.InstID,
            "action", signal.Action,
            "reason", "insufficient balance or position limit",
        )
        return
    }

    // 3. Calculate quantity
    quantity := s.calculateQuantity(signal, position)

    // 4. Execute order
    if err := s.orderExecutor.PlaceOrder(ctx, signal, quantity); err != nil {
        s.logger.Error("Failed to execute order", "error", err)
        return
    }

    s.logger.Info("Order placed",
        "instId", signal.InstID,
        "action", signal.Action,
        "price", signal.Price,
        "quantity", quantity,
    )
}
```

### 5. Order Service - Validation Logic

```go
func (s *SignalSubscriber) canExecute(signal signals.TradingSignal, position Position) bool {
    // Check signal freshness (not stale)
    if time.Since(signal.Timestamp) > 5*time.Second {
        return false
    }

    // Check balance for buy orders
    if signal.Action == signals.ActionBuy || signal.Action == signals.ActionSell {
        if position.AvailableBalance < signal.Price*0.01 { // Min order size
            return false
        }
    }

    // Check position size for sell/close orders
    if signal.Action == signals.ActionSell || signal.Action == signals.ActionCloseLong {
        if position.Size <= 0 {
            return false
        }
    }

    // Check risk limits
    if !s.riskManager.CheckLimits(position) {
        return false
    }

    return true
}

func (s *SignalSubscriber) calculateQuantity(signal signals.TradingSignal, position Position) float64 {
    // Grid-based quantity calculation
    baseQuantity := 0.01 // Base order size

    // Apply position sizing based on balance
    maxQuantity := position.AvailableBalance * 0.1 / signal.Price

    if baseQuantity > maxQuantity {
        return maxQuantity
    }

    return baseQuantity
}
```

---

## Implementation Status

### ✅ Completed
- Market Data Service (WebSocket → Redis Pub/Sub)
- Trading Strategy Service - Domain Layer (DDD)
- Trading Strategy Service - Application Layer (DDD)

### 🔄 In Progress - Architecture Redesign ⭐
- **Hybrid Model**: Strategy publishes signals, Order validates and executes
- Replacing gRPC with Redis Pub/Sub for Strategy → Order communication
- Refactoring Strategy Service to be signal generator
- Refactoring Order Service to be signal subscriber + validator

### 📋 Next Steps
1. **Create shared signal types** (`go-packages/signals/`)
   - Define `TradingSignal` struct
   - Define action types (BUY/SELL/CLOSE_LONG/CLOSE_SHORT)
   - Define Redis channel patterns

2. **Implement Strategy Service as Signal Publisher**
   - Subscribe to Market Data price updates
   - Calculate grid logic and generate signals
   - Publish signals to Redis Pub/Sub
   - Remove any Order Service dependencies

3. **Implement Order Service as Signal Subscriber**
   - Subscribe to Strategy signals via Redis Pub/Sub
   - Implement signal validation logic
   - Calculate order quantities based on position state
   - Execute orders via OKX API

4. **Implement Order Service position management**
   - Track current positions from OKX
   - Calculate cost basis and P&L
   - Manage order lifecycle
   - Maintain position state in memory + database

5. **Test end-to-end flow**
   - Market Data → Strategy (price updates)
   - Strategy → Redis (signal publishing)
   - Redis → Order Service (signal subscription)
   - Order Service → OKX (order execution)

6. **Implement Order fill monitoring**
   - OKX Private WebSocket subscription
   - Update position state on fills
   - Persist trade history to database

---

*Document created: 2025-10-14*
*Last updated: 2025-10-16* ⭐ **Major architecture revision: Hybrid Model - Strategy publishes signals, Order validates and executes**
