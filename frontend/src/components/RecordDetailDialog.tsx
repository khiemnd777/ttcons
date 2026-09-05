import { ReactNode } from 'react';
import { Dialog } from './Dialog';

export type DetailField<T extends Record<string, any>> = {
  key: string;
  label: string;
  render?: (row: T) => ReactNode;
  hidden?: (row: T) => boolean;
};

function formatValue(value: unknown): ReactNode {
  if (value === null || value === undefined || value === '') return '-';
  if (typeof value === 'boolean') return value ? 'Có' : 'Không';
  if (Array.isArray(value)) return value.length ? value.join(', ') : '-';
  if (typeof value === 'object') return <pre className="whitespace-pre-wrap break-words text-xs leading-5 text-slate-600">{JSON.stringify(value, null, 2)}</pre>;
  return String(value);
}

export function RecordDetailDialog<T extends Record<string, any>>({
  open,
  title,
  record,
  fields,
  onClose,
}: {
  open: boolean;
  title: string;
  record: T | null;
  fields: DetailField<T>[];
  onClose: () => void;
}) {
  return (
    <Dialog open={open && Boolean(record)} title={title} onClose={onClose} size="lg">
      {record ? <RecordDetailFields record={record} fields={fields} /> : null}
    </Dialog>
  );
}

export function RecordDetailFields<T extends Record<string, any>>({ record, fields }: { record: T; fields: DetailField<T>[] }) {
  return <dl className="grid gap-3 md:grid-cols-2">
    {fields.filter((field) => !field.hidden?.(record)).map((field) => (
      <div key={field.key} className="rounded-md border border-line bg-slate-50 px-3 py-2">
        <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">{field.label}</dt>
        <dd className="mt-1 break-words text-sm text-ink">{field.render ? field.render(record) : formatValue(record[field.key])}</dd>
      </div>
    ))}
  </dl>;
}
