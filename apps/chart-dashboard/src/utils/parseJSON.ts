import type { OKXCandleResponse, CandleData } from '../types';

/**
 * 解析 OKX 歷史 K 線 JSON 數據
 *
 * @param json - OKX API 返回的 JSON 對象
 * @returns 解析後的 K 線數據數組（從舊到新排序）
 */
export function parseCandleJSON(json: OKXCandleResponse): CandleData[] {
  if (!json.data || !Array.isArray(json.data)) {
    throw new Error('Invalid JSON format: missing or invalid data field');
  }

  if (json.code !== '0') {
    throw new Error(`API error: ${json.msg}`);
  }

  const candles = json.data
    .map((item) => {
      try {
        // 將毫秒轉換為秒（Lightweight Charts 使用 Unix timestamp 秒級）
        const timeInSeconds = parseInt(item[0]) / 1000;

        return {
          time: timeInSeconds, // number 可以自動兼容 Time 類型
          open: parseFloat(item[1]),
          high: parseFloat(item[2]),
          low: parseFloat(item[3]),
          close: parseFloat(item[4]),
        };
      } catch (error) {
        console.error('Failed to parse candle data:', item, error);
        return null;
      }
    })
    .filter((candle): candle is CandleData => candle !== null);

  // OKX 返回的數據是從新到舊，需要反轉成從舊到新
  return candles.reverse();
}

/**
 * 驗證 K 線數據是否有效
 */
export function validateCandleData(candles: CandleData[]): boolean {
  if (candles.length === 0) {
    return false;
  }

  // 檢查每根 K 線的有效性
  return candles.every((candle) => {
    // Time 類型可以是 number、string 或 BusinessDay 對象
    // 這裡簡化處理：只驗證基本的 OHLC 數據有效性
    return (
      candle.high >= candle.low &&
      candle.high >= candle.open &&
      candle.high >= candle.close &&
      candle.low <= candle.open &&
      candle.low <= candle.close &&
      candle.time !== undefined && // 確保 time 存在
      candle.time !== null
    );
  });
}
