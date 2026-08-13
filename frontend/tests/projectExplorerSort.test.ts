import assert from 'node:assert/strict';
import test from 'node:test';
import type { Project } from '../src/lib/types/domain.ts';
import { statusLabel } from '../src/lib/utils/status.ts';
import {
  nextProjectExplorerSort,
  projectExplorerSortFromValue,
  sortProjectExplorerRows,
} from '../src/modules/projects/projectExplorerSort.ts';

function project(id: string, values: Partial<Project>): Project {
  return {
    id,
    code: `PRJ-${id}`,
    name: `Công trình ${id}`,
    client_name: 'Chủ đầu tư',
    status: 'planning',
    progress_percent: 0,
    workflow_entry_mode: 'survey_required',
    ...values,
  };
}

test('project explorer preserves API order until a sort is selected and never mutates its source', () => {
  const rows = [project('2', {}), project('1', {})];
  const sourceIds = rows.map((row) => row.id);

  assert.deepEqual(sortProjectExplorerRows(rows, null).map((row) => row.id), ['2', '1']);
  assert.deepEqual(rows.map((row) => row.id), sourceIds);
});

test('project explorer sorts the displayed project tree naturally by code then name', () => {
  const rows = [
    project('10', { code: 'B3-10', name: 'Beta' }),
    project('2b', { code: 'B3-2', name: 'Zulu' }),
    project('2a', { code: 'B3-2', name: 'An Phú' }),
  ];

  assert.deepEqual(
    sortProjectExplorerRows(rows, { key: 'project', direction: 'asc' }).map((row) => row.id),
    ['2a', '2b', '10'],
  );
  assert.deepEqual(
    sortProjectExplorerRows(rows, { key: 'project', direction: 'desc' }).map((row) => row.id),
    ['10', '2b', '2a'],
  );
});

test('project explorer sorts statuses by Vietnamese display label', () => {
  const rows = [
    project('active', { status: 'active' }),
    project('planning', { status: 'planning' }),
  ];

  assert.deepEqual(
    sortProjectExplorerRows(rows, { key: 'status', direction: 'asc' }, statusLabel).map((row) => row.id),
    ['planning', 'active'],
  );
});

test('project explorer keeps unassigned projects last in either owner direction', () => {
  const rows = [
    project('none', { pm_name: undefined }),
    project('binh', { pm_name: 'Bình' }),
    project('anh', { pm_name: 'Ánh' }),
  ];

  assert.deepEqual(
    sortProjectExplorerRows(rows, { key: 'owner', direction: 'asc' }).map((row) => row.id),
    ['anh', 'binh', 'none'],
  );
  assert.deepEqual(
    sortProjectExplorerRows(rows, { key: 'owner', direction: 'desc' }).map((row) => row.id),
    ['binh', 'anh', 'none'],
  );
});

test('project explorer sort controls start ascending, toggle direction, and reject invalid values', () => {
  const ascending = nextProjectExplorerSort('status', null);
  assert.deepEqual(ascending, { key: 'status', direction: 'asc' });
  assert.deepEqual(nextProjectExplorerSort('status', ascending), { key: 'status', direction: 'desc' });
  assert.deepEqual(projectExplorerSortFromValue('owner:desc'), { key: 'owner', direction: 'desc' });
  assert.equal(projectExplorerSortFromValue('progress:asc'), null);
  assert.equal(projectExplorerSortFromValue('owner:sideways'), null);
});
