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
│ Service │  │ Service  │  │         │  │ Service  │
└─────────┘  └──────────┘  └─────────┘  └──────────┘
     │             │             │              │
     │             │ gRPC        │              │
     │ Redis       └────────────►│              │
     │ Pub/Sub                   │              │
     │                           │              │
     └───────────────────────────┴──────────────┘
                   │
              ┌────┴────┐
              │         │
         ┌────▼───┐ ┌──▼─────┐
         │ Redis  │ │Postgres│
         │(Cache) │ │  (DB)  │
         └────────┘ └────────┘
```

### Communication Patterns

- **Market Data → Strategy**: Redis Pub/Sub (broadcast, one-to-many)
- **Strategy → Order**: gRPC (request-response, needs confirmation) ⭐
- **Order → OKX**: REST API + Private WebSocket (external API)
- **All Services → Redis/DB**: Direct connection

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

### 2. Trading Strategy Service (Go) - DDD Architecture

**Purpose**: Calculate trading strategies and generate signals

**Responsibilities**:
- Subscribe to Market Data Service price updates (via Redis Pub/Sub)
- Calculate grid strategy (grid lines, trigger conditions)
- Generate buy/sell signals (Signal value objects)
- Manage grid state and positions (GridAggregate)
- **Send signals to Order Service via gRPC** ⭐

**Architecture**: Domain-Driven Design (DDD)
- **Domain Layer**: Pure business logic (GridAggregate, Price, Signal, Calculator)
- **Application Layer**: Use case orchestration (StrategyService)
- **Infrastructure Layer**: Technical implementations (Redis subscriber, gRPC client)

**Grid Strategy Parameters**:
- Upper price bound
- Lower price bound
- Number of grid levels
- Grid spacing (arithmetic/geometric)
- Position sizing per grid

**Communication**:
- Input: Price updates from Market Data Service (via Redis Pub/Sub)
- Output: Trading signals to Order Service (via gRPC) ⭐

**Why gRPC for Strategy → Order**:
- ✅ Synchronous response (know if order was accepted/rejected)
- ✅ Error handling (balance insufficient, invalid price, etc.)
- ✅ Type safety (Protocol Buffers)
- ✅ Retry mechanism support

---

### 3. Order Service (Go)

**Purpose**: Execute orders and manage order lifecycle ⭐ **The ONLY service with OKX API Key**

**Responsibilities**:
- **Receive trading signals via gRPC** (from Trading Strategy Service)
- **Execute orders via OKX REST API** (place buy/sell orders)
- **Subscribe to OKX Private WebSocket** (monitor order fills)
- **Decide when order is filled** and trigger exit orders ⭐
- **Set take-profit and stop-loss orders** after entry fills
- Track order status (pending, filled, cancelled)
- Handle order failures and retries
- Maintain order history in PostgreSQL

**gRPC Service Definition**:
```protobuf
service OrderService {
  rpc SubmitSignal(SignalRequest) returns (SignalResponse);
  rpc GetOrderStatus(OrderStatusRequest) returns (OrderStatusResponse);
  rpc CancelOrder(CancelOrderRequest) returns (CancelOrderResponse);
}
```

**OKX API Integration**:
```
# REST API (Private - Requires API Key)
POST /api/v5/trade/order           # Place order
GET  /api/v5/trade/order           # Get order details
GET  /api/v5/trade/orders-pending  # Get pending orders
POST /api/v5/trade/cancel-order    # Cancel order

# WebSocket (Private - Requires Signature)
wss://ws.okx.com:8443/ws/v5/private
Subscribe: {"op":"subscribe","args":[{"channel":"orders","instType":"SPOT"}]}
```

**Order Lifecycle** ⭐:
1. **Receive Signal** (via gRPC from Strategy Service)
2. **Place Order** (via OKX REST API)
3. **Monitor Fill** (via OKX Private WebSocket)
4. **On Fill**: Set exit orders (take-profit + stop-loss)
5. **Update Database** (order state, position)

**Features**:
- Order retry logic with exponential backoff
- Order validation before submission
- Real-time order status updates via Private WebSocket
- Automatic exit order placement on fill

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
Redis Pub/Sub (market:candle:1m:ETH-USDT)
  ↓ Subscribe
Trading Strategy Service
  ↓ Calculate Grid Strategy
Generate Signal {Action: BUY, Price: 2500, Quantity: 0.01}
```

### 2. Order Execution Flow (gRPC - Request/Response) ⭐
```
Trading Strategy Service
  ↓ gRPC Call: SubmitSignal(signal)
Order Service (gRPC Server)
  ↓ Validate & Place Order
OKX REST API (POST /api/v5/trade/order)
  ↓ Return Order ID
Order Service
  ↓ Response: {success: true, orderId: "123456"}
Trading Strategy Service ← Receives confirmation
  ↓ Continue or handle error
PostgreSQL (orders table)
```

### 3. Order Fill Monitoring Flow (Private WebSocket) ⭐
```
Order Service
  ↓ Subscribe (with API Key signature)
OKX Private WebSocket (orders channel)
  ↓ Push fill event
Order Update: {orderId: "123456", state: "filled", avgPx: "2500.5"}
  ↓ Handle fill event
Order Service - onOrderFilled()
  ↓ 1. Update database
  ↓ 2. Update position
  ↓ 3. Set exit orders (take-profit + stop-loss)
OKX REST API (Place exit orders)
```

### 4. Risk Check Flow (Optional Future Feature)
```
Order Service (Before Execute)
  → Risk Manager Service
  → Approve/Reject
  → Continue/Abort Order
```

### Communication Pattern Summary

