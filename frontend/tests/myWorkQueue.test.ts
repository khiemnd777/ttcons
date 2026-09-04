import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';

const read = (path: string) => readFile(new URL(path, import.meta.url), 'utf8');

async function queueHelpers() {
  const source = await read('../src/modules/my-work/myWorkDomain.ts');
  const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const exports: Record<string, any> = {};
  const require = createRequire(import.meta.url);
  new Function('exports', 'require', output)(exports, (name: string) => name.includes('workItemPresentation') ? {} : require(name));
  return exports;
}

test('default accounting precedence includes startup/workflow cards and preserves each source order', async () => {
  const { orderedWorkListEntries, workQueueEntryKey } = await queueHelpers();
  const work = { kind: 'work_item', item: { id: 'work-1' } };
  const nextWork = { kind: 'work_item', item: { id: 'work-2' } };
  const accounting = { kind: 'accounting_handoff', item: { id: 'a', handoff_id: 'a' } };
  const nextAccounting = { kind: 'accounting_handoff', item: { id: 'b', handoff_id: 'b' } };
  const entries = [work, accounting, nextWork, nextAccounting];
  const startups = [{ id: 'startup' }];
  const actions = [{ id: 'advance' }];
  const ordered = orderedWorkListEntries(entries, startups, actions, true);
  assert.deepEqual(ordered.map(workQueueEntryKey), ['accounting:a', 'accounting:b', 'startup', 'advance', 'work-1', 'work-2']);
  assert.deepEqual(orderedWorkListEntries(entries, startups, actions, false).map(workQueueEntryKey), ['startup', 'advance', 'work-1', 'accounting:a', 'work-2', 'accounting:b']);
  assert.deepEqual(entries, [work, accounting, nextWork, nextAccounting], 'do not mutate cached pages');
  assert.deepEqual(orderedWorkListEntries([work], startups, actions, true).map(workQueueEntryKey), ['startup', 'advance', 'work-1']);
  assert.deepEqual(orderedWorkListEntries([], [], [], true), []);
});

test('render and default selection share ordered entries; valid selection is preserved', async () => {
  const page = await read('../src/modules/my-work/MyWorkPage.tsx');
  assert.match(page, /orderedWorkListEntries\(queueEntries, startupActionRows, workflowActionRows, sort === 'priority'\)/);
  assert.match(page, /if \(mode === 'work' && workItems.isLoading\) return/);
  assert.match(page, /\? orderedEntries\.map\(workQueueEntryKey\)/);
  assert.match(page, /!selectedEntryId \|\| !entryIds.includes\(selectedEntryId\)/);
  assert.match(page, /setSelectedEntryId\(entryIds\[0\]\)/);
  assert.match(page, /prioritizeAccounting=\{sort === 'priority'\}/);
});

test('mixed queue query preserves source filters and server cursor, legacy path stays separate', async () => {
  const { buildWorkQueuePath, buildWorkPath } = await queueHelpers();
  const filters = { queue: 'responsible', bucket: 'waiting_supplement', project: 'p', status: 'returned', deadline: '', workType: '', priority: '', search: 'PO 12', sort: 'newest', entryKind: 'accounting_handoff' };
  const url = new URL(buildWorkQueuePath(filters, 'offset-token'), 'http://local');
  assert.equal(url.pathname, '/me/work-queue');
  for (const [key, value] of Object.entries({ project_id: 'p', search: 'PO 12', cursor: 'offset-token', entry_kind: 'accounting_handoff', bucket: 'waiting_supplement', status: 'returned', limit: '30' })) assert.equal(url.searchParams.get(key), value);
  assert.ok(buildWorkPath(filters, '').startsWith('/me/work-items?'));
});

test('project startup keeps quick counts stable while a sibling bucket filters the list', async () => {
  const { projectStartupVisibility } = await queueHelpers();
  const common = { queue: 'responsible', entryKind: '', status: '', deadline: '', workType: '', priority: '' };

  assert.deepEqual(projectStartupVisibility({ ...common, bucket: 'need_action' }), {
    contributesToSummary: true,
    rowsVisible: true,
  });
  assert.deepEqual(projectStartupVisibility({ ...common, bucket: 'waiting_supplement' }), {
    contributesToSummary: true,
    rowsVisible: false,
  });

  const baseSummary = { total: 4, need_action: 1 };
  const startupSummary = { total: 2, need_action: 2 };
  const waitingSupplementVisibility = projectStartupVisibility({ ...common, bucket: 'waiting_supplement' });
  assert.equal(baseSummary.total + (waitingSupplementVisibility.contributesToSummary ? startupSummary.total : 0), 6);
  assert.equal(baseSummary.need_action + (waitingSupplementVisibility.contributesToSummary ? startupSummary.need_action : 0), 3);

  assert.deepEqual(projectStartupVisibility({ ...common, bucket: 'waiting_supplement', entryKind: 'accounting_handoff' }), {
    contributesToSummary: false,
    rowsVisible: false,
  });
});

test('same UUID in distinct source types remains two entries, duplicate pages do not duplicate a handoff', async () => {
  const { dedupeWorkQueue, workQueueEntryKey } = await queueHelpers();
  const work = { kind: 'work_item', item: { id: 'same' } };
  const handoff = { kind: 'accounting_handoff', item: { id: 'same', handoff_id: 'same', status: 'handed_off' } };
  const updated = { ...handoff, item: { ...handoff.item, status: 'acknowledged' } };
  const result = dedupeWorkQueue([work, handoff, updated]);
  assert.equal(result.length, 2);
  assert.deepEqual(result.map(workQueueEntryKey), ['same', 'accounting:same']);
  assert.equal(result[1].item.status, 'acknowledged');
});

test('both views share canonical actions and invalidation; observer navigation retains project register', async () => {
  const [page, lane, actions, card, shell, router, register] = await Promise.all([
    read('../src/modules/my-work/MyWorkPage.tsx'), read('../src/modules/my-work/AccountingWorkLane.tsx'),
    read('../src/modules/project-finance/accountingWorkActions.ts'), read('../src/modules/my-work/AccountingWorkCard.tsx'),
    read('../src/layouts/AppShell.tsx'), read('../src/app/router.tsx'), read('../src/modules/project-finance/AccountingHandoffPage.tsx'),
  ]);
  for (const view of [page, lane]) assert.match(view, /useAccountingWorkActions\(\)/);
  for (const family of ['my-accounting-work', 'my-work-items', 'my-work-scope-summary', 'my-work-navigation-summary', 'project-finance']) assert.ok(actions.includes(`'${family}'`));
  assert.match(register, /invalidateAccountingWork\(queryClient, projectId\)/);
  assert.match(page, /label="Nghiệp vụ Kế toán"/);
  assert.match(page, /entryKind !== 'accounting_handoff'/);
  assert.match(card, /Nhóm Kế toán · hàng chờ chung/);
  assert.match(card, /source_route/);
  assert.doesNotMatch(card, /due_at|assignee_staff_id|run_node_name|evidence_count/);
  assert.match(shell, /to: '\/accounting'[^\n]+excludePermissions: \['project\.reconciliation\.view'\]/);
  assert.match(router, /path: 'accounting'.*ProjectFinancePage/);
});
