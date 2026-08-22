import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('partner catalogs gate current-page deletion to active records and catalog management', async () => {
  const page = await source('../src/modules/core-data/CoreDataPage.tsx');
  assert.match(page, /hasPermission\('catalog\.manage'\) && status === 'active'/);
  assert.match(page, /selection=\{canDelete/);
  assert.match(page, /kind="partner"/);
  assert.match(page, /ids=\{selectedPartnerIDs\}/);
  assert.match(page, /queryClient\.invalidateQueries\(\{ queryKey: \['partners'\] \}\)/);
});

test('project explorer selection is permission-gated, current-page scoped and responsive', async () => {
  const page = await source('../src/modules/projects/ProjectsPage.tsx');
  assert.match(page, /hasAnyPermission\(\['project\.delete'\]\)/);
  assert.match(page, /filteredProjects\.forEach\(\(project\) => next\.add\(project\.id\)\)/);
  assert.match(page, /setSelectedProjectIds\(new Set\(\)\);\s*\}, \[explorerParams\]\)/);
  assert.match(page, /kind="project"/);
  assert.match(page, /absolute left-4 top-4 lg:static/);
  assert.match(page, /Chọn tất cả công trình trên trang hiện tại/);
});

test('partner and project deletion share preview, explicit confirmation and idempotent execute', async () => {
  const dialog = await source('../src/components/BulkDeleteDialog.tsx');
  const bar = await source('../src/components/BulkSelectionBar.tsx');
  assert.match(dialog, /\/partners\/deletions/);
  assert.match(dialog, /\/projects\/deletions/);
  assert.match(dialog, /preview_hash/);
  assert.match(dialog, /idempotency_key/);
  assert.match(dialog, /crypto\.randomUUID\(\)/);
  assert.match(dialog, /preview\?\.blocked_count === 0 && confirmed/);
  assert.match(dialog, /Tôi xác nhận/);
  assert.match(bar, /bottom-\[calc\(0\.75rem\+env\(safe-area-inset-bottom\)\)\]/);
});
