import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(`../src/${path}`, import.meta.url), 'utf8');
}

test('shared PageHeader keeps standard density by default and exposes an opt-in compact treatment', async () => {
  const pageHeader = await source('components/PageHeader.tsx');

  assert.match(pageHeader, /density = 'standard'/);
  assert.match(pageHeader, /density\?: 'standard' \| 'compact'/);
  assert.match(pageHeader, /data-page-header-density=\{density\}/);
  assert.match(pageHeader, /compact \? 'mb-3 gap-2 pb-2' : 'mb-5 gap-3 pb-5'/);
  assert.match(pageHeader, /compact \? 'max-w-full truncate text-lg/);
  assert.match(pageHeader, /line-clamp-2 min-w-0 text-xs/);
});

test('every project-scoped business page opts into compact page chrome while shared global pages stay standard', async () => {
  const staticScopedPages = [
    'modules/planning/PreparationPage.tsx',
    'modules/planning/PlanningPage.tsx',
    'modules/boq/BOQPage.tsx',
    'modules/contracts/ContractsPage.tsx',
  ];

  for (const path of staticScopedPages) {
    assert.match(await source(path), /density="compact"/, `${path} must use compact page chrome`);
  }

  assert.match(await source('modules/acceptance/AcceptancePage.tsx'), /density=\{global \? 'standard' : 'compact'\}/);
  assert.match(await source('modules/audit/AuditPage.tsx'), /density=\{scoped \? 'compact' : 'standard'\}/);
  assert.match(await source('modules/handover/HandoverPage.tsx'), /density=\{id \? 'compact' : 'standard'\}/);
  assert.match(await source('modules/procurement/ProcurementPage.tsx'), /density=\{scoped \? 'compact' : 'standard'\}/);
  assert.match(await source('modules/operations/OperationsPage.tsx'), /density=\{global \? 'standard' : 'compact'\}/);

  const warehouse = await source('modules/warehouse/WarehousePage.tsx');
  assert.equal(warehouse.match(/density=\{scoped \? 'compact' : 'standard'\}/g)?.length, 2);

  for (const path of [
    'modules/project-finance/AccountingHandoffPage.tsx',
    'modules/project-finance/ProjectFinancePage.tsx',
  ]) {
    const finance = await source(path);
    assert.match(finance, /global \? <header className="border-b border-line pb-5">/);
    assert.match(finance, /<PageHeader\s+density="compact"/);
  }
});

test('project workspace shell remains the visible project identity owner including sample status', async () => {
  const workspace = await source('modules/projects/ProjectWorkspaceLayout.tsx');

  assert.match(workspace, /\{project \? `\$\{project\.code\} · \$\{project\.name\}`/);
  assert.match(workspace, /project\?\.client_name \? `Chủ đầu tư:/);
  assert.match(workspace, /project\?\.status \? `Trạng thái:/);
  assert.match(workspace, /project\?\.is_sample \? <SampleDataBadge compact \/>/);
});
