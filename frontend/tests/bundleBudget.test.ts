import assert from 'node:assert/strict';
import test from 'node:test';
import { evaluateBundleBudget } from '../scripts/check-bundle-budget.mjs';

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
