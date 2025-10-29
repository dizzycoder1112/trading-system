import { useCallback, useRef } from 'react';
import Papa from 'papaparse';
import type { CandleData, TradeData } from '../types';
import { parseCandleJSON, validateCandleData } from '../utils/parseJSON';
import { parseTradeCSV, validateTradeData } from '../utils/parseCSV';

interface FileUploaderProps {
  onCandlesLoaded: (candles: CandleData[]) => void;
  onTradesLoaded: (trades: TradeData[]) => void;
}

export function FileUploader({ onCandlesLoaded, onTradesLoaded }: FileUploaderProps) {
  const candleInputRef = useRef<HTMLInputElement>(null);
  const tradeInputRef = useRef<HTMLInputElement>(null);

  // 處理 K 線 JSON 文件上傳
  const handleCandleUpload = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;

      const reader = new FileReader();
      reader.onload = (event) => {
        try {
          const json = JSON.parse(event.target?.result as string);
          const candles = parseCandleJSON(json);

          if (!validateCandleData(candles)) {
            alert('K 線數據驗證失敗，請檢查文件格式');
            return;
          }

          console.log(`✅ 成功載入 ${candles.length} 根 K 線`);
          onCandlesLoaded(candles);
        } catch (error) {
          console.error('Failed to parse JSON:', error);
          alert(`解析 JSON 失敗: ${error instanceof Error ? error.message : '未知錯誤'}`);
        }
      };
      reader.onerror = () => {
        alert('讀取文件失敗');
      };
      reader.readAsText(file);
    },
    [onCandlesLoaded],
  );

  // 處理交易記錄 CSV 文件上傳
  const handleTradeUpload = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;

      Papa.parse<Record<string, string>>(file, {
        header: true,
        skipEmptyLines: true,
        // PapaParse ParseResult 泛型：data 是 CSV 行數組，每行是字串鍵值對
        complete: (results) => {
          try {
            const trades = parseTradeCSV(results.data);

            if (!validateTradeData(trades)) {
              alert('交易數據驗證失敗，請檢查文件格式');
              return;
            }

            console.log(`✅ 成功載入 ${trades.length} 筆交易記錄`);
            onTradesLoaded(trades);
          } catch (error) {
            console.error('Failed to parse CSV:', error);
            alert(`解析 CSV 失敗: ${error instanceof Error ? error.message : '未知錯誤'}`);
          }
        },
        // PapaParse Error 類型
        error: (error: Error) => {
          console.error('Failed to read CSV:', error);
          alert(`讀取 CSV 失敗: ${error.message}`);
        },
      });
    },
    [onTradesLoaded],
  );

  // 清除選擇的文件
  const clearCandleFile = () => {
    if (candleInputRef.current) {
      candleInputRef.current.value = '';
    }
  };

  const clearTradeFile = () => {
    if (tradeInputRef.current) {
      tradeInputRef.current.value = '';
    }
  };

  return (
    <div style={styles.container}>
      <h2 style={styles.title}>📁 數據導入</h2>

      <div style={styles.uploadSection}>
        <label htmlFor="candle-upload" style={styles.label}>
          📊 K 線數據 (histories.json)
        </label>
        <div style={styles.inputGroup}>
          <input
            ref={candleInputRef}
            id="candle-upload"
            type="file"
            accept=".json"
            onChange={handleCandleUpload}
            style={styles.input}
          />
          <button onClick={clearCandleFile} style={styles.clearButton}>
            清除
          </button>
        </div>
        <p style={styles.hint}>OKX 歷史 K 線數據（JSON 格式）</p>
      </div>

      <div style={styles.uploadSection}>
        <label htmlFor="trade-upload" style={styles.label}>
          💹 交易記錄 (trades.csv)
        </label>
        <div style={styles.inputGroup}>
          <input
            ref={tradeInputRef}
            id="trade-upload"
            type="file"
            accept=".csv"
            onChange={handleTradeUpload}
            style={styles.input}
          />
          <button onClick={clearTradeFile} style={styles.clearButton}>
            清除
          </button>
        </div>
        <p style={styles.hint}>回測生成的交易記錄（CSV 格式）</p>
      </div>
    </div>
  );
}

// 簡單的內聯樣式（後續可以替換為 CSS 模組或 Tailwind）
const styles = {
  container: {
    padding: '20px',
    backgroundColor: '#f5f5f5',
    borderRadius: '8px',
    marginBottom: '20px',
  },
  title: {
    margin: '0 0 20px 0',
    fontSize: '24px',
    color: '#333',
  },
  uploadSection: {
    marginBottom: '20px',
  },
  label: {
    display: 'block',
    marginBottom: '8px',
    fontSize: '16px',
    fontWeight: 'bold' as const,
    color: '#555',
  },
  inputGroup: {
    display: 'flex',
    gap: '10px',
    alignItems: 'center',
  },
  input: {
    flex: 1,
    padding: '8px',
    fontSize: '14px',
    border: '1px solid #ddd',
    borderRadius: '4px',
    backgroundColor: 'white',
  },
  clearButton: {
    padding: '8px 16px',
    fontSize: '14px',
    backgroundColor: '#f44336',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
  },
  hint: {
    margin: '8px 0 0 0',
    fontSize: '12px',
    color: '#888',
  },
};
