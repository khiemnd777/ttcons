import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('future work keeps the warning and presents start requirements before completion requirements', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const startPanel = detail.indexOf('<WorkItemStartRequirementPanel item={detailItem} />');
  const completionPanel = detail.indexOf('<WorkItemRequirementPanel rows={requirementRows}');

  assert.notEqual(startPanel, -1);
  assert.notEqual(completionPanel, -1);
  assert.ok(startPanel < completionPanel);
  assert.match(detail, /Công việc chưa đến lượt · Chỉ xem/);
  assert.match(detail, /Điều kiện để bắt đầu/);
  assert.match(detail, /open={unmetCount > 0}/);
  assert.match(detail, /requirement\.can_open && requirement\.work_item_id/);
});

test('early start uses an explicit audited confirmation instead of unlocking all future mutations', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');

  assert.match(detail, /action !== 'start' \|\| !detailItem\.start_readiness\?\.can_bypass/);
  assert.match(detail, /bypass_start_requirements: bypassStartRequirements/);
  assert.match(detail, /start_requirements_token: startRequirementsToken/);
  assert.match(detail, /Lý do bắt đầu sớm/);
  assert.match(detail, /Bắt đầu ngay/);
  assert.match(detail, /Đang thực hiện sớm · Workflow chưa chuyển đến node này/);
  assert.match(detail, /Tiến độ workflow chính không bị dịch chuyển/);
});

test('work-item domain contract keeps start readiness separate from completion requirements', async () => {
  const domain = await source('../src/lib/types/domain.ts');

  assert.match(domain, /start_readiness\?: WorkItemStartReadiness/);
  assert.match(domain, /export type WorkItemStartReadiness/);
  assert.match(domain, /requirements: WorkItemStartRequirement\[\]/);
  assert.match(domain, /completion_requirements\?: WorkItemCompletionRequirement\[\]/);
});
