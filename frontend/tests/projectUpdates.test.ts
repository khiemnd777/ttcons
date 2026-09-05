import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkItemActivity } from '../src/lib/types/domain.ts';
import { loadProjectUpdates, projectActivityReadBoundary, projectUpdateCount, projectUpdateIDs } from '../src/modules/projects/projectUpdates.ts';

test('batch keys are stable when row order changes and never duplicate a project', () => {
  assert.equal(projectUpdateIDs(['b', 'a', 'b']), 'a,b');
  assert.equal(projectUpdateIDs([]), '');
});

test('read-through cannot swallow unloaded pages or a hidden event-type subset', () => {
  const items = [{ id: 'newest' }, { id: 'older' }] as WorkItemActivity[];
  assert.equal(projectActivityReadBoundary(items, true, false), undefined);
  assert.equal(projectActivityReadBoundary(items, false, true), undefined);
  assert.equal(projectActivityReadBoundary(items, false, false), 'newest');
  assert.equal(projectActivityReadBoundary([], false, false), undefined);
  assert.equal(projectActivityReadBoundary([{ id: 'withdrawn', withdrawn_at: 'now' }, ...items] as WorkItemActivity[], false, false), 'newest');
});

test('compact badge preserves zero and caps large unread counts', () => {
  assert.equal(projectUpdateCount(0), '0');
  assert.equal(projectUpdateCount(4), '4');
  assert.equal(projectUpdateCount(100), '99+');
});

test('large update requests deduplicate and respect the 100-project API limit', async () => {
  const ids = Array.from({ length: 251 }, (_, index) => `project-${String(index).padStart(3, '0')}`);
  const requests: string[][] = [];
  const result = await loadProjectUpdates([...ids].reverse().concat(ids.slice(0, 20)), async (batch) => {
    const projectIDs = batch.split(',');
    requests.push(projectIDs);
    return { items: projectIDs.map((project_id) => ({ project_id, notification_unread: 1, discussion_unread: 2, system_unread: 3, latest: null })) };
  });
  assert.deepEqual(requests.map((batch) => batch.length), [100, 100, 51]);
  assert.deepEqual(requests.flat(), ids);
  assert.deepEqual(result.items.map((item) => item.project_id), ids);
});

test('empty batches do not request data; failed batches cannot return partial summaries', async () => {
  assert.deepEqual(await loadProjectUpdates([], async () => { throw new Error('unexpected request'); }), { items: [] });
  let calls = 0;
  const ids = Array.from({ length: 201 }, (_, index) => `project-${index}`);
  await assert.rejects(loadProjectUpdates(ids, async () => {
    calls++;
    if (calls === 2) throw new Error('permission changed');
    return { items: [] };
  }), /permission changed/);
  assert.equal(calls, 2);
});