| Flow | Technology | Pattern | Reason |
|------|-----------|---------|--------|
| Market Data → Strategy | Redis Pub/Sub | Broadcast (1-to-many) | Multiple strategies can subscribe |
| **Strategy → Order** | **gRPC** ⭐ | Request-Response | Need confirmation & error handling |
| Order → OKX | REST + WebSocket | External API | OKX's API design |
| Order → Database | Direct | Internal | Same service |

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
go-redis/redis            # Redis client
lib/pq                    # PostgreSQL driver
google.golang.org/grpc    # gRPC framework ⭐
google.golang.org/protobuf # Protocol Buffers ⭐
gin-gonic/gin             # HTTP framework (optional REST API)
spf13/viper               # Configuration management
uber-go/zap               # Structured logging
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
│   ├── market-data-server/        # Go service (WebSocket → Redis)
│   ├── trading-strategy-server/   # Go service (DDD, gRPC client)
│   ├── order-service/             # Go service (gRPC server, OKX API)
│   ├── risk-manager/              # Go service (future)
│   └── dashboard/                 # TypeScript (optional, future)
├── go-packages/                   # Shared Go packages
│   ├── logger/                    # Unified logger system
│   └── websocket/                 # Generic WebSocket client
├── shared/
│   └── proto/                     # Protocol Buffers ⭐
│       └── order/
│           └── order.proto        # Order service gRPC definition
├── config/
│   ├── development.yaml
│   ├── production.yaml
│   └── testing.yaml
├── scripts/
│   ├── setup.sh
│   ├── deploy.sh
│   └── generate-proto.sh          # Generate Go code from .proto
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

### Q: Redis Pub/Sub vs gRPC for Strategy → Order?
**A: gRPC** ⭐ **Key Decision**
- ❌ Redis Pub/Sub: Fire-and-forget, no confirmation, no error handling
- ✅ gRPC: Request-response, immediate feedback, type-safe, retry support
- Use Redis Pub/Sub for broadcast (Market Data → Strategy)
- Use gRPC for request-response (Strategy → Order)

### Q: Who decides when an order is filled?
**A: Order Service** ⭐
- Order Service is the ONLY service with OKX API Key
- Order Service subscribes to OKX Private WebSocket (orders channel)
- Order Service monitors fill events and triggers exit orders
- Strategy Service doesn't need to know if order filled (decoupled)

### Q: Message Queue needed?
**A: Not needed for Strategy → Order**
- gRPC provides better guarantees for this use case
- Redis Pub/Sub sufficient for Market Data → Strategy (broadcast)
- Future: Consider message queue for async tasks (notifications, analytics)

---

## gRPC Setup Guide

### 1. Define Protocol Buffers

Create `shared/proto/order/order.proto`:

```protobuf
syntax = "proto3";

package order;
option go_package = "dizzycoder.xyz/trading-system/shared/proto/order";

import "google/protobuf/timestamp.proto";

service OrderService {
  rpc SubmitSignal(SignalRequest) returns (SignalResponse);
  rpc GetOrderStatus(OrderStatusRequest) returns (OrderStatusResponse);
  rpc CancelOrder(CancelOrderRequest) returns (CancelOrderResponse);
}

message SignalRequest {
  string inst_id = 1;
  string action = 2;            // BUY or SELL
  double price = 3;
  double quantity = 4;
  string reason = 5;
  google.protobuf.Timestamp timestamp = 6;
}

message SignalResponse {
  bool success = 1;
  string order_id = 2;
  string message = 3;
  OrderStatus status = 4;
}

enum OrderStatus {
  PENDING = 0;
  ACCEPTED = 1;
  REJECTED = 2;
  FILLED = 3;
  CANCELLED = 4;
}
```

### 2. Generate Go Code

```bash
# Install protoc compiler and Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       shared/proto/order/order.proto
```

### 3. Order Service - gRPC Server

```go
// apps/order-service/internal/grpc/server.go
type OrderServer struct {
    pb.UnimplementedOrderServiceServer
    orderManager *order.Manager
    logger       logger.Logger
}

func (s *OrderServer) SubmitSignal(ctx context.Context, req *pb.SignalRequest) (*pb.SignalResponse, error) {
    // Validate, place order, return response
}
```

### 4. Strategy Service - gRPC Client

```go
// apps/trading-strategy-server/internal/infrastructure/grpc/order_client.go
type OrderClient struct {
    client pb.OrderServiceClient
    conn   *grpc.ClientConn
}

// Implements application.SignalPublisher interface
func (c *OrderClient) Publish(ctx context.Context, signal strategy.Signal) error {
    req := &pb.SignalRequest{
        InstId: signal.InstID(),
        Action: string(signal.Action()),
        // ...
    }
    resp, err := c.client.SubmitSignal(ctx, req)
    // Handle response
}
```

---

## Implementation Status

### ✅ Completed
- Market Data Service (WebSocket → Redis Pub/Sub)
- Trading Strategy Service - Domain Layer (DDD)
- Trading Strategy Service - Application Layer (DDD)

### 🔄 In Progress
- Trading Strategy Service - Infrastructure Layer (Redis subscriber, gRPC client)

### 📋 Next Steps
1. Define gRPC Protocol Buffers (`shared/proto/order/order.proto`)
2. Generate Go code from .proto files
3. Implement Order Service (gRPC server, OKX API integration)
4. Complete Strategy Service infrastructure (gRPC client)
5. Test end-to-end flow (Market Data → Strategy → Order)
6. Implement Order fill monitoring (OKX Private WebSocket)

---

*Document created: 2025-10-14*
*Last updated: 2025-10-14*
