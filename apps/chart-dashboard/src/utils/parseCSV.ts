import type { UTCTimestamp, LineData } from 'lightweight-charts';
import type { TradeData, ChartMarker } from '../types';

/**
 * 解析交易記錄 CSV 數據
 *
 * CSV 格式：TradeID,Time,Action,Price,PositionSize,Balance,PnL%,PnL,Fee,Reason,PositionID
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
          pnlPercent: parseFloat(row['PnL%'] || row.PnLPercent || '0'),
          pnl: parseFloat(row.PnL || '0'),
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
    // 將日期字符串轉換為 Unix timestamp（秒）
    // UTCTimestamp 是 lightweight-charts 的 nominal type，需使用 type assertion
    const timeInSeconds = (new Date(trade.time).getTime() / 1000) as UTCTimestamp;

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
 * 遍歷交易記錄，追蹤未平倉部位的平均成本
 * - 將同一時間的交易合併處理（先 CLOSE 後 OPEN）
 * - 每個時間點只記錄一個數據點
 * - 開倉時：更新平均成本（加權平均）
 * - 平倉時：保持平均成本不變（部分平倉）或清空（完全平倉）
 *
 * @param trades - 交易數據數組
 * @returns 平均成本線數據數組
 */
export function calculateCostBasisLine(trades: TradeData[]): LineData[] {
  if (trades.length === 0) return [];

  // 按時間分組交易
  const tradesByTime = new Map<number, TradeData[]>();
  for (const trade of trades) {
    const timeInSeconds = Math.floor(new Date(trade.time).getTime() / 1000);
    if (!tradesByTime.has(timeInSeconds)) {
      tradesByTime.set(timeInSeconds, []);
    }
    tradesByTime.get(timeInSeconds)!.push(trade);
  }

  // 按時間排序
  const sortedTimes = Array.from(tradesByTime.keys()).sort((a, b) => a - b);

  const costBasisData: LineData[] = [];
  let currentSize = 0; // 當前總持倉大小
  let totalCost = 0; // 總成本（所有部位的 price * size 加總）

  // 遍歷每個時間點
  for (const timeInSeconds of sortedTimes) {
    const tradesAtTime = tradesByTime.get(timeInSeconds)!;

    // 先處理所有 CLOSE（平倉）
    for (const trade of tradesAtTime.filter((t) => t.action === 'CLOSE')) {
      const avgCost = currentSize > 0 ? totalCost / currentSize : 0;
      currentSize -= trade.positionSize;

      if (currentSize <= 0) {
        currentSize = 0;
        totalCost = 0;
      } else {
        totalCost = avgCost * currentSize;
      }
    }

    // 再處理所有 OPEN（開倉）
    for (const trade of tradesAtTime.filter((t) => t.action === 'OPEN')) {
      totalCost += trade.price * trade.positionSize;
      currentSize += trade.positionSize;
    }

    // 在這個時間點，記錄最終的平均成本（如果有持倉）
    if (currentSize > 0) {
      const avgCost = totalCost / currentSize;
      costBasisData.push({
        time: timeInSeconds as UTCTimestamp,
        value: avgCost,
      });
    }
  }

  return costBasisData;
}
