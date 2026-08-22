import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('Staff deletion is permission-gated and uses preview plus idempotent execute', async () => {
  const page = await source('../src/modules/staff/StaffPage.tsx');
  const dialog = await source('../src/modules/staff/StaffDeleteDialog.tsx');
  assert.match(page, /hasPermission\('staff\.delete'\) && canManageAccountSecurity/);
  assert.match(page, /selection=\{canDelete/);
  assert.match(page, /<StaffDeleteDialog/);
  assert.match(dialog, /'\/staff\/deletions\/preview'/);
  assert.match(dialog, /'\/staff\/deletions\/execute'/);
  assert.match(dialog, /preview_hash/);
  assert.match(dialog, /idempotency_key/);
  assert.match(dialog, /blocked_count === 0/);
  assert.match(dialog, /cần phân công lại/i);
});

test('DataTable selection supports current-page desktop and mobile controls', async () => {
  const table = await source('../src/components/DataTable.tsx');
  assert.match(table, /export type DataTableSelection/);
  assert.match(table, /visibleRowKeys/);
  assert.match(table, /toggleVisibleRows/);
  assert.match(table, /renderMobileCard\(row\)/);
  assert.match(table, /SelectionCheckbox/);
});

test('Deleted identities share an explicit suffix and live ownership warning', async () => {
  const label = await source('../src/components/IdentityReferenceLabel.tsx');
  const identityText = await source('../src/components/identityReference.ts');
  const projects = await source('../src/modules/projects/ProjectsPage.tsx');
  const workItem = await source('../src/modules/work-items/WorkItemCard.tsx');
  const reports = await source('../src/modules/work-reports/WorkReportsPage.tsx');
  const audit = await source('../src/modules/audit/AuditPage.tsx');
  assert.match(identityText, /\(tài khoản đã xoá\)/);
  assert.match(label, /Cần phân công lại/);
  assert.match(projects, /pm_account_deleted/);
  assert.match(workItem, /assignee_account_deleted/);
  assert.match(reports, /author_account_deleted/);
  assert.match(audit, /actor_account_deleted/);
});
