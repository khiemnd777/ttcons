import assert from 'node:assert/strict';
import test from 'node:test';
import { canOpenRecord, loadSourceRecord, recordPath, recordRead, relatedRecordTargets, selectSourceRecord, typedRecordTarget, type RecordKind, type RecordTarget } from '../src/modules/_shared/recordNavigation.ts';

const id = '11111111-1111-4111-8111-111111111111';
const projectId = '22222222-2222-4222-8222-222222222222';
const other = '33333333-3333-4333-8333-333333333333';
const target = (kind: RecordKind): RecordTarget => ({ kind, id, projectId, workItemId: other });
const destinations: [RecordKind, string][] = [
  ['boq_item', '/boq'], ['procurement_request', '/procurement'], ['work_item', `/work-items/${id}`],
  ['acceptance', '/acceptance'], ['handover', '/handover'], ['stock_movement', `/source-records/stock_movement/${id}`],
  ['rework', '/rework'], ['supplier_quotation', '/boq'], ['stock_document', '/warehouse'], ['partner', '/core-data/output'],
  ['replanning', '/rework'], ['planning_daily_report', `/work-items/${other}`], ['meeting_minutes', `/work-items/${other}`],
  ['planning_checklist', `/work-items/${other}`], ['role', '/roles'], ['audit', '/audit'],
];
for (const [kind, suffix] of destinations) test(`${kind}: exact destination retains source identity`, () => {
  const url = new URL(recordPath(target(kind))!, 'http://localhost');
  assert.ok(url.pathname.endsWith(suffix), url.href);
  if (!url.pathname.includes('/work-items/') && !url.pathname.includes('/source-records/')) {
    assert.equal(url.searchParams.get('record'), id);
    assert.equal(url.searchParams.get('record_type'), kind);
  }
  assert.equal(url.searchParams.has('task'), false);
  assert.equal(url.searchParams.has('receipt'), false, 'opening a source must not start a receipt form');
});

test('invalid IDs and URLs never produce navigation or data reads', async () => {
  for (const bad of ['', 'MAU-KT-MS-01', '../other', 'javascript:alert(1)', `${id}?edit=true`]) {
    assert.equal(recordPath({ kind: 'boq_item', id: bad, projectId }), null);
    let reads = 0;
    await assert.rejects(loadSourceRecord({ kind: 'boq_item', id: bad, projectId }, async () => { reads++; return {}; }));
    assert.equal(reads, 0);
  }
  assert.equal(recordPath({ kind: 'boq_item', id, projectId: 'wrong' }), null);
  assert.equal(recordPath({ kind: 'planning_checklist', id, projectId }), null, 'form ID is not a work item ID');
  assert.equal(typedRecordTarget('unknown_legacy_type', id, projectId), null);
});

test('exact ID selection never falls back to the first row or crosses projects', () => {
  const t = target('rework');
  const exact = { id, project_id: projectId, status: 'approved' };
  assert.equal(selectSourceRecord(t, [{ ...exact, id: other }, exact]), exact);
  for (const rows of [[], [{ ...exact, id: other }], [{ ...exact, project_id: other }], [{ ...exact, status: 'cancelled' }], [{ ...exact, deleted_at: '2026-01-01' }]]) {
    assert.throws(() => selectSourceRecord(t, rows));
  }
  assert.throws(() => selectSourceRecord(target('acceptance'), { ...exact, id: other }));
  assert.throws(() => selectSourceRecord(target('boq_item'), { ...exact, status: 'deleted' }));
});

test('audit uses exact server filtering before pagination and validates the result', () => {
  const t = target('audit');
  assert.match(recordRead(t).path, new RegExp(`record_id=${id}`));
  assert.equal(selectSourceRecord(t, { items: [{ id, project_id: projectId }] }).id, id);
  assert.throws(() => selectSourceRecord(t, { items: [{ id: other, project_id: projectId }] }));
});

test('stock movement, stock document and incoming acceptance keep distinct identities', () => {
  assert.notEqual(recordPath(target('stock_movement')), recordPath(target('stock_document')));
  const refs = relatedRecordTargets('stock_movement', { id, project_id: projectId, stock_document_id: other, reference_type: 'incoming_acceptance', reference_id: id });
  assert.equal(refs.find(ref => ref.kind === 'stock_document')?.id, other);
  assert.equal(refs.find(ref => ref.kind === 'incoming_acceptance')?.id, id);
  assert.match(recordRead(target('incoming_acceptance')).path, /\/incoming-acceptances\//);
});

test('quotation source uses its parent ID, never a quotation item ID or display code', () => {
  const row = { id, project_id: projectId, selected_quotation_item_id: other, selected_quotation_code: 'BG-01', order_reference: 'PO-01' };
  assert.deepEqual(relatedRecordTargets('boq_item', row), []);
  assert.equal(relatedRecordTargets('boq_item', { ...row, selected_quotation_id: id })[0].id, id);
});

test('links respect destination permission and excluded accounting-observer access', () => {
  const permissions = (...keys: string[]) => (key: string) => keys.includes(key);
  assert.equal(canOpenRecord(target('procurement_request'), permissions('project.view')), false);
  assert.equal(canOpenRecord(target('procurement_request'), permissions('procurement.view')), false);
  assert.equal(canOpenRecord(target('procurement_request'), permissions('project.view', 'procurement.view')), true);
  assert.equal(canOpenRecord(target('stock_document'), permissions('project.view', 'material.view')), false);
  assert.equal(canOpenRecord(target('audit'), permissions('project.view', 'audit.view', 'project.reconciliation.view')), false);
  assert.equal(canOpenRecord(target('incoming_acceptance'), permissions('warehouse.view')), true);
});

test('related records deduplicate shared IDs without creating a self link', () => {
  const refs = relatedRecordTargets('work_item', { id, project_id: projectId, work_item_id: id, source_work_item_id: other, source_type: 'work_item', source_id: other });
  assert.equal(refs.length, 1);
  assert.equal(refs[0].id, other);
});
