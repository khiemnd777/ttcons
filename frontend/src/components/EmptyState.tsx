import { ReactNode } from 'react';

export function EmptyState({ title = 'Chưa có dữ liệu', action }: { title?: string; action?: ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed border-slate-300 bg-white p-8 text-center">
      <p className="text-sm font-medium text-slate-700">{title}</p>
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}
