import { FormEvent, useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { ChevronDown, X } from 'lucide-react';
import { useToast } from '../../components/ToastProvider';

type FieldType = 'text' | 'number' | 'date' | 'textarea' | 'select' | 'multi-select' | 'checkbox';
type FieldOption = { label: string; value: string };
type FormValues = Record<string, any>;

export type FieldDef = {
  name: string;
  label: string;
  type?: FieldType;
  options?: FieldOption[];
  placeholder?: string;
  helperText?: string;
  defaultValue?: any;
  required?: boolean | string;
  min?: number;
  max?: number;
  fullWidth?: boolean;
  hiddenWhen?: (values: FormValues) => boolean;
  disabledWhen?: (values: FormValues) => boolean;
  validate?: (value: any, values: FormValues) => string | undefined;
};

type FormErrors = Record<string, string>;
type FormLayout = 'single' | 'two-column';

function fieldInitialValue(field: FieldDef) {
  if (field.defaultValue !== undefined) return field.defaultValue;
  if (field.type === 'multi-select') return [];
  return field.type === 'checkbox' ? false : '';
}

function isEmptyValue(value: any) {
  if (Array.isArray(value)) return value.length === 0;
  if (typeof value === 'string') return value.trim() === '';
  return value === null || value === undefined || value === '';
}

function isFieldVisible(field: FieldDef, values: FormValues) {
  return !field.hiddenWhen?.(values);
}

function visibleValues(fields: FieldDef[], values: FormValues) {
  return Object.fromEntries(fields.filter((field) => isFieldVisible(field, values)).map((field) => [field.name, values[field.name]]));
}

export function SimpleForm({
  fields,
  submitLabel = 'Lưu',
  layout = 'single',
  resetOnSubmit = false,
  onSubmit,
}: {
  fields: FieldDef[];
  submitLabel?: string;
  layout?: FormLayout;
  resetOnSubmit?: boolean;
  onSubmit: (values: FormValues) => Promise<unknown> | unknown;
}) {
  const toast = useToast();
  const initialValues = useMemo(() => Object.fromEntries(fields.map((field) => [field.name, fieldInitialValue(field)])), [fields]);
  const [values, setValues] = useState<Record<string, any>>(initialValues);
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);
  const visibleFields = fields.filter((field) => isFieldVisible(field, values));

  useEffect(() => {
    setValues(initialValues);
    setErrors({});
  }, [initialValues]);

  function validateValues() {
    const nextErrors: FormErrors = {};
    for (const field of visibleFields) {
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

  function setFieldValue(field: FieldDef, rawValue: string | boolean | string[]) {
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
      const labels = visibleFields.filter((field) => nextErrors[field.name]).map((field) => field.label);
      toast.warn({
        title: 'Kiểm tra dữ liệu',
        description: `Thiếu hoặc sai: ${labels.join(', ')}`,
      });
      return;
    }
    setSaving(true);
    try {
      await onSubmit(visibleValues(fields, values));
      if (resetOnSubmit) {
        setValues(initialValues);
        setErrors({});
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="space-y-4" onSubmit={handleSubmit}>
      <div className={clsx('grid gap-4', layout === 'two-column' ? 'md:grid-cols-2' : 'grid-cols-1')}>
      {visibleFields.map((field) => {
        const error = errors[field.name];
        const disabled = saving || Boolean(field.disabledWhen?.(values));
        const controlClassName = clsx(
          'mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring',
          error ? 'border-rose-300 bg-rose-50' : 'border-line',
          disabled && 'cursor-not-allowed bg-slate-100 text-slate-500',
        );

        return (
        <label key={field.name} className={clsx('block', layout === 'two-column' && (field.fullWidth || field.type === 'textarea') && 'md:col-span-2')}>
          <span className="text-sm font-semibold text-slate-700">{field.label}</span>
          {field.required ? <span className="ml-1 text-rose-600">*</span> : null}
          {field.type === 'textarea' ? (
            <textarea
              className={clsx(controlClassName, 'min-h-24')}
              placeholder={field.placeholder}
              value={values[field.name] ?? ''}
              onChange={(event) => setFieldValue(field, event.target.value)}
              aria-invalid={Boolean(error)}
              disabled={disabled}
            />
          ) : field.type === 'select' ? (
            <select
              className={controlClassName}
              value={values[field.name] ?? ''}
              onChange={(event) => setFieldValue(field, event.target.value)}
              aria-invalid={Boolean(error)}
              disabled={disabled}
            >
              <option value="">Chọn</option>
              {field.options?.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          ) : field.type === 'multi-select' ? (
            <MultiSelectControl
              value={Array.isArray(values[field.name]) ? values[field.name] : []}
              options={field.options ?? []}
              placeholder={field.placeholder ?? 'Gõ để tìm'}
              disabled={disabled}
              invalid={Boolean(error)}
              onChange={(value) => setFieldValue(field, value)}
            />
          ) : field.type === 'checkbox' ? (
            <span className="mt-2 flex items-center gap-2">
              <input
                type="checkbox"
                className={clsx('h-4 w-4 rounded', error ? 'border-rose-300 bg-rose-50' : 'border-line', disabled && 'cursor-not-allowed opacity-60')}
                checked={Boolean(values[field.name])}
                onChange={(event) => setFieldValue(field, event.target.checked)}
                aria-invalid={Boolean(error)}
                disabled={disabled}
              />
              {field.helperText ? <span className="text-xs text-slate-500">{field.helperText}</span> : null}
            </span>
          ) : (
            <input
              type={field.type ?? 'text'}
              className={controlClassName}
              placeholder={field.placeholder}
              value={values[field.name] ?? ''}
              onChange={(event) => setFieldValue(field, event.target.value)}
              aria-invalid={Boolean(error)}
              disabled={disabled}
            />
          )}
          {field.type !== 'checkbox' && field.helperText ? <p className="mt-1 text-xs text-slate-500">{field.helperText}</p> : null}
          {error ? <p className="mt-1 text-xs font-medium text-rose-700">{error}</p> : null}
        </label>
        );
      })}
      </div>
      <button disabled={saving} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
        {saving ? 'Đang lưu...' : submitLabel}
      </button>
    </form>
  );
}

function MultiSelectControl({
  value,
  options,
  placeholder,
  disabled,
  invalid,
  onChange,
}: {
  value: string[];
  options: FieldOption[];
  placeholder: string;
  disabled: boolean;
  invalid: boolean;
  onChange: (value: string[]) => void;
}) {
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const selected = useMemo(() => new Set(value), [value]);
  const labelByValue = useMemo(() => new Map(options.map((option) => [option.value, option.label])), [options]);
  const availableOptions = useMemo(() => options.filter((option) => !selected.has(option.value)), [options, selected]);
  const filteredOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase('vi-VN');
    if (!normalizedQuery) return availableOptions;
    return availableOptions.filter((option) => `${option.label} ${option.value}`.toLocaleLowerCase('vi-VN').includes(normalizedQuery));
  }, [availableOptions, query]);

  const addValue = (nextValue: string) => {
    if (disabled || selected.has(nextValue)) return;
    onChange([...value, nextValue]);
    setQuery('');
    setOpen(false);
  };

  return (
    <div className={clsx('mt-1 rounded-md border px-2 py-2', invalid ? 'border-rose-300 bg-rose-50' : 'border-line', disabled && 'bg-slate-100')}>
      <div className="flex min-h-8 flex-wrap gap-1">
        {value.length ? value.map((selectedValue) => (
          <span key={selectedValue} className="inline-flex max-w-full items-center gap-1 rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">
            <span className="truncate">{labelByValue.get(selectedValue) ?? selectedValue}</span>
            <button
              type="button"
              title={`Bỏ ${labelByValue.get(selectedValue) ?? selectedValue}`}
              className="rounded p-0.5 hover:bg-blue-100 disabled:opacity-40"
              disabled={disabled}
              onClick={() => onChange(value.filter((item) => item !== selectedValue))}
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        )) : <span className="px-1 py-1 text-xs font-medium text-slate-500">Chưa chọn</span>}
      </div>
      <div
        className="relative mt-2"
        onBlur={(event) => {
          const nextTarget = event.relatedTarget as Node | null;
          if (!nextTarget || !event.currentTarget.contains(nextTarget)) setOpen(false);
        }}
      >
        <div className="flex overflow-hidden rounded-md border border-line bg-white focus-within:ring-2 focus-within:ring-blue-100">
          <input
            className="min-w-0 flex-1 px-3 py-2 text-sm outline-none disabled:bg-slate-100"
            value={query}
            disabled={disabled || availableOptions.length === 0}
            placeholder={availableOptions.length ? placeholder : 'Đã chọn hết'}
            onFocus={() => {
              if (!disabled && availableOptions.length) setOpen(true);
            }}
            onChange={(event) => {
              setQuery(event.target.value);
              setOpen(true);
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                if (filteredOptions[0]) addValue(filteredOptions[0].value);
              }
              if (event.key === 'Escape') setOpen(false);
            }}
          />
          <button
            type="button"
            title="Mở danh sách"
            className="grid w-10 place-items-center border-l border-line text-slate-500 hover:bg-slate-50 disabled:bg-slate-100 disabled:text-slate-300"
            disabled={disabled || availableOptions.length === 0}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => setOpen((current) => !current)}
          >
            <ChevronDown className={clsx('h-4 w-4 transition', open && 'rotate-180')} />
          </button>
        </div>
        {open && !disabled && availableOptions.length ? (
          <div className="mt-1 max-h-56 overflow-auto rounded-md border border-line bg-white py-1 shadow-panel">
            {filteredOptions.length ? filteredOptions.map((option) => (
              <button
                key={option.value}
                type="button"
                className="block w-full px-3 py-2 text-left text-sm hover:bg-blue-50"
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => addValue(option.value)}
              >
                <span className="block font-semibold text-ink">{option.label}</span>
                <span className="block text-xs text-slate-500">{option.value}</span>
              </button>
            )) : (
              <div className="px-3 py-2 text-sm font-medium text-slate-500">Không tìm thấy lựa chọn phù hợp</div>
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}
