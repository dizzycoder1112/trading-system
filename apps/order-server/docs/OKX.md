關鍵文件
order API doc: https://www.okx.com/docs-v5/en/#order-book-trading-trade-post-place-order
order fills websocket: https://www.okx.com/docs-v5/en/#order-book-trading-trade-ws-fills-channel




掛單 request:
url: POST /api/v5/trade/order

### Request Parameters

| 參數 | 類型 | 必填 | 說明 |
|------|------|------|------|
| `instId` | String | Yes | Instrument ID, e.g. BTC-USDT |
| `tdMode` | String | Yes | Trade mode: `cross` / `isolated` / `cash` / `spot_isolated` |
| `ccy` | String | No | Margin currency |
| `clOrdId` | String | No | Client Order ID (up to 32 chars) |
| `tag` | String | No | Order tag (up to 16 chars) |
| `side` | String | Yes | Order side: `buy` / `sell` |
| `posSide` | String | Conditional | Position side: `long` / `short` / `net` (FUTURES/SWAP only) |
| `ordType` | String | Yes | Order type: `market` / `limit` / `post_only` / `fok` / `ioc` / `optimal_limit_ioc` / `mmp` / `mmp_and_post_only` / `elp` |
| `sz` | String | Yes | Quantity to buy or sell |
| `px` | String | Conditional | Order price (required for limit orders) |
| `pxUsd` | String | Conditional | Place options orders in USD |
| `pxVol` | String | Conditional | Place options orders based on implied volatility |
| `reduceOnly` | Boolean | No | Whether orders can only reduce position size (default: false) |
| `tgtCcy` | String | No | Target currency: `base_ccy` / `quote_ccy` (SPOT Market Orders) |
| `banAmend` | Boolean | No | Disallow system amending size (SPOT Market Orders) |
| `pxAmendType` | String | No | Price amendment type: `0` (reject) / `1` (amend to best price) |
| `tradeQuoteCcy` | String | No | Quote currency for trading (SPOT only) |
| `stpMode` | String | No | Self trade prevention: `cancel_maker` / `cancel_taker` / `cancel_both` |
| `attachAlgoOrds` | Array | No | TP/SL information attached when placing order |

### attachAlgoOrds 參數

| 參數 | 類型 | 必填 | 說明 |
|------|------|------|------|
| `attachAlgoClOrdId` | String | No | Client-supplied Algo ID (up to 32 chars) |
| `tpTriggerPx` | String | Conditional | Take-profit trigger price |
| `tpTriggerRatio` | String | Conditional | TP trigger ratio (0.3 = 30%), FUTURES/SWAP only |
| `tpOrdPx` | String | Conditional | Take-profit order price (-1 = market) |
| `tpOrdKind` | String | No | TP order kind: `condition` / `limit` (default: condition) |
| `slTriggerPx` | String | Conditional | Stop-loss trigger price |
| `slTriggerRatio` | String | Conditional | SL trigger ratio (0.3 = 30%), FUTURES/SWAP only |
| `slOrdPx` | String | Conditional | Stop-loss order price (-1 = market) |
| `tpTriggerPxType` | String | No | TP trigger price type: `last` / `index` / `mark` (default: last) |
| `slTriggerPxType` | String | No | SL trigger price type: `last` / `index` / `mark` (default: last) |
| `sz` | String | Conditional | Size (for split TPs only) |
| `amendPxOnTriggerType` | String | No | Enable Cost-price SL: `0` (disable) / `1` (enable) |

---

