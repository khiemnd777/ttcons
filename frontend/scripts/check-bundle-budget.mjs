import { gzipSync } from 'node:zlib';
import { readFile, readdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

export const DEFAULT_GZIP_LIMIT_BYTES = 250 * 1024;
export const DEFAULT_RAW_LIMIT_BYTES = 500 * 1000;
export const DEFAULT_MINIMUM_CHUNK_COUNT = 20;

export function evaluateBundleBudget({
  assets,
  entryFile,
  limitBytes = DEFAULT_GZIP_LIMIT_BYTES,
  rawLimitBytes = DEFAULT_RAW_LIMIT_BYTES,
  minimumChunkCount = DEFAULT_MINIMUM_CHUNK_COUNT,
}) {
  const results = [...assets]
    .sort((left, right) => left.file.localeCompare(right.file))
    .map((asset) => ({
      ...asset,
      rawBytes: Buffer.byteLength(asset.contents),
      gzipBytes: gzipSync(asset.contents, { level: 9 }).byteLength,
      isEntry: asset.file === entryFile,
    }));

  return {
    limitBytes,
    rawLimitBytes,
    minimumChunkCount,
    results,
    entryFound: results.some((asset) => asset.isEntry),
    chunkCount: results.filter((asset) => !asset.isEntry).length,
    violations: results.filter((asset) => asset.rawBytes > rawLimitBytes || asset.gzipBytes > limitBytes),
  };
}

function formatKiB(bytes) {
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

function formatKB(bytes) {
  return `${(bytes / 1000).toFixed(2)} kB`;
}

async function collectJavaScriptAssets(distDirectory) {
  const assetsDirectory = path.join(distDirectory, 'assets');
  const names = await readdir(assetsDirectory);
  const javascriptNames = names.filter((name) => name.endsWith('.js'));

  return Promise.all(
    javascriptNames.map(async (name) => ({
      file: `assets/${name}`,
      contents: await readFile(path.join(assetsDirectory, name)),
    })),
  );
}

function findEntryFile(indexHtml) {
  const match = indexHtml.match(/<script\b[^>]*\btype=["']module["'][^>]*\bsrc=["']\/([^"']+\.js)["'][^>]*>/i)
    ?? indexHtml.match(/<script\b[^>]*\bsrc=["']\/([^"']+\.js)["'][^>]*\btype=["']module["'][^>]*>/i);
  if (!match) throw new Error('Không tìm thấy JavaScript entry trong dist/index.html. Hãy chạy npm run build trước.');
  return match[1];
}

async function main() {
  const frontendDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
  const distDirectory = path.join(frontendDirectory, 'dist');
  const indexHtml = await readFile(path.join(distDirectory, 'index.html'), 'utf8');
  const entryFile = findEntryFile(indexHtml);
  const assets = await collectJavaScriptAssets(distDirectory);
  if (assets.length === 0) throw new Error('Không tìm thấy JavaScript asset trong dist/assets.');

  const report = evaluateBundleBudget({ assets, entryFile });
  console.log(`Bundle budget cho entry và từng JavaScript chunk: raw ${formatKB(report.rawLimitBytes)}, gzip ${formatKiB(report.limitBytes)}.`);
  console.log(`Code-splitting gate: ${report.chunkCount} chunks (tối thiểu ${report.minimumChunkCount}).`);
  for (const result of report.results) {
    console.log(`${result.isEntry ? 'entry' : 'chunk'}\traw ${formatKB(result.rawBytes)}\tgzip ${formatKiB(result.gzipBytes)}\t${result.file}`);
  }

  if (report.violations.length > 0) {
    const details = report.violations.map((asset) => {
      const exceeded = [];
      if (asset.rawBytes > report.rawLimitBytes) exceeded.push(`raw ${asset.rawBytes} > ${report.rawLimitBytes} bytes`);
      if (asset.gzipBytes > report.limitBytes) exceeded.push(`gzip ${asset.gzipBytes} > ${report.limitBytes} bytes`);
      return `${asset.file} (${exceeded.join('; ')})`;
    }).join(', ');
    throw new Error(`Vượt bundle budget: ${details}`);
  }

  if (!report.entryFound) {
    throw new Error(`Entry ${entryFile} không tồn tại trong danh sách JavaScript assets.`);
  }
  if (report.chunkCount < report.minimumChunkCount) {
    throw new Error(`Chỉ có ${report.chunkCount} JavaScript chunks; yêu cầu tối thiểu ${report.minimumChunkCount}.`);
  }
}

const invokedAsScript = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedAsScript) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  });
}
