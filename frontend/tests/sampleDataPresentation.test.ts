import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('sample data tab and actions stay developer-permission gated', async () => {
  const settings = await source('../src/modules/settings/SettingsPage.tsx');
  const panel = await source('../src/modules/settings/SampleDataPanel.tsx');

  assert.match(settings, /hasPermission\('sample_data\.create'\)/);
  assert.match(settings, /canCreateSampleData \? <TabButton/);
  assert.match(settings, />Dữ liệu mẫu<\/TabButton>/);
  assert.match(settings, /tab === 'sample-data' && canCreateSampleData \? <SampleDataPanel/);
  assert.match(panel, /get<SampleDataOptions>\('\/settings\/sample-data\/options'\)/);
  assert.match(panel, /post<SampleUserProvisionResult>\('\/settings\/sample-data\/users'/);
  assert.match(panel, /post<SampleDataResult>\('\/settings\/sample-data\/projects'/);
  assert.match(panel, /pipeline_template_id: pipelineID/);
  assert.doesNotMatch(panel, /accounting_book_id|accounting_books|accountingBookID/);
  assert.match(panel, /const canCreate = Boolean\(selectedPipeline\?\.compatible\)/);
  assert.match(panel, /profile !== 'completed' \|\| Boolean\(selectedPipeline\?\.completed_compatible\)/);
  assert.match(panel, /<BlockerList blockers=\{selectedPipeline.completed_blockers\}/);
});

test('sample data profile flow exposes approved progress and confirmation semantics', async () => {
  const panel = await source('../src/modules/settings/SampleDataPanel.tsx');

  assert.match(panel, /'draft'.*'planning'.*'completed'/s);
  assert.match(panel, /data\.progress_presets\.map/);
  assert.match(panel, /profile === 'draft' \? 0 : profile === 'completed' \? 100 : progress/);
  assert.match(panel, /crypto\.randomUUID\(\)/);
  assert.match(panel, /title="Xác nhận tạo dữ liệu mẫu"/);
  assert.match(panel, /Mở Bàn làm việc/);
  assert.match(panel, /Chọn Pipeline đã phát hành/);
  assert.doesNotMatch(panel, /Chọn sổ kế toán|Chưa có sổ kế toán|Sổ kế toán chưa đủ điều kiện/);
  assert.match(panel, /Math\.floor\(finalProgress \* selectedPipeline\.selected_step_count \/ 100\)/);
  assert.doesNotMatch(panel, /Pipeline Rev8/);
  assert.doesNotMatch(panel, /20 bước/);
  assert.match(panel, /Tài khoản mẫu theo vai trò/);
  assert.match(panel, /Dev240489\./);
  assert.match(panel, /Không tài khoản người dùng thật nào được thêm hoặc giao việc/);
});

test('sample identity is visible in project and operational-report presentation', async () => {
  const projects = await source('../src/modules/projects/ProjectsPage.tsx');
  const projectWorkspace = await source('../src/modules/projects/ProjectWorkspaceLayout.tsx');
  const reports = await source('../src/modules/reports/OperationalReportPanel.tsx');
  const staff = await source('../src/modules/staff/StaffPage.tsx');

  assert.match(projects, /project\.is_sample \? <SampleDataBadge/);
  assert.match(projectWorkspace, /project\?\.is_sample \? <SampleDataBadge compact \/>/);
  assert.match(reports, /row\.is_sample \? <SampleDataBadge/);
  assert.match(staff, /row\.is_sample \? <SampleDataBadge compact/);
  assert.match(staff, /disabled=\{!row\.user_id \|\| row\.is_sample\}/);
});