attachAlgoOrds - 附帶止盈止損

  下主單時，可以一起設定 TP/SL，成交後自動生效。

  參數解釋

  | 參數             | 說明                                |
  |----------------|-----------------------------------|
  | tpTriggerPx    | 止盈觸發價 — 價格到這裡觸發止盈                 |
  | tpOrdPx        | 止盈委託價 — 觸發後掛單的價格（-1 = 市價）         |
  | tpTriggerRatio | 止盈觸發比例 — 用百分比代替絕對價格（0.3 = 30%）    |
  | tpOrdKind      | 止盈類型 — condition（條件單）或 limit（限價單） |

  ---
  兩種止盈模式

  1. Condition TP（條件止盈）

  開倉價: 2500
  tpTriggerPx: 2510  ← 觸發價
  tpOrdPx: 2509      ← 觸發後掛這個價

  流程：價格漲到 2510 → 觸發 → 掛 2509 限價賣單

  2. Limit TP（限價止盈）

  開倉價: 2500
  tpOrdPx: 2510      ← 直接掛限價單

  流程：直接掛 2510 賣單等成交（不需要觸發價）

  ---
  對你策略的意義

  {
    "instId": "ETH-USDT-SWAP",
    "side": "buy",
    "ordType": "limit",
    "px": "2500",           // 開倉價
    "sz": "0.08",
    "attachAlgoOrds": [{
      "tpOrdKind": "limit",
      "tpOrdPx": "2504"     // 止盈價 (0.15%)
    }]
  }


掛單 response:
 | 回傳欄位    | 說明                       |
  |---------|--------------------------|
  | ordId   | 交易所分配的訂單 ID ✅            |
  | clOrdId | 你自己定義的 Client Order ID ✅ |
  | tag     | 訂單標籤                     |

  沒有 positionId！

  ---
  關聯買賣單的方式

  OKX 不會自動幫你關聯，你要自己設計：

  方法 1：用 clOrdId 自己關聯

  // 開倉單
  {
    "clOrdId": "open_001",
    "side": "buy",
    "attachAlgoOrds": [{
      "attachAlgoClOrdId": "tp_001",  // ← 關聯的止盈單 ID
      "tpOrdPx": "2504"
    }]
  }

  你的系統記錄：
  open_001 (開倉) ↔ tp_001 (止盈)

  方法 2：用 attachAlgoOrds 自動綁定

  如果用 attachAlgoOrds 下單，TP 單會在主單成交後自動生效，OKX 內部會關聯，你只需要追蹤主單 ordId。

  ---
  對你策略的設計

  下單時：
    clOrdId = "pos_20241204_001"
    attachAlgoClOrdId = "tp_20241204_001"

  你的 DB：
    | open_order_id | tp_order_id | status |
    | pos_20241204_001 | tp_20241204_001 | pending |

  Positions channel 收到平倉通知：
    → 查 DB 更新狀態

  這樣你就能追蹤哪筆開倉對應哪筆平倉了。

---

order fills websocket:
Fills Channel Push Data Schema

  | 參數         | 類型     | 說明                              |
  |------------|--------|---------------------------------|
  | arg        | Object | Successfully subscribed channel |
  | > channel  | String | Channel name                    |
  | > uid      | String | User Identifier                 |
  | > instId   | String | Instrument ID                   |
  | data       | Array  | Subscribed data                 |
  | > instId   | String | Instrument ID                   |
  | > fillSz   | String | 成交數量 ⭐                          |
  | > fillPx   | String | 成交價格 ⭐                          |
  | > side     | String | 方向：buy / sell                   |
  | > ts       | String | 成交時間                            |
  | > ordId    | String | Order ID ⭐                      |
  | > clOrdId  | String | Client Order ID ⭐               |
  | > tradeId  | String | Trade ID                        |
  | > execType | String | T: Taker / M: Maker             |
  | > count    | String | 聚合的成交筆數                         |




  ----

  下單：clOrdId = "open_001"
       ↓
  Fills channel 推送：
    clOrdId: "open_001"   ← 就是這筆！
    fillSz: "0.08"
    fillPx: "2500"
    side: "buy"
       ↓
  你知道 open_001 成交了！

  ---
  完整設計

  | Channel   | 用途                        |
  |-----------|---------------------------|
  | fills     | 訂單成交通知（有 ordId, clOrdId）⭐ |
  | positions | 持倉狀態變化（盈虧、數量）             |


---

## DB Schema Design

### 下單流程
```
POST /api/v5/trade/order
    ↓
Response: ordId, clOrdId (沒有 positionId)
    ↓
Fills Channel 推送成交通知 (ordId, clOrdId, fillSz, fillPx)
```

### 關聯方式
- **clOrdId**: 自己定義，用來追蹤開倉單和止盈單的關係
- **attachAlgoOrds**: 下單時附帶 TP/SL，成交後自動生效

### 關鍵欄位對照

