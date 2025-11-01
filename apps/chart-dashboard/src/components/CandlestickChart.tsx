import { useEffect, useRef, useState } from 'react';
import {
  createChart,
  CandlestickSeries,
  LineSeries,
  createSeriesMarkers,
} from 'lightweight-charts';
import type {
  IChartApi,
  ISeriesApi,
  CandlestickSeriesPartialOptions,
  LineSeriesPartialOptions,
  ISeriesMarkersPluginApi,
  Time,
  LineData,
  CandlestickData,
  MouseEventParams,
} from 'lightweight-charts';
import type { CandleData, ChartMarker, TradeData } from '../types';

interface CandlestickChartProps {
  data: CandleData[];
  trades?: TradeData[]; // ⭐ 新增：交易數據
  markers?: ChartMarker[];
  costBasisLine?: LineData[]; // 平均成本線數據
  width?: number;
  height?: number;
}

interface LegendData {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  avgCost?: number;
  openCount?: number; // ⭐ 開倉數量
  closeCount?: number; // ⭐ 關倉數量
  openPositions?: number; // ⭐ 當前持倉數量
}

export function CandlestickChart({
  data,
  trades = [], // ⭐ 接收交易數據
  markers = [],
  costBasisLine = [],
  width,
  height = 600,
}: CandlestickChartProps) {
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const costBasisSeriesRef = useRef<ISeriesApi<'Line'> | null>(null);
  const markersPluginRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);

  // 圖例數據狀態
  const [legendData, setLegendData] = useState<LegendData | null>(null);

  // ⭐ 檢查 trades 數據
  useEffect(() => {
    if (trades.length > 0) {
      console.log('✅ Trades loaded:', trades.length);
    }
  }, [trades]);

  useEffect(() => {
    if (!chartContainerRef.current) return;

    // 計算寬度：如果沒有指定則使用容器寬度
    const chartWidth = width || chartContainerRef.current.clientWidth;

    // 創建圖表
    const chart = createChart(chartContainerRef.current, {
      width: chartWidth,
      height: height,
      layout: {
        background: { color: '#1e1e1e' },
        textColor: '#d1d4dc',
      },
      grid: {
        vertLines: { color: '#2b2b43' },
        horzLines: { color: '#2b2b43' },
      },
      crosshair: {
        mode: 1, // Normal crosshair mode
      },
      rightPriceScale: {
        borderColor: '#2b2b43',
      },
      timeScale: {
        borderColor: '#2b2b43',
        timeVisible: true,
        secondsVisible: false,
      },
    });

    chartRef.current = chart;

    // 添加 K 線系列（v5 API: 使用 addSeries + CandlestickSeries）
    const candlestickOptions: CandlestickSeriesPartialOptions = {
      upColor: '#26a69a',
      downColor: '#ef5350',
      borderVisible: false,
      wickUpColor: '#26a69a',
      wickDownColor: '#ef5350',
    };

    const candlestickSeries = chart.addSeries(CandlestickSeries, candlestickOptions);
    seriesRef.current = candlestickSeries;

    // 添加平均成本線系列
    const costBasisOptions: LineSeriesPartialOptions = {
      color: '#FFA500', // 橙色
      lineWidth: 2,
      lineStyle: 2, // 虛線
      title: '平均成本',
    };

    const costBasisSeries = chart.addSeries(LineSeries, costBasisOptions);
    costBasisSeriesRef.current = costBasisSeries;

    // 響應式調整
    const handleResize = () => {
      if (chartContainerRef.current && chartRef.current) {
        const newWidth = width || chartContainerRef.current.clientWidth;
        chartRef.current.applyOptions({ width: newWidth });
      }
    };

    window.addEventListener('resize', handleResize);

    // 清理函數
    return () => {
      window.removeEventListener('resize', handleResize);
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
      costBasisSeriesRef.current = null;
    };
  }, [width, height]);

  // 更新 K 線數據
  useEffect(() => {
    if (!seriesRef.current || data.length === 0) return;

    try {
      seriesRef.current.setData(data);
      console.log(`✅ 圖表已顯示 ${data.length} 根 K 線`);

      // 自動縮放到合適的視圖
      if (chartRef.current) {
        chartRef.current.timeScale().fitContent();
      }
    } catch (error) {
      console.error('Failed to set candle data:', error);
    }
  }, [data]);

  // 更新平均成本線數據
  useEffect(() => {
    if (!costBasisSeriesRef.current) return;

    try {
      if (costBasisLine.length > 0) {
        costBasisSeriesRef.current.setData(costBasisLine);
        console.log(`✅ 圖表已顯示平均成本線 (${costBasisLine.length} 個數據點)`);
      }
    } catch (error) {
      console.error('Failed to set cost basis line:', error);
    }
  }, [costBasisLine]);

  // ⭐ 監聽 Crosshair 移動事件（獨立 useEffect，依賴 trades）
  useEffect(() => {
    if (!chartRef.current || !seriesRef.current || !costBasisSeriesRef.current) return;

    const chart = chartRef.current;
    const candlestickSeries = seriesRef.current;
    const costBasisSeries = costBasisSeriesRef.current;

    const crosshairMoveHandler = (param: MouseEventParams) => {
      if (!param.time) {
        setLegendData(null);
        return;
      }

      const candleData = param.seriesData.get(candlestickSeries) as CandlestickData | undefined;
      const costBasisData = param.seriesData.get(costBasisSeries) as LineData | undefined;

      if (candleData) {
        const currentTimeStr = new Date((param.time as number) * 1000).toLocaleString('zh-TW', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
        });

        const currentTimestamp = param.time as number;
        const tradesAtThisTime = trades.filter((trade) => {
          const tradeTimestamp = Math.floor(Date.parse(trade.time + 'Z') / 1000);
          const tradeCandleTime = Math.floor(tradeTimestamp / 300) * 300;
          return tradeCandleTime === currentTimestamp;
        });

        const openCount = tradesAtThisTime.filter((t) => t.action === 'OPEN').length;
        const closeCount = tradesAtThisTime.filter((t) => t.action === 'CLOSE').length;

        // ⭐ 計算當前持倉量：找出當前時間點或之前最近的交易
        let openPositions: number | undefined = undefined;

        // 找出當前時間點或之前的所有交易
        const tradesUpToNow = trades.filter((trade) => {
          const tradeTimestamp = Math.floor(Date.parse(trade.time + 'Z') / 1000);
          const tradeCandleTime = Math.floor(tradeTimestamp / 300) * 300;
          return tradeCandleTime <= currentTimestamp;
        });

        // 取最後一筆交易的 OpenPositionValue（持倉量）
        if (tradesUpToNow.length > 0) {
          const lastTrade = tradesUpToNow[tradesUpToNow.length - 1];
          if (lastTrade.openPositionValue > 0) {
            openPositions = lastTrade.openPositionValue;
          }
        }

        setLegendData({
          time: currentTimeStr,
          open: candleData.open,
          high: candleData.high,
          low: candleData.low,
          close: candleData.close,
          avgCost: costBasisData?.value,
          openCount: openCount > 0 ? openCount : undefined,
          closeCount: closeCount > 0 ? closeCount : undefined,
          openPositions,
        });
      }
    };

    chart.subscribeCrosshairMove(crosshairMoveHandler);

    return () => {
      chart.unsubscribeCrosshairMove(crosshairMoveHandler);
    };
  }, [trades]); // ⭐ 當 trades 更新時重新訂閱

  // 更新標記（v5 使用 createSeriesMarkers plugin）
  useEffect(() => {
    if (!seriesRef.current) return;

    try {
      // 轉換為 Lightweight Charts v5 需要的格式
      const chartMarkers = markers.map((marker) => ({
        time: marker.time,
        position: marker.position,
        color: marker.color,
        shape: marker.shape,
        text: marker.text || '',
      }));

      // 如果已經有 markers plugin，先移除
      if (markersPluginRef.current) {
        markersPluginRef.current.setMarkers([]);
      }

      // 創建新的 markers plugin
      if (chartMarkers.length > 0) {
        markersPluginRef.current = createSeriesMarkers(seriesRef.current, chartMarkers);
        console.log(`✅ 圖表已顯示 ${markers.length} 個交易標記`);
      }
    } catch (error) {
      console.error('Failed to set markers:', error);
    }
  }, [markers]);

  return (
    <div style={styles.container}>
      {/* ⭐ Legend - 顯示當前 K 線數據 */}
      {legendData && (
        <div style={styles.legend}>
          <div style={styles.legendRow}>
            <span style={styles.legendLabel}>時間:</span>
            <span style={styles.legendValue}>{legendData.time}</span>
          </div>
          <div style={styles.legendRow}>
            <span style={styles.legendLabel}>O:</span>
            <span style={{ ...styles.legendValue, color: '#888' }}>
              {legendData.open.toFixed(2)}
            </span>
            <span style={styles.legendLabel}>H:</span>
            <span style={{ ...styles.legendValue, color: '#26a69a' }}>
              {legendData.high.toFixed(2)}
            </span>
            <span style={styles.legendLabel}>L:</span>
            <span style={{ ...styles.legendValue, color: '#ef5350' }}>
              {legendData.low.toFixed(2)}
            </span>
            <span style={styles.legendLabel}>C:</span>
            <span
              style={{
                ...styles.legendValue,
                color: legendData.close >= legendData.open ? '#26a69a' : '#ef5350',
              }}
            >
              {legendData.close.toFixed(2)}
            </span>
            {legendData.avgCost && (
              <>
                <span style={styles.legendLabel}>平均成本:</span>
                <span style={{ ...styles.legendValue, color: '#FFA500' }}>
                  {legendData.avgCost.toFixed(2)}
                </span>
              </>
            )}
          </div>
          {/* ⭐ 交易操作統計 */}
          {(legendData.openCount || legendData.closeCount || legendData.openPositions !== undefined) && (
            <div style={styles.legendRow}>
              {legendData.openCount && (
                <>
                  <span style={styles.legendLabel}>開倉:</span>
                  <span style={{ ...styles.legendValue, color: '#2196F3' }}>
                    {legendData.openCount} 筆
                  </span>
                </>
              )}
              {legendData.closeCount && (
                <>
                  <span style={styles.legendLabel}>平倉:</span>
                  <span style={{ ...styles.legendValue, color: '#9C27B0' }}>
                    {legendData.closeCount} 筆
                  </span>
                </>
              )}
              {legendData.openPositions !== undefined && (
                <>
                  <span style={styles.legendLabel}>持倉量:</span>
                  <span style={{ ...styles.legendValue, color: '#FFD700' }}>
                    {legendData.openPositions.toFixed(2)}
                  </span>
                </>
              )}
            </div>
          )}
        </div>
      )}

      <div ref={chartContainerRef} style={styles.chartWrapper} />

      {data.length === 0 && (
        <div style={styles.placeholder}>
          <p>📊 請先導入 K 線數據 (histories.json)</p>
        </div>
      )}
    </div>
  );
}

const styles = {
  container: {
    position: 'relative' as const,
    width: '100%',
    marginBottom: '20px',
  },
  legend: {
    position: 'absolute' as const,
    top: '10px',
    left: '10px',
    zIndex: 10,
    backgroundColor: 'rgba(30, 30, 30, 0.9)',
    padding: '10px 15px',
    borderRadius: '6px',
    border: '1px solid #2b2b43',
    fontSize: '13px',
    lineHeight: '1.6',
    pointerEvents: 'none' as const,
  },
  legendRow: {
    display: 'flex',
    gap: '12px',
    alignItems: 'center',
  },
  legendLabel: {
    color: '#888',
    fontWeight: 500,
    minWidth: '20px',
  },
  legendValue: {
    color: '#d1d4dc',
    fontWeight: 600,
    fontFamily: 'monospace',
  },
  chartWrapper: {
    width: '100%',
  },
  placeholder: {
    position: 'absolute' as const,
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#1e1e1e',
    color: '#888',
    fontSize: '18px',
  },
};
