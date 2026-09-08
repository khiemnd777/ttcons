import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function staffPageSource() {
  return readFile(new URL('../src/modules/staff/StaffPage.tsx', import.meta.url), 'utf8');
}

function roleDialogSource(source: string) {
  return source.slice(source.indexOf('function StaffRoleDialog'), source.indexOf('function InlineQueryError'));
}

test('staff role assignment keeps the approved RBAC contract', async () => {
  const source = await staffPageSource();

  assert.match(source, /hasPermission\('permission\.assign'\)/);
  assert.match(source, /post\(`\/users\/\$\{userId\}\/roles`, \{ role_ids: roleIds, project_id: projectId \}\)/);
  assert.match(source, /roles=\{\(roles\.data \?\? \[\]\)\.filter\(isAssignableAppRole\)\}/);
  assert.match(source, /Chọn ít nhất một vai trò cho nhân sự/);
});

test('staff role dialog renders a local responsive selection table', async () => {
  const dialog = roleDialogSource(await staffPageSource());

  assert.match(dialog, /title="Gán vai trò"/);
  assert.match(dialog, /<table className="min-w-full border-separate border-spacing-0">/);
  assert.match(dialog, /consflow-table-viewport max-h-\[42vh\]/);
  assert.match(dialog, /<thead className="bg-slate-50">/);
  assert.match(dialog, />Chọn<\/th>/);
  assert.match(dialog, />Vai trò<\/th>/);
  assert.match(dialog, />Mã kỹ thuật<\/th>/);
  assert.match(dialog, /sm:hidden/);
  assert.match(dialog, /sm:table-cell/);
  assert.doesNotMatch(dialog, /<DataTable/);
  assert.doesNotMatch(dialog, /Chọn tất cả/);
});

test('staff role dialog exposes search, selected filtering and explicit feedback states', async () => {
  const dialog = roleDialogSource(await staffPageSource());

  assert.match(dialog, /placeholder="Tìm theo tên hoặc mã vai trò\.\.\."/);
  assert.match(dialog, /Tất cả \{roles\.length\}/);
  assert.match(dialog, /Đã chọn \{selected\.size\}/);
  assert.match(dialog, /Không tải được danh sách vai trò/);
  assert.match(dialog, /Không tải được danh sách công trình/);
  assert.match(dialog, /Chưa có vai trò người dùng App để gán/);
  assert.match(dialog, /Không tìm thấy vai trò phù hợp/);
  assert.match(dialog, /Chọn ít nhất một vai trò trước khi lưu/);
});
