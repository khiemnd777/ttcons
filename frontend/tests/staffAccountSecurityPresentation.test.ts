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
  const passwordInput = await source('../src/components/PasswordInput.tsx');

  assert.match(page, /hasPermission\('staff\.account_security\.manage'\)/);
  assert.match(page, /title="Chỉnh sửa nhân sự"/);
  assert.match(page, /title: 'Thông tin nhân sự'/);
  assert.match(page, /title: 'Bảo mật tài khoản'/);
  assert.match(page, /name: 'new_password'[\s\S]*?type: 'password'/);
  assert.match(page, /name: 'new_password_confirmation'[\s\S]*?type: 'password'/);
  assert.match(page, /name: 'recovery_email'[\s\S]*?type: 'email'/);
  assert.match(page, /recovery_email_masked\?: string/);
  assert.match(page, /label: 'Email khôi phục mới'/);
  assert.match(page, /Hiện tại: \$\{record\.recovery_email_masked\} · Đã xác minh/);
  assert.match(page, /name: 'recovery_email'[\s\S]*?defaultValue: ''/);
  assert.doesNotMatch(page, /defaultValue:\s*record\.recovery_email_masked/);
  assert.match(page, /Email mới được xác minh ngay, không gửi email xác nhận/);
  assert.match(page, /Nhân sự chưa liên kết tài khoản đăng nhập/);
  assert.match(page, /await queryClient\.invalidateQueries\(\{ queryKey: \['staff'\] \}\)/);
  assert.match(simpleForm, /'email' \| 'password'/);
  assert.match(simpleForm, /<PasswordInput/);
  assert.match(passwordInput, /aria-label=\{passwordVisible \? 'Ẩn mật khẩu' : 'Hiện mật khẩu'\}/);
  assert.match(simpleForm, /<form ref=\{formRef\} className="space-y-4" noValidate onSubmit=\{handleSubmit\}>/);
});

test('staff creation requires account-security authority and creates one password plus recovery email', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');

  assert.match(page, /hasPermission\('staff\.create'\) && hasPermission\('staff\.account_security\.manage'\)/);
  assert.match(page, /title="Tạo nhân sự"/);
  assert.match(page, /name: 'password'[\s\S]*?suggestValue: generateSuggestedPassword/);
  assert.match(page, /name: 'recovery_email'[\s\S]*?label: 'Email khôi phục'/);
  assert.match(page, /fields: \['password', 'recovery_email'\]/);
  assert.doesNotMatch(page, /fields: \['password', 'password_confirmation'/);
  assert.match(page, /Tạo nhân sự & tài khoản/);
});

test('staff department and position are optional with explicit empty-state presentation', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');

  assert.match(page, /name: 'department'[\s\S]*?placeholder: 'Không bắt buộc'[\s\S]*?Dùng cho phân công công việc theo phòng ban/);
  assert.match(page, /name: 'position'[\s\S]*?placeholder: 'Không bắt buộc'[\s\S]*?không cấp quyền hệ thống/);
  assert.doesNotMatch(page, /name: 'department'[^\n]*required: true/);
  assert.doesNotMatch(page, /name: 'position'[^\n]*required: true/);
  assert.match(page, /render: \(row\) => optionalStaffValue\(row\.department\)/);
  assert.match(page, /render: \(row\) => optionalStaffValue\(row\.position\)/);
  assert.match(page, /return text \|\| <span className="text-slate-400">—<\/span>/);
});

test('staff self-password reset logs the current admin out', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');

  assert.match(page, /if \(result\.reauth_required\)/);
  assert.match(page, /await logout\(\)/);
  assert.match(page, /navigate\('\/login', \{ replace: true \}\)/);
});
