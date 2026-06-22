import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, RefreshCw, Save, Send } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type { BOQItem, SubcontractorReconciliationItem, SubcontractorReconciliationResponse } from '../../lib/types/domain';

type RowDraft = {
  item_index: number;
  boq_item_id: string;
  confirmed_quantity: string;
  decision: string;
  comment: string;
};

type ReconciliationPayload = {
  comment: string;
  items: Array<{
    item_index: number;
    boq_item_id: string;
    confirmed_quantity: number | null;
    decision: string;
    comment: string;
  }>;
};

const decisionOptions = [
  { value: 'accepted', label: 'Chấp nhận' },
  { value: 'adjust_quantity', label: 'Điều chỉnh KL' },
  { value: 'rework_required', label: 'Yêu cầu làm lại' },
  { value: 'waiting_evidence', label: 'Chờ evidence' },
  { value: 'rejected', label: 'Từ chối' },
];

const quantityFormatter = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 3 });

export function SubcontractorReconciliationPanel({
  workItemId,
  projectId,
}: {
  workItemId: string;
  projectId?: string;
}) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [comment, setComment] = useState('');
  const [rows, setRows] = useState<RowDraft[]>([]);

  const reconciliation = useQuery({
    queryKey: ['subcontractor-reconciliation', workItemId],
    queryFn: () => get<SubcontractorReconciliationResponse>(`/work-items/${workItemId}/subcontractor-reconciliation`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  function resetFromResponse(response?: SubcontractorReconciliationResponse) {
    const record = response?.reconciliation;
    setComment(record?.comment ?? '');
    setRows((record?.items ?? []).map(rowDraftFromItem));
  }

  useEffect(() => {
    resetFromResponse(reconciliation.data);
  }, [reconciliation.data]);

  const record = reconciliation.data?.reconciliation;
  const items = record?.items ?? [];
  const boqItems = reconciliation.data?.boq_items ?? [];
  const rowsByIndex = useMemo(() => new Map(rows.map((row) => [row.item_index, row])), [rows]);
  const locked = record?.status === 'submitted';

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['subcontractor-reconciliation', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    if (projectId) queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
  };

  function buildPayload(): ReconciliationPayload | null {
    const invalid: string[] = [];
    const sourceRows = items.map((item) => rowsByIndex.get(item.item_index) ?? rowDraftFromItem(item));
    const payloadRows = sourceRows.map((row) => {
      const confirmedText = row.confirmed_quantity.trim();
      const confirmedQuantity = confirmedText === '' ? null : Number(confirmedText);
      if (confirmedQuantity !== null && (!Number.isFinite(confirmedQuantity) || confirmedQuantity < 0)) {
        invalid.push(`Dòng ${row.item_index + 1}: KL xác nhận`);
      }
      if (row.decision === 'adjust_quantity' && confirmedQuantity === null) {
        invalid.push(`Dòng ${row.item_index + 1}: KL điều chỉnh`);
      }
      return {
        item_index: row.item_index,
        boq_item_id: row.boq_item_id,
        confirmed_quantity: confirmedQuantity,
        decision: row.decision,
        comment: row.comment.trim(),
      };
    });
    if (invalid.length) {
      toast.warn({ title: 'Dữ liệu đối soát chưa hợp lệ', description: invalid.join(', ') });
      return null;
    }
    return { comment: comment.trim(), items: payloadRows };
  }

  const saveDraft = useMutation({
    mutationFn: async () => {
      const payload = buildPayload();
      if (!payload) return null;
      return put<SubcontractorReconciliationResponse>(`/work-items/${workItemId}/subcontractor-reconciliation`, payload);
    },
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      toast.success('Đã lưu nháp đối soát thầu phụ');
    },
    onError: (error) => toast.error('Không lưu được đối soát thầu phụ', error),
  });

  const submitReconciliation = useMutation({
    mutationFn: async () => {
      const payload = buildPayload();
      if (!payload) return null;
      return post<SubcontractorReconciliationResponse>(`/work-items/${workItemId}/subcontractor-reconciliation/submit`, payload);
    },
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      toast.success('Đã submit đối soát thầu phụ');
    },
    onError: (error) => toast.error('Không submit được đối soát thầu phụ', error),
  });
  const pending = reconciliation.isFetching || saveDraft.isPending || submitReconciliation.isPending;

  function updateRow(itemIndex: number, patch: Partial<RowDraft>) {
    setRows((current) => current.map((row) => (row.item_index === itemIndex ? { ...row, ...patch } : row)));
  }

  if (reconciliation.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát thầu phụ</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (reconciliation.error) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát thầu phụ</h3>
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-semibold text-amber-900">Không tải được dữ liệu đối soát</p>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-amber-300 px-3 py-2 text-sm font-semibold text-amber-900 hover:bg-amber-100"
              onClick={() => reconciliation.refetch()}
            >
              <RefreshCw className="h-4 w-4" />
              Tải lại
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (!reconciliation.data?.available) {
    const reason = reconciliation.data?.reason ?? '';
    if (!reason.toLowerCase().includes('submit')) return null;
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát thầu phụ</h3>
        <EmptyState title={reason || 'Submit checklist thầu phụ trước khi đối soát.'} />
      </section>
    );
  }

  if (!record) return null;

  return (
    <section>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">Đối soát thầu phụ</h3>
          <p className="mt-1 text-xs text-slate-500">
            {[record.subcontractor_name, record.work_package, record.construction_area].filter(Boolean).join(' · ') || 'Checklist thầu phụ'}
          </p>
        </div>
        <StatusBadge status={record.status} />
      </div>

      <div className="rounded-md border border-line bg-white p-3">
        <div className="grid gap-2 text-xs sm:grid-cols-2 xl:grid-cols-5">
          <SummaryChip label="Tổng" value={summaryCount(record.summary_json, 'total', items.length)} />
          <SummaryChip label="Chấp nhận" value={summaryCount(record.summary_json, 'accepted')} tone="emerald" />
          <SummaryChip label="Điều chỉnh" value={summaryCount(record.summary_json, 'adjust_quantity')} tone="blue" />
          <SummaryChip label="Làm lại" value={summaryCount(record.summary_json, 'rework_required')} tone="rose" />
          <SummaryChip label="Chờ evidence" value={summaryCount(record.summary_json, 'waiting_evidence')} tone="amber" />
        </div>

        {locked ? (
          <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
            Đã submit bởi {record.submitted_by_name ?? 'người dùng'} {record.submitted_at ? `ngày ${dateText(record.submitted_at)}` : ''}.
          </div>
        ) : null}

        <div className="mt-3">
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor="subcontractor-reconciliation-comment">
            Ghi chú chung
          </label>
          <textarea
            id="subcontractor-reconciliation-comment"
            className="mt-1 min-h-16 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={comment}
            onChange={(event) => setComment(event.target.value)}
            disabled={locked || pending}
            placeholder="Ghi chú nội bộ khi lưu / submit đối soát"
          />
        </div>

        {items.length ? (
          <div className="mt-3 divide-y divide-line border-y border-line">
            {items.map((item) => (
              <ReconciliationItemRow
                key={item.item_index}
                item={item}
                draft={rowsByIndex.get(item.item_index) ?? rowDraftFromItem(item)}
                boqItems={boqItems}
                disabled={locked || pending}
                onChange={(patch) => updateRow(item.item_index, patch)}
              />
            ))}
          </div>
        ) : (
          <EmptyState title="Checklist chưa có dòng đối soát." />
        )}

        <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3">
          <p className="text-xs text-slate-500">
            Revision form #{record.form_revision_no ?? 0}
            {record.updated_at ? ` · cập nhật ${dateText(record.updated_at)}` : ''}
          </p>
          <div className="flex flex-wrap justify-end gap-2">
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={locked || pending || !items.length}
              onClick={() => saveDraft.mutate()}
            >
              {saveDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              Lưu nháp
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={locked || pending || !items.length}
              onClick={() => submitReconciliation.mutate()}
            >
              {submitReconciliation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              Submit đối soát
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}

function ReconciliationItemRow({
  item,
  draft,
  boqItems,
  disabled,
  onChange,
}: {
  item: SubcontractorReconciliationItem;
  draft: RowDraft;
  boqItems: BOQItem[];
  disabled: boolean;
  onChange: (patch: Partial<RowDraft>) => void;
}) {
  const selectedBOQExists = Boolean(draft.boq_item_id && boqItems.some((boq) => boq.id === draft.boq_item_id));
  const confirmed = parseQuantity(draft.confirmed_quantity);
  const claimed = numberOrNull(item.claimed_quantity);
  const variance = confirmed !== null && claimed !== null ? confirmed - claimed : numberOrNull(item.variance_quantity);

  return (
    <div className="grid gap-3 py-3 lg:grid-cols-[1.2fr_1fr_0.9fr_1fr]">
      <div className="min-w-0">
        <p className="text-sm font-semibold text-ink">{item.checklist_title}</p>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {item.status ? <StatusBadge status={item.status} /> : null}
          {item.owner_response ? <StatusBadge status={item.owner_response} /> : null}
          {item.evidence_id ? <StatusBadge status={item.evidence_status ?? 'submitted'} /> : null}
        </div>
        {item.evidence_name ? <p className="mt-2 truncate text-xs text-slate-500">Evidence: {item.evidence_name}</p> : null}
        {item.boq_item_ref ? <p className="mt-1 truncate text-xs text-slate-500">Ref: {item.boq_item_ref}</p> : null}
      </div>

      <div>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`boq-${item.item_index}`}>
          BOQ
        </label>
        <select
          id={`boq-${item.item_index}`}
          className="mt-1 w-full rounded-md border border-line bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.boq_item_id}
          onChange={(event) => onChange({ boq_item_id: event.target.value })}
          disabled={disabled}
        >
          <option value="">Chưa map BOQ</option>
          {draft.boq_item_id && !selectedBOQExists ? (
            <option value={draft.boq_item_id}>{item.boq_work_item_name ?? draft.boq_item_id}</option>
          ) : null}
          {boqItems.map((boq) => (
            <option key={boq.id} value={boq.id}>
              {boqItemLabel(boq)}
            </option>
          ))}
        </select>
        <p className="mt-1 text-xs text-slate-500">
          {item.current_boq_work_item_name ?? item.boq_work_item_name ?? 'Chưa có BOQ hiện tại'}
        </p>
      </div>

      <div className="grid grid-cols-3 gap-2 lg:block lg:space-y-2">
        <QuantityMetric label="Claim" value={item.claimed_quantity} unit={item.unit} />
        <label className="block">
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Xác nhận</span>
          <input
            className="mt-1 h-9 w-full rounded-md border border-line px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            type="number"
            min="0"
            step="0.001"
            value={draft.confirmed_quantity}
            onChange={(event) => onChange({ confirmed_quantity: event.target.value })}
            disabled={disabled}
          />
        </label>
        <QuantityMetric label="Chênh" value={variance} unit={item.unit} signed />
      </div>

      <div>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`decision-${item.item_index}`}>
          Quyết định
        </label>
        <select
          id={`decision-${item.item_index}`}
          className="mt-1 w-full rounded-md border border-line bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.decision}
          onChange={(event) => onChange({ decision: event.target.value })}
          disabled={disabled}
        >
          {decisionOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <input
          className="mt-2 h-9 w-full rounded-md border border-line px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.comment}
          onChange={(event) => onChange({ comment: event.target.value })}
          disabled={disabled}
          placeholder="Ghi chú dòng"
        />
      </div>
    </div>
  );
}

function SummaryChip({
  label,
  value,
  tone = 'slate',
}: {
  label: string;
  value: number;
  tone?: 'slate' | 'emerald' | 'blue' | 'rose' | 'amber';
}) {
  const toneClass = {
    slate: 'bg-slate-50 text-slate-700 ring-slate-200',
    emerald: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    blue: 'bg-blue-50 text-blue-700 ring-blue-200',
    rose: 'bg-rose-50 text-rose-700 ring-rose-200',
    amber: 'bg-amber-50 text-amber-800 ring-amber-200',
  }[tone];
  return (
    <div className={clsx('rounded-md px-3 py-2 ring-1 ring-inset', toneClass)}>
      <p className="text-xs font-semibold">{label}</p>
      <p className="mt-1 text-lg font-bold leading-none">{value}</p>
    </div>
  );
}

function QuantityMetric({ label, value, unit, signed = false }: { label: string; value: unknown; unit?: string; signed?: boolean }) {
  const numeric = numberOrNull(value);
  return (
    <div>
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className={clsx('mt-1 text-sm font-semibold', signed && numeric !== null && numeric < 0 ? 'text-rose-700' : 'text-ink')}>
        {numeric === null ? '-' : `${signed && numeric > 0 ? '+' : ''}${quantityFormatter.format(numeric)}${unit ? ` ${unit}` : ''}`}
      </p>
    </div>
  );
}

function rowDraftFromItem(item: SubcontractorReconciliationItem): RowDraft {
  return {
    item_index: Number(item.item_index ?? 0),
    boq_item_id: item.boq_item_id ?? '',
    confirmed_quantity: inputNumber(item.confirmed_quantity),
    decision: item.decision || 'waiting_evidence',
    comment: item.comment ?? '',
  };
}

function boqItemLabel(item: BOQItem) {
  const quantity = numberOrNull(item.quantity);
  const prefix = item.category ? `${item.category} · ` : '';
  const suffix = quantity === null ? '' : ` (${quantityFormatter.format(quantity)} ${item.unit ?? ''})`;
  return `${prefix}${item.work_item_name}${suffix}`;
}

function inputNumber(value: unknown) {
  const numeric = numberOrNull(value);
  return numeric === null ? '' : String(numeric);
}

function parseQuantity(value: string) {
  if (value.trim() === '') return null;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : null;
}

function numberOrNull(value: unknown) {
  if (value === null || value === undefined || value === '') return null;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : null;
}

function summaryCount(summary: unknown, key: string, fallback = 0) {
  if (!summary || typeof summary !== 'object') return fallback;
  const value = (summary as Record<string, unknown>)[key];
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : fallback;
}
