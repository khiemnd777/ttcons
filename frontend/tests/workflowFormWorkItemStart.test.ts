import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

async function workItemDetailSource() {
  const [page, domain] = await Promise.all([
    source('../src/modules/work-items/WorkItemDetailDialog.tsx'),
    source('../src/modules/work-items/workItemDetailDomain.ts'),
  ]);
  return `${page}\n${domain}`;
}

test('workflow form first-write actions state that they also start the work item', async () => {
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');
  const detail = await workItemDetailSource();

  assert.match(panel, /startsWorkItem: boolean/);
  assert.match(panel, /saveLabel: string/);
  assert.match(panel, /Đã bắt đầu đầu việc và lưu nháp hồ sơ nghiệp vụ/);
  assert.match(panel, /Đã bắt đầu đầu việc và chốt hồ sơ nghiệp vụ/);
  assert.match(detail, /state\?\.saveLabel \?\? 'Lưu nháp'/);
  assert.match(detail, /permittedActions\(detailItem, workflowFormActionState\?\.submitted\)/);
  assert.match(detail, /statusActions = statusActions\.filter\(\(action\) => action !== 'start'\)/);
});

test('submitted construction handoff form opens the domain action before explicit completion', async () => {
  const detail = await workItemDetailSource();
  const handoff = await source('../src/modules/planning/ConstructionHandoffPanel.tsx');

  assert.match(detail, /item\.domain_action_key === 'construction\.activate_handoff'/);
  assert.match(detail, /Mở bàn giao thi công/);
  assert.match(detail, /Kiểm tra điều kiện & chốt bước/);
  assert.match(detail, /constructionHandoffNavigationState\(detailItem\.project_id, location\.state\)/);
  assert.match(handoff, /constructionHandoffIntentMatches\(location\.state, projectId\)/);
  assert.match(handoff, /consumeConstructionHandoffNavigationState\(location\.state\)/);
  assert.match(handoff, /setItems\(\(handoff\.data\.candidates \?\? \[\]\)\.map\(editableCandidate\)\)/);
  assert.match(handoff, /queryClient\.invalidateQueries\(\{ queryKey: \['work-item'\] \}\)/);
});
