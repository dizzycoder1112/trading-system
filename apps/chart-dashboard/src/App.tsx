import { useState } from 'react';
import { FileUploader } from './components/FileUploader';
import { CandlestickChart } from './components/CandlestickChart';
import type { CandleData, TradeData } from './types';
import { calculateCostBasisLine } from './utils/parseCSV';

function App() {
  const [candles, setCandles] = useState<CandleData[]>([]);
  const [trades, setTrades] = useState<TradeData[]>([]);

  const handleCandlesLoaded = (loadedCandles: CandleData[]) => {
    setCandles(loadedCandles);
  };

  const handleTradesLoaded = (loadedTrades: TradeData[]) => {
    setTrades(loadedTrades);
  };

  // 將交易記錄轉換為圖表標記（暫時停用）
  const markers: never[] = []; // 暫時不顯示標記

  // 計算平均成本線
  const costBasisLine = trades.length > 0 ? calculateCostBasisLine(trades) : [];

  // 調試信息：檢查平均成本線數據
  if (costBasisLine.length > 0) {
    console.log('平均成本線數據點數量:', costBasisLine.length);
    console.log('前5個數據點:', costBasisLine.slice(0, 5));
    console.log('後5個數據點:', costBasisLine.slice(-5));
  }

  // 統計信息
  const stats = {
    totalCandles: candles.length,
    totalTrades: trades.length,
    openTrades: trades.filter((t) => t.action === 'OPEN').length,
    closeTrades: trades.filter((t) => t.action === 'CLOSE').length,
  };

  return (
    <div style={styles.app}>
      <header style={styles.header}>
        <h1 style={styles.title}>📈 交易回測可視化工具</h1>
        <p style={styles.subtitle}>導入歷史數據和交易記錄，分析回測結果</p>
      </header>

      <main style={styles.main}>
        {/* 文件上傳區域 */}
        <FileUploader onCandlesLoaded={handleCandlesLoaded} onTradesLoaded={handleTradesLoaded} />

        {/* 統計面板 */}
        {(candles.length > 0 || trades.length > 0) && (
          <div style={styles.statsPanel}>
            <div style={styles.statItem}>
              <span style={styles.statLabel}>K 線數量:</span>
              <span style={styles.statValue}>{stats.totalCandles}</span>
            </div>
            <div style={styles.statItem}>
              <span style={styles.statLabel}>交易次數:</span>
              <span style={styles.statValue}>{stats.totalTrades}</span>
            </div>
            <div style={styles.statItem}>
              <span style={styles.statLabel}>開倉:</span>
              <span style={styles.statValue}>{stats.openTrades}</span>
            </div>
            <div style={styles.statItem}>
              <span style={styles.statLabel}>平倉:</span>
              <span style={styles.statValue}>{stats.closeTrades}</span>
            </div>
          </div>
        )}

        {/* K 線圖表 */}
        <div style={styles.chartContainer}>
          <CandlestickChart
            data={candles}
            trades={trades}
            markers={markers}
            costBasisLine={costBasisLine}
          />
        </div>

        {/* 提示信息 */}
        {candles.length === 0 && (
          <div style={styles.instructions}>
            <h3>📝 使用說明</h3>
            <ol>
              <li>
                導入 <strong>histories.json</strong> 文件（K 線數據）
                <br />
                <small>位置: apps/trading-strategy-server/data/.../histories.json</small>
              </li>
              <li>
                （可選）導入 <strong>trades.csv</strong> 文件（交易記錄）
                <br />
                <small>位置: apps/trading-strategy-server/data/.../backtest_trades_pos300/trades.csv</small>
              </li>
              <li>查看 K 線圖表和交易標記</li>
            </ol>
          </div>
        )}
      </main>

      <footer style={styles.footer}>
        <p>Trading System Dashboard v1.0</p>
      </footer>
    </div>
  );
}

const styles = {
  app: {
    minHeight: '100vh',
    backgroundColor: '#121212',
    color: '#ffffff',
  },
  header: {
    padding: '20px',
    backgroundColor: '#1e1e1e',
    borderBottom: '1px solid #2b2b43',
  },
  title: {
    margin: 0,
    fontSize: '32px',
    fontWeight: 'bold' as const,
  },
  subtitle: {
    margin: '8px 0 0 0',
    fontSize: '16px',
    color: '#888',
  },
  main: {
    padding: '20px',
    maxWidth: '1400px',
    margin: '0 auto',
  },
  statsPanel: {
    display: 'flex',
    gap: '20px',
    padding: '15px',
    backgroundColor: '#1e1e1e',
    borderRadius: '8px',
    marginBottom: '20px',
  },
  statItem: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '5px',
  },
  statLabel: {
    fontSize: '12px',
    color: '#888',
  },
  statValue: {
    fontSize: '24px',
    fontWeight: 'bold' as const,
    color: '#26a69a',
  },
  chartContainer: {
    backgroundColor: '#1e1e1e',
    borderRadius: '8px',
    padding: '20px',
    marginBottom: '20px',
  },
  instructions: {
    padding: '30px',
    backgroundColor: '#1e1e1e',
    borderRadius: '8px',
    maxWidth: '800px',
    margin: '0 auto',
  },
  footer: {
    padding: '20px',
    textAlign: 'center' as const,
    borderTop: '1px solid #2b2b43',
    color: '#888',
    fontSize: '14px',
  },
};

export default App;
