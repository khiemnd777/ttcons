import assert from 'node:assert/strict';
import test from 'node:test';
import { workReportReviewCapabilities } from '../src/modules/work-reports/workReportReviewCapabilities.ts';

const context = { current: true, ready: true, status: 'acknowledged', canReview: true, canManage: false, recipientID: 'pm', userID: 'deputy' };

test('co-reviewer can acknowledge the current revision after PM has already read it', () => {
  assert.deepEqual(workReportReviewCapabilities({ ...context, capabilities: { can_acknowledge: true, can_request_changes: true, can_discuss: true } }), { acknowledge: true, requestChanges: true, discuss: true });
});

test('server decisions keep read acknowledgement separate from change requests', () => {
  assert.deepEqual(workReportReviewCapabilities({ ...context, capabilities: { can_acknowledge: false, can_request_changes: true, can_discuss: true } }), { acknowledge: false, requestChanges: true, discuss: true });
});

test('historical revisions cannot expose current-review decisions', () => {
  const result = workReportReviewCapabilities({ ...context, current: false, capabilities: { can_acknowledge: true, can_request_changes: true, can_discuss: true } });
  assert.equal(result.acknowledge, false);
  assert.equal(result.requestChanges, false);
});

test('loading and errors hide stale decisions and discussion', () => {
  assert.deepEqual(workReportReviewCapabilities({ ...context, ready: false, capabilities: { can_acknowledge: true, can_request_changes: true, can_discuss: true } }), { acknowledge: false, requestChanges: false, discuss: false });
});

test('legacy responses grant review only to the original recipient or manager', () => {
  assert.equal(workReportReviewCapabilities({ ...context, status: 'submitted' }).acknowledge, false);
  assert.equal(workReportReviewCapabilities({ ...context, status: 'submitted', userID: 'pm' }).acknowledge, true);
  assert.equal(workReportReviewCapabilities({ ...context, canManage: true }).acknowledge, false);
  assert.equal(workReportReviewCapabilities({ ...context, status: 'submitted', canManage: true }).acknowledge, true);
});
