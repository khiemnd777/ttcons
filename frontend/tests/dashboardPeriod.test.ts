import assert from 'node:assert/strict';
import test from 'node:test';
import {
  convertDashboardPeriod,
  dashboardCurrentPeriod,
  dashboardPeriodFromSearchParams,
  dashboardPeriodLabel,
  dashboardPeriodQuery,
  moveDashboardPeriod,
} from '../src/modules/dashboard/dashboardPeriod.ts';

const now = new Date('2026-08-18T01:00:00.000Z');

test('uses the current month in the Dashboard business timezone by default', () => {
  assert.deepEqual(dashboardCurrentPeriod('month', now), { periodType: 'month', year: 2026, periodNo: 8 });
  assert.deepEqual(dashboardPeriodFromSearchParams(new URLSearchParams(), now), { periodType: 'month', year: 2026, periodNo: 8 });
});

test('rejects invalid and future periods from the URL', () => {
  assert.deepEqual(
    dashboardPeriodFromSearchParams(new URLSearchParams('period_type=quarter&year=2026&period_no=4'), now),
    { periodType: 'quarter', year: 2026, periodNo: 3 },
  );
  assert.deepEqual(
    dashboardPeriodFromSearchParams(new URLSearchParams('period_type=month&year=2026&period_no=13'), now),
    { periodType: 'month', year: 2026, periodNo: 8 },
  );
});

test('moves periods across year boundaries and converts around the anchor month', () => {
  assert.deepEqual(moveDashboardPeriod({ periodType: 'month', year: 2026, periodNo: 1 }, -1), { periodType: 'month', year: 2025, periodNo: 12 });
  assert.deepEqual(moveDashboardPeriod({ periodType: 'quarter', year: 2026, periodNo: 4 }, 1), { periodType: 'quarter', year: 2027, periodNo: 1 });
  assert.deepEqual(convertDashboardPeriod({ periodType: 'quarter', year: 2026, periodNo: 3 }, 'month'), { periodType: 'month', year: 2026, periodNo: 7 });
});

test('serializes and labels semantic Dashboard periods', () => {
  assert.equal(dashboardPeriodQuery({ periodType: 'month', year: 2026, periodNo: 8 }), 'period_type=month&year=2026&period_no=8');
  assert.equal(dashboardPeriodQuery({ periodType: 'year', year: 2026, periodNo: 0 }), 'period_type=year&year=2026');
  assert.equal(dashboardPeriodLabel({ periodType: 'quarter', year: 2026, periodNo: 3 }), 'Quý 3/2026');
});
