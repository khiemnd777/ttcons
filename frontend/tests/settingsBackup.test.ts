import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('settings backup keeps export readable and restore developer-only', async () => {
  const page = await source('../src/modules/settings/SettingsPage.tsx');
  const backup = await source('../src/modules/settings/SettingsBackupPanel.tsx');

  assert.match(page, /<SettingsBackupPanel canRestore=\{canUpdate\}/);
  assert.match(backup, /downloadBlob\('\/settings\/backup'\)/);
  assert.match(backup, /\{canRestore \? <button[^>]*>.*Khôi phục từ JSON/s);
  assert.match(backup, /Admin được tải bản sao; chỉ vai trò Nhà phát triển được khôi phục/);
});

test('settings restore always previews the same bounded JSON before an optimistic apply', async () => {
  const backup = await source('../src/modules/settings/SettingsBackupPanel.tsx');

  assert.match(backup, /const maxBackupFileSize = 256 \* 1024/);
  assert.match(backup, /accept="\.json,application\/json"/);
  assert.match(backup, /postForm<BackupPreview>\('\/settings\/backup\/preview'/);
  assert.match(backup, /body\.append\('preview_hash', checked\.file_hash\)/);
  assert.match(backup, /body\.append\('expected_versions', JSON\.stringify\(checked\.expected_versions\)\)/);
  assert.match(backup, /postForm<RestoreResult>\('\/settings\/backup\/restore'/);
  assert.match(backup, /File không chứa giá trị secret/);
  assert.match(backup, /consflow-table-viewport mt-2 hidden.*md:block/s);
  assert.match(backup, /space-y-2 md:hidden/);
});
