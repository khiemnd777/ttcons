import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const dashboardSource = readFileSync(
  new URL('../src/modules/dashboard/DashboardPage.tsx', import.meta.url),
  'utf8',
);

test('keeps Dashboard as a management summary instead of a personal action queue', () => {
  assert.match(dashboardSource, /title="Dashboard tổng hợp"/);
  assert.match(dashboardSource, /\/reports\/dashboard\?/);
  assert.match(dashboardSource, /\/reports\/dashboard\/projects\?/);
  assert.doesNotMatch(dashboardSource, /\/approvals\/pending/);
  assert.doesNotMatch(dashboardSource, /ApprovalDecisionActions|ApprovalPanel|approvalCanDecide/);
  assert.doesNotMatch(dashboardSource, /title="Yêu cầu đang chờ phê duyệt"/);
});

test('uses the shared action-first table convention for the project portfolio', () => {
  assert.match(dashboardSource, /<DataTable/);
  assert.match(dashboardSource, /key: 'action'/);
  assert.match(dashboardSource, /action: true/);
  assert.match(dashboardSource, /renderMobileCard=/);
});

test('keeps financial rows permission-sensitive', () => {
  assert.match(dashboardSource, /capabilities\.finance_visible/);
  assert.match(dashboardSource, /project\.finance_visible/);
  assert.match(dashboardSource, /Không có quyền xem tài chính công trình này/);
});
