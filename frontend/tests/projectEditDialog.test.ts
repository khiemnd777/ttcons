import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function projectsPageSource() {
  return readFile(new URL('../src/modules/projects/ProjectsPage.tsx', import.meta.url), 'utf8');
}

test('project explorer keeps one pencil entry point for project review and editing', async () => {
  const projectsPage = await projectsPageSource();

  assert.doesNotMatch(projectsPage, /\bEye\b/);
  assert.doesNotMatch(projectsPage, /RecordDetailDialog/);
  assert.doesNotMatch(projectsPage, /onView/);
  assert.doesNotMatch(projectsPage, /Xem nhanh công trình/);
  assert.match(projectsPage, /aria-label="Chỉnh sửa công trình"/);
  assert.match(projectsPage, /<Pencil className="h-4 w-4" \/>/);
});

test('project edit dialog preserves read-only detail context above the editable form', async () => {
  const projectsPage = await projectsPageSource();

  assert.match(projectsPage, /function ProjectEditSummary/);
  assert.match(projectsPage, /<ProjectEditSummary project=\{editingProject\} \/>/);
  assert.match(projectsPage, /<StatusBadge status=\{project\.status\} \/>/);
  assert.match(projectsPage, /project\.progress_percent \?\? 0/);
  assert.match(projectsPage, /project\.partner_name \|\| '-'/);
  assert.match(projectsPage, /project\.partner_tax_code \|\| '-'/);
  assert.match(projectsPage, /onCancel=\{\(\) => setEditingProject\(null\)\}/);
});

test('project edit dialog uses plain-language owner selection wording', async () => {
  const projectsPage = await projectsPageSource();

  assert.match(projectsPage, /label: 'Chọn chủ đầu tư có sẵn'/);
  assert.match(projectsPage, />Chủ đầu tư đã chọn<\/dt>/);
  assert.match(projectsPage, /helperText: 'Khi chọn, hệ thống lưu tên chủ đầu tư hiện tại vào công trình\.'/);
  assert.doesNotMatch(projectsPage, /Chủ đầu tư catalog/);
  assert.doesNotMatch(projectsPage, /backend tự snapshot/);
});
