import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(`../src/${path}`, import.meta.url), 'utf8');
}

test('shared PageHeader keeps the compact operational hierarchy used by project workspaces', async () => {
  const pageHeader = await source('components/PageHeader.tsx');

  assert.match(pageHeader, /mb-3 flex gap-2 border-b border-line pb-2[\s\S]*compactMobile \? 'flex-row items-center justify-between' : 'flex-col'/);
  assert.match(pageHeader, /max-w-full truncate text-lg font-semibold leading-6/);
  assert.match(pageHeader, /compactMobile \? 'hidden md:block' : 'line-clamp-2'[\s\S]*min-w-0 text-xs/);
  assert.match(pageHeader, /md:flex md:items-baseline md:gap-2/);
  assert.doesNotMatch(pageHeader, /density|text-2xl font-semibold tracking-normal text-ink/);
});

test('global and project business pages share the same compact page chrome', async () => {
  const pageHeaderConsumers = [
    'modules/planning/PreparationPage.tsx',
    'modules/planning/PlanningPage.tsx',
    'modules/boq/BOQPage.tsx',
    'modules/contracts/ContractsPage.tsx',
    'modules/my-work/MyWorkPage.tsx',
    'modules/projects/ProjectsPage.tsx',
    'modules/dashboard/DashboardPage.tsx',
    'modules/core-data/CoreDataPage.tsx',
    'modules/acceptance/AcceptancePage.tsx',
    'modules/audit/AuditPage.tsx',
    'modules/handover/HandoverPage.tsx',
    'modules/operations/OperationsPage.tsx',
    'modules/procurement/ProcurementPage.tsx',
    'modules/warehouse/WarehousePage.tsx',
  ];

  for (const path of pageHeaderConsumers) {
    const page = await source(path);
    assert.match(page, /<PageHeader/, `${path} must reuse PageHeader`);
    assert.doesNotMatch(page, /<PageHeader[\s\S]{0,160}density=/, `${path} must not restore a large header variant`);
  }

  for (const path of [
    'modules/project-finance/AccountingHandoffPage.tsx',
  ]) {
    const finance = await source(path);
    assert.match(finance, /<PageHeader/);
    assert.match(finance, /horizontalFrom=\{global \? 'xl' : 'md'\}/);
    assert.doesNotMatch(finance, /global \? <header className="border-b border-line pb-5">/);
  }

  const reports = await source('modules/reports/ReportsPage.tsx');
  assert.match(reports, /<PageHeader[\s\S]*title="Thống kê"/);
  assert.doesNotMatch(reports, /text-xl font-semibold tracking-normal text-ink md:text-2xl">Thống kê/);

  const workReports = await source('modules/work-reports/WorkReportsPage.tsx');
  assert.match(workReports, /mb-3 border-b border-line pb-2/);
  assert.match(workReports, /text-lg font-semibold leading-6 tracking-normal text-ink/);
  assert.match(workReports, /line-clamp-2 min-w-0 text-xs/);
});

test('project workspace shell remains the visible project identity owner including sample status', async () => {
  const workspace = await source('modules/projects/ProjectWorkspaceLayout.tsx');

  assert.match(workspace, /\{project \? `\$\{project\.code\} · \$\{project\.name\}`/);
  assert.match(workspace, /project\?\.client_name \? `Chủ đầu tư:/);
  assert.match(workspace, /project\?\.status \? `Trạng thái:/);
  assert.match(workspace, /project\?\.is_sample \? <SampleDataBadge compact \/>/);
});
