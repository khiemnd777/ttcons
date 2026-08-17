import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import type { WorkItem } from '../src/lib/types/domain.ts';
import { groupPreparationItems } from '../src/modules/planning/preparationGrouping.ts';
import { buildWorkItemWorkflowNavigation } from '../src/modules/work-items/workItemWorkflowNavigation.ts';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('Preparation defaults to a full-width action queue without the rejected card rail', async () => {
  const page = await source('../src/modules/planning/PreparationPage.tsx');

  assert.match(page, /useState<PreparationFilter>\('need_action'\)/);
  assert.match(page, /data-preparation-mode="queue"/);
  assert.match(page, /aria-label="Danh sách hồ sơ chuẩn bị"/);
  assert.match(page, /Tìm tên hồ sơ, bước, người phụ trách/);
  assert.match(page, /data-preparation-desktop-table/);
  assert.match(page, /data-preparation-mobile-list/);
  assert.match(page, /className="hidden xl:block" data-preparation-desktop-table/);
  assert.match(page, /className="xl:hidden" data-preparation-mobile-list/);
  assert.match(page, /data-preparation-stage-group/);
  assert.match(page, /aria-expanded=\{!collapsed\}/);
  assert.match(page, /Giai đoạn \$\{group\.name\}/);
  assert.doesNotMatch(page, /xl:grid-cols-\[19rem_minmax\(0,1fr\)\]/);
  assert.doesNotMatch(page, /<WorkItemCard/);
  assert.doesNotMatch(page, /Chọn một hồ sơ để bắt đầu xử lý/);
});

test('Preparation stage groups preserve workflow order and expose names without stage ordinals', () => {
  const items = [
    { id: 'survey-1', run_node_id: 'survey', run_node_name: 'Khảo sát', run_node_order_index: 2, title: 'Khảo sát yêu cầu' },
    { id: 'survey-2', run_node_id: 'survey', run_node_name: 'Khảo sát', run_node_order_index: 2, title: 'Chụp ảnh hiện trạng' },
    { id: 'meeting-1', run_node_id: 'meeting', run_node_name: 'Họp báo cáo khảo sát', run_node_order_index: 3, title: 'Bóc khối lượng' },
  ] as WorkItem[];

  const groups = groupPreparationItems(items);

  assert.deepEqual(groups.map((group) => ({ name: group.name, ids: group.items.map((item) => item.id) })), [
    { name: 'Khảo sát', ids: ['survey-1', 'survey-2'] },
    { name: 'Họp báo cáo khảo sát', ids: ['meeting-1'] },
  ]);
  assert.ok(groups.every((group) => !/^\d+\./.test(group.name)));
});

test('Preparation detail derives compact previous and next context only inside the current stage', () => {
  const items = [
    { id: 'survey-3', pipeline_run_id: 'run-1', run_node_id: 'survey', run_node_name: 'Khảo sát', run_step_name: 'Nắm phong cách thiết kế', run_step_order_index: 3, title: 'Nắm phong cách thiết kế' },
    { id: 'meeting-1', pipeline_run_id: 'run-1', run_node_id: 'meeting', run_node_name: 'Họp báo cáo khảo sát', run_step_name: 'Bóc khối lượng', run_step_order_index: 1, title: 'Bóc khối lượng' },
    { id: 'survey-1', pipeline_run_id: 'run-1', run_node_id: 'survey', run_node_name: 'Khảo sát', run_step_name: 'Khảo sát yêu cầu của chủ đầu tư', run_step_order_index: 1, title: 'Khảo sát yêu cầu của chủ đầu tư' },
    { id: 'survey-2', pipeline_run_id: 'run-1', run_node_id: 'survey', run_node_name: 'Khảo sát', run_step_name: 'Chụp và tải ảnh hiện trạng', run_step_order_index: 2, title: 'Chụp và tải ảnh hiện trạng' },
  ] as WorkItem[];

  const context = buildWorkItemWorkflowNavigation(items, 'survey-2');
  assert.deepEqual({
    stageName: context?.stageName,
    position: context?.position,
    total: context?.total,
    previous: context?.previous?.id,
    next: context?.next?.id,
  }, {
    stageName: 'Khảo sát',
    position: 2,
    total: 3,
    previous: 'survey-1',
    next: 'survey-3',
  });

  const first = buildWorkItemWorkflowNavigation(items, 'survey-1');
  assert.equal(first?.previous, undefined);
  assert.equal(first?.next?.id, 'survey-2');
  assert.equal(buildWorkItemWorkflowNavigation(items, 'meeting-1')?.next, undefined);
});

test('Preparation detail replaces the queue and gives the existing work-item workspace one canvas', async () => {
  const page = await source('../src/modules/planning/PreparationPage.tsx');
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');

  const detailMode = page.indexOf('if (taskId)');
  const queueMode = page.indexOf('data-preparation-mode="queue"');
  assert.ok(detailMode !== -1 && queueMode !== -1 && detailMode < queueMode);
  assert.match(page, /data-preparation-mode="detail"/);
  assert.match(page, /returnTo \? 'Quay lại' : 'Danh sách hồ sơ'/);
  assert.match(page, /<WorkItemDetailWorkspace/);
  assert.match(page, /workflowNavigation=\{workflowNavigation\}/);
  assert.match(page, /window\.scrollTo\(\{ top: 0, behavior: 'auto' \}\)/);
  assert.match(page, /\}, \[taskId\]\);/);
  assert.equal((page.match(/<WorkItemDetailWorkspace/g) ?? []).length, 1);
  assert.match(detail, /data-work-item-workflow-context/);
  assert.match(detail, /aria-label="Vị trí trong giai đoạn"/);
  assert.match(detail, /min-h-11[\s\S]*sm:flex-row/);
  assert.match(detail, /Bước \{navigation\.position\}\/\{navigation\.total\}/);
  assert.match(detail, /!workflowNavigation \? <p[\s\S]*context\.node/);
});

test('project business tree highlights only the active child inside expanded groups', async () => {
  const workspace = await source('../src/modules/projects/ProjectWorkspaceLayout.tsx');

  assert.match(workspace, /group\.children\.length[\s\S]*\? 'text-slate-700 hover:bg-slate-50 hover:text-ink'/);
  assert.match(workspace, /aria-current=\{active \? 'page' : undefined\}/);
  assert.match(workspace, /active \? 'bg-blue-50 text-blue-700'/);
  assert.doesNotMatch(workspace, /groupActive \? 'bg-blue-50 text-blue-700' : 'text-slate-700/);
});
