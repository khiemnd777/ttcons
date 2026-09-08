import { type ReactNode } from 'react';
import { FileText } from 'lucide-react';
import './recordDetail.css';

/** Read-only record presentation. Business state and actions stay with the caller. */
export function RecordSummary({ eyebrow, title, subtitle, status, value, valueLabel, icon }: {
  eyebrow?: ReactNode; title: ReactNode; subtitle?: ReactNode; status?: ReactNode;
  value?: ReactNode; valueLabel?: string; icon?: ReactNode;
}) {
  return <header className="record-summary">
    <div className="record-summary-identity">
      <span className="record-summary-icon" aria-hidden="true">{icon ?? <FileText className="h-5 w-5" />}</span>
      <div className="min-w-0">
        {eyebrow ? <p className="mb-1 text-xs font-semibold tracking-wide text-slate-500">{eyebrow}</p> : null}
        <h3 className="break-words text-xl font-bold leading-snug text-slate-900">{title}</h3>
        {subtitle ? <div className="mt-2 break-words text-sm leading-6 text-slate-600">{subtitle}</div> : null}
      </div>
    </div>
    {status || value != null ? <div className="record-summary-aside">
      {status ? <div>{status}</div> : null}
      {value != null ? <div><div className="record-summary-value">{value}</div>{valueLabel ? <p className="mt-1 text-xs text-slate-500">{valueLabel}</p> : null}</div> : null}
    </div> : null}
  </header>;
}

export function RecordSection({ title, description, aside, children }: {
  title: string; description?: string; aside?: ReactNode; children: ReactNode;
}) {
  return <section className="min-w-0">
    <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2 border-b border-slate-200 pb-2">
      <div><h4 className="text-sm font-bold text-slate-900">{title}</h4>{description ? <p className="mt-1 text-xs leading-5 text-slate-500">{description}</p> : null}</div>
      {aside ? <div className="text-xs font-medium text-slate-500">{aside}</div> : null}
    </div>
    {children}
  </section>;
}

export function RecordMetrics({ children }: { children: ReactNode }) {
  return <dl className="record-metrics">{children}</dl>;
}

export function RecordMetric({ label, children }: { label: string; children: ReactNode }) {
  return <div className="min-w-0"><dt className="text-xs font-medium text-slate-500">{label}</dt><dd className="mt-1.5 break-words text-sm font-semibold leading-6 text-slate-900">{children}</dd></div>;
}

export function RecordEmpty({ children }: { children: ReactNode }) {
  return <div className="flex items-start gap-3 rounded-lg bg-slate-50 px-4 py-5 text-sm leading-6 text-slate-500"><FileText className="mt-0.5 h-5 w-5 shrink-0 text-slate-400" aria-hidden="true" /><p>{children}</p></div>;
}

export type RecordColumn<T> = { key: string; label: string; render: (row: T) => ReactNode; align?: 'right'; };

/** Container width, rather than viewport width, chooses table versus labelled cards. */
export function RecordTable<T>({ rows, columns, rowKey, empty }: {
  rows: T[]; columns: RecordColumn<T>[]; rowKey: (row: T, index: number) => string; empty: string;
}) {
  if (!rows.length) return <RecordEmpty>{empty}</RecordEmpty>;
  return <div className="record-table-container">
    <div className="record-table-desktop consflow-table-viewport rounded-lg border border-slate-200">
      <table className="w-full text-sm">
        <thead className="bg-slate-50 text-left text-xs text-slate-500"><tr>{columns.map(column => <th key={column.key} scope="col" className={`px-4 py-3 font-semibold ${column.align === 'right' ? 'text-right' : ''}`}>{column.label}</th>)}</tr></thead>
        <tbody className="divide-y divide-slate-100">{rows.map((row, index) => <tr key={rowKey(row, index)} className="align-top">{columns.map(column => <td key={column.key} className={`px-4 py-4 ${column.align === 'right' ? 'text-right tabular-nums' : ''}`}>{column.render(row)}</td>)}</tr>)}</tbody>
      </table>
    </div>
    <div className="record-table-mobile space-y-3">{rows.map((row, index) => <article key={rowKey(row, index)} className="rounded-lg border border-slate-200 p-4">
      <div className="min-w-0 break-words text-sm">{columns[0]?.render(row)}</div>
      <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-3 border-t border-slate-100 pt-3">{columns.slice(1).map(column => <RecordMetric key={column.key} label={column.label}>{column.render(row)}</RecordMetric>)}</dl>
    </article>)}</div>
  </div>;
}
