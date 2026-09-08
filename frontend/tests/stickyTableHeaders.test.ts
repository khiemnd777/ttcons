import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('semantic table headers use one page- and dialog-aware sticky convention', async () => {
  const [styles, dialogStyles, table] = await Promise.all([
    source('../src/index.css'),
    source('../src/components/dialog.css'),
    source('../src/components/DataTable.tsx'),
  ]);

  assert.match(styles, /\.consflow-table-viewport\s*\{\s*max-height: min\(64dvh, 42rem\);\s*overflow: auto;/s);
  assert.match(styles, /\.consflow-table-viewport > table > thead\s*\{\s*--consflow-table-header-background: #f8fafc;/s);
  assert.match(styles, /thead\.bg-slate-100\s*\{\s*--consflow-table-header-background: #f1f5f9;/s);
  assert.match(styles, /thead\.bg-white\s*\{\s*--consflow-table-header-background: #ffffff;/s);
  assert.match(styles, /\.consflow-table-viewport > table > thead > tr > th\s*\{\s*position: sticky;\s*top: 0;/s);
  assert.doesNotMatch(styles, /main table > thead/);
  assert.match(styles, /background-color: var\(--consflow-table-header-background\);/);
  assert.doesNotMatch(styles, /background-color: inherit/);
  assert.match(dialogStyles, /\.dialog-body\s*\{\s*--consflow-table-sticky-top: 0px;/s);
  assert.match(table, /consflow-table-viewport/);
});

test('every current semantic table has the shared local scroll viewport', async () => {
  const tableOwners = [
    '../src/components/BulkDeleteDialog.tsx',
    '../src/components/DataTable.tsx',
    '../src/components/RecordDetailLayout.tsx',
    '../src/modules/acceptance/AcceptancePage.tsx',
    '../src/modules/accounting/ManualJournalDialog.tsx',
    '../src/modules/boq/SupplierQuotationWorkspace.tsx',
    '../src/modules/contracts/ContractsPage.tsx',
    '../src/modules/handover/HandoverWorkspace.tsx',
    '../src/modules/operations/ConstructionReportFormDialog.tsx',
    '../src/modules/operations/ConstructionWorkspaceLists.tsx',
    '../src/modules/planning/PlanningPage.tsx',
    '../src/modules/projects/ProjectDetailPage.tsx',
    '../src/modules/projects/ProjectImportDialog.tsx',
    '../src/modules/projects/ProjectWorkTab.tsx',
    '../src/modules/quality/QualityPage.tsx',
    '../src/modules/settings/SettingsBackupPanel.tsx',
    '../src/modules/staff/StaffDeleteDialog.tsx',
    '../src/modules/staff/StaffImportDialog.tsx',
    '../src/modules/staff/StaffPage.tsx',
    '../src/modules/warehouse/WarehousePage.tsx',
    '../src/modules/work-items/WorkflowFormStructuredList.tsx',
    '../src/modules/workflow-definitions/WorkflowDefinitionImportDialog.tsx',
    '../src/modules/workflow-forms/WorkflowFormsPage.tsx',
  ];
  const ownerSources = await Promise.all(tableOwners.map(source));
  const tableCount = ownerSources.reduce((count, owner) => count + (owner.match(/<table\b/g) ?? []).length, 0);
  const viewportCount = ownerSources.reduce((count, owner) => count + (owner.match(/consflow-table-viewport(?!-)/g) ?? []).length, 0);

  assert.equal(tableCount, 27);
  assert.equal(viewportCount, tableCount);
  assert.equal(ownerSources.some((owner) => /<thead[^>]*\bsticky\b/.test(owner)), false);
});

test('verified CSS-grid tables use the shared sticky header class', async () => {
  const [projects, evidence, projectDetail] = await Promise.all([
    source('../src/modules/projects/ProjectsPage.tsx'),
    source('../src/modules/work-items/EvidenceGallery.tsx'),
    source('../src/modules/projects/ProjectDetailPage.tsx'),
  ]);

  assert.match(projects, /consflow-sticky-grid-header hidden \$\{gridClass\}/);
  assert.equal((evidence.match(/consflow-sticky-grid-header/g) ?? []).length, 2);
  assert.equal((projectDetail.match(/consflow-sticky-grid-header/g) ?? []).length, 2);
});
