import { ReactNode } from 'react';

export function StatCard({ label, value, sub, icon }: { label: string; value: ReactNode; sub?: string; icon?: ReactNode }) {
  return (
    <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
          <div className="mt-2 text-2xl font-semibold text-ink">{value}</div>
        </div>
        {icon ? <div className="rounded-md bg-slate-100 p-2 text-slate-700">{icon}</div> : null}
      </div>
      {sub ? <p className="mt-2 text-xs text-slate-500">{sub}</p> : null}
    </div>
  );
}
