export { dateTimeText as eventLogDateTime } from '../../lib/utils/format.ts';

export type EventLogCategory = 'unexpected_5xx' | 'panic' | 'controlled_503';
export type EventLogDatePreset = '7' | '30' | '90' | 'custom';

export const eventLogCategoryLabels: Record<EventLogCategory, string> = {
  unexpected_5xx: '500 bất ngờ',
  panic: 'Panic',
  controlled_503: '503 có kiểm soát',
};

export function eventLogCategoryLabel(value: string) {
  return eventLogCategoryLabels[value as EventLogCategory] ?? value;
}

export function eventLogCategoryTone(value: string) {
  if (value === 'panic') return 'border-violet-200 bg-violet-50 text-violet-700';
  if (value === 'controlled_503') return 'border-amber-200 bg-amber-50 text-amber-700';
  return 'border-rose-200 bg-rose-50 text-rose-700';
}

export function eventLogDateRange(preset: EventLogDatePreset, customFrom: string, customTo: string, now = new Date()) {
  if (preset === 'custom') return { from: customFrom, to: customTo };
  const to = new Date(now);
  const from = new Date(to);
  from.setDate(from.getDate() - Number(preset) + 1);
  return { from: localDate(from), to: localDate(to) };
}

export function shortEventLogID(value: string) {
  return value.length > 20 ? `${value.slice(0, 10)}…${value.slice(-6)}` : value;
}

function localDate(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
