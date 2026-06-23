import clsx from 'clsx';

export type FilterOption = {
  value: string;
  label: string;
  count?: number;
};

export function FilterBar({
  label,
  value,
  options,
  onChange,
  disabled = false,
}: {
  label?: string;
  value: string;
  options: FilterOption[];
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex flex-col gap-2 rounded-md border border-line bg-slate-50 px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
      {label ? <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</span> : null}
      <div className="flex min-w-0 flex-wrap gap-1.5">
        {options.map((option) => {
          const active = option.value === value;
          return (
            <button
              key={option.value}
              type="button"
              disabled={disabled}
              onClick={() => onChange(option.value)}
              className={clsx(
                'inline-flex min-h-8 items-center gap-1.5 rounded-md border px-2.5 text-xs font-semibold transition',
                active ? 'border-blue-200 bg-blue-600 text-white' : 'border-line bg-white text-slate-600 hover:bg-slate-100',
                disabled && 'cursor-not-allowed opacity-50',
              )}
            >
              <span>{option.label}</span>
              {typeof option.count === 'number' ? (
                <span className={clsx('rounded px-1.5 py-0.5 text-[11px]', active ? 'bg-white/20 text-white' : 'bg-slate-100 text-slate-500')}>
                  {option.count}
                </span>
              ) : null}
            </button>
          );
        })}
      </div>
    </div>
  );
}
