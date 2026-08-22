import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('Staff exposes import only behind the complete privileged permission set', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');
  assert.match(page, /hasPermission\('staff\.import'\) && canCreate && canAssign/);
  assert.match(page, /<StaffImportDialog/);
  assert.match(page, />Import<\/button>/);
});

test('Staff import keeps the exact approved eight-column contract and browser worker parsing', async () => {
  const helper = await source('../src/modules/staff/staffImport.ts');
  const worker = await source('../src/modules/staff/staffImport.worker.ts');
  for (const header of ['full_name', 'login_email', 'password', 'recovery_email', 'phone', 'department', 'position', 'role_code']) {
    assert.match(helper, new RegExp(`'${header}'`));
  }
  assert.doesNotMatch(helper.match(/STAFF_IMPORT_HEADERS = \[[\s\S]*?\]/)?.[0] ?? '', /status/);
  assert.match(helper, /new Worker\(new URL\('\.\/staffImport\.worker\.ts'/);
  assert.match(worker, /sheet accounts/);
  assert.match(worker, /không được chứa công thức/);
  assert.match(helper, /new Set\(\['admin', 'client', 'developer'\]\)/);
});

test('Import preview and execute never render a plaintext password', async () => {
  const dialog = await source('../src/modules/staff/StaffImportDialog.tsx');
  assert.match(dialog, /'\/staff\/imports\/preview'/);
  assert.match(dialog, /'\/staff\/imports\/execute'/);
  assert.match(dialog, /preview_hash/);
  assert.match(dialog, /idempotency_key/);
  assert.match(dialog, /row\.password_valid/);
  assert.doesNotMatch(dialog, /row\.password(?!_valid)/);
  assert.match(dialog, /đổi mật khẩu lần đầu/);
});

test('Imported accounts are routed to the standalone initial password change screen', async () => {
  const router = await source('../src/app/router.tsx');
  const page = await source('../src/modules/auth/InitialPasswordChangePage.tsx');
  const types = await source('../src/lib/types/domain.ts');
  assert.match(types, /must_change_password\?: boolean/);
  assert.match(router, /user\.must_change_password.*change-initial-password/);
  assert.match(router, /path: '\/change-initial-password'/);
  assert.match(page, /put\('\/auth\/password'/);
  assert.match(page, /passwordValueSchema/);
  assert.match(page, /PasswordInput/);
});
