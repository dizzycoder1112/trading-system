# Backtesting Engine

回測引擎 - 用於測試交易策略的歷史表現

## 功能

- 載入歷史 K 線數據
- 模擬開倉和平倉
- 計算交易指標（收益率、最大回撤、勝率等）
- 支持參數優化

## 使用方式

```bash
# 運行回測
go run cmd/main.go --data=data/btc-5m.json

# 指定參數
go run cmd/main.go \
  --data=data/btc-5m.json \
  --initial-balance=10000 \
  --take-profit=0.0015 \
  --open-discount=0.001
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
