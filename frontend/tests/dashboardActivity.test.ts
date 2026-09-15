import assert from 'node:assert/strict';
import test from 'node:test';
import { dashboardActivityDate, dashboardActivityDay, dashboardActivityGroups, dashboardActivityHref, dashboardActivityQuery } from '../src/modules/dashboard/dashboardActivity.ts';
import type { DashboardActivity } from '../src/modules/dashboard/dashboardTypes.ts';

test('activity filters are independent of the cashflow period and preserve Vietnamese search', () => {
  const params = new URLSearchParams(dashboardActivityQuery('7d', '  Khảo sát & hồ sơ  ', 'started', 'cursor-value'));
  assert.equal(params.get('search'), 'Khảo sát & hồ sơ');
  assert.equal(params.get('range'), '7d');
  assert.equal(params.get('limit'), '20');
  assert.equal(params.get('cursor'), 'cursor-value');
  assert.equal(params.has('period_type'), false);
});

test('day groups use Vietnamese midnight even when the browser date is a different UTC day', () => {
  assert.equal(dashboardActivityDay('2026-09-05T17:00:00Z'), '06-09-2026');
  assert.equal(dashboardActivityDay('2026-09-05T16:59:59Z'), '05-09-2026');
  assert.equal(dashboardActivityDate('2026-09-03T07:26:55Z'), '14:26 03-09-2026');
  assert.equal(dashboardActivityDay('invalid'), 'Không rõ ngày');
  const a = { id: 'a', created_at: '2026-09-05T17:00:00Z' } as DashboardActivity;
  const b = { id: 'b', created_at: '2026-09-05T16:59:59Z' } as DashboardActivity;
  assert.deepEqual(dashboardActivityGroups([a, a, b]).map((g) => [g.label, g.items.length]), [['06-09-2026', 1], ['05-09-2026', 1]]);
});

test('detail navigation only accepts internal project and work item identifiers', () => {
  const project = '10000000-0000-0000-0000-000000000001';
  const work = '20000000-0000-0000-0000-000000000001';
  const item = { project_id: project, work_item_id: work } as DashboardActivity;
  assert.equal(dashboardActivityHref(item), `/projects/${project}/work-items/${work}`);
  assert.equal(dashboardActivityHref({ ...item, work_item_id: '' }), `/projects/${project}/discussion`);
  assert.equal(dashboardActivityHref({ ...item, project_id: '//external' }), '');
});
