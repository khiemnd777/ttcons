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

test('statistics use one global project scope and report-specific responsive cards', async () => {
  const reportsPage = await source('../src/modules/reports/ReportsPage.tsx');
  const projectPicker = await source('../src/modules/reports/ReportProjectPicker.tsx');
  const asyncProjectPicker = await source('../src/modules/_shared/AsyncProjectPicker.tsx');
  const projectScopeActions = await source('../src/modules/_shared/ProjectScopeActions.tsx');
  const reportPanel = await source('../src/modules/reports/OperationalReportPanel.tsx');
  const reportRegistry = await source('../src/modules/reports/reportRegistry.ts');
  const reportFilters = await source('../src/modules/reports/ReportFilters.tsx');

  assert.match(reportsPage, />Loại thống kê</);
  assert.doesNotMatch(reportsPage, /Phạm vi báo cáo|Công trình đã áp dụng được giữ khi/);
  assert.ok(reportsPage.indexOf('<ReportProjectPicker') < reportsPage.indexOf('lg:grid-cols-[16rem_minmax(0,1fr)]'));
  assert.ok(reportsPage.indexOf('<ReportProjectPicker') < reportsPage.indexOf('</header>'));
  assert.match(reportsPage, /xl:grid-cols-\[minmax\(0,1fr\)_36rem\]/);
  assert.match(reportsPage, /border-t border-line pt-4 xl:border-0 xl:pt-0/);
  assert.doesNotMatch(reportsPage, /rounded-lg border border-line bg-white p-4 shadow-panel/);
  assert.match(reportsPage, /aria-label="Bộ lọc công trình thống kê"/);
  assert.match(reportsPage, /<ReportScopeActions/);
  assert.match(reportsPage, /hasAnyPermission\(\['statistics\.export'\]\)/);
  assert.match(reportsPage, /<ReportExportButton/);
  assert.match(reportsPage, /<ActiveReport[\s\S]*reportKey=\{activeReport\.key\}[\s\S]*projectId=\{appliedProjectId\}/);
  assert.match(reportsPage, /params\.set\('search', searchQuery\.trim\(\)\)/);
  assert.match(reportsPage, /params\.set\('sort', sortState\.key\)/);
  assert.match(reportsPage, /params\.set\('direction', sortState\.direction\)/);
  assert.match(projectPicker, /endpoint="\/statistics\/options"/);
  assert.match(projectPicker, /queryKey="statistics-project-options"/);
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
  assert.match(reportPanel, /sortState: DataTableSort/);
  assert.match(reportPanel, /searchQuery: string/);
  assert.doesNotMatch(reportPanel, /ReportProjectField|ReportFilterActions|useSearchParams/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <ProjectReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <WarehouseReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <ProcurementReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /renderMobileCard=\{\(row\) => <StaffReportCard row=\{row\} \/>\}/);
  assert.match(reportPanel, /getRowKey=\{\(row\) => `\$\{row\.project_id\}:\$\{row\.status\}:\$\{row\.budget_status\}`\}/);
  assert.match(reportPanel, /key: 'project_name', header: 'Công trình', sortable: true/);
  assert.match(reportPanel, /searchPlaceholder="Tìm công trình, trạng thái mua sắm, ngân sách..."/);
  assert.match(reportPanel, /function procurementStatisticsSourcePath\(projectId: string\) \{\s+return `\/projects\/\$\{projectId\}\/procurement`;\s+\}/);
  assert.equal((reportPanel.match(/procurementStatisticsSourcePath\(row\.project_id\)/g) ?? []).length, 2);
  assert.match(reportRegistry, /Giá trị yêu cầu theo công trình, trạng thái và ngân sách/);
  assert.match(reportFilters, /<ProjectScopeActions/);
  assert.match(projectScopeActions, /grid-cols-\[minmax\(0,1fr\)_auto\]/);
  assert.match(projectScopeActions, /bg-transparent/);
  assert.match(reportFilters, /grid-cols-\[minmax\(0,1fr\)_auto_auto\]/);
  assert.match(reportFilters, /disabled=\{exporting \|\| exportDisabled\}/);
});

