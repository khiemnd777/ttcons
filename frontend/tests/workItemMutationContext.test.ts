import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  withWorkItemMutationContext,
  workItemAutoStartedNotice,
} from '../src/modules/work-items/workItemMutationContext.ts';

test('domain payload opts into auto start only with an explicit work item id', () => {
  assert.deepEqual(withWorkItemMutationContext({ comment: 'ok' }, ''), { comment: 'ok' });
  assert.deepEqual(withWorkItemMutationContext({ comment: 'ok' }, ' work-1 '), {
    comment: 'ok',
    work_item_context: { work_item_id: 'work-1', intent: 'start_on_success' },
  });
});

test('transition notice is truthful and only shown when backend reports auto start', () => {
  assert.equal(workItemAutoStartedNotice({}), '');
  assert.equal(workItemAutoStartedNotice({ work_item_transition: { auto_started: false, work_item_title: 'A' } }), '');
  assert.equal(
    workItemAutoStartedNotice({ work_item_transition: { auto_started: true, work_item_title: 'Hồ sơ A' } }),
    ' Đầu việc “Hồ sơ A” đã chuyển sang Đang làm.',
  );
});

test('official module workspaces share the context bridge and domain items do not expose a separate start CTA', async () => {
  const files = await Promise.all([
    '../src/modules/boq/BOQPage.tsx',
    '../src/modules/boq/SupplierQuotationWorkspace.tsx',
    '../src/modules/contracts/ContractsPage.tsx',
    '../src/modules/procurement/ProcurementPage.tsx',
    '../src/modules/warehouse/WarehousePage.tsx',
    '../src/modules/planning/ConstructionHandoffPanel.tsx',
    '../src/modules/handover/HandoverPage.tsx',
    '../src/modules/project-finance/ProjectFinancePage.tsx',
    '../src/modules/project-finance/AccountingHandoffPage.tsx',
  ].map((path) => readFile(new URL(path, import.meta.url), 'utf8')));
  for (const source of files) {
    assert.match(source, /withWorkItemMutationContext/);
    assert.match(source, /syncWorkItemMutationTransition/);
  }

  const taskContext = await readFile(new URL('../src/modules/projects/ProjectTaskContext.tsx', import.meta.url), 'utf8');
  const detail = await readFile(new URL('../src/modules/work-items/WorkItemDetailDialog.tsx', import.meta.url), 'utf8');
  assert.match(taskContext, /Thao tác nghiệp vụ đầu tiên lưu thành công sẽ tự chuyển đầu việc sang Đang làm/);
  assert.match(detail, /isDomainExecutionTarget\(item\.execution_target\)[\s\S]*action !== 'start'/);
});
