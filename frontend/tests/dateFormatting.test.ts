import assert from 'node:assert/strict';
import test from 'node:test';
import { dateText, dateTimeText, timeText } from '../src/lib/utils/format.ts';
import { groupActivitiesByDay } from '../src/modules/work-items/activityThreadDomain.ts';
import type { WorkItemActivity } from '../src/lib/types/domain';

test('absolute dates use padded full dates and minute precision in Vietnam time', () => {
  const value = '2026-09-03T07:26:55.123456789Z';
  assert.equal(dateText(value), '03-09-2026');
  assert.equal(timeText(value), '14:26');
  assert.equal(dateTimeText(value), '14:26 03-09-2026');
  assert.equal(dateTimeText(new Date(value)), '14:26 03-09-2026');
  assert.equal(dateTimeText(Date.parse(value)), '14:26 03-09-2026');
  assert.notEqual(dateTimeText(0), '-');
});

test('calendar dates and Vietnam midnight are independent of the host timezone', () => {
  assert.equal(dateText('2026-09-03'), '03-09-2026');
  assert.equal(dateTimeText('2026-09-03'), '03-09-2026');
  assert.equal(timeText('2026-09-03'), '-');
  assert.equal(dateTimeText('2026-09-03T16:59:59Z'), '23:59 03-09-2026');
  assert.equal(dateTimeText('2026-09-03T17:00:00Z'), '00:00 04-09-2026');
  assert.equal(dateTimeText('2026-09-03T14:26:55+07:00'), '14:26 03-09-2026');
  assert.equal(dateTimeText('2026-09-03T14:26'), '14:26 03-09-2026');
  assert.equal(dateTimeText('2026-09-03 14:26:55'), '14:26 03-09-2026');
  assert.equal(dateText('2024-02-29'), '29-02-2024');
});

test('time-only values keep wall time, omit seconds and reject invalid clocks', () => {
  assert.equal(timeText('00:00'), '00:00');
  assert.equal(timeText('14:26:55'), '14:26');
  assert.equal(timeText('23:59:59.123'), '23:59');
  for (const value of ['24:00', '10:60', '14:26:60']) assert.equal(timeText(value), '-');
});

test('missing or invalid values return safe contextual placeholders without rolling dates', () => {
  for (const value of [undefined, null, '', 'invalid', '2026-02-29', '2026-02-30T14:00Z', '2026-13-01', '2026-09-03T24:00', new Date('invalid'), Infinity, {}]) {
    assert.equal(dateText(value), '-');
    assert.equal(timeText(value), '-');
    assert.equal(dateTimeText(value), '-');
  }
  assert.equal(dateText(undefined, 'Chưa chốt'), 'Chưa chốt');
  assert.equal(dateTimeText('invalid', 'Không rõ thời gian'), 'Không rõ thời gian');
});

test('shared activity grouping uses the same Vietnam calendar date as its visible label', () => {
  const items = [
    { id: 'a', created_at: '2020-01-02T01:00:00Z' },
    { id: 'b', created_at: '2020-01-01T17:00:00Z' },
    { id: 'c', created_at: '2020-01-01T16:59:59Z' },
  ] as WorkItemActivity[];
  const groups = groupActivitiesByDay(items);
  assert.deepEqual(groups.map(({ label, items }) => [label, items.map(({ id }) => id)]), [
    ['02-01-2020', ['a', 'b']], ['01-01-2020', ['c']],
  ]);
});