test('work report workspace is a delivery register while source drafts stay in construction', async () => {
  const page = await source('../src/modules/work-reports/WorkReportsPage.tsx');
  const reviewDialog = await source('../src/modules/work-reports/WorkReportReviewDialog.tsx');
  const evidenceGallery = await source('../src/modules/work-items/EvidenceGallery.tsx');
  const evidenceViewer = await source('../src/modules/_shared/EvidenceRecordViewerDialog.tsx');
  const router = await source('../src/app/router.tsx');
  const shell = await source('../src/layouts/AppShell.tsx');

  assert.match(router, /path: '\/reports', element: guarded\(<WorkReportsPage \/>, \['work_report\.view'\]\)/);
  assert.match(router, /path: '\/statistics\/:reportKey'/);
  assert.match(shell, /to: '\/reports', label: 'Báo cáo'.*work_report\.view/);
  assert.match(shell, /to: '\/statistics', label: 'Thống kê'.*statistics\.view/);
  assert.match(page, /key: 'mine', label: 'Đã gửi'/);
  assert.match(page, /const canOpenSource = hasPermission\('operation\.view'\)/);
  assert.match(page, /canOpenSource \? \[\{ key: 'source' as const, label: 'Nháp tại nguồn' \}\] : \[\]/);
  assert.match(page, /Chờ tôi xem/);
  assert.match(page, /hasPermission\('work_report\.create'\).*hasPermission\('operation\.report'\).*hasPermission\('operation\.view'\)/);
  assert.match(page, /<WorkReportReviewDialog/);
  assert.doesNotMatch(page, /function WorkReportDialog/);
  assert.match(reviewDialog, /Nội dung &amp; minh chứng/);
  assert.match(reviewDialog, /Lịch sử &amp; phản hồi/);
  assert.match(reviewDialog, /Các lần gửi/);
  assert.match(reviewDialog, /xl:grid-cols-\[minmax\(0,1fr\)_22rem\]/);
  assert.match(reviewDialog, /xl:sticky xl:top-0/);
  assert.match(reviewDialog, /Yêu cầu bổ sung/);
  assert.match(reviewDialog, /Xác nhận đã xem/);
  assert.match(reviewDialog, /không phải nghiệm thu chất lượng/);
  assert.match(reviewDialog, /PDF bản này/);
  assert.match(reviewDialog, /variant="review"/);
  assert.match(reviewDialog, /initialVisibleCount=\{5\}/);
  assert.match(reviewDialog, /\/work-reports\/\$\{detail\.work_report_id\}\/revisions\/\$\{selectedRevision\.revision_no\}\/evidence\/\$\{record\.id\}\/file/);
  assert.match(evidenceGallery, /variant\?: 'cards' \| 'list' \| 'review'/);
  assert.match(evidenceGallery, /protectedFilePath=\{selected \? resolveEvidenceFilePath/);
  assert.match(evidenceViewer, /protectedAccessDescription/);
  assert.match(page, /endpoint="\/work-reports\/projects"/);
  assert.match(page, /format: ExportFormat/);
  assert.match(page, /\/operations\?tab=timeline&report=\$\{encodeURIComponent\(row\.source_id\)\}/);
  assert.match(page, /Mở tại Thi công/);
  assert.match(page, /get<WorkReportSummary>\(`\/work-reports\/summary/);
  assert.match(page, /scopeCount\(reportSummary\.data, item\.key/);
  assert.match(page, /searchParams\.get\('report'\)/);
  assert.match(page, /searchParams\.get\('scope'\)/);
  assert.doesNotMatch(page, /Gửi báo cáo cho người quản lý/);
  assert.doesNotMatch(page, /\/work-reports\/daily-reports\/\$\{detail\.source_id\}\/submit/);
});
