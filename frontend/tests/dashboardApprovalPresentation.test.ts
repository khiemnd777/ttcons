import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const dashboardSource = readFileSync(
  new URL('../src/modules/dashboard/DashboardPage.tsx', import.meta.url),
  'utf8',
);
const activitySource = readFileSync(new URL('../src/modules/dashboard/DashboardActivityPanel.tsx', import.meta.url), 'utf8');
const summarySource = readFileSync(new URL('../src/modules/dashboard/DashboardSummaryPanels.tsx', import.meta.url), 'utf8');

test('keeps Dashboard as a management summary instead of a personal action queue', () => {
  assert.match(dashboardSource, /title="Dashboard tổng hợp"/);
  assert.match(dashboardSource, /\/reports\/dashboard\?/);
  assert.doesNotMatch(dashboardSource, /\/reports\/dashboard\/projects/);
  assert.match(activitySource, /\/reports\/dashboard\/activities\?/);
  assert.doesNotMatch(dashboardSource, /\/approvals\/pending/);
  assert.doesNotMatch(dashboardSource, /ApprovalDecisionActions|ApprovalPanel|approvalCanDecide/);
  assert.doesNotMatch(dashboardSource, /title="Yêu cầu đang chờ phê duyệt"/);
});

test('activity review remains read-only and uses the shared accessible dialog', () => {
  assert.match(activitySource, /<Dialog open manageFocus/);
  assert.doesNotMatch(activitySource, /useMutation|\bpost\(|\bput\(|read-through|read-all/);
  assert.match(activitySource, /feed\.isError \? \[\]/);
  assert.match(activitySource, /\['dashboard-activities', actorID, range, search, activityType\]/);
});

test('dashboard retains operational summary and has no internal finance panel', () => {
  assert.match(dashboardSource, /<ProjectSnapshotPanel snapshot=\{dashboard.data.project_snapshot\}/);
  assert.doesNotMatch(dashboardSource + summarySource, /CashflowPanel|finance_visible|data.cashflow|Dòng tiền theo kỳ/);
});
