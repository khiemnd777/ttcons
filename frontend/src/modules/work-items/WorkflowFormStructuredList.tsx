import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import clsx from 'clsx';
import { CurrencyInput } from '../../components/CurrencyInput';
import { Dialog } from '../../components/Dialog';
import { StatusBadge } from '../../components/StatusBadge';
import type { AnyRecord, EvidenceRecord, WorkflowFormField } from '../../lib/types/domain';
import { workflowStructuredOtherDetailRequired } from './workflowOtherDetailRules';
import { evidenceOptionLabel, textValue } from './workflowFormValueUtils';

type ChecklistStats = {
  total: number;
  complete: number;
  needsAction: number;
  withEvidence: number;
};

type StructuredFieldSpec = {
  key: string;
  label: string;
  type: 'text' | 'textarea' | 'number' | 'currency' | 'date' | 'select' | 'evidence_reference' | string;
  required?: boolean;
  options?: Array<{ value: string; label?: string }>;
  currencyCode?: string;
  allowNegative?: boolean;
  maximumFractionDigits?: number;
  minLength?: number;
  maxLength?: number;
  min?: number;
  max?: number;
};

export function StructuredListControl({
  templateCode,
  field,
  value,
  evidence,
  disabled,
  displayOnly,
  invalid,
  onChange,
}: {
  templateCode?: string;
  field: WorkflowFormField;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
  invalid: boolean;
  onChange: (value: unknown) => void;
}) {
  const rows = structuredRows(value);
  const specs = structuredItemFields(field);
  const addRow = () => {
    const nextRow = Object.fromEntries(specs.map((spec) => [spec.key, '']));
    onChange([...rows, nextRow]);
  };
  const updateCell = (rowIndex: number, key: string, nextValue: string) => {
    onChange(rows.map((row, index) => (index === rowIndex ? { ...row, [key]: nextValue } : row)));
  };
  const removeRow = (rowIndex: number) => {
    onChange(rows.filter((_, index) => index !== rowIndex));
  };
  return (
    <StructuredListTable
      templateCode={templateCode}
      fieldKey={field.field_key}
      label={field.label}
      rows={rows}
      specs={specs}
      evidence={evidence}
      disabled={disabled}
      displayOnly={displayOnly}
      invalid={invalid}
      emptyLabel="Chưa có dòng nào"
      onAddRow={addRow}
      onRemoveRow={removeRow}
      onUpdateCell={updateCell}
    />
  );
}

export function ChecklistStructuredListControl({
  templateCode,
  field,
  value,
  evidence,
  disabled,
  displayOnly,
  invalid,
  onChange,
}: {
  templateCode?: string;
  field: WorkflowFormField;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
  invalid: boolean;
  onChange: (value: unknown) => void;
}) {
  const rows = structuredRows(value);
  const specs = structuredItemFields(field);
  const stats = checklistStatsFromRows(rows);
  const addRow = () => {
    const nextRow = Object.fromEntries(specs.map((spec) => [spec.key, defaultChecklistCellValue(spec)]));
    onChange([...rows, nextRow]);
  };
  const updateCell = (rowIndex: number, key: string, nextValue: string) => {
    onChange(rows.map((row, index) => (index === rowIndex ? { ...row, [key]: nextValue } : row)));
  };
  const removeRow = (rowIndex: number) => {
    onChange(rows.filter((_, index) => index !== rowIndex));
  };

  return (
    <StructuredListTable
      templateCode={templateCode}
      fieldKey={field.field_key}
      label={field.label}
      rows={rows}
      specs={specs}
      evidence={evidence}
      disabled={disabled}
      displayOnly={displayOnly}
      invalid={invalid}
      checklist
      emptyLabel="Chưa có dòng checklist"
      summary={(
        <div className="flex flex-wrap gap-1.5 text-xs font-semibold">
          <ChecklistStat label="Dòng" value={stats.total} />
          <ChecklistStat label="Hoàn tất/Không áp dụng" value={stats.complete} />
          <ChecklistStat label="Cần xử lý" value={stats.needsAction} tone={stats.needsAction ? 'warn' : 'ok'} />
          <ChecklistStat label="Minh chứng" value={stats.withEvidence} />
        </div>
      )}
      onAddRow={addRow}
      onRemoveRow={removeRow}
      onUpdateCell={updateCell}
    />
  );
}

