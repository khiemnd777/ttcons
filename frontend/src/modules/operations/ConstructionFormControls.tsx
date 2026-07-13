import type { ReactNode } from 'react';
import clsx from 'clsx';

export type FormErrors = Record<string, string>;

export function FieldBlock({ label, required, error, helper, className, children }: { label: string; required?: boolean; error?: string; helper?: string; className?: string; children: ReactNode }) {
  return <label className={clsx('block', className)}>
    <span className="text-sm font-semibold text-slate-700">{label}{required ? <span className="ml-1 text-rose-600">*</span> : null}</span>
    {children}
    {error ? <span className="mt-1 block text-xs font-medium text-rose-700">{error}</span> : helper ? <span className="mt-1 block text-xs text-slate-500">{helper}</span> : null}
  </label>;
}

export function controlClass(error?: string, disabled = false) {
  return clsx(
    'mt-1 w-full rounded-md border bg-white px-3 py-2 text-sm text-slate-800 outline-none transition focus:ring-2 focus:ring-blue-100',
    error ? 'border-rose-300 bg-rose-50' : 'border-line focus:border-blue-400',
    disabled && 'cursor-not-allowed bg-slate-100 text-slate-500',
  );
}

export function requiredErrors(values: Record<string, string>, fields: Array<{ name: string; label: string }>) {
  return Object.fromEntries(fields.filter((field) => !values[field.name]?.trim()).map((field) => [field.name, `${field.label} là bắt buộc`]));
}

export function errorSummary(errors: FormErrors) {
  return Object.values(errors).join(', ');
}

export function SectionTitle({ children }: { children: ReactNode }) {
  return <h3 className="border-b border-line pb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{children}</h3>;
}
