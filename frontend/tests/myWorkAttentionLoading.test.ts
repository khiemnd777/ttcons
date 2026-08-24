import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('My Work loads the attention summary independently and details only in attention mode', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /\/me\/activity-attention\/summary/);
  assert.match(page, /queryKey: \['activity-attention', 'summary', project\]/);
  assert.match(page, /queryKey: \['activity-attention', 'list', project\]/);
  assert.match(page, /enabled: mode === 'attention'/);
  assert.match(page, /if \(mode === 'attention'\) void activityAttention\.refetch\(\)/);
  assert.match(page, /activityAttentionSummary\.data\?\.has_more\s*\? '120\+'/);
  assert.match(page, /activityAttentionSummary\.isLoading\s*\? '…'/);
  assert.match(page, /activityAttentionSummary\.isError\s*\? '—'/);
  assert.doesNotMatch(page, /label="Trao đổi cần phản hồi" count=\{attentionRows\.length\}/);
});

test('attention invalidation refreshes both summary and detail query families', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /invalidateQueries\(\{ queryKey: \['activity-attention'\] \}\)/);
});
