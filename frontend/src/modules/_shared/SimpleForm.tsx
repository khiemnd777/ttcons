import { FormEvent, useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { useToast } from '../../components/ToastProvider';

export type FieldDef = {
  name: string;
  label: string;
  type?: 'text' | 'number' | 'date' | 'textarea' | 'select' | 'checkbox';
  options?: { label: string; value: string }[];
  placeholder?: string;
  defaultValue?: any;
  required?: boolean | string;
  min?: number;
  max?: number;
  validate?: (value: any, values: Record<string, any>) => string | undefined;
};

type FormErrors = Record<string, string>;

function fieldInitialValue(field: FieldDef) {
  if (field.defaultValue !== undefined) return field.defaultValue;
  return field.type === 'checkbox' ? false : '';
}

function isEmptyValue(value: any) {
  if (typeof value === 'string') return value.trim() === '';
  return value === null || value === undefined || value === '';
}

export function SimpleForm({
  fields,
  submitLabel = 'Lưu',
  onSubmit,
}: {
  fields: FieldDef[];
  submitLabel?: string;
  onSubmit: (values: Record<string, any>) => Promise<unknown> | unknown;
}) {
  const toast = useToast();
  const initialValues = useMemo(() => Object.fromEntries(fields.map((field) => [field.name, fieldInitialValue(field)])), [fields]);
  const [values, setValues] = useState<Record<string, any>>(initialValues);
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setValues(initialValues);
    setErrors({});
  }, [initialValues]);

  function validateValues() {
    const nextErrors: FormErrors = {};
    for (const field of fields) {
      const value = values[field.name];
      if (field.required && (field.type === 'checkbox' ? value !== true : isEmptyValue(value))) {
        nextErrors[field.name] = typeof field.required === 'string' ? field.required : `${field.label} là bắt buộc`;
        continue;
      }
      if (field.type === 'number' && !isEmptyValue(value)) {
        const numericValue = Number(value);
        if (Number.isNaN(numericValue)) {
          nextErrors[field.name] = `${field.label} phải là số hợp lệ`;
          continue;
        }
        if (field.min !== undefined && numericValue < field.min) {
          nextErrors[field.name] = `${field.label} phải lớn hơn hoặc bằng ${field.min}`;
          continue;
        }
        if (field.max !== undefined && numericValue > field.max) {
          nextErrors[field.name] = `${field.label} phải nhỏ hơn hoặc bằng ${field.max}`;
          continue;
        }
      }
      const customError = field.validate?.(value, values);
      if (customError) nextErrors[field.name] = customError;
    }
    return nextErrors;
  }

  function setFieldValue(field: FieldDef, rawValue: string | boolean) {
    const value = field.type === 'number' && rawValue !== '' ? Number(rawValue) : rawValue;
    setValues((current) => ({ ...current, [field.name]: value }));
    setErrors((current) => {
      if (!current[field.name]) return current;
      const { [field.name]: _removed, ...rest } = current;
      return rest;
    });
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const nextErrors = validateValues();
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      const labels = fields.filter((field) => nextErrors[field.name]).map((field) => field.label);
      toast.warn({
        title: 'Kiểm tra dữ liệu',
        description: `Thiếu hoặc sai: ${labels.join(', ')}`,
      });
      return;
    }
    setSaving(true);
    try {
      await onSubmit(values);
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="space-y-4" onSubmit={handleSubmit}>
      {fields.map((field) => {
        const error = errors[field.name];
        const controlClassName = clsx(
          'mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring',
          error ? 'border-rose-300 bg-rose-50' : 'border-line',
        );

        return (
        <label key={field.name} className="block">
          <span className="text-sm font-semibold text-slate-700">{field.label}</span>
          {field.required ? <span className="ml-1 text-rose-600">*</span> : null}
          {field.type === 'textarea' ? (
            <textarea
              className={clsx(controlClassName, 'min-h-24')}
              placeholder={field.placeholder}
              value={values[field.name] ?? ''}
              onChange={(event) => setFieldValue(field, event.target.value)}
              aria-invalid={Boolean(error)}
            />
          ) : field.type === 'select' ? (
            <select
              className={controlClassName}
              value={values[field.name] ?? ''}
              onChange={(event) => setFieldValue(field, event.target.value)}
              aria-invalid={Boolean(error)}
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
              className={clsx('mt-2 h-4 w-4 rounded', error ? 'border-rose-300 bg-rose-50' : 'border-line')}
              checked={Boolean(values[field.name])}
              onChange={(event) => setFieldValue(field, event.target.checked)}
              aria-invalid={Boolean(error)}
            />
          ) : (
            <input
              type={field.type ?? 'text'}
              className={controlClassName}
              placeholder={field.placeholder}
              value={values[field.name] ?? ''}
              onChange={(event) => setFieldValue(field, event.target.value)}
              aria-invalid={Boolean(error)}
            />
          )}
          {error ? <p className="mt-1 text-xs font-medium text-rose-700">{error}</p> : null}
        </label>
        );
      })}
      <button disabled={saving} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
        {saving ? 'Đang lưu...' : submitLabel}
      </button>
    </form>
  );
}
