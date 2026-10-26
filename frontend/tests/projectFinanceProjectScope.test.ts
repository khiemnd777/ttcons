import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('global accounting handoff uses a draft/applied asynchronous project scope', async () => {
  const financePage = await source('../src/modules/project-finance/AccountingHandoffPage.tsx');

  assert.doesNotMatch(financePage, /\/projects'\)|projects\.data\?\.\[0\]/);
  assert.match(financePage, /const \[draftProjectId, setDraftProjectId\] = useState\(requestedProjectId\)/);
  assert.match(financePage, /const projectId = id \?\? \(global \? requestedProjectId : ''\)/);
  assert.match(financePage, /params\.set\('project', draftProjectId\)/);
  assert.match(financePage, /params\.delete\('project'\)/);
  assert.match(financePage, /endpoint="\/project-finance\/projects"/);
  assert.match(financePage, /allowEmptySelection=\{false\}/);
  assert.match(financePage, /applyDisabled=\{!draftProjectId \|\| draftProjectId === projectId\}/);
  assert.match(financePage, /aria-label="Bộ lọc công trình bàn giao kế toán"/);
  assert.match(financePage, /queryKey: \['project-finance', projectId\]/);
  assert.doesNotMatch(financePage, /project-finance\/export|downloadBlob/);
});

test('new accounting handoff is source-backed and contains no duplicate accounting capture', async () => {
  const financePage = await source('../src/modules/project-finance/AccountingHandoffPage.tsx');
  const actionDialog = await source('../src/modules/project-finance/AccountingHandoffActionDialog.tsx');
  const compatibilityWrapper = await source('../src/modules/project-finance/ProjectFinancePage.tsx');

  assert.match(financePage, /Hồ sơ bàn giao kế toán/);
  assert.match(financePage, /Hạch toán, hóa đơn, thuế, thanh toán và báo cáo tài chính thực hiện trên MISA/);
  assert.match(financePage, /source_type: item\.source_type, source_id: item\.source_id/);
  assert.match(financePage, /\/projects\/\$\{projectId\}\/accounting-handoffs/);
  assert.match(financePage, /\/accounting-handoffs\/\$\{item\.handoff_id\}\/\$\{action\}/);
  assert.match(actionDialog, /'handoff' \| 'acknowledge' \| 'return'/);
  assert.match(financePage, /<AccountingHandoffActionDialog/);
  assert.doesNotMatch(financePage, /Ghi nhận chi phí vật tư|Lập đề nghị thu\/chi|Báo cáo quản trị|provisional_profit|amount_before_vat|vat_amount/);
  assert.match(compatibilityWrapper, /return <AccountingHandoffPage global=\{global\} \/>/);
  assert.doesNotMatch(compatibilityWrapper, /LegacyProjectFinancePage|useQuery|project-finance\/options|material-costs|cash-requests/);
  // Optional help shares the header action slot; the same global/project branches remain.
  assert.match(financePage, /<ContextHelp label="Phạm vi hồ sơ bàn giao kế toán"/);
  assert.match(financePage, /<\/ContextHelp>\{global \? \([\s\S]*\) : id \? <ProjectBackLink \/> : undefined\}<\/>\}/);
});

test('true-empty accounting handoff guides authorized users to canonical source modules', async () => {
  const financePage = await source('../src/modules/project-finance/AccountingHandoffPage.tsx');

  assert.match(financePage, /data\?\.items\.length === 0/);
  assert.match(financePage, /hasAnyPermission\(\['procurement\.view'\]\)/);
  assert.match(financePage, /hasAnyPermission\(\['contract\.view'\]\)/);
  assert.match(financePage, /to=\{`\/projects\/\$\{projectId\}\/procurement`\}/);
  assert.match(financePage, /to=\{`\/projects\/\$\{projectId\}\/contracts`\}/);
  assert.match(financePage, /Hồ sơ được tạo từ nguồn đủ điều kiện:/);
  assert.match(financePage, /Không có hồ sơ phù hợp với bộ lọc hoặc tìm kiếm/);
  assert.doesNotMatch(financePage, /Tạo hồ sơ kế toán|Tạo bàn giao kế toán/);
});
