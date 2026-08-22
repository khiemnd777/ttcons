import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('account recovery pages are public and login exposes the forgot-password entry', async () => {
  const router = await source('../src/app/router.tsx');
  const login = await source('../src/modules/auth/LoginPage.tsx');

  assert.match(login, /<PasswordInput/);

  assert.match(router, /path: '\/forgot-password'[\s\S]*?<ForgotPasswordPage \/>/);
  assert.match(router, /path: '\/reset-password'[\s\S]*?<ResetPasswordPage \/>/);
  assert.match(router, /path: '\/verify-recovery-email'[\s\S]*?<VerifyRecoveryEmailPage \/>/);
  assert.match(login, /to="\/forgot-password">Quên mật khẩu\?</);
});

test('recovery tokens use URL fragments and are removed from browser history', async () => {
  const helper = await source('../src/modules/auth/recoveryToken.ts');
  const reset = await source('../src/modules/auth/ResetPasswordPage.tsx');
  const verify = await source('../src/modules/auth/VerifyRecoveryEmailPage.tsx');

  assert.match(reset, /passwordValueSchema/);
  assert.match(reset, /<PasswordInput/);

  assert.match(helper, /window\.location\.hash/);
  assert.match(helper, /window\.history\.replaceState/);
  assert.match(reset, /clearRecoveryTokenFragment\(\)/);
  assert.match(verify, /clearRecoveryTokenFragment\(\)/);
  assert.doesNotMatch(reset, /location\.search/);
  assert.doesNotMatch(verify, /location\.search/);
});

test('profile presents only masked recovery email state and requires current password for mutations', async () => {
  const profile = await source('../src/modules/profile/ProfilePage.tsx');
  const types = await source('../src/lib/types/domain.ts');

  assert.match(profile, /Email khôi phục/);
  assert.match(profile, /put<[^>]+>\('\/auth\/recovery-email'/);
  assert.match(profile, /method: 'DELETE'/);
  assert.match(profile, /current_password/);
  assert.match(types, /recovery_email_masked\?: string/);
  assert.match(types, /pending_recovery_email_masked\?: string/);
  assert.doesNotMatch(types, /\n\s*recovery_email\?: string/);
});

test('forgot-password success copy does not reveal account availability', async () => {
  const forgot = await source('../src/modules/auth/ForgotPasswordPage.tsx');
  assert.match(forgot, /Nếu tài khoản có email khôi phục hợp lệ/);
  assert.doesNotMatch(forgot, /không tồn tại|không tìm thấy tài khoản/i);
});
