import clsx from 'clsx';

export function payableAgingLabel(bucket?: string, overdueDays?: number) {
  return debtAgingLabel(bucket, overdueDays);
}

export function debtAgingLabel(bucket?: string, overdueDays?: number) {
  if (bucket === 'settled') return 'Đã tất toán';
  if (bucket === 'no_due_date') return 'Chưa có hạn';
  if (bucket === 'not_due') return 'Chưa đến hạn';
  const days = Number(overdueDays ?? 0);
  if (days > 0) return `Quá hạn ${days} ngày`;
  return '-';
}

export function agingBadgeClass(bucket?: string) {
  return clsx(
    'inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold',
    bucket === 'settled' && 'bg-emerald-50 text-emerald-700',
    bucket === 'not_due' && 'bg-blue-50 text-blue-700',
    bucket === 'no_due_date' && 'bg-slate-100 text-slate-600',
    bucket === '1_30' && 'bg-amber-50 text-amber-700',
    bucket === '31_60' && 'bg-orange-50 text-orange-700',
    (bucket === '61_90' || bucket === '90_plus') && 'bg-rose-50 text-rose-700',
    !bucket && 'bg-slate-100 text-slate-600',
  );
}

export function reconciliationLabel(status?: string, issueCount?: number) {
  const count = Number(issueCount ?? 0);
  if (status === 'needs_review' || count > 0) return `${count || 1} cảnh báo`;
  return 'OK';
}

export function reconciliationClass(status?: string, issueCount?: number) {
  const count = Number(issueCount ?? 0);
  return clsx(
    'inline-flex whitespace-nowrap rounded-full px-2 py-1 text-xs font-semibold',
    status === 'needs_review' || count > 0 ? 'bg-amber-50 text-amber-700' : 'bg-emerald-50 text-emerald-700',
  );
}

export function warningSeverityClass(severity?: string) {
  return clsx(
    'rounded-md border px-3 py-2 text-sm',
    severity === 'critical' && 'border-rose-100 bg-rose-50 text-rose-700',
    severity === 'warning' && 'border-amber-100 bg-amber-50 text-amber-700',
    severity === 'info' && 'border-blue-100 bg-blue-50 text-blue-700',
    !severity && 'border-slate-100 bg-slate-50 text-slate-700',
  );
}
