import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { domainTypesSource } from './domainTypesSource.ts';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('profile route is available to every authenticated user without a new permission', async () => {
  const router = await source('../src/app/router.tsx');

  assert.match(router, /const ProfilePage = lazy/);
  assert.match(router, /path: '\/profile', element: guarded\(<ProfilePage \/>\)/);
  assert.doesNotMatch(router, /path: '\/profile'[^\n]+\['[^']+\.view'\]/);
});

test('standard shell exposes profile entry on desktop and mobile user surfaces', async () => {
  const appShell = await source('../src/layouts/AppShell.tsx');

  assert.match(appShell, /aria-label="Mở hồ sơ cá nhân"/);
  assert.match(appShell, /navigate\('\/profile'\)/);
  assert.match(appShell, />\s*Hồ sơ cá nhân\s*</);
  assert.ok((appShell.match(/navigate\('\/profile'\)/g) ?? []).length >= 2, 'desktop and mobile must both open Profile');
});

test('profile page keeps personal and password edits in shared dialogs', async () => {
  const profile = await source('../src/modules/profile/ProfilePage.tsx');

  assert.match(profile, /title="Hồ sơ cá nhân"/);
  assert.match(profile, /Thông tin cá nhân/);
  assert.match(profile, /Bảo mật/);
  assert.match(profile, /<Dialog[\s\S]+title="Chỉnh sửa thông tin"/);
  assert.match(profile, /<Dialog[\s\S]+title="Đổi mật khẩu"/);
  assert.match(profile, /put<ProfileUpdateResponse>\('\/auth\/profile'/);
  assert.match(profile, /put<\{ password_changed: boolean; reauth_required: boolean \}>\('\/auth\/password'/);
  assert.match(profile, /emailChanged \?/);
  assert.match(profile, /await logout\(\)/);
  assert.match(profile, /navigate\('\/login', \{ replace: true \}\)/);
  assert.match(profile, /toast\.warn|warnInvalidFields/);
  assert.match(profile, /toast\.success/);
  assert.match(profile, /toast\.error/);
  assert.match(profile, /passwordValueSchema/);
  assert.match(profile, /<PasswordInput/);
});

test('auth context can refresh the user summary after a non-sensitive profile update', async () => {
  const auth = await source('../src/lib/auth/AuthContext.tsx');
  const types = await domainTypesSource();

  assert.match(auth, /refreshUser: \(\) => Promise<User>/);
  assert.match(auth, /async refreshUser\(\)/);
  assert.match(auth, /api<User>\('\/auth\/me'\)/);
  assert.match(types, /phone\?: string/);
  assert.match(types, /department\?: string/);
  assert.match(types, /position\?: string/);
});
