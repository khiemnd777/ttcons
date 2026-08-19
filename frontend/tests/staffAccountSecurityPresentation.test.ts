import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('staff table uses one edit action while retaining role assignment', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');

  assert.match(page, /<RowActions onEdit=/);
  assert.doesNotMatch(page, /onView=/);
  assert.doesNotMatch(page, /RecordDetailDialog/);
  assert.match(page, /<ShieldCheck className="h-4 w-4"/);
});

test('staff edit dialog exposes permission-gated direct account security fields', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');
  const simpleForm = await source('../src/modules/_shared/SimpleForm.tsx');

  assert.match(page, /hasPermission\('staff\.account_security\.manage'\)/);
  assert.match(page, /title="Chỉnh sửa nhân sự"/);
  assert.match(page, /title: 'Thông tin nhân sự'/);
  assert.match(page, /title: 'Bảo mật tài khoản'/);
  assert.match(page, /name: 'new_password'[\s\S]*?type: 'password'/);
  assert.match(page, /name: 'recovery_email'[\s\S]*?type: 'email'/);
  assert.match(page, /recovery_email_masked\?: string/);
  assert.match(page, /label: 'Email khôi phục mới'/);
  assert.match(page, /Hiện tại: \$\{record\.recovery_email_masked\} · Đã xác minh/);
  assert.match(page, /name: 'recovery_email'[\s\S]*?defaultValue: ''/);
  assert.doesNotMatch(page, /defaultValue:\s*record\.recovery_email_masked/);
  assert.match(page, /Email mới được xác minh ngay, không gửi email xác nhận/);
  assert.match(page, /Nhân sự chưa liên kết tài khoản đăng nhập/);
  assert.doesNotMatch(page, /new_password_confirmation/);
  assert.match(page, /await queryClient\.invalidateQueries\(\{ queryKey: \['staff'\] \}\)/);
  assert.match(simpleForm, /'email' \| 'password'/);
  assert.match(simpleForm, /<form className="space-y-4" noValidate onSubmit=\{handleSubmit\}>/);
});

test('staff self-password reset logs the current admin out', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');

  assert.match(page, /if \(result\.reauth_required\)/);
  assert.match(page, /await logout\(\)/);
  assert.match(page, /navigate\('\/login', \{ replace: true \}\)/);
});
