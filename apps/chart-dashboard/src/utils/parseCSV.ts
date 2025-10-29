import type { UTCTimestamp, LineData } from 'lightweight-charts';
import type { TradeData, ChartMarker } from '../types';

/**
 * 解析交易記錄 CSV 數據
 *
 * CSV 格式：TradeID,Time,Action,Price,PositionSize,Balance,OpenPositionValue,PnL%,PnL,AvgCost,PnL%_Avg,PnL_Avg,Fee,Reason,PositionID
 *
 * @param data - PapaParse 解析後的數據數組
 * @returns 解析後的交易數據數組
 */
// PapaParse 解析後的 CSV 行類型（每行是字串鍵值對）
export function parseTradeCSV(data: Record<string, string>[]): TradeData[] {
  return data
    .filter((row) => row.TradeID && row.TradeID.trim() !== '') // 過濾空行和標題行
    .map((row) => {
      try {
        return {
          tradeId: parseInt(row.TradeID),
          time: row.Time,
          action: row.Action.toUpperCase() as 'OPEN' | 'CLOSE',
          price: parseFloat(row.Price),
          positionSize: parseFloat(row.PositionSize),
          balance: parseFloat(row.Balance),
          openPositionValue: parseFloat(row.OpenPositionValue || '0'), // ⭐ 新增
          pnlPercent: parseFloat(row['PnL%'] || row.PnLPercent || '0'),
          pnl: parseFloat(row.PnL || '0'),
          avgCost: parseFloat(row.AvgCost || '0'), // ⭐ 新增
          pnlPercentAvg: parseFloat(row['PnL%_Avg'] || row.PnLPercentAvg || '0'), // ⭐ 新增
          pnlAvg: parseFloat(row.PnL_Avg || row.PnLAvg || '0'), // ⭐ 新增
          fee: parseFloat(row.Fee),
          reason: row.Reason || '',
          positionId: row.PositionID || '',
        };
      } catch (error) {
        console.error('Failed to parse trade row:', row, error);
        return null;
      }
    })
    .filter((trade): trade is TradeData => trade !== null);
}

/**
 * 將交易數據轉換為圖表標記
 *
 * @param trades - 交易數據數組
 * @returns 圖表標記數組
 */
export function tradesToMarkers(trades: TradeData[]): ChartMarker[] {
  return trades.map((trade) => {
    // ⭐ 將日期字符串轉換為 Unix timestamp（秒）
    // CSV 中的時間是 UTC+0，需要明確指定為 UTC 時間
    const timeInSeconds = (Date.parse(trade.time + 'Z') / 1000) as UTCTimestamp;

    if (trade.action === 'OPEN') {
      return {
        time: timeInSeconds,
        position: 'belowBar' as const,
        color: '#2196F3', // 藍色 - 開倉
        shape: 'arrowUp' as const,
        text: `開 ${trade.positionId}`,
        size: 1,
      };
    } else {
      // CLOSE
      const profitColor = trade.pnl >= 0 ? '#4CAF50' : '#F44336'; // 綠色盈利 / 紅色虧損
      const profitSign = trade.pnl > 0 ? '+' : '';

      return {
        time: timeInSeconds,
        position: 'aboveBar' as const,
        color: profitColor,
        shape: 'arrowDown' as const,
        text: `平 ${profitSign}${trade.pnl.toFixed(2)}`,
        size: 1,
      };
    }
  });
}

/**
 * 驗證交易數據是否有效
 */
export function validateTradeData(trades: TradeData[]): boolean {
  if (trades.length === 0) {
    return false;
  }

  return trades.every((trade) => {
    return (
      trade.tradeId > 0 &&
      trade.price > 0 &&
      (trade.action === 'OPEN' || trade.action === 'CLOSE') &&
      trade.time.length > 0
    );
  });
}

/**
 * 計算平均成本線數據
 *
 * ⭐ 簡化版本：只使用 OPEN 交易來構建平均成本線
 * - 只有 OPEN 交易會改變平均成本
 * - CLOSE 交易的 AvgCost 是平倉前的狀態，不應該顯示在成本線上
 * - 當所有倉位被平倉後（打平出場），下一個 OPEN 會自動形成新的成本線
 *
 * @param trades - 交易數據數組
 * @returns 平均成本線數據數組
 */
export function calculateCostBasisLine(trades: TradeData[]): LineData[] {
  if (trades.length === 0) return [];

  const costBasisData: LineData[] = [];

  // ⭐ 只遍歷 OPEN 交易
  const openTrades = trades.filter((trade) => trade.action === 'OPEN');

  for (const trade of openTrades) {
    // ⭐ CSV 中的時間是 UTC+0，需要明確指定為 UTC 時間（添加 'Z' 後綴）
    const timeInSeconds = Math.floor(Date.parse(trade.time + 'Z') / 1000);

    // ⭐ OPEN 交易的 AvgCost 是開倉後的平均成本
    if (trade.avgCost > 0) {
      costBasisData.push({
        time: timeInSeconds as UTCTimestamp,
        value: trade.avgCost,
      });
    }
  }

  return costBasisData;
}
