import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import {
  evidenceUploadBatchLimit,
  evidenceUploadConcurrency,
  evidenceUploadOverallProgress,
  failedEvidenceUploadRowIds,
  runEvidenceUploadQueue,
} from '../src/modules/work-items/evidenceUploadQueue.ts';

test('evidence upload queue keeps the approved batch and concurrency limits', () => {
  assert.equal(evidenceUploadBatchLimit, 10);
  assert.equal(evidenceUploadConcurrency, 3);
});

test('overall progress is weighted by file size and clamped', () => {
  assert.equal(evidenceUploadOverallProgress([]), 0);
  assert.equal(evidenceUploadOverallProgress([
    { fileSize: 100, progress: 100 },
    { fileSize: 300, progress: 0 },
  ]), 25);
  assert.equal(evidenceUploadOverallProgress([
    { fileSize: 100, progress: 150 },
    { fileSize: 100, progress: -20 },
  ]), 50);
});

test('queue uploads no more than three rows concurrently and preserves partial results', async () => {
  let active = 0;
  let maxActive = 0;
  const started: number[] = [];
  const completed: number[] = [];

  const results = await runEvidenceUploadQueue([0, 1, 2, 3, 4, 5, 6], async (item) => {
    active += 1;
    maxActive = Math.max(maxActive, active);
    started.push(item);
    await new Promise((resolve) => setTimeout(resolve, 4));
    active -= 1;
    if (item === 2 || item === 5) throw new Error(`failed-${item}`);
    completed.push(item);
    return item;
  });

  assert.equal(maxActive, 3);
  assert.deepEqual(started, [0, 1, 2, 3, 4, 5, 6]);
  assert.deepEqual(completed, [0, 1, 3, 4, 6]);
  assert.deepEqual(results.map((result) => result.status), [
    'fulfilled',
    'fulfilled',
    'rejected',
    'fulfilled',
    'fulfilled',
    'rejected',
    'fulfilled',
  ]);
});

test('settled batch identifies only failed rows for retry and clears an all-success batch', () => {
  const rows = [{ id: 'first' }, { id: 'second' }, { id: 'third' }];
  const partial = failedEvidenceUploadRowIds(rows, [
    { status: 'fulfilled', value: undefined },
    { status: 'rejected', reason: new Error('failed') },
    { status: 'fulfilled', value: undefined },
  ]);
  const success = failedEvidenceUploadRowIds(rows, rows.map(() => ({ status: 'fulfilled', value: undefined })));

  assert.deepEqual([...partial], ['second']);
  assert.deepEqual([...success], []);
});

test('work-item evidence UI exposes row add, retry and progress states', async () => {
  const panel = await readFile(new URL('../src/modules/work-items/EvidencePanel.tsx', import.meta.url), 'utf8');

  assert.match(panel, /evidenceUploadBatchLimit/);
  assert.match(panel, /evidenceUploadConcurrency/);
  assert.match(panel, /runEvidenceUploadQueue\(rows/);
  assert.match(panel, />\s*Thêm\s*</);
  assert.match(panel, /Thử lại tệp/);
  assert.match(panel, /Tổng tiến độ \{overallProgress\}%/);
  assert.match(panel, /status: percent >= 100 \? 'processing' : 'uploading'/);
  assert.match(panel, /status: 'success', progress: 100/);
  assert.match(panel, /status: 'error', progress: 0/);
  assert.match(panel, /setUploadRows\(\[createEvidenceUploadRow\(\)\]\)/);
  assert.match(panel, /current\.filter\(\(row\) => failedRowIds\.has\(row\.id\)\)/);
  assert.match(panel, /Đã tải xong \$\{successCount\} tệp/);
  assert.match(panel, /formData\.append\('visibility', 'internal'\)/);
});

test('progress upload transport preserves auth, refresh and structured API errors', async () => {
  const client = await readFile(new URL('../src/lib/api/client.ts', import.meta.url), 'utf8');

  assert.match(client, /export function postFormWithProgress/);
  assert.match(client, /new XMLHttpRequest\(\)/);
  assert.match(client, /xhr\.upload\.onprogress/);
  assert.match(client, /xhr\.withCredentials = true/);
  assert.match(client, /xhr\.setRequestHeader\('Authorization', `Bearer \$\{token\}`\)/);
  assert.match(client, /shouldRefresh\(path, error\.status, error\.code\)/);
  assert.match(client, /const refreshedToken = await refreshAccessToken\(\)/);
  assert.match(client, /json\?\.error\?\.details/);
});
