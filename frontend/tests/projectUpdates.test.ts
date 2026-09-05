import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkItemActivity } from '../src/lib/types/domain.ts';
import { projectActivityReadBoundary, projectUpdateCount, projectUpdateIDs } from '../src/modules/projects/projectUpdates.ts';

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
