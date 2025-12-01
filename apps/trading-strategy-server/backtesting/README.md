# Backtesting Engine

回測引擎 - 用於測試交易策略的歷史表現

## 功能

- 載入歷史 K 線數據
- 模擬開倉和平倉
- 計算交易指標（收益率、最大回撤、勝率等）
- 支持參數優化

## 使用方式

```bash
# 從 trading-strategy-server 目錄運行
cd apps/trading-strategy-server

# 基本使用（使用默認參數）
go run ./cmd/backtest/main.go --data=../../data/xxx/history.json

# 自定義參數
go run ./cmd/backtest/main.go \
  --data=../../data/20240930-20241005/ETH-USDT-SWAP/5m/history.json \
  --initial-balance=20000 \
  --position-size=200 \
  --take-profit-min=0.002 \
  --take-profit-max=0.003 \
  --break-even-profit-min=1 \
  --break-even-profit-max=20 \
  --enable-trend-filter=true \
  --enable-red-candle-filter=true \
  --enable-auto-funding=true \
  --auto-funding-amount=5000 \
  --auto-funding-idle=288
```

查看完整參數：`go run ./cmd/backtest/main.go --help`

## 可用參數

| 參數 | 默認值 | 說明 |
|------|--------|------|
| `--data` | (必填) | 歷史數據文件路徑 |
| `--initial-balance` | 10000 | 初始資金 (USDT) |
| `--position-size` | 100 | 單次開倉大小 (USDT) |
| `--fee-rate` | 0.0005 | 手續費率 (0.05%) |
| `--take-profit-min` | 0.0015 | 最小止盈百分比 (0.15%) |
| `--take-profit-max` | 0.01 | 最大止盈百分比 (1%) |
| `--break-even-profit-min` | -0.1 | 打平最小目標盈利 (USDT) |
| `--break-even-profit-max` | 20.0 | 打平最大目標盈利 (USDT) |
| `--enable-trend-filter` | false | 是否啟用趨勢過濾 |
| `--enable-red-candle-filter` | true | 虧損時只在紅K開倉 |
| `--enable-auto-funding` | true | 是否啟用自動注資 |
| `--auto-funding-amount` | 5000 | 自動注資金額 (USDT) |
| `--auto-funding-idle` | 12 | 觸發注資的閒置K線數 |

## 輸出結果

回測完成後會在數據文件同目錄下生成：

```
data/
├── backtest_trades_pos{size}/
│   ├── trades.csv           # 交易日誌（包含每筆交易詳情）
│   ├── report.md            # 回測報告（Markdown格式）
│   └── rounds_detail.csv    # 打平輪次詳細記錄
```

## 歷史數據格式

使用 OKX API 格式的 JSON 數據：

```json
{
  "code": "0",
  "msg": "",
  "data": [
    ["1693497600000", "25000.5", "25100.0", "24900.0", "25050.0", "1000", "..."],
    ["1693498200000", "25050.0", "25150.0", "25000.0", "25100.0", "1200", "..."]
  ]
}
```

## 開發

```bash
# 安裝依賴
go mod download

# 運行測試
go test ./...
```

> 詳細架構請參考 [backtesting/CLAUDE.md](./CLAUDE.md)
