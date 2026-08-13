import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('workflow form panel describes submit as locking an internal version', async () => {
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(panel, /form\.data\.auto_advance_when_ready \? 'Xác nhận và tiếp tục' : 'Chốt hồ sơ'/);
  assert.match(panel, /toast\.success\('Đã chốt hồ sơ nghiệp vụ'\)/);
  assert.match(panel, /Nội dung đã khóa và được lưu vào đầu việc; chưa gửi cho người duyệt\./);
  assert.doesNotMatch(panel, /'Gửi hồ sơ'/);
});

test('workflow form summaries use the contextual status helper', async () => {
  const presentation = await source('../src/modules/work-items/workItemPresentation.ts');
  const status = await source('../src/lib/utils/status.ts');
  const projectWorkspace = await source('../src/modules/projects/ProjectWorkspaceLayout.tsx');
  const workItemCard = await source('../src/modules/work-items/WorkItemCard.tsx');
  const workItemDetail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const activityThread = await source('../src/modules/work-items/ActivityThread.tsx');
  const planningPage = await source('../src/modules/planning/PlanningPage.tsx');

  assert.match(presentation, /if \(status === 'submitted'\) return 'Đã chốt'/);
  assert.match(presentation, /if \(value\?\.workflow_form_status !== 'submitted'\) return 'Chốt hồ sơ nghiệp vụ'/);
  assert.match(presentation, /submitted: 'Đã chốt hồ sơ'/);
  assert.match(presentation, /comment === 'Đã gửi biểu mẫu quy trình'/);
  assert.match(presentation, /return 'Đã chốt hồ sơ nghiệp vụ'/);
  assert.match(status, /submitted: 'Đã gửi'/);
  assert.match(status, /workflow_form_submitted: 'Đã chốt hồ sơ nghiệp vụ'/);
  assert.match(projectWorkspace, /Hồ sơ \{workflowFormStatusLabel\(step\.workflow_form_status\)\}/);
  assert.match(workItemCard, /workflowFormStatusLabel\(item\.workflow_form_status\)/);
  assert.match(workItemDetail, /workflowFormChipText\(item\).*workflowFormStatusLabel\(item\.workflow_form_status\)/s);
  assert.match(workItemDetail, /workflowFormActivityComment\(activity\)/);
  assert.match(activityThread, /workflowFormActivityComment\(item\)/);
  assert.match(planningPage, /PlanningMetric label="Hồ sơ đã chốt"/);
});
