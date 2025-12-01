# Chart Dashboard

回測結果視覺化工具，用於顯示 K 線圖、交易標記與平均成本線。

![Chart Dashboard 截圖](./screenshot/screenshot.png)

## 啟動

```bash
cd apps/chart-dashboard
pnpm install  # 首次需要安裝依賴
pnpm dev      # 啟動開發服務器 (http://localhost:5173)
```

## 使用方式

1. 準備數據文件：
   - `history.json`：OKX 歷史 K 線數據
   - `trades.csv`：回測產生的交易記錄
2. 啟動 chart-dashboard
3. 在「數據導入」區塊：
   - 點擊「K 線數據」的 Choose File 載入 `history.json`
   - 點擊「交易記錄」的 Choose File 載入 `trades.csv`
4. 查看 K 線圖、交易標記與平均成本線

**圖表說明**：

- **橘色虛線**：平均成本線變化
- **滑鼠懸停**：顯示交易細節（OHLC、平倉筆數、開倉比數、持倉量）

## 功能

- **K 線圖表**：顯示歷史價格走勢
- **開倉/平倉標記**：在圖表上標示交易點位
- **平均成本線**：顯示持倉的平均成本變化
- **資金曲線**：追蹤資金變化

## 技術棧

- **框架**：React 19 + TypeScript
- **建構工具**：Vite
- **圖表庫**：[Lightweight Charts](https://tradingview.github.io/lightweight-charts/)
- **CSV 解析**：PapaParse

## 未來開發

### 效能優化
- [ ] **虛擬滾動 (Virtual Scrolling)**：只渲染可視區域內的數據，改善大量數據時的滑動順暢度

### UX 優化
- [ ] **時間點跳轉**：輸入日期時間，快速移動到指定時間點

### 功能擴展
- [ ] **最大回撤 (Max Drawdown)**：顯示資金曲線的最大回撤百分比與時間區間
- [ ] **勝率統計**：顯示交易勝率、平均盈虧比
- [ ] **交易統計面板**：總交易次數、平均持倉時間、最大連續虧損次數
- [ ] **時間區間篩選**：可選擇特定日期範圍查看回測結果
- [ ] **多策略對比**：同時載入多組回測結果進行比較

## 開發

```bash
pnpm dev      # 開發模式
pnpm build    # 建構生產版本
pnpm preview  # 預覽生產版本
pnpm lint     # 執行 ESLint
```
