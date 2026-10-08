import assert from 'node:assert/strict';
import { access, readFile } from 'node:fs/promises';
import { typedRecordTarget } from '../src/modules/_shared/recordNavigation.ts';
import { activityTopic } from '../src/modules/work-items/activityTopicDomain.ts';
import test from 'node:test';

test('retired accounting UI has no mounted route or source owner', async () => {
  const router = await readFile(new URL('../src/app/router.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(router, /modules\/accounting\/|AccountingPage|journals|trial-balance|tax-periods/);
  assert.match(router, /path: 'accounting'.*ProjectFinancePage/);
  assert.match(router, /path: '\/accounting'.*ProjectFinancePage global/);
  await assert.rejects(access(new URL('../src/modules/accounting/AccountingPage.tsx', import.meta.url)), { code: 'ENOENT' });
});

test('internal debt and payment source kinds cannot expose navigation or previews', () => {
  const id = '11111111-1111-4111-8111-111111111111';
  const projectId = '22222222-2222-4222-8222-222222222222';
  for (const kind of ['payable', 'receivable', 'payment']) {
    assert.equal(typedRecordTarget(kind, id, projectId), null);
    assert.equal(activityTopic({ scope_type: kind, scope_id: id }, projectId), null);
  }
});

test('contract milestones keep commercial terms and have no ledger opening or retention release actions', async () => {
  const page = await readFile(new URL('../src/modules/contracts/ContractsPage.tsx', import.meta.url), 'utf8');
  const presentation = await readFile(new URL('../src/modules/contracts/contractsPresentation.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(page + presentation, /open-obligation|contract-retentions|canReleaseRetention|milestoneObligationAction|RetentionTable|Mở công nợ/);
  assert.match(page, /milestoneEligibleAction/);
  assert.match(page, /row.retention_amount/);
  assert.match(page, /variationProcurementAction/);
});

test('project finance entry always delegates to the current handoff experience', async () => {
  const entry = await readFile(new URL('../src/modules/project-finance/ProjectFinancePage.tsx', import.meta.url), 'utf8');
  assert.match(entry, /return <AccountingHandoffPage global=\{global\} \/>/);
  assert.doesNotMatch(entry, /legacy|useQuery|useMutation|material-costs|cash-requests/);
});
