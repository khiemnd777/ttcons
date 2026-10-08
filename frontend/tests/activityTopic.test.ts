import assert from 'node:assert/strict';
import test from 'node:test';
import {
  activityTopic, activityTopicPath, activityTopicRecord, activityVisibilityLabel, ActivityTopicUnavailable,
} from '../src/modules/work-items/activityTopicDomain.ts';

const projectId = '74c409ae-eb91-434a-baa8-7b3fa4422b9a';
const otherProject = '84c409ae-eb91-434a-baa8-7b3fa4422b9a';
const recordId = '11111111-1111-4111-8111-111111111111';
const otherRecord = '22222222-2222-4222-8222-222222222222';
const boq = activityTopic({ scope_type: 'boq_item', scope_id: recordId, target_label: 'Sơn nội thất [MẪU]' }, projectId)!;

test('a topic identifies the selected source, never its title or an associated work item', () => {
  assert.equal(boq.label, 'Sơn nội thất [MẪU]');
  assert.equal(activityTopicPath(boq), `/boq-items/${recordId}`);
  const linked = activityTopic({ scope_type: 'boq_item', scope_id: recordId, work_item_id: otherRecord }, projectId)!;
  assert.equal(linked.id, recordId);
  assert.equal(activityTopic({ scope_type: 'boq_item', scope_id: recordId, work_item_title: 'Đầu việc khác' }, projectId)?.label, 'Dòng BOQ');
  assert.equal(activityTopic({ scope_type: 'boq_item', work_item_id: otherRecord }, projectId), null);
  assert.equal(activityTopic({ work_item_id: recordId }, projectId)?.scope, 'work_item');
});

test('general project messages have no redundant topic; malformed and unsupported links fail closed', () => {
  assert.equal(activityTopic({ scope_type: 'project', scope_id: projectId }, projectId), null);
  assert.equal(activityTopic({ scope_type: 'project', scope_id: projectId, work_item_id: recordId }, projectId), null);
  assert.equal(activityTopic({}, projectId), null);
  for (const scope_type of ['__proto__', 'constructor', 'unknown', 'https://example.com']) {
    assert.equal(activityTopic({ scope_type, scope_id: recordId }, projectId), null);
  }
  assert.equal(activityTopic({ scope_type: 'boq_item', scope_id: '../other' }, projectId), null);
  assert.equal(activityTopic({ scope_type: 'boq_item', scope_id: recordId }, 'bad-project'), null);
});

test('every selectable source uses its actual read API, with legacy list semantics kept explicit', () => {
  const paths = {
    work_item: `/work-items/${recordId}`, boq_item: `/boq-items/${recordId}`,
    procurement_request: `/procurement-requests/${recordId}`, acceptance: `/acceptances/${recordId}`,
    handover: `/handover/${recordId}`, rework: `/projects/${projectId}/reworks`,
    stock_movement: `/stock/movements?project_id=${projectId}`,
  };
  for (const [scope_type, path] of Object.entries(paths)) {
    const topic = activityTopic({ scope_type, scope_id: recordId }, projectId)!;
    assert.equal(activityTopicPath(topic), path, scope_type);
  }
});

test('detail reads reject mismatched project/record and unavailable source rows before rendering', () => {
  const record = { id: recordId, project_id: projectId, status: 'approved', quantity: 25 };
  assert.equal(activityTopicRecord(boq, record), record);
  for (const invalid of [null, [], {}, { ...record, id: otherRecord }, { ...record, project_id: otherProject },
    { ...record, status: 'cancelled' }, { ...record, status: 'deleted' }, { ...record, deleted_at: '2026-09-05' }]) {
    assert.throws(() => activityTopicRecord(boq, invalid), ActivityTopicUnavailable);
  }
});

test('stock list lookup requires both exact record and project even when titles match', () => {
  const topic = activityTopic({ scope_type: 'stock_movement', scope_id: recordId }, projectId)!;
  const selected = { id: recordId, project_id: projectId, status: 'approved', movement_code: 'KHO-01' };
  const rows = [{ ...selected, id: otherRecord }, { ...selected, project_id: otherProject }, selected];
  assert.equal(activityTopicRecord(topic, rows), selected);
  assert.throws(() => activityTopicRecord(topic, rows.slice(0, 2)), ActivityTopicUnavailable);
  assert.throws(() => activityTopicRecord(topic, { items: rows }), ActivityTopicUnavailable);
  assert.throws(() => activityTopicRecord(topic, []), ActivityTopicUnavailable);
});

test('scope display omits the project default but keeps historical audience distinctions', () => {
  assert.equal(activityVisibilityLabel('project'), null);
  assert.equal(activityVisibilityLabel(undefined), null);
  assert.equal(activityVisibilityLabel('internal'), 'Nội bộ');
  assert.equal(activityVisibilityLabel('client'), 'Chủ đầu tư');
  assert.equal(activityVisibilityLabel('future_scope'), 'future_scope');
});
