import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import {
  mergeProjectProgressPages,
  projectProgressFilterQuery,
  projectProgressStageQuery,
  projectProgressUpdateIDs,
} from '../src/modules/projects/projectProgressBoard.ts';
import type { ProjectProgressItem, ProjectProgressStagePage } from '../src/lib/types/domain.ts';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('project progress query parameters are server-side and cursor-safe', () => {
  const filter = projectProgressFilterQuery({
    search: '  Nhà phố Thủ Đức  ',
    pmStaffId: '30000000-0000-0000-0000-000000000002',
    status: 'planning',
  });
  const params = new URLSearchParams(projectProgressStageQuery(filter, 'opaque-cursor'));

  assert.equal(params.get('search'), 'Nhà phố Thủ Đức');
  assert.equal(params.get('pm_staff_id'), '30000000-0000-0000-0000-000000000002');
  assert.equal(params.get('status'), 'planning');
  assert.equal(params.get('limit'), '25');
  assert.equal(params.get('cursor'), 'opaque-cursor');
});

test('project progress page merging de-duplicates moving rows across cursor pages', () => {
  const pages = [
    { items: [{ id: 'project-1', name: 'A' }, { id: 'project-2', name: 'B' }] },
    { items: [{ id: 'project-2', name: 'B updated' }, { id: 'project-3', name: 'C' }] },
  ] as ProjectProgressStagePage[];

  assert.deepEqual(
    mergeProjectProgressPages(pages).map((item) => [item.id, item.name]),
    [['project-1', 'A'], ['project-2', 'B updated'], ['project-3', 'C']],
  );
});

test('deeply paged boards request updates only for the virtual window in near columns', () => {
  const items = Array.from({ length: 250 }, (_, index) => ({ id: `project-${index}` })) as ProjectProgressItem[];
  const window = [{ index: 202 }, { index: 203 }, { index: 204 }, { index: 203 }, { index: 300 }];
  assert.deepEqual(projectProgressUpdateIDs(items, window, true), ['project-202', 'project-203', 'project-204']);
  assert.deepEqual(projectProgressUpdateIDs(items, window, false), []);
  assert.deepEqual(projectProgressUpdateIDs(items, [], true), []);
  assert.deepEqual(projectProgressUpdateIDs([], window, true), []);
});

test('project progress UI uses actor-scoped infinite queries, lazy columns, virtualization, and safe SSE refresh', async () => {
  const page = await source('../src/modules/projects/ProjectProgressPage.tsx');
  const packageJSON = await source('../package.json');

  assert.match(page, /project-progress-board-summary/);
  assert.match(page, /project-progress-board-stage/);
  assert.match(page, /actorQueryKey/);
  assert.match(page, /useInfiniteQuery/);
  assert.match(page, /IntersectionObserver/);
  assert.match(page, /rootMargin: '0px 600px 0px 600px'/);
  assert.match(page, /useVirtualizer/);
  assert.match(page, /Tải thêm 25/);
  assert.match(page, /data\?\.pages\.length \?\? 0\) > 1/);
  assert.match(page, /Có dữ liệu mới/);
  assert.match(page, /resetQueries/);
  assert.match(page, /refetchType: 'active'/);
  assert.match(packageJSON, /"@tanstack\/react-virtual"/);
});
