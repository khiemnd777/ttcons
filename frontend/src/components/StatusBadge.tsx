import clsx from 'clsx';
import { statusLabel } from '../lib/utils/status';

const toneByStatus: Record<string, string> = {
  active: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  inactive: 'bg-slate-100 text-slate-500 ring-slate-200',
  draft: 'bg-slate-100 text-slate-700 ring-slate-200',
  submitted: 'bg-sky-50 text-sky-700 ring-sky-200',
  confirmed: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  handed_to_accounting: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
  processed: 'bg-teal-50 text-teal-700 ring-teal-200',
  waiting_approval: 'bg-amber-50 text-amber-800 ring-amber-200',
  waiting_review: 'bg-amber-50 text-amber-800 ring-amber-200',
  waiting_evidence: 'bg-violet-50 text-violet-700 ring-violet-200',
  waiting_confirmation: 'bg-amber-50 text-amber-800 ring-amber-200',
  pending: 'bg-amber-50 text-amber-800 ring-amber-200',
  approved: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  verified: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  issued: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  accepted: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  published: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  ordered: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
  in_progress: 'bg-blue-50 text-blue-700 ring-blue-200',
  planning: 'bg-blue-50 text-blue-700 ring-blue-200',
  checking: 'bg-blue-50 text-blue-700 ring-blue-200',
  blocked: 'bg-rose-50 text-rose-700 ring-rose-200',
  rejected: 'bg-rose-50 text-rose-700 ring-rose-200',
  needs_changes: 'bg-orange-50 text-orange-800 ring-orange-200',
  changes_requested: 'bg-orange-50 text-orange-800 ring-orange-200',
  fail: 'bg-rose-50 text-rose-700 ring-rose-200',
  completed: 'bg-teal-50 text-teal-700 ring-teal-200',
  done: 'bg-teal-50 text-teal-700 ring-teal-200',
  received: 'bg-teal-50 text-teal-700 ring-teal-200',
  partially_received: 'bg-teal-50 text-teal-700 ring-teal-200',
  review: 'bg-amber-50 text-amber-700 ring-amber-200',
  signed: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
  suspended: 'bg-orange-50 text-orange-700 ring-orange-200',
  eligible: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
  obligation_opened: 'bg-blue-50 text-blue-700 ring-blue-200',
  held: 'bg-slate-100 text-slate-700 ring-slate-200',
  partially_released: 'bg-amber-50 text-amber-700 ring-amber-200',
  released: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  incorporated: 'bg-violet-50 text-violet-700 ring-violet-200',
  superseded: 'bg-slate-100 text-slate-600 ring-slate-200',
  scheduled: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
  inspected: 'bg-blue-50 text-blue-700 ring-blue-200',
  conditional: 'bg-amber-50 text-amber-700 ring-amber-200',
  root_cause: 'bg-orange-50 text-orange-700 ring-orange-200',
  corrective_action: 'bg-amber-50 text-amber-700 ring-amber-200',
  reinspection: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
  skipped: 'bg-slate-100 text-slate-600 ring-slate-200',
  paid: 'bg-teal-50 text-teal-700 ring-teal-200',
  partially_paid: 'bg-teal-50 text-teal-700 ring-teal-200',
  settled: 'bg-teal-50 text-teal-700 ring-teal-200',
  partially_settled: 'bg-amber-50 text-amber-800 ring-amber-200',
  handed_over: 'bg-teal-50 text-teal-700 ring-teal-200',
  recorded: 'bg-sky-50 text-sky-700 ring-sky-200',
  revised: 'bg-violet-50 text-violet-700 ring-violet-200',
  overdue: 'bg-red-50 text-red-700 ring-red-200',
  voided: 'bg-slate-100 text-slate-500 ring-slate-200',
  cancelled: 'bg-slate-100 text-slate-500 ring-slate-200',
  deleted: 'bg-slate-100 text-slate-500 ring-slate-200',
};

export function StatusBadge({ status, className }: { status?: string; className?: string }) {
  return (
    <span
      className={clsx(
        'inline-flex items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset',
        toneByStatus[status ?? ''] ?? 'bg-slate-100 text-slate-700 ring-slate-200',
        className,
      )}
    >
      {statusLabel(status)}
    </span>
  );
}
