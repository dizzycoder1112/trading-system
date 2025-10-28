#!/usr/bin/env tsx

/**
 * OKX 歷史數據下載腳本
 *
 * 使用方式：
 * npx tsx scripts/download_okx_history.ts \
 *   --inst-id=ETH-USDT-SWAP \
 *   --bar=5m \
 *   --after=2025-10-01T00:00:00 \
 *   --before=2025-10-26T00:00:00
 */

interface OKXCandleResponse {
  code: string;
  msg: string;
  data: string[][];
}

interface DownloadOptions {
  instId: string;
  bar: string;
  after: string; // ISO 8601 時間字符串
  before: string; // ISO 8601 時間字符串
  outputDir: string;
}

const API_BASE = 'https://www.okx.com';
const RATE_LIMIT_DELAY = 250; // 250ms（安全起見，20 req/2s = 100ms，我們用 250ms）
const MAX_LIMIT = 300; // OKX 每次最多返回 300 條

/**
 * 解析命令行參數
 */
function parseArgs(): DownloadOptions {
  const args = process.argv.slice(2);
  const options: Partial<DownloadOptions> = {
    outputDir: 'apps/trading-strategy-server/data',
  };

  for (const arg of args) {
    if (arg.startsWith('--inst-id=')) {
      options.instId = arg.split('=')[1];
    } else if (arg.startsWith('--bar=')) {
      options.bar = arg.split('=')[1];
    } else if (arg.startsWith('--after=')) {
      options.after = arg.split('=')[1];
    } else if (arg.startsWith('--before=')) {
      options.before = arg.split('=')[1];
    } else if (arg.startsWith('--output=')) {
      options.outputDir = arg.split('=')[1];
    }
  }

  if (!options.instId || !options.bar || !options.after || !options.before) {
    console.error('❌ 缺少必要參數');
    console.log('使用方式：');
    console.log('  npx tsx scripts/download_okx_history.ts \\');
    console.log('    --inst-id=ETH-USDT-SWAP \\');
    console.log('    --bar=5m \\');
    console.log('    --after=2025-10-01T00:00:00 \\');
    console.log('    --before=2025-10-26T00:00:00 \\');
    console.log('    [--output=apps/backtesting/data]');
    process.exit(1);
  }

  return options as DownloadOptions;
}

/**
 * ISO 8601 時間字符串轉 OKX 時間戳（毫秒）
 */
function parseTimestamp(isoString: string): number {
  const date = new Date(isoString);
  if (isNaN(date.getTime())) {
    throw new Error(`無效的時間格式: ${isoString}`);
  }
  return date.getTime();
}

/**
 * 時間戳轉可讀格式
 */
function formatTimestamp(ts: number): string {
  return new Date(ts).toISOString().replace('T', ' ').substring(0, 19);
}

/**
 * Sleep 函數
 */
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * 請求 OKX API（帶重試）
 */
async function fetchCandles(
  instId: string,
  bar: string,
  after?: string,
  before?: string,
  retries = 3,
): Promise<string[][]> {
  const params = new URLSearchParams({
    instId,
    bar,
    limit: MAX_LIMIT.toString(),
  });

  if (after) params.append('after', after);
  if (before) params.append('before', before);

  const url = `${API_BASE}/api/v5/market/history-candles?${params}`;

  // 調試：打印請求 URL
  console.log(`   🔗 URL: ${url}`);

  for (let i = 0; i < retries; i++) {
    try {
      const response = await fetch(url);

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const json: OKXCandleResponse = await response.json();

      if (json.code !== '0') {
        throw new Error(`OKX API Error: ${json.msg}`);
      }

      console.log(`   📦 API 返回: ${json.data.length} 條數據`);

      return json.data;
    } catch (error) {
      console.error(`⚠️  請求失敗 (嘗試 ${i + 1}/${retries}): ${error}`);

      if (i < retries - 1) {
        await sleep(1000 * (i + 1)); // 指數退避
      } else {
        throw error;
      }
    }
  }

  return [];
}

