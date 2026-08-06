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

test('operational reports use compact mobile context and report-specific cards', async () => {
  const reportsPage = await source('../src/modules/reports/ReportsPage.tsx');
  const reportPanel = await source('../src/modules/reports/OperationalReportPanel.tsx');
  const reportFilters = await source('../src/modules/reports/ReportFilters.tsx');

  assert.match(reportsPage, />Loại báo cáo</);
  assert.match(reportsPage, /mb-4 hidden rounded-lg border border-line bg-white px-4 py-3 shadow-panel lg:block/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <ProjectReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <WarehouseReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <ProcurementReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <StaffReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /mobilePrimary/);
  assert.match(reportFilters, /grid-cols-\[minmax\(0,1fr\)_auto_auto\]/);
  assert.match(reportFilters, /disabled=\{exporting \|\| exportDisabled\}/);
});
