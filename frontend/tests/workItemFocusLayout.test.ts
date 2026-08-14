import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('work-item form actions are owned by the sticky right action rail', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const primarySurface = await source('../src/modules/work-items/WorkItemPrimarySurface.tsx');
  const formPanel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(detail, /xl:grid-cols-\[minmax\(0,1fr\)_18rem\]/);
  assert.match(detail, /xl:sticky xl:top-\[5\.5rem\]/);
  assert.match(detail, /data-workflow-form-submit/);
  assert.match(detail, /onSubmitWorkflowForm/);
  assert.match(detail, /onSaveWorkflowForm/);
  assert.match(detail, />Thao tác khác</);
  assert.match(detail, /noteOpen \? 'Ẩn ghi chú'/);

  assert.match(primarySurface, /externalActions/);
  assert.match(primarySurface, /actionRef=\{workflowFormActionRef\}/);
  assert.match(formPanel, /useImperativeHandle\(actionRef/);
  assert.match(formPanel, /showActions=\{!externalActions\}/);
});

test('work-item secondary content uses progressive disclosure', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const formPanel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');
  const projectWorkspace = await source('../src/modules/projects/ProjectWorkspaceLayout.tsx');
  const detailPage = await source('../src/modules/work-items/WorkItemDetailPage.tsx');

  assert.match(detail, /<details className=\{clsx\('group rounded-md border bg-white shadow-panel'/);
  assert.match(detail, /Trao đổi & lịch sử/);
  assert.doesNotMatch(detailPage, /<PageHeader/);
  assert.doesNotMatch(detailPage, /Quay lại/);

  assert.match(formPanel, /setFieldsOpen\(!submitted\)/);
  assert.match(formPanel, /setDocumentsOpen\(submitted\)/);
  assert.match(formPanel, /Mở sau khi chốt hồ sơ để ký, đóng dấu hoặc tạo PDF/);
  assert.match(formPanel, /<details className="group rounded-md border border-line bg-white">/);

  assert.match(projectWorkspace, /\{nodes\.map\(\(node, index\) => \(/);
  assert.doesNotMatch(projectWorkspace, /visibleNodeEntries/);
  assert.doesNotMatch(projectWorkspace, /showAllNodes/);
  assert.doesNotMatch(projectWorkspace, /Xem toàn bộ quy trình/);
  assert.doesNotMatch(projectWorkspace, /Thu gọn về giai đoạn hiện tại/);
});

test('upcoming workflow forms keep the primary layout while remaining read-only', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const primarySurface = await source('../src/modules/work-items/WorkItemPrimarySurface.tsx');

  assert.match(detail, /const resolvedPrimarySurface = detailItem \? resolveWorkItemPrimarySurface\(detailItem\) : null/);
  assert.match(detail, /futureReadOnly && resolvedPrimarySurface\?\.kind !== 'workflow_form'/);
  assert.match(detail, /futureReadOnly && !primarySurface/);
  assert.match(detail, /readOnly=\{futureReadOnly\}/);
  assert.doesNotMatch(detail, /hasWorkflowFormSummary\(item\) && \(readOnly \|\| !primarySurfaceKind/);
  assert.match(detail, /hasWorkflowFormSummary\(item\) && \(!primarySurfaceKind \|\| !primarySurfaceUsesWorkflowForm\(primarySurfaceKind\)\)/);

  assert.match(primarySurface, /readOnly = false/);
  assert.match(primarySurface, /readOnly=\{readOnly\}/);
  assert.match(primarySurface, /externalActions/);
});

test('every form-backed work item uses the shared primary form before domain surface routing', async () => {
  const primarySurface = await source('../src/modules/work-items/WorkItemPrimarySurface.tsx');
  const formPriority = primarySurface.indexOf('const hasWorkflowForm = hasWorkflowFormSummary(item);');
  const configuredPriority = primarySurface.indexOf('const configuredSurface = configuredPrimarySurface(item);');

  assert.notEqual(formPriority, -1);
  assert.notEqual(configuredPriority, -1);
  assert.ok(formPriority < configuredPriority);
  assert.match(
    primarySurface,
    /if \(hasWorkflowForm\) \{\s+return \{\s+kind: 'workflow_form',[\s\S]*?\};\s+\}\s+\n\s+const configuredSurface/,
  );

  for (const kind of ['approval', 'contract', 'quality', 'finance', 'handover', 'incoming_acceptance', 'outgoing_acceptance', 'owner_client_reconciliation', 'subcontractor_reconciliation']) {
    assert.match(primarySurface, new RegExp(`${kind}: \\{`));
  }
});
