import { RefreshCw, RotateCcw, Search } from 'lucide-react';

export function ProjectScopeActions({
  applying,
  applyDisabled = false,
  resetDisabled = false,
  onApply,
  onReset,
}: {
  applying: boolean;
  applyDisabled?: boolean;
  resetDisabled?: boolean;
  onApply: () => void;
  onReset: () => void;
}) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-2 sm:flex sm:flex-nowrap">
      <button
        type="button"
        className="order-2 inline-flex h-10 items-center justify-center gap-2 rounded-md bg-transparent px-3 text-sm font-semibold text-slate-600 hover:bg-slate-100 hover:text-ink disabled:cursor-not-allowed disabled:opacity-60 sm:order-1"
        onClick={onReset}
        disabled={resetDisabled}
      >
        <RotateCcw className="h-4 w-4" />
        Đặt lại
      </button>
      <button
        type="button"
        className="order-1 inline-flex h-10 min-w-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 sm:order-2"
        onClick={onApply}
        disabled={applying || applyDisabled}
      >
        {applying ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
        Áp dụng
      </button>
    </div>
  );
}
