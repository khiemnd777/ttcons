import type { DashboardPeriod, DashboardPeriodType } from './dashboardTypes';

const dashboardTimeZone = 'Asia/Ho_Chi_Minh';

export function dashboardCurrentPeriod(periodType: DashboardPeriodType = 'month', now = new Date()): DashboardPeriod {
  const today = dashboardDateParts(now);
  return {
    periodType,
    year: today.year,
    periodNo: periodType === 'month' ? today.month : periodType === 'quarter' ? Math.floor((today.month - 1) / 3) + 1 : 0,
  };
}

export function dashboardPeriodFromSearchParams(params: URLSearchParams, now = new Date()): DashboardPeriod {
  const requestedType = params.get('period_type');
  const periodType: DashboardPeriodType = requestedType === 'quarter' || requestedType === 'year' ? requestedType : 'month';
  const current = dashboardCurrentPeriod(periodType, now);
  const year = integerParam(params.get('year')) ?? current.year;
  const periodNo = periodType === 'year' ? 0 : integerParam(params.get('period_no')) ?? current.periodNo;
  const candidate = { periodType, year, periodNo };
  return isValidDashboardPeriod(candidate) && !isFutureDashboardPeriod(candidate, now) ? candidate : current;
}

export function dashboardPeriodQuery(period: DashboardPeriod) {
  const params = new URLSearchParams();
  params.set('period_type', period.periodType);
  params.set('year', String(period.year));
  if (period.periodType !== 'year') params.set('period_no', String(period.periodNo));
  return params.toString();
}

export function moveDashboardPeriod(period: DashboardPeriod, offset: number): DashboardPeriod {
  if (period.periodType === 'year') {
    return { ...period, year: period.year + offset };
  }
  const unitsPerYear = period.periodType === 'month' ? 12 : 4;
  const zeroBased = period.year * unitsPerYear + period.periodNo - 1 + offset;
  const year = Math.floor(zeroBased / unitsPerYear);
  const periodNo = ((zeroBased % unitsPerYear) + unitsPerYear) % unitsPerYear + 1;
  return { ...period, year, periodNo };
}

export function convertDashboardPeriod(period: DashboardPeriod, periodType: DashboardPeriodType): DashboardPeriod {
  if (periodType === period.periodType) return period;
  const anchorMonth = period.periodType === 'month' ? period.periodNo : period.periodType === 'quarter' ? (period.periodNo - 1) * 3 + 1 : 1;
  return {
    periodType,
    year: period.year,
    periodNo: periodType === 'month' ? anchorMonth : periodType === 'quarter' ? Math.floor((anchorMonth - 1) / 3) + 1 : 0,
  };
}

export function dashboardPeriodLabel(period: DashboardPeriod) {
  if (period.periodType === 'month') return `Tháng ${String(period.periodNo).padStart(2, '0')}/${period.year}`;
  if (period.periodType === 'quarter') return `Quý ${period.periodNo}/${period.year}`;
  return `Năm ${period.year}`;
}

function isValidDashboardPeriod(period: DashboardPeriod) {
  if (!Number.isInteger(period.year) || period.year < 1900) return false;
  if (period.periodType === 'month') return Number.isInteger(period.periodNo) && period.periodNo >= 1 && period.periodNo <= 12;
  if (period.periodType === 'quarter') return Number.isInteger(period.periodNo) && period.periodNo >= 1 && period.periodNo <= 4;
  return period.periodNo === 0;
}

function isFutureDashboardPeriod(period: DashboardPeriod, now: Date) {
  const current = dashboardCurrentPeriod(period.periodType, now);
  if (period.year !== current.year) return period.year > current.year;
  return period.periodType !== 'year' && period.periodNo > current.periodNo;
}

function integerParam(value: string | null) {
  if (!value || !/^\d+$/.test(value)) return undefined;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) ? parsed : undefined;
}

function dashboardDateParts(date: Date) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: dashboardTimeZone,
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
  }).formatToParts(date);
  const value = (type: Intl.DateTimeFormatPartTypes) => Number(parts.find((part) => part.type === type)?.value ?? 0);
  return { year: value('year'), month: value('month'), day: value('day') };
}