function StructuredListTable({
  templateCode,
  fieldKey,
  label,
  rows,
  specs,
  evidence,
  disabled,
  displayOnly,
  invalid,
  checklist = false,
  emptyLabel,
  summary,
  onAddRow,
  onRemoveRow,
  onUpdateCell,
}: {
  templateCode?: string;
  fieldKey: string;
  label: string;
  rows: AnyRecord[];
  specs: StructuredFieldSpec[];
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
  invalid: boolean;
  checklist?: boolean;
  emptyLabel: string;
  summary?: ReactNode;
  onAddRow: () => void;
  onRemoveRow: (rowIndex: number) => void;
  onUpdateCell: (rowIndex: number, key: string, value: string) => void;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const scrollViewportRef = useRef<HTMLDivElement | null>(null);
  const focusRowRef = useRef<number | null>(null);
  const [hasHorizontalOverflow, setHasHorizontalOverflow] = useState(false);
  const [pendingRemoveRowIndex, setPendingRemoveRowIndex] = useState<number | null>(null);
  const minWidth = structuredTableMinWidth(specs, !displayOnly);
  const pendingRemoveRow = pendingRemoveRowIndex === null ? undefined : rows[pendingRemoveRowIndex];
  const pendingRemoveSummary = pendingRemoveRow ? structuredRowDeleteSummary(pendingRemoveRow, specs, evidence) : '';

  useEffect(() => {
    const rowIndex = focusRowRef.current;
    if (rowIndex === null) return undefined;
    const frame = window.requestAnimationFrame(() => {
      const candidates = Array.from(
        containerRef.current?.querySelectorAll<HTMLElement>(
          `[data-structured-row-index="${rowIndex}"] input:not(:disabled), [data-structured-row-index="${rowIndex}"] select:not(:disabled), [data-structured-row-index="${rowIndex}"] textarea:not(:disabled)`,
        ) ?? [],
      );
      const target = candidates.find((candidate) => candidate.getClientRects().length > 0) ?? candidates[0];
      target?.focus();
      focusRowRef.current = null;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [rows.length]);

  useEffect(() => {
    const viewport = scrollViewportRef.current;
    if (!viewport) return undefined;
    const updateOverflow = () => {
      setHasHorizontalOverflow(viewport.scrollWidth > viewport.clientWidth + 1);
    };
    const frame = window.requestAnimationFrame(updateOverflow);
    const resizeObserver = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(updateOverflow);
    resizeObserver?.observe(viewport);
    const table = viewport.querySelector('table');
    if (table) resizeObserver?.observe(table);
    window.addEventListener('resize', updateOverflow);
    return () => {
      window.cancelAnimationFrame(frame);
      resizeObserver?.disconnect();
      window.removeEventListener('resize', updateOverflow);
    };
  }, [displayOnly, minWidth, rows.length]);

  const addAndFocusRow = () => {
    focusRowRef.current = rows.length;
    onAddRow();
  };
  const confirmRemoveRow = () => {
    if (pendingRemoveRowIndex === null) return;
    onRemoveRow(pendingRemoveRowIndex);
    setPendingRemoveRowIndex(null);
  };

  return (
    <div
      ref={containerRef}
      data-structured-list-table
      className={clsx('mt-1 min-w-0 w-full max-w-full rounded-md border p-2', invalid ? 'border-rose-300 bg-rose-50/40' : 'border-line bg-white')}
    >
      {summary ? <div className="mb-2">{summary}</div> : null}
      {rows.length ? (
        <>
          <div
            ref={scrollViewportRef}
            className="structured-list-scroll hidden min-w-0 w-full max-w-full overflow-x-auto rounded-md border border-line md:block"
            role="region"
            aria-label={`${label}: bảng có thể cuộn ngang`}
            tabIndex={hasHorizontalOverflow ? 0 : undefined}
          >
            <table className="w-full table-fixed border-collapse text-left text-sm" style={{ minWidth }}>
              <caption className="sr-only">{label}</caption>
              <colgroup>
                {!displayOnly ? <col style={{ width: 52 }} /> : null}
                <col style={{ width: 48 }} />
                {specs.map((spec) => <col key={spec.key} style={{ width: structuredColumnWidth(spec) }} />)}
              </colgroup>
              <thead className="bg-slate-100 text-xs font-semibold text-slate-600">
                <tr>
                  {!displayOnly ? <th scope="col" className="border-b border-line px-2 py-2"><span className="sr-only">Thao tác</span></th> : null}
                  <th scope="col" className="border-b border-line px-2 py-2 text-center">#</th>
                  {specs.map((spec) => (
                    <th key={spec.key} scope="col" className="border-b border-l border-line px-2 py-2 align-bottom">
                      {spec.label}{structuredSpecRequired(templateCode, fieldKey, rows, spec) ? <span className="ml-1 text-rose-600">*</span> : null}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-line bg-white">
                {rows.map((row, rowIndex) => {
                  const rowInvalid = structuredRowInvalid(templateCode, fieldKey, specs, row, invalid, checklist);
                  return (
                    <tr key={rowIndex} data-structured-row-index={rowIndex} className={rowInvalid ? 'bg-rose-50/50' : undefined}>
                      {!displayOnly ? (
                        <td className="p-2 text-center align-top">
                          <StructuredDeleteButton disabled={disabled} rowIndex={rowIndex} onRemove={setPendingRemoveRowIndex} />
                        </td>
                      ) : null}
                      <th scope="row" className="px-2 py-3 text-center align-top text-xs font-semibold text-slate-500">{rowIndex + 1}</th>
                      {specs.map((spec) => (
                        <td key={spec.key} className="border-l border-line p-2 align-top">
                          <StructuredCell
                            required={spec.required || workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key)}
                            spec={spec}
                            value={row[spec.key]}
                            evidence={evidence}
                            disabled={disabled}
                            displayOnly={displayOnly}
                            invalid={structuredCellInvalid(templateCode, fieldKey, spec, row, invalid, checklist)}
                            issue={invalid ? structuredCellIssue(templateCode, fieldKey, spec, row, checklist) : ''}
                            showLabel={false}
                            onChange={(nextValue) => onUpdateCell(rowIndex, spec.key, nextValue)}
                          />
                        </td>
                      ))}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {hasHorizontalOverflow ? (
            <p className="mt-1 hidden items-center justify-between gap-3 px-1 text-[11px] font-medium text-slate-500 md:flex">
              <span aria-hidden="true">←</span>
              <span>Kéo ngang để xem các cột còn lại</span>
              <span aria-hidden="true">→</span>
            </p>
          ) : null}

          <div className="space-y-2 md:hidden">
            {rows.map((row, rowIndex) => {
              const rowInvalid = structuredRowInvalid(templateCode, fieldKey, specs, row, invalid, checklist);
              return (
                <section
                  key={rowIndex}
                  data-structured-row-index={rowIndex}
                  aria-label={`${label}, dòng ${rowIndex + 1}`}
                  className={clsx('rounded-md border bg-white p-2', rowInvalid ? 'border-rose-300 bg-rose-50/40' : 'border-line')}
                >
                  <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Dòng {rowIndex + 1}</span>
                      {checklist ? <StatusBadge status={textValue(row.status || 'draft')} /> : null}
                      {checklist && row.evidence_id ? <span className="truncate text-xs font-medium text-slate-500">Đã liên kết minh chứng</span> : null}
                    </div>
                    {!displayOnly ? <StructuredDeleteButton disabled={disabled} rowIndex={rowIndex} onRemove={setPendingRemoveRowIndex} /> : null}
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {specs.map((spec) => (
                      <StructuredCell
                        key={spec.key}
                        required={spec.required || workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key)}
                        spec={spec}
                        value={row[spec.key]}
                        evidence={evidence}
                        disabled={disabled}
                        displayOnly={displayOnly}
                        invalid={structuredCellInvalid(templateCode, fieldKey, spec, row, invalid, checklist)}
                        issue={invalid ? structuredCellIssue(templateCode, fieldKey, spec, row, checklist) : ''}
                        showLabel
                        onChange={(nextValue) => onUpdateCell(rowIndex, spec.key, nextValue)}
                      />
                    ))}
                  </div>
                </section>
              );
            })}
          </div>
        </>
      ) : (
        <div className="rounded-md border border-dashed border-line bg-white px-3 py-5 text-center text-sm font-medium text-slate-500">{emptyLabel}</div>
      )}
      {!displayOnly ? (
        <button
          type="button"
          className="mt-2 inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={disabled || specs.length === 0}
          onClick={addAndFocusRow}
        >
          <Plus className="h-4 w-4" />
          Thêm dòng
        </button>
      ) : null}
      <Dialog
        open={pendingRemoveRowIndex !== null}
        title="Xóa dòng này?"
        description={pendingRemoveRowIndex === null ? undefined : `Dòng ${pendingRemoveRowIndex + 1} sẽ bị xóa khỏi “${label}”.`}
        size="sm"
        onClose={() => setPendingRemoveRowIndex(null)}
        footer={(
          <div className="flex items-center justify-end gap-2">
            <button
              type="button"
              autoFocus
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-100"
              onClick={() => setPendingRemoveRowIndex(null)}
            >
              Hủy
            </button>
            <button
              type="button"
              className="rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 focus:outline-none focus:ring-2 focus:ring-rose-200"
              onClick={confirmRemoveRow}
            >
              Xóa dòng
            </button>
          </div>
        )}
      >
        <div className="space-y-3 text-sm text-slate-600">
          {pendingRemoveSummary ? (
            <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Dòng đã chọn</p>
              <p className="mt-1 font-medium text-slate-700">{pendingRemoveSummary}</p>
            </div>
          ) : null}
          <p>Dữ liệu chỉ được lưu chính thức khi anh chọn Lưu nháp hoặc Chốt hồ sơ.</p>
        </div>
      </Dialog>
    </div>
  );
}

function StructuredDeleteButton({ disabled, rowIndex, onRemove }: { disabled: boolean; rowIndex: number; onRemove: (rowIndex: number) => void }) {
  return (
    <button
      type="button"
      aria-label={`Xóa dòng ${rowIndex + 1}`}
      title="Xóa dòng"
      className="rounded-md border border-line bg-white p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 focus:outline-none focus:ring-2 focus:ring-rose-100 disabled:cursor-not-allowed disabled:opacity-40"
      disabled={disabled}
      onClick={() => onRemove(rowIndex)}
    >
      <Trash2 className="h-4 w-4" />
    </button>
  );
}

function ChecklistStat({ label, value, tone = 'neutral' }: { label: string; value: number; tone?: 'neutral' | 'ok' | 'warn' }) {
  return (
    <span className={clsx(
      'rounded-md px-2 py-1 ring-1 ring-inset',
      tone === 'warn' ? 'bg-amber-50 text-amber-800 ring-amber-200' : tone === 'ok' ? 'bg-emerald-50 text-emerald-700 ring-emerald-200' : 'bg-white text-slate-600 ring-line',
    )}>
      {label}: {value}
    </span>
  );
}

function defaultChecklistCellValue(spec: StructuredFieldSpec) {
  return '';
}

export function checklistStatsFromRows(rows: AnyRecord[]): ChecklistStats {
  const complete = rows.filter((row) => ['done', 'not_applicable'].includes(String(row.status ?? ''))).length;
  return {
    total: rows.length,
    complete,
    needsAction: rows.length - complete,
    withEvidence: rows.filter((row) => Boolean(row.evidence_id)).length,
  };
}

function StructuredCell({
  required,
  spec,
  value,
  evidence,
  disabled,
  displayOnly,
  invalid,
  issue,
  showLabel,
  onChange,
}: {
  required: boolean;
  spec: StructuredFieldSpec;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
  invalid: boolean;
  issue: string;
  showLabel: boolean;
  onChange: (value: string) => void;
}) {
  const className = clsx(
    showLabel && 'mt-1',
    'w-full rounded-md border bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100',
    invalid ? 'border-rose-300 bg-rose-50' : 'border-line',
  );
  const selectableEvidence = evidence.filter((record) => record.status !== 'rejected');
  if (displayOnly) {
    return (
      <div className="min-w-0">
        <span className={clsx('text-xs font-semibold text-slate-600', !showLabel && 'sr-only')}>
          {spec.label}{required ? <span className="ml-1 text-rose-600">*</span> : null}
        </span>
        <p className={clsx('whitespace-pre-wrap break-words text-sm text-slate-700', showLabel && 'mt-1')}>
          {structuredCellDisplayValue(spec, value, evidence)}
        </p>
      </div>
    );
  }
  return (
    <label className="block min-w-0">
      <span className={clsx('text-xs font-semibold text-slate-600', !showLabel && 'sr-only')}>
        {spec.label}{required ? <span className="ml-1 text-rose-600">*</span> : null}
      </span>
      {spec.type === 'textarea' ? (
        <textarea className={className} rows={2} value={textValue(value)} disabled={disabled} aria-invalid={invalid || undefined} onChange={(event) => onChange(event.target.value)} />
      ) : spec.type === 'currency' ? (
        <CurrencyInput
          className={showLabel ? 'mt-1' : undefined}
          inputClassName="px-2 py-2 text-sm"
          value={textValue(value)}
          currencyCode={spec.currencyCode}
          allowNegative={spec.allowNegative}
          maximumFractionDigits={spec.maximumFractionDigits}
          invalid={invalid}
          disabled={disabled}
          onValueChange={({ numericString }) => onChange(numericString)}
        />
      ) : spec.type === 'select' ? (
        <select className={className} value={textValue(value)} disabled={disabled} aria-invalid={invalid || undefined} onChange={(event) => onChange(event.target.value)}>
          <option value="">Chọn</option>
          {(spec.options ?? []).map((option) => <option key={option.value} value={option.value}>{option.label ?? option.value}</option>)}
        </select>
      ) : spec.type === 'evidence_reference' ? (
        <select className={className} value={textValue(value)} disabled={disabled || selectableEvidence.length === 0} aria-invalid={invalid || undefined} onChange={(event) => onChange(event.target.value)}>
          <option value="">{selectableEvidence.length ? 'Chọn minh chứng' : 'Chưa có minh chứng'}</option>
          {selectableEvidence.map((record) => (
            <option key={record.id} value={record.id}>
              {evidenceOptionLabel(record)}
            </option>
          ))}
        </select>
      ) : (
        <input
          className={className}
          type={spec.type === 'date' || spec.type === 'number' ? spec.type : 'text'}
          value={textValue(value)}
          disabled={disabled}
          aria-invalid={invalid || undefined}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
      {issue ? <p className="mt-1 text-xs font-medium text-rose-700">{issue}</p> : null}
    </label>
  );
}

function structuredCellDisplayValue(spec: StructuredFieldSpec, value: unknown, evidence: EvidenceRecord[]) {
  const rawValue = textValue(value);
  if (!rawValue) return '—';
  if (spec.type === 'select') {
    return spec.options?.find((option) => option.value === rawValue)?.label ?? rawValue;
  }
  if (spec.type === 'evidence_reference') {
    const record = evidence.find((item) => item.id === rawValue);
    return record ? evidenceOptionLabel(record) : rawValue;
  }
  if (spec.type === 'currency') {
    return `${rawValue} ${spec.currencyCode ?? 'VND'}`;
  }
  return rawValue;
}

function structuredRowDeleteSummary(row: AnyRecord, specs: StructuredFieldSpec[], evidence: EvidenceRecord[]) {
  for (const spec of specs) {
    if (!textValue(row[spec.key]).trim()) continue;
    return `${spec.label}: ${structuredCellDisplayValue(spec, row[spec.key], evidence)}`;
  }
  return '';
}

export function structuredCellIssue(
  templateCode: string | undefined,
  fieldKey: string,
  spec: StructuredFieldSpec,
  row: AnyRecord,
  checklist: boolean,
) {
  const rawValue = row[spec.key];
  const text = textValue(rawValue).trim();
  const required = spec.required || workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key);
  if (!text) return required ? `${spec.label} là bắt buộc` : '';

  if (spec.type === 'text' || spec.type === 'textarea') {
    const length = Array.from(text).length;
    if (spec.minLength !== undefined && length < spec.minLength) return `${spec.label} phải có ít nhất ${spec.minLength} ký tự`;
    if (spec.maxLength !== undefined && length > spec.maxLength) return `${spec.label} không được vượt quá ${spec.maxLength} ký tự`;
  }
  if (spec.type === 'number' || spec.type === 'currency') {
    const numeric = Number(text);
    if (!Number.isFinite(numeric)) return `${spec.label} phải là số`;
    if (spec.min !== undefined && numeric < spec.min) return `${spec.label} phải lớn hơn hoặc bằng ${spec.min}`;
    if (spec.max !== undefined && numeric > spec.max) return `${spec.label} phải nhỏ hơn hoặc bằng ${spec.max}`;
  }
  if (spec.type === 'date') {
    const date = /^\d{4}-\d{2}-\d{2}$/.test(text) ? new Date(`${text}T00:00:00Z`) : null;
    if (!date || Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== text) return `${spec.label} không hợp lệ`;
  }
  if (spec.type === 'select' && spec.options?.length && !spec.options.some((option) => option.value === text)) {
    return `${spec.label} không nằm trong schema`;
  }
  if (checklist && spec.key === 'status' && !['done', 'not_applicable'].includes(text)) {
    return 'Trạng thái phải là Hoàn tất hoặc Không áp dụng trước khi chốt';
  }
  return '';
}

function structuredSpecRequired(
  templateCode: string | undefined,
  fieldKey: string,
  rows: AnyRecord[],
  spec: StructuredFieldSpec,
) {
  return Boolean(spec.required)
    || rows.some((row) => workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key));
}

function structuredCellInvalid(
  templateCode: string | undefined,
  fieldKey: string,
  spec: StructuredFieldSpec,
  row: AnyRecord,
  listInvalid: boolean,
  checklist: boolean,
) {
  if (!listInvalid) return false;
  return Boolean(structuredCellIssue(templateCode, fieldKey, spec, row, checklist));
}

function structuredRowInvalid(
  templateCode: string | undefined,
  fieldKey: string,
  specs: StructuredFieldSpec[],
  row: AnyRecord,
  listInvalid: boolean,
  checklist: boolean,
) {
  return specs.some((spec) => structuredCellInvalid(templateCode, fieldKey, spec, row, listInvalid, checklist));
}

function structuredColumnWidth(spec: StructuredFieldSpec) {
  switch (spec.type) {
    case 'textarea':
      return 280;
    case 'evidence_reference':
      return 260;
    case 'select':
      return 190;
    case 'date':
      return 160;
    case 'number':
    case 'currency':
      return 150;
    default:
      return 190;
  }
}

function structuredTableMinWidth(specs: StructuredFieldSpec[], withActions: boolean) {
  return 48 + specs.reduce((total, spec) => total + structuredColumnWidth(spec), 0) + (withActions ? 52 : 0);
}

export function structuredRows(value: unknown): AnyRecord[] {
  if (!Array.isArray(value)) return [];
  return value.filter((row): row is AnyRecord => Boolean(row) && typeof row === 'object' && !Array.isArray(row));
}

function optionalFiniteNumber(value: unknown) {
  if (value === null || value === undefined || String(value).trim() === '') return undefined;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : undefined;
}

export function structuredItemFields(field: WorkflowFormField): StructuredFieldSpec[] {
  const rawFields = Array.isArray(field.validation_json?.item_fields) ? field.validation_json.item_fields : [];
  return rawFields
    .filter((spec): spec is AnyRecord => Boolean(spec) && typeof spec === 'object' && !Array.isArray(spec) && Boolean(spec.key))
    .map((spec) => ({
      key: String(spec.key),
      label: String(spec.label ?? spec.key),
      type: String(spec.type ?? 'text'),
      required: Boolean(spec.required),
      currencyCode: spec.currency_code ? String(spec.currency_code) : undefined,
      allowNegative: Boolean(spec.allow_negative),
      maximumFractionDigits: spec.maximum_fraction_digits === undefined ? undefined : Number(spec.maximum_fraction_digits),
      minLength: optionalFiniteNumber(spec.min_length),
      maxLength: optionalFiniteNumber(spec.max_length),
      min: optionalFiniteNumber(spec.min),
      max: optionalFiniteNumber(spec.max),
      options: Array.isArray(spec.options)
        ? spec.options
            .filter((option): option is AnyRecord => Boolean(option) && typeof option === 'object' && !Array.isArray(option) && Boolean(option.value))
            .map((option) => ({ value: String(option.value), label: option.label ? String(option.label) : undefined }))
        : [],
    }));
}
