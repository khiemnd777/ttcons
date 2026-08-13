import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const dashboardSource = readFileSync(
  new URL('../src/modules/dashboard/DashboardPage.tsx', import.meta.url),
  'utf8',
);

test('keeps Dashboard approvals as a scoped management queue with explicit ownership context', () => {
  assert.match(dashboardSource, /get<ApprovalRequest\[\]>\('\/approvals\/pending'\)/);
  assert.doesNotMatch(dashboardSource, /\/approvals\/pending\?decidable_only=true/);
  assert.match(dashboardSource, /title="Yêu cầu đang chờ phê duyệt"/);
  assert.match(dashboardSource, />Người gửi:<\/dt>/);
  assert.match(dashboardSource, /approvalRequesterLabel\(approval\)/);
  assert.match(dashboardSource, />Chờ duyệt:<\/dt>/);
  assert.match(dashboardSource, /approvalRequiredRolesLabel\(approval\)/);
  assert.match(dashboardSource, /canDecide \? \(/);
  assert.match(dashboardSource, /Chỉ theo dõi/);
});
