import { Download, RefreshCw, RotateCcw, Search } from 'lucide-react';
import clsx from 'clsx';
import type { ReportProjectOption } from '../../lib/types/domain';

export function ReportProjectField({
  value,
  options,
  onChange,
  disabled,
}: {
  value: string;
  options: ReportProjectOption[];
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <label className="block w-full sm:min-w-52 sm:w-auto">
      <span className="text-xs font-semibold text-slate-600">Công trình</span>
      <select
        className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100 disabled:text-slate-500"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
      >
        <option value="">Tất cả công trình được phép xem</option>
        {options.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
      </select>
    </label>
  );
}

export function ReportDateField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="block min-w-36">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <input
        type="date"
        className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

export function ReportSelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<[string, string]> }) {
  return (
    <label className="block min-w-36">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <select
        className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map(([optionValue, labelText]) => <option key={optionValue} value={optionValue}>{labelText}</option>)}
      </select>
    </label>
  );
}

export function ReportFilterActions({
  applying,
  exporting,
  exportDisabled = false,
  onApply,
  onReset,
  onExport,
  mobilePrimary = false,
}: {
  applying: boolean;
  exporting?: boolean;
  exportDisabled?: boolean;
  onApply: () => void;
  onReset: () => void;
  onExport: () => void;
  mobilePrimary?: boolean;
}) {
  return (
    <div className={clsx('items-end gap-2', mobilePrimary ? 'grid grid-cols-[minmax(0,1fr)_auto_auto] sm:flex sm:flex-wrap' : 'flex flex-wrap')}>
      <button
        type="button"
        className={clsx(
          'inline-flex items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100',
          mobilePrimary && 'order-2 sm:order-1',
        )}
        onClick={onReset}
      >
        <RotateCcw className="h-4 w-4" />
        Đặt lại
      </button>
      <button
        type="button"
        className={clsx(
          'inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60',
          mobilePrimary && 'order-1 min-w-0 sm:order-2',
        )}
        onClick={onApply}
        disabled={applying}
      >
        {applying ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
        Áp dụng
      </button>
      <button
        type="button"
        className={clsx(
          'inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-60',
          mobilePrimary && 'order-3',
          mobilePrimary && exporting && 'text-xs sm:text-sm',
        )}
        onClick={onExport}
        disabled={exporting || exportDisabled}
      >
        {exporting ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
        <span>{exporting ? 'Đang xuất…' : 'CSV'}</span>
      </button>
    </div>
  );
}
