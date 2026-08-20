import assert from 'node:assert/strict';
import test from 'node:test';
import { resolveRequestID } from '../src/lib/api/requestID.ts';
import {
  eventLogCategoryLabel,
  eventLogDateRange,
  shortEventLogID,
} from '../src/modules/event-logs/eventLogPresentation.ts';

test('event log categories use business-facing labels', () => {
  assert.equal(eventLogCategoryLabel('unexpected_5xx'), '500 bất ngờ');
  assert.equal(eventLogCategoryLabel('panic'), 'Panic');
  assert.equal(eventLogCategoryLabel('controlled_503'), '503 có kiểm soát');
});

test('default event log ranges are inclusive and local-date based', () => {
  assert.deepEqual(eventLogDateRange('7', '', '', new Date(2026, 7, 20, 12)), {
    from: '2026-08-14',
    to: '2026-08-20',
  });
  assert.deepEqual(eventLogDateRange('custom', '2026-08-01', '2026-08-05'), {
    from: '2026-08-01',
    to: '2026-08-05',
  });
});

test('request ID prefers the response header and falls back to envelope details', () => {
  assert.equal(resolveRequestID({ request_id: 'details-id' }, 'header-id'), 'header-id');
  assert.equal(resolveRequestID({ request_id: 'details-id' }, null), 'details-id');
  assert.equal(resolveRequestID({ request_id: 123 }, null), undefined);
  assert.equal(shortEventLogID('123456789012345678901234567890'), '1234567890…567890');
});
