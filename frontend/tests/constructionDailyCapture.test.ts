import assert from 'node:assert/strict';
import test from 'node:test';
import { emptyMaterialArrival, emptyNextPlan, emptyBlockerCapture, loadDailyWorkRows, serializeDailyWorkRows, validateDailyWorkRows, loadDailyCapture, serializeDailyCapture, validateDailyCapture } from '../src/modules/operations/constructionDailyCapture.ts';

test('new daily reports keep unknown assessments and absent arrivals out of saved facts', () => {
  const capture = loadDailyCapture({});
  assert.equal(capture.assessments.length, 12);
  assert.deepEqual(serializeDailyCapture(capture.assessments, [emptyMaterialArrival()]), { supervision_assessments: [], material_arrival_items: [] });
});
test('native saved assessments and arrival identities round trip independently from material usage', () => {
  const capture = loadDailyCapture({ supervision_assessments: [{ code: 'safety', assessment: 'fail', note: 'Thiếu lan can' }], material_arrival_items: [{ row_id: 'arrival-1', material_name: 'Cát', quantity: 0, unit: 'm³' }], material_usage_items: [{ material_name: 'Xi măng', quantity: 2 }] });
  const payload = serializeDailyCapture(capture.assessments, capture.arrivals);
  assert.deepEqual(payload.supervision_assessments, [{ code: 'safety', assessment: 'fail', note: 'Thiếu lan can' }]);
  assert.equal(payload.material_arrival_items[0].row_id, 'arrival-1');
  assert.equal(payload.material_arrival_items[0].quantity, 0);
  assert.equal(payload.material_arrival_items[0].material_name, 'Cát');
});
test('optional arrival quantity stays missing; invalid supplied values fail validation', () => {
  const row = { ...emptyMaterialArrival(), material_name: 'Thép' };
  assert.equal(validateDailyCapture([row]), '');
  assert.equal(serializeDailyCapture([], [row]).material_arrival_items[0].quantity, null);
  assert.ok(validateDailyCapture([{ ...row, quantity: '-1' }]));
  assert.ok(validateDailyCapture([{ ...row, quantity: 'Infinity' }]));
  assert.ok(validateDailyCapture([{ ...row, material_name: '', origin: 'Kho' }]));
});

test('supplied arrival quantity including zero requires a unit; omitted quantity has no unit gate', () => {
  const row = { ...emptyMaterialArrival(), material_name: 'Cát' };
  assert.equal(validateDailyCapture([{ ...row, quantity: '0' }]), 'Đơn vị vật tư nhập trường khi có số lượng');
  assert.equal(validateDailyCapture([{ ...row, quantity: '2', unit: 'm³' }]), '');
  assert.equal(validateDailyCapture([{ ...row, quantity: '', unit: '' }]), '');
});

test('native plans and risks load duplicate titles without losing identity or saved facts', () => {
  const data = loadDailyWorkRows({ next_plan_items: [{ row_id: 'plan-a', title: 'Lắp cửa', receiver: 'An', progress_percent: 0 }, { row_id: 'plan-b', title: 'Lắp cửa', receiver: 'Bình', progress_percent: null }], blockers: [{ row_id: 'risk-a', content: 'Thiếu cửa', owner: 'Lan', cause: 'Chậm giao', detected_at: '2026-10-08' }] }, ['Lắp cửa'], ['Thiếu cửa']);
  assert.equal(data.nextPlans.length, 2);
  assert.deepEqual(data.nextPlans.map(row => row.row_id), ['plan-a', 'plan-b']);
  assert.equal(data.nextPlans[0].progress_percent, '0');
  assert.equal(data.nextPlans[1].progress_percent, '');
  assert.equal(data.blockers[0].cause, 'Chậm giao');
  const payload = serializeDailyWorkRows(data.nextPlans.filter(row => row.row_id !== 'plan-a'), data.blockers.map(row => ({ ...row, owner: '', cause: '' })));
  assert.equal(payload.next_items.length, 1);
  assert.equal(payload.next_items[0].row_id, 'plan-b');
  assert.equal(payload.next_items[0].progress_percent, null);
  assert.equal(payload.blockers[0].owner, '');
  assert.equal(payload.blockers[0].cause, '');
});
test('new native work rows filter blank placeholders, retain zero and explicit clears', () => {
  const plan = { ...emptyNextPlan('Lắp cửa'), progress_percent: '0', receiver: 'An', issued_date: '2026-10-08' };
  const risk = { ...emptyBlockerCapture('Thiếu cửa'), severity: 'high', owner: 'Lan' };
  assert.equal(validateDailyWorkRows([plan], [risk]).next_items, undefined);
  const payload = serializeDailyWorkRows([emptyNextPlan(), plan], [emptyBlockerCapture(), risk]);
  assert.equal(payload.next_items.length, 1);
  assert.equal(payload.blockers.length, 1);
  assert.equal(payload.next_items[0].progress_percent, 0);
  assert.equal(payload.next_items[0].row_id, plan.row_id);
  assert.equal(payload.blockers[0].row_id, risk.row_id);
});
test('supplied row detail errors are keyed to exact native row before save/upload', () => {
  const plan = { ...emptyNextPlan(), receiver: 'An', progress_percent: '101', issued_date: '2026-02-30' };
  const risk = { ...emptyBlockerCapture(), owner: 'Lan', severity: 'fatal', target_date: '9999-99-99' };
  const errors = validateDailyWorkRows([plan], [risk]);
  for (const key of ['title', 'progress_percent', 'issued_date']) assert.ok(errors[`next_${plan.row_id}_${key}`]);
  for (const key of ['content', 'severity', 'target_date']) assert.ok(errors[`blocker_${risk.row_id}_${key}`]);
  assert.ok(validateDailyWorkRows([{ ...plan, title: 'A'.repeat(181) }], [])[`next_${plan.row_id}_title`]);
});
