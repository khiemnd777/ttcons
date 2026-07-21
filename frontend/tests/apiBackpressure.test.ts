import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isStableSSEEvent,
  nextSSEFailureCount,
  parseRetryAfter,
  reconnectDelayMs,
  shouldRetryQuery,
  streamStatusAfterFailure,
  SSE_OFFLINE_AFTER_ATTEMPTS,
  SSE_RECONNECT_MAX_DELAY_MS,
} from '../src/lib/api/backpressure.ts';

test('parseRetryAfter supports seconds and HTTP dates', () => {
  assert.equal(parseRetryAfter('12'), 12_000);
  assert.equal(parseRetryAfter('0.5'), 500);
  assert.equal(parseRetryAfter('Mon, 20 Jul 2026 10:00:10 GMT', Date.parse('Mon, 20 Jul 2026 10:00:00 GMT')), 10_000);
  assert.equal(parseRetryAfter('invalid'), undefined);
});

test('reconnectDelayMs uses exponential backoff, jitter, Retry-After and a cap', () => {
  assert.equal(reconnectDelayMs(0, undefined, () => 0.5), 2_000);
  assert.equal(reconnectDelayMs(3, undefined, () => 0.5), 16_000);
  assert.equal(reconnectDelayMs(0, 10_000, () => 0), 10_000);
  assert.equal(reconnectDelayMs(20, 120_000, () => 1), SSE_RECONNECT_MAX_DELAY_MS);
});

test('rapid open-close cycles keep increasing failures until stable stream evidence', () => {
  let failures = 0;
  failures = nextSSEFailureCount(failures, isStableSSEEvent('connected'));
  assert.equal(failures, 1);
  assert.equal(reconnectDelayMs(failures - 1, undefined, () => 0.5), 2_000);

  failures = nextSSEFailureCount(failures, isStableSSEEvent('connected'));
  assert.equal(failures, 2);
  assert.equal(reconnectDelayMs(failures - 1, undefined, () => 0.5), 4_000);

  failures = nextSSEFailureCount(failures, isStableSSEEvent('activity_changed'));
  assert.equal(failures, 1);
  assert.equal(reconnectDelayMs(failures - 1, undefined, () => 0.5), 2_000);
});

test('initial connected event is not stable evidence but heartbeat and activity are', () => {
  assert.equal(isStableSSEEvent('connected'), false);
  assert.equal(isStableSSEEvent('heartbeat'), true);
  assert.equal(isStableSSEEvent('activity_changed'), true);
});

test('stream status becomes offline at the consecutive failure threshold', () => {
  let failures = 0;
  for (let attempt = 0; attempt < SSE_OFFLINE_AFTER_ATTEMPTS; attempt += 1) {
    failures = nextSSEFailureCount(failures, isStableSSEEvent('connected'));
  }
  assert.equal(streamStatusAfterFailure(failures - 1), 'reconnecting');
  assert.equal(streamStatusAfterFailure(failures), 'offline');
});

test('shouldRetryQuery retries one transient failure but never HTTP 4xx', () => {
  assert.equal(shouldRetryQuery(0, new TypeError('network')), true);
  assert.equal(shouldRetryQuery(1, new TypeError('network')), false);
  assert.equal(shouldRetryQuery(0, { status: 503 }), true);
  assert.equal(shouldRetryQuery(0, { status: 429 }), false);
  assert.equal(shouldRetryQuery(0, { status: 401 }), false);
  assert.equal(shouldRetryQuery(0, { status: 404 }), false);
  assert.equal(shouldRetryQuery(0, { name: 'AbortError' }), false);
});
