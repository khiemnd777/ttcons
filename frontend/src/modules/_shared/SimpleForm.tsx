import { FormEvent, useState } from 'react';

export type FieldDef = {
  name: string;
  label: string;
  type?: 'text' | 'number' | 'date' | 'textarea' | 'select' | 'checkbox';
  options?: { label: string; value: string }[];
  placeholder?: string;
  defaultValue?: any;
};

export function SimpleForm({
  fields,
  submitLabel = 'Lưu',
  onSubmit,
}: {
  fields: FieldDef[];
  submitLabel?: string;
  onSubmit: (values: Record<string, any>) => Promise<unknown> | unknown;
}) {
  const [values, setValues] = useState<Record<string, any>>(() =>
    Object.fromEntries(fields.map((field) => [field.name, field.defaultValue ?? (field.type === 'checkbox' ? false : '')])),
  );
  const [saving, setSaving] = useState(false);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    try {
      await onSubmit(values);
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="space-y-4" onSubmit={handleSubmit}>
      {fields.map((field) => (
        <label key={field.name} className="block">
          <span className="text-sm font-semibold text-slate-700">{field.label}</span>
          {field.type === 'textarea' ? (
            <textarea
              className="mt-1 min-h-24 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring"
              placeholder={field.placeholder}
              value={values[field.name] ?? ''}
              onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))}
            />
          ) : field.type === 'select' ? (
            <select
              className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring"
              value={values[field.name] ?? ''}
              onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))}
            >
              <option value="">Chọn</option>
              {field.options?.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          ) : field.type === 'checkbox' ? (
            <input
              type="checkbox"
              className="mt-2 h-4 w-4 rounded border-line"
              checked={Boolean(values[field.name])}
              onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.checked }))}
            />
          ) : (
            <input
              type={field.type ?? 'text'}
              className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring"
              placeholder={field.placeholder}
              value={values[field.name] ?? ''}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  [field.name]: field.type === 'number' ? Number(event.target.value) : event.target.value,
                }))
              }
            />
          )}
        </label>
      ))}
      <button disabled={saving} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
        {saving ? 'Đang lưu...' : submitLabel}
      </button>
    </form>
  );
}
