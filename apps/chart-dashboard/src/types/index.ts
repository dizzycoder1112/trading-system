import type { UTCTimestamp } from 'lightweight-charts';

// K線數據類型（使用 UTCTimestamp 以確保類型一致性）
export interface CandleData {
  time: UTCTimestamp; // Unix timestamp in seconds
  open: number;
  high: number;
  low: number;
  close: number;
}

// OKX API 返回的 JSON 格式
export interface OKXCandleResponse {
  code: string;
  msg: string;
  data: string[][]; // [timestamp, open, high, low, close, vol, volCcy, volCcyQuote, confirm]
}

// 交易記錄類型
export interface TradeData {
  tradeId: number;
  time: string; // 時間字符串
  action: 'OPEN' | 'CLOSE';
  price: number;
  positionSize: number;
  balance: number;
  openPositionValue: number; // ⭐ 累計持倉總價值
  pnlPercent: number; // 基於單筆開倉價的盈虧百分比
  pnl: number; // 基於單筆開倉價的盈虧金額
  avgCost: number; // ⭐ 平倉時的平均成本
  pnlPercentAvg: number; // ⭐ 基於平均成本的盈虧百分比
  pnlAvg: number; // ⭐ 基於平均成本的盈虧金額
  fee: number;
  reason: string;
  positionId: string;
}

// 圖表標記類型
export interface ChartMarker {
  time: UTCTimestamp; // Unix timestamp in seconds
  position: 'aboveBar' | 'belowBar' | 'inBar';
  color: string;
  shape: 'circle' | 'square' | 'arrowUp' | 'arrowDown';
  text?: string;
  size?: number;
}

// 持倉快照類型
export interface PositionSnapshot {
  time: string;
  count: number;
  avgCost: number;
  totalSize: number;
  unrealizedPnL: number;
}
