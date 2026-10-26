import type { ReactNode } from 'react';

/** Named secondary action. Business transitions keep their visible text labels. */
export function IconAction({ label, icon, onClick, disabled = false }: {
  label: string; icon: ReactNode; onClick: () => void; disabled?: boolean;
}) {
  return (
    <button type="button" aria-label={label} title={label} disabled={disabled} onClick={onClick}
      className="inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-md border border-line bg-white text-slate-600 hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-50">
      <span aria-hidden="true">{icon}</span>
    </button>
  );
}