| 來源 | 關鍵欄位 |
|------|----------|
| 下單請求 | instId, clOrdId, side, ordType, sz, px, attachAlgoClOrdId |
| 下單回應 | ordId, clOrdId |
| Fills 推送 | ordId, clOrdId, fillSz, fillPx, tradeId, side |

---

### Schema

```sql
-- 訂單表：記錄所有下單請求和狀態
CREATE TABLE orders (
    id              SERIAL PRIMARY KEY,

    -- OKX 識別碼
    ord_id          VARCHAR(32),        -- 交易所回傳的 Order ID
    cl_ord_id       VARCHAR(32) UNIQUE NOT NULL,  -- 我們定義的 Client Order ID

    -- 訂單內容
    inst_id         VARCHAR(32) NOT NULL,  -- e.g. ETH-USDT-SWAP
    side            VARCHAR(8) NOT NULL,   -- buy / sell
    ord_type        VARCHAR(16) NOT NULL,  -- limit / market / post_only
    sz              DECIMAL(20,8) NOT NULL, -- 數量
    px              DECIMAL(20,8),          -- 價格 (market order 可為 null)

    -- 狀態追蹤
    status          VARCHAR(16) NOT NULL DEFAULT 'pending',
                    -- pending -> submitted -> live -> filled / canceled

    -- 關聯
    parent_cl_ord_id VARCHAR(32),  -- 如果是 TP/SL 單，指向主單的 cl_ord_id

    -- 成交統計 (從 fills 彙總)
    filled_sz       DECIMAL(20,8) DEFAULT 0,
    avg_fill_px     DECIMAL(20,8),

    -- 時間戳
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    submitted_at    TIMESTAMP,    -- 送出到 OKX 的時間
    filled_at       TIMESTAMP,    -- 完全成交的時間
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 成交記錄表：來自 Fills Channel 的推送
CREATE TABLE fills (
    id              SERIAL PRIMARY KEY,

    -- OKX 識別碼
    trade_id        VARCHAR(32) NOT NULL,
    ord_id          VARCHAR(32) NOT NULL,
    cl_ord_id       VARCHAR(32) NOT NULL,

    -- 成交內容
    inst_id         VARCHAR(32) NOT NULL,
    side            VARCHAR(8) NOT NULL,
    fill_sz         DECIMAL(20,8) NOT NULL,  -- 成交數量
    fill_px         DECIMAL(20,8) NOT NULL,  -- 成交價格
    exec_type       VARCHAR(8),              -- T: Taker / M: Maker

    -- 時間
    ts              BIGINT NOT NULL,         -- OKX 的成交時間戳
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(trade_id, ord_id)
);

-- 索引
CREATE INDEX idx_orders_cl_ord_id ON orders(cl_ord_id);
CREATE INDEX idx_orders_parent ON orders(parent_cl_ord_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_fills_cl_ord_id ON fills(cl_ord_id);
CREATE INDEX idx_fills_ord_id ON fills(ord_id);
```

---

### 使用範例

```sql
-- 開倉 + TP 下單
INSERT INTO orders (cl_ord_id, inst_id, side, ord_type, sz, px, status)
VALUES ('open_20241204_001', 'ETH-USDT-SWAP', 'buy', 'limit', 0.08, 2500, 'pending');

INSERT INTO orders (cl_ord_id, inst_id, side, ord_type, sz, px, status, parent_cl_ord_id)
VALUES ('tp_20241204_001', 'ETH-USDT-SWAP', 'sell', 'limit', 0.08, 2504, 'pending', 'open_20241204_001');

-- 收到 OKX 回應後更新
UPDATE orders SET ord_id = '123456', status = 'submitted' WHERE cl_ord_id = 'open_20241204_001';

-- 收到 Fills 推送後
INSERT INTO fills (trade_id, ord_id, cl_ord_id, inst_id, side, fill_sz, fill_px, ts)
VALUES ('789', '123456', 'open_20241204_001', 'ETH-USDT-SWAP', 'buy', 0.08, 2500, 1701676800000);

UPDATE orders SET status = 'filled', filled_sz = 0.08, avg_fill_px = 2500, filled_at = NOW()
WHERE cl_ord_id = 'open_20241204_001';
```

