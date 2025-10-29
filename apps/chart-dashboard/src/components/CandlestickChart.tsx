import { useEffect, useRef } from 'react';
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
} from 'lightweight-charts';
import type { CandleData, ChartMarker } from '../types';

interface CandlestickChartProps {
  data: CandleData[];
  markers?: ChartMarker[];
  costBasisLine?: LineData[]; // 平均成本線數據
  width?: number;
  height?: number;
}

export function CandlestickChart({
  data,
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
