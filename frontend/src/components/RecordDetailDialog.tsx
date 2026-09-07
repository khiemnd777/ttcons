import { ReactNode } from 'react';
import { Dialog } from './Dialog';
import { RecordSection, RecordSummary } from './RecordDetailLayout';

export type DetailField<T extends Record<string, any>> = {
  key: string;
  label: string;
  render?: (row: T) => ReactNode;
  hidden?: (row: T) => boolean;
  wide?: boolean;
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
  summary,
  sections,
  related,
  footer,
}: {
  open: boolean;
  title: string;
  record: T | null;
  fields: DetailField<T>[];
  onClose: () => void;
  summary?: (record: T) => ReactNode;
  sections?: { title: string; keys: string[]; collapsed?: boolean }[];
  related?: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <Dialog manageFocus open={open && Boolean(record)} title={title} onClose={onClose} size="lg" footer={footer}>
      {record ? <div className="space-y-6">
        {summary ? summary(record) : <RecordSummary title={title} />}
        {sections ? sections.map(section => section.collapsed
          ? <details key={section.title} className="rounded-lg border border-slate-200 px-4 py-3"><summary className="cursor-pointer text-sm font-semibold text-slate-600">{section.title}</summary><RecordDetailFields record={record} fields={fields.filter(field => section.keys.includes(field.key))} /></details>
          : <RecordSection key={section.title} title={section.title}><RecordDetailFields record={record} fields={fields.filter(field => section.keys.includes(field.key))} /></RecordSection>)
          : <RecordDetailFields record={record} fields={fields} />}
        {related}
      </div> : null}
    </Dialog>
  );
}

export function RecordDetailFields<T extends Record<string, any>>({ record, fields }: { record: T; fields: DetailField<T>[] }) {
  return <dl className="record-fields">
    {fields.filter((field) => !field.hidden?.(record)).map((field) => (
      <div key={field.key} className={`record-field${field.wide ? ' record-field-wide' : ''}`}>
        <dt className="text-xs font-medium text-slate-500">{field.label}</dt>
        <dd className="mt-1.5 whitespace-pre-wrap break-words text-sm leading-6 text-ink">{field.render ? field.render(record) : formatValue(record[field.key])}</dd>
      </div>
    ))}
  </dl>;
}
