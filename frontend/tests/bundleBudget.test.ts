import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_RAW_LIMIT_BYTES, evaluateBundleBudget } from '../scripts/check-bundle-budget.mjs';

test('bundle budget reports entry and chunk violations deterministically', () => {
  const report = evaluateBundleBudget({
    entryFile: 'assets/index.js',
    limitBytes: 50,
    minimumChunkCount: 2,
    assets: [
      { file: 'assets/route.js', contents: Buffer.from('a'.repeat(4096)) },
      { file: 'assets/index.js', contents: Buffer.from('entry') },
      { file: 'assets/vendor.js', contents: Buffer.from(Array.from({ length: 256 }, (_, index) => String.fromCharCode(index)).join('')) },
    ],
  });

  assert.deepEqual(report.results.map(({ file }) => file), [
    'assets/index.js',
    'assets/route.js',
    'assets/vendor.js',
  ]);
  assert.equal(report.results[0]?.isEntry, true);
  assert.equal(report.entryFound, true);
  assert.equal(report.chunkCount, 2);
  assert.deepEqual(report.violations.map(({ file }) => file), ['assets/vendor.js']);
});

test('raw budget catches compressible oversized entry and worker files while accepting exactly 500 kB', () => {
  assert.equal(DEFAULT_RAW_LIMIT_BYTES, 500_000);
  const report = evaluateBundleBudget({
    entryFile: 'assets/index.js',
    assets: [
      { file: 'assets/index.js', contents: Buffer.alloc(500_001, 'a') },
      { file: 'assets/sheet.worker.js', contents: Buffer.alloc(500_001, 'b') },
      { file: 'assets/vendor.js', contents: Buffer.alloc(500_000, 'c') },
    ],
  });

  assert.ok(report.results.every((asset) => asset.gzipBytes < report.limitBytes));
  assert.deepEqual(report.violations.map(({ file }) => file), ['assets/index.js', 'assets/sheet.worker.js']);
});

test('raw budget counts UTF-8 bytes rather than character length', () => {
  const report = evaluateBundleBudget({
    entryFile: 'assets/index.js',
    rawLimitBytes: 3,
    assets: [{ file: 'assets/index.js', contents: 'ắa' }],
  });

  assert.equal(report.results[0]?.rawBytes, 4);
  assert.equal(report.violations.length, 1);
});
