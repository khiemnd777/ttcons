import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('Projects exposes data package actions and selection behind dedicated permissions', async () => {
  const page = await source('../src/modules/projects/ProjectsPage.tsx');
  assert.match(page, /hasAnyPermission\(\['project\.import'\]\)/);
  assert.match(page, /hasAnyPermission\(\['project\.export'\]\)/);
  assert.match(page, /const canSelect = canDelete \|\| canExport/);
  assert.match(page, />\s*Xuất dữ liệu\s*<\/button>/);
  assert.match(page, />\s*Nhập dữ liệu\s*<\/button>/);
  assert.match(page, /<ProjectImportDialog/);
  assert.match(page, /<ProjectExportDialog/);
});

test('project import keeps the exact nine-field versioned Excel and JSON contract in a browser worker', async () => {
  const helper = await source('../src/modules/projects/projectExcel.ts');
  const worker = await source('../src/modules/projects/projectExcel.worker.ts');
  const headerBlock = helper.match(/PROJECT_EXCEL_IMPORT_HEADERS = \[[\s\S]*?\]/)?.[0] ?? '';
  const headers = [
    'project_code', 'project_name', 'client_partner_code', 'client_name_unregistered', 'address',
    'planned_start_date', 'planned_end_date', 'estimated_budget_vnd', 'pipeline_ref',
  ];
  for (const header of headers) assert.match(headerBlock, new RegExp(`'${header}'`));
  assert.doesNotMatch(headerBlock, /contract_amount/);
  assert.match(helper, /PROJECT_EXCEL_WORKBOOK_VERSION = '1'/);
  assert.match(helper, /PROJECT_DATA_PACKAGE_VERSION = '1'/);
  assert.match(helper, /new Worker\(new URL\('\.\/projectExcel\.worker\.ts'/);
  assert.match(worker, /'Cong_trinh'/);
  assert.match(worker, /'Huong_dan'/);
  assert.match(worker, /không được chứa công thức/);
  assert.match(worker, /dateNF: 'yyyy-mm-dd'/);
  assert.match(worker, /consflow\.project-package/);
  assert.match(worker, /assertExactKeys/);
});

test('project import is blocked until Pipeline and Form Fields are ready and remains create-only', async () => {
  const dialog = await source('../src/modules/projects/ProjectImportDialog.tsx');
  assert.match(dialog, /'\/projects\/imports\/readiness'/);
  assert.match(dialog, /`\/projects\/imports\/template\?format=\$\{sourceFormat\}`/);
  assert.match(dialog, /'\/projects\/imports\/preview'/);
  assert.match(dialog, /'\/projects\/imports\/execute'/);
  assert.match(dialog, /Cần cập nhật dữ liệu nền trước khi import/);
  assert.match(dialog, /to=\{blocker\.route\}/);
  assert.match(dialog, /preview_hash/);
  assert.match(dialog, /idempotency_key/);
  assert.match(dialog, /không khởi chạy Pipeline/);
  assert.match(dialog, /source_format: sourceFormat/);
  assert.match(dialog, /JSON \(\.json\)/);
});

test('project export preserves current filters and documents Excel and JSON references', async () => {
  const dialog = await source('../src/modules/projects/ProjectExportDialog.tsx');
  assert.match(dialog, /scope,/);
  assert.match(dialog, /search: filters\.search/);
  assert.match(dialog, /status: filters\.status/);
  assert.match(dialog, /sort: filters\.sort/);
  assert.match(dialog, /direction: filters\.direction/);
  assert.match(dialog, /format,/);
  assert.match(dialog, /Excel \(\.xlsx\)/);
  assert.match(dialog, /JSON \(\.json\)/);
  assert.match(dialog, /Pipeline\/Form\/Field\/Client references/);
  assert.match(dialog, /Giá trị hợp đồng chỉ là snapshot đọc/);
});