/**
 * 下載所有歷史數據（分頁）
 */
async function downloadAllCandles(options: DownloadOptions): Promise<string[][]> {
  const { instId, bar, after, before } = options;

  const afterTs = parseTimestamp(after);
  const beforeTs = parseTimestamp(before);

  console.log('📥 開始下載歷史數據...');
  console.log(`   交易對: ${instId}`);
  console.log(`   週期: ${bar}`);
  console.log(`   時間範圍: ${formatTimestamp(afterTs)} ~ ${formatTimestamp(beforeTs)}`);
  console.log('');

  const allData: string[][] = [];
  // 從 beforeTs 開始往前查（使用 after 參數，因為 after 返回早於指定時間的數據）
  let currentAfter: string | undefined = beforeTs.toString();
  let page = 1;

  while (true) {
    console.log(`📄 Page ${page}: 請求中...`);

    // 只使用 after 參數（不能同時使用 before 和 after）
    const data = await fetchCandles(instId, bar, currentAfter, undefined);

    if (data.length === 0) {
      console.log('✅ 沒有更多數據');
      break;
    }

    allData.push(...data);
    console.log(`   ├─ 獲取 ${data.length} 條數據`);
    console.log(`   └─ 累計 ${allData.length} 條`);

    // 檢查是否還有更多數據
    if (data.length < MAX_LIMIT) {
      console.log('✅ 已獲取所有數據');
      break;
    }

    // 獲取最後一條的時間戳作為下次的 after
    const lastCandle = data[data.length - 1];
    const lastTs = parseInt(lastCandle[0]);

    // 檢查是否超過起始時間
    if (lastTs <= afterTs) {
      console.log('✅ 已到達起始時間');
      break;
    }

    currentAfter = lastCandle[0];
    page++;

    // Rate limiting
    await sleep(RATE_LIMIT_DELAY);
  }

  console.log('');
  console.log(`✅ 總共下載 ${allData.length} 條數據`);

  return allData;
}

/**
 * 生成文件夾名稱
 * 格式: 20240930-20241001-5m-ETH-USDT-SWAP
 */
function generateFolderName(options: DownloadOptions): string {
  const { instId, bar, after, before } = options;

  const afterDate = new Date(after).toISOString().split('T')[0].replace(/-/g, '');
  const beforeDate = new Date(before).toISOString().split('T')[0].replace(/-/g, '');

  return `${afterDate}-${beforeDate}-${bar}-${instId}`;
}

/**
 * 保存數據到文件
 * 輸出路徑: {outputDir}/{folderName}/histories.json
 * 例如: apps/trading-strategy-server/data/20240930-20241001-5m-ETH-USDT-SWAP/histories.json
 */
async function saveToFile(options: DownloadOptions, data: string[][]): Promise<void> {
  const folderName = generateFolderName(options);
  const folderPath = `${options.outputDir}/${folderName}`;
  const filepath = `${folderPath}/histories.json`;

  const output: OKXCandleResponse = {
    code: '0',
    msg: '',
    data,
  };

  const fs = await import('fs/promises');

  // 確保目錄存在（創建文件夾）
  await fs.mkdir(folderPath, { recursive: true });

  // 寫入文件
  await fs.writeFile(filepath, JSON.stringify(output, null, 2), 'utf-8');

  console.log(`💾 數據已保存到: ${filepath}`);
}

/**
 * 主函數
 */
async function main() {
  try {
    const options = parseArgs();

    const data = await downloadAllCandles(options);

    if (data.length === 0) {
      console.log('⚠️  沒有數據可保存');
      return;
    }

    await saveToFile(options, data);

    console.log('');
    console.log('✅ 完成！');
  } catch (error) {
    console.error('❌ 錯誤:', error);
    process.exit(1);
  }
}

main();
