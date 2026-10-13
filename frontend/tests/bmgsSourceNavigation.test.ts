import assert from 'node:assert/strict';
import test from 'node:test';
import { canOpenRecord, loadSourceRecord, recordPath, recordRead, RecordUnavailable, type RecordTarget } from '../src/modules/_shared/recordNavigation.ts';
import { parseBMGSRevision, resolveBMGSContractVariation } from '../src/modules/_shared/bmgsSourceNavigation.ts';
const project = '10000000-0000-4000-8000-000000000001';
const source = '10000000-0000-4000-8000-000000000002';
const contract = '10000000-0000-4000-8000-000000000003';
const other = '10000000-0000-4000-8000-000000000004';
test('daily and action generated links preserve exact source ID/project and choose canonical owner tab', () => {
  for (const kind of ['daily_report', 'action'] as const) {
    const url = new URL(recordPath({kind, id: source, projectId: project})!, 'https://local.invalid');
    assert.equal(url.pathname, `/projects/${project}/operations`);
    assert.equal(url.searchParams.get('record'), source); assert.equal(url.searchParams.get('record_type'), kind);
    assert.equal(url.searchParams.get('tab'), kind === 'action' ? 'actions' : 'timeline');
    assert.equal(canOpenRecord({kind, id: source, projectId: project}, () => false), false);
    assert.equal(canOpenRecord({kind, id: source, projectId: project}, p => ['project.view', 'operation.view'].includes(p)), true);
  }
});
test('daily source exact scoped collection resolution rejects wrong project/deleted/stale IDs and surfaces fetch failure', async () => {
  const target: RecordTarget = {kind: 'daily_report', id: source, projectId: project};
  assert.deepEqual(recordRead(target), {path: `/projects/${project}/daily-reports`, collection: 'array'});
  const row = {id: source, project_id: project, status: 'draft'};
  assert.equal(await loadSourceRecord(target, async () => [row]), row);
  for (const payload of [[], [{...row, id: other}], [{...row, project_id: other}], [{...row, status: 'deleted'}]]) await assert.rejects(loadSourceRecord(target, async () => payload), RecordUnavailable);
  await assert.rejects(loadSourceRecord(target, async () => {throw new Error('transport');}), /transport/);
});
test('action opens only exact scoped workspace action; unrelated report/check entries cannot match', async () => {
  const target: RecordTarget = {kind: 'action', id: source, projectId: project};
  assert.deepEqual(recordRead(target), {path: `/projects/${project}/construction-workspace`, collection: 'actions'});
  const row = {id: source, project_id: project, status: 'open'};
  assert.equal(await loadSourceRecord(target, async () => ({actions: [row], checks: []})), row);
  for (const payload of [{actions: [], checks: [row]}, {actions: [{...row, project_id: other}]}, {actions: [{...row, status: 'cancelled'}]}]) await assert.rejects(loadSourceRecord(target, async () => payload), RecordUnavailable);
});
test('contract variation query resolves actual parent membership and rejects stale/wrong-parent/wrong-project references', () => {
  const params = new URLSearchParams({record: contract, record_type: 'contract', tab: 'variations', variation: source});
  const variation = {id: source, contract_id: contract, status: 'draft'};
  const parent = {id: contract, project_id: project, variations: [variation]};
  assert.equal(resolveBMGSContractVariation(params, parent, project), variation);
  assert.throws(() => resolveBMGSContractVariation(params, {...parent, project_id: other}, project), RecordUnavailable);
  assert.throws(() => resolveBMGSContractVariation(params, {...parent, variations: [{...variation, contract_id: other}]}, project), RecordUnavailable);
  assert.throws(() => resolveBMGSContractVariation(params, {...parent, variations: []}, project), RecordUnavailable);
  assert.throws(() => resolveBMGSContractVariation(new URLSearchParams({record: contract, record_type: 'contract', variation: 'PL-01'}), parent, project), RecordUnavailable);
  assert.equal(resolveBMGSContractVariation(new URLSearchParams(), parent, project), null);
});
test('historical revision query accepts only exact positive integers without fallback coercion', () => {
  assert.equal(parseBMGSRevision(null), undefined); assert.equal(parseBMGSRevision('2'), 2);
  for (const value of ['', '0', '-1', '2.5', '02', '2e1', ' 2', '999999999999999999999']) assert.equal(parseBMGSRevision(value), null);
});
