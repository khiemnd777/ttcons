export function currency(value: unknown) {
  const amount = Number(value ?? 0);
  return new Intl.NumberFormat('vi-VN', {
    style: 'currency',
    currency: 'VND',
    maximumFractionDigits: 0,
  }).format(Number.isFinite(amount) ? amount : 0);
}

const dateTimeFormatter = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'Asia/Ho_Chi_Minh',
  day: '2-digit', month: '2-digit', year: 'numeric',
  hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
});

function validCalendarDate(value: string) {
  const parsed = new Date(`${value}T00:00:00Z`);
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value;
}

function displayDateParts(value: unknown): { date: string; time?: string } | undefined {
  let date: Date;
  if (value instanceof Date) date = value;
  else if (typeof value === 'number') date = new Date(value);
  else if (typeof value === 'string') {
    const text = value.trim();
    if (/^\d{4}-\d{2}-\d{2}$/.test(text)) {
      // A business calendar date is not an instant and must never shift with the device timezone.
      return validCalendarDate(text) ? { date: text.split('-').reverse().join('-') } : undefined;
    }
    const match = text.match(/^(\d{4}-\d{2}-\d{2})[T ](\d{2}):(\d{2})(?::(\d{2})(?:\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?$/i);
    if (!match || !validCalendarDate(match[1]) || Number(match[2]) > 23 || Number(match[3]) > 59 || Number(match[4] ?? 0) > 59) return undefined;
    // Offset-free date-times are already wall time; do not let the device timezone reinterpret them.
    if (!match[5]) return { date: match[1].split('-').reverse().join('-'), time: `${match[2]}:${match[3]}` };
    date = new Date(text.replace(' ', 'T'));
  } else return undefined;
  if (!Number.isFinite(date.getTime())) return undefined;
  const parts = Object.fromEntries(dateTimeFormatter.formatToParts(date).map(({ type, value }) => [type, value]));
  return { date: `${parts.day}-${parts.month}-${parts.year.padStart(4, '0')}`, time: `${parts.hour}:${parts.minute}` };
}

/** Display only. Keep ISO values for inputs, API payloads, sorting and date arithmetic. */
export function dateText(value: unknown, fallback = '-') {
  return displayDateParts(value)?.date ?? fallback;
}

export function timeText(value: unknown, fallback = '-') {
  if (typeof value === 'string') {
    const time = value.trim().match(/^(\d{2}):(\d{2})(?::(\d{2})(?:\.\d+)?)?$/);
    if (time) return Number(time[1]) < 24 && Number(time[2]) < 60 && Number(time[3] ?? 0) < 60 ? `${time[1]}:${time[2]}` : fallback;
  }
  return displayDateParts(value)?.time ?? fallback;
}

export function dateTimeText(value: unknown, fallback = '-') {
  const parts = displayDateParts(value);
  if (!parts) return fallback;
  return parts.time ? `${parts.time} ${parts.date}` : parts.date;
}

export function percent(value: unknown) {
  return `${Math.round(Number(value ?? 0))}%`;
}
