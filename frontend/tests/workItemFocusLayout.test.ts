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
  const requirementPanel = await source('../src/modules/work-items/WorkItemRequirementPanel.tsx');
  const formPanel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');
  const projectWorkspace = await source('../src/modules/projects/ProjectWorkspaceLayout.tsx');
  const detailPage = await source('../src/modules/work-items/WorkItemDetailPage.tsx');

  assert.match(detail, /<WorkItemRequirementPanel rows=\{requirementRows\}/);
  assert.match(requirementPanel, /<details[\s\S]*className=\{clsx\('group rounded-md border bg-white shadow-panel'/);
  assert.match(detail, /Trao đổi & lịch sử/);
  assert.doesNotMatch(detailPage, /<PageHeader/);
  assert.doesNotMatch(detailPage, /Quay lại/);

  assert.match(formPanel, /setFieldsOpen\(!submitted\)/);
  assert.match(formPanel, /setDocumentsOpen\(submitted\)/);
  assert.match(formPanel, /Hoàn tất bước 1 để ký, đóng dấu hoặc tạo PDF/);
  assert.doesNotMatch(formPanel, /form\.data\.events\?\.length \? <WorkflowFormEventsPanel/);

  assert.match(projectWorkspace, /\{nodes\.map\(\(node, index\) => \(/);
  assert.doesNotMatch(projectWorkspace, /visibleNodeEntries/);
  assert.doesNotMatch(projectWorkspace, /showAllNodes/);
  assert.doesNotMatch(projectWorkspace, /Xem toàn bộ quy trình/);
  assert.doesNotMatch(projectWorkspace, /Thu gọn về giai đoạn hiện tại/);
});

test('primary evidence has one visual section while the supporting tab stays embedded', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const primarySurface = await source('../src/modules/work-items/WorkItemPrimarySurface.tsx');
  const evidencePanel = await source('../src/modules/work-items/EvidencePanel.tsx');

  assert.match(primarySurface, /case 'evidence':[\s\S]*?<EvidencePanel[^>]*variant="primary"/);
  assert.match(detail, /activeTab === 'evidence' \? <EvidencePanel[^>]*variant="embedded"/);
  assert.match(evidencePanel, /variant = 'embedded'/);
  assert.match(
    evidencePanel,
    /variant === 'primary'[\s\S]*?'rounded-lg border border-line bg-white p-4 shadow-panel'[\s\S]*?: 'rounded-md'/,
  );
});

test('evidence leads the supporting tabs without interrupting activity or form history', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const formPanel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(detail, /useState<SupportTabKey>\('evidence'\)/);
  assert.match(detail, /setSelectedTab\('evidence'\);[\s\S]*\[item\.id\]/);
  assert.match(detail, /type ActivityHistoryFilter = 'all' \| 'discussion' \| 'form'/);
  assert.match(detail, /\{ id: 'all', label: 'Tất cả'/);
  assert.match(detail, /\{ id: 'discussion', label: 'Trao đổi'/);
  assert.match(detail, /filters\.push\(\{ id: 'form', label: 'Hồ sơ'/);
  assert.match(detail, /<WorkflowFormEventsPanel events=\{workflowFormEvents\}/);

  const evidenceTabIndex = detail.indexOf("id: 'evidence'");
  const activityTabIndex = detail.indexOf("id: 'activity'", evidenceTabIndex);
  assert.ok(activityTabIndex !== -1 && evidenceTabIndex !== -1);
  assert.ok(evidenceTabIndex < activityTabIndex);

  const processIndex = formPanel.indexOf('data-workflow-process="two-step"');
  const eventPanelIndex = formPanel.indexOf('export function WorkflowFormEventsPanel');
  assert.ok(processIndex !== -1 && eventPanelIndex !== -1);
  assert.ok(processIndex < eventPanelIndex);
  assert.doesNotMatch(formPanel.slice(processIndex, eventPanelIndex), /WorkflowFormEventsPanel/);
});

test('workflow form uses one integrated vertical process card for its real sections', async () => {
  const formPanel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(formPanel, /<ol[\s\S]*data-workflow-process="two-step"[\s\S]*overflow-hidden rounded-lg border/);
  assert.match(formPanel, /<li[\s\S]*data-workflow-step="content"[\s\S]*aria-current=\{!submitted \|\| amending \? 'step' : undefined\}/);
  assert.match(formPanel, /<li[\s\S]*data-workflow-step="documents"[\s\S]*aria-current=\{submitted && !amending \? 'step' : undefined\}/);
  assert.match(formPanel, /data-workflow-step-rail/);
  assert.match(formPanel, /data-workflow-process-line="content-to-documents"/);
  assert.doesNotMatch(formPanel, /ArrowDown/);
  assert.match(formPanel, /grid-cols-\[2\.25rem_minmax\(0,1fr\)\][\s\S]*sm:grid-cols-\[2\.75rem_minmax\(0,1fr\)\]/);
  assert.doesNotMatch(formPanel, /className="flex h-8 items-center pl-\[1\.125rem\]"/);
  assert.doesNotMatch(formPanel, /WorkflowFormSequence/);
  assert.doesNotMatch(formPanel, /ArrowRight/);
  assert.match(formPanel, /disabled=\{!submitted \|\| amending\}/);
  assert.match(formPanel, /Hoàn tất bước 1 để ký, đóng dấu hoặc tạo PDF\./);
  assert.match(formPanel, /Đã chốt/);
  assert.match(formPanel, /validationIssues\.length[\s\S]*cần xử lý/);

  const step1Index = formPanel.indexOf('aria-hidden="true">1</span>');
  const processLineIndex = formPanel.indexOf('data-workflow-process-line="content-to-documents"');
  const step2Index = formPanel.indexOf('aria-hidden="true">2</span>');
  assert.ok(step1Index !== -1 && processLineIndex !== -1 && step2Index !== -1);
  assert.ok(step1Index < processLineIndex && processLineIndex < step2Index);
});

test('upcoming workflow forms keep the primary layout while remaining read-only', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const primarySurface = await source('../src/modules/work-items/WorkItemPrimarySurface.tsx');

  assert.match(detail, /const resolvedPrimarySurface = detailItem \? resolveWorkItemPrimarySurface\(detailItem\) : null/);
  assert.match(detail, /futureReadOnly && resolvedPrimarySurface\?\.kind !== 'workflow_form'/);
  assert.match(detail, /futureReadOnly && !primarySurface/);
  assert.match(detail, /const panelsReadOnly = readOnly \|\| futureReadOnly/);
  assert.match(detail, /readOnly=\{panelsReadOnly\}/);
  assert.doesNotMatch(detail, /hasWorkflowFormSummary\(item\) && \(readOnly \|\| !primarySurfaceKind/);
  assert.match(detail, /hasWorkflowForm && \(!primarySurfaceKind \|\| !primarySurfaceUsesWorkflowForm\(primarySurfaceKind\)\)/);

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
