import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('shared data table keeps desktop rows and supports opt-in mobile cards', async () => {
  const dataTable = await source('../src/components/DataTable.tsx');

  assert.match(dataTable, /renderMobileCard\?: \(row: T\) => ReactNode/);
  assert.match(dataTable, /renderMobileCard && 'hidden md:block'/);
  assert.match(dataTable, /space-y-3 bg-slate-50\/70 p-3 md:hidden/);
  assert.match(dataTable, /aria-label="Sắp xếp báo cáo"/);
  assert.match(dataTable, /Xóa nội dung tìm kiếm/);
});

test('operational reports use one global project scope and report-specific responsive cards', async () => {
  const reportsPage = await source('../src/modules/reports/ReportsPage.tsx');
  const projectPicker = await source('../src/modules/reports/ReportProjectPicker.tsx');
  const asyncProjectPicker = await source('../src/modules/_shared/AsyncProjectPicker.tsx');
  const projectScopeActions = await source('../src/modules/_shared/ProjectScopeActions.tsx');
  const reportPanel = await source('../src/modules/reports/OperationalReportPanel.tsx');
  const reportFilters = await source('../src/modules/reports/ReportFilters.tsx');

  assert.match(reportsPage, />Loại báo cáo</);
  assert.doesNotMatch(reportsPage, /Phạm vi báo cáo|Công trình đã áp dụng được giữ khi/);
  assert.ok(reportsPage.indexOf('<ReportProjectPicker') < reportsPage.indexOf('lg:grid-cols-[16rem_minmax(0,1fr)]'));
  assert.ok(reportsPage.indexOf('<ReportProjectPicker') < reportsPage.indexOf('</header>'));
  assert.match(reportsPage, /xl:grid-cols-\[minmax\(0,1fr\)_36rem\]/);
  assert.match(reportsPage, /border-t border-line pt-4 xl:border-0 xl:pt-0/);
  assert.doesNotMatch(reportsPage, /rounded-lg border border-line bg-white p-4 shadow-panel/);
  assert.match(reportsPage, /aria-label="Bộ lọc công trình báo cáo"/);
  assert.match(reportsPage, /<ReportScopeActions/);
  assert.match(reportsPage, /<ReportExportButton/);
  assert.match(reportsPage, /<ActiveReport reportKey=\{activeReport\.key\} projectId=\{appliedProjectId\} \/>/);
  assert.match(projectPicker, /endpoint="\/reports\/options"/);
  assert.match(projectPicker, /queryKey="report-project-options"/);
  assert.match(projectPicker, /emptyLabel="Tất cả công trình được phép xem"/);
  assert.match(asyncProjectPicker, /useInfiniteQuery/);
  assert.match(asyncProjectPicker, /const PROJECT_PAGE_SIZE = 20/);
  assert.match(asyncProjectPicker, /params\.set\('limit', String\(PROJECT_PAGE_SIZE\)\)/);
  assert.match(asyncProjectPicker, /params\.set\('selected_id', value\)/);
  assert.match(asyncProjectPicker, /params\.set\('search', deferredSearch\)/);
  assert.match(asyncProjectPicker, /params\.set\('cursor', String\(pageParam\)\)/);
  assert.match(asyncProjectPicker, />Tải thêm</);
  assert.match(asyncProjectPicker, /project\.code/);
  assert.match(asyncProjectPicker, /sm:flex-1/);
  assert.match(asyncProjectPicker, /selectedProject\.code/);
  assert.match(asyncProjectPicker, /selectedProject\.name/);
  assert.match(asyncProjectPicker, /bg-slate-100/);
  assert.match(reportPanel, /projectId: string/);
  assert.doesNotMatch(reportPanel, /ReportProjectField|ReportFilterActions|useSearchParams/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <ProjectReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <WarehouseReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <ProcurementReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <StaffReportCard row=\{row\} \/>\}/);
  assert.match(reportFilters, /<ProjectScopeActions/);
  assert.match(projectScopeActions, /grid-cols-\[minmax\(0,1fr\)_auto\]/);
  assert.match(projectScopeActions, /bg-transparent/);
  assert.match(reportFilters, /grid-cols-\[minmax\(0,1fr\)_auto_auto\]/);
  assert.match(reportFilters, /disabled=\{exporting \|\| exportDisabled\}/);
});
