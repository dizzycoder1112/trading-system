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
│ Market  │  │  Grid    │  │ Order   │  │ Risk     │
│ Data    │  │ Engine   │  │ Manager │  │ Manager  │
│ Service │  │ Service  │  │ Service │  │ Service  │
└─────────┘  └──────────┘  └─────────┘  └──────────┘
     │             │             │              │
     └─────────────┴─────────────┴──────────────┘
                   │
              ┌────┴────┐
              │         │
         ┌────▼───┐ ┌──▼─────┐
         │ Redis  │ │Postgres│
         │(Cache) │ │  (DB)  │
         └────────┘ └────────┘
```

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

### 2. Grid Engine Service (Go)

**Responsibilities**:
- Subscribe to Market Data Service price updates
- Calculate grid strategy (grid lines, trigger conditions)
- Generate buy/sell signals
- Manage grid state and positions
- Implement grid algorithm logic

**Grid Strategy Parameters**:
- Upper price bound
- Lower price bound
- Number of grid levels
- Grid spacing (arithmetic/geometric)
- Position sizing per grid

**Communication**:
- Input: Price updates from Market Data Service (via Redis/MQ)
- Output: Trading signals to Order Manager Service

---

### 3. Order Manager Service (Go)

**Responsibilities**:
- Receive trading signals from Grid Engine
- Execute orders via OKX REST API
- Track order status (pending, filled, cancelled)
- Handle order failures and retries
- Maintain order history

**OKX API Integration**:
```
POST /api/v5/trade/order           # Place order
GET  /api/v5/trade/order           # Get order details
GET  /api/v5/trade/orders-pending  # Get pending orders
POST /api/v5/trade/cancel-order    # Cancel order
```

**Features**:
- Order retry logic with exponential backoff
- Order validation before submission
- Real-time order status updates

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

### Price Update Flow
```
OKX WebSocket
  → Market Data Service
  → Redis Pub/Sub
  → Grid Engine Service
  → Trading Signal
```

### Order Execution Flow
```
Grid Engine (Signal)
  → Order Manager Service
  → OKX REST API
  → Order Status Update
  → PostgreSQL (Order Record)
```

### Risk Check Flow
```
Order Manager (Before Execute)
  → Risk Manager Service
  → Approve/Reject
  → Continue/Abort Order
```

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
gorilla/websocket    # WebSocket client
go-redis/redis       # Redis client
lib/pq              # PostgreSQL driver
gin-gonic/gin       # HTTP framework
spf13/viper         # Configuration management
uber-go/zap         # Structured logging
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
├── services/
│   ├── market-data/          # Go service
│   ├── grid-engine/          # Go service
│   ├── order-manager/        # Go service
│   ├── risk-manager/         # Go service
│   └── dashboard/            # TypeScript (optional)
├── shared/
│   ├── proto/                # Protocol buffers (if using gRPC)
│   ├── types/                # Shared type definitions
│   └── utils/                # Common utilities
├── config/
│   ├── development.yaml
│   ├── production.yaml
│   └── testing.yaml
├── scripts/
│   ├── setup.sh
│   └── deploy.sh
├── docker-compose.yml
├── Makefile
└── CLAUDE.md                 # This file
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

### Q: Message Queue needed?
**A: Optional initially, recommended for production**
- Start with Redis Pub/Sub
- Migrate to NATS/Kafka for better reliability

---

## Next Steps

1. Pull your monorepo template
2. Set up the Market Data Service first
3. Implement OKX WebSocket connection
4. Test price data flow through Redis
5. Move to Grid Engine implementation

---

*Document created: 2025-10-14*
*Last updated: 2025-10-14*
