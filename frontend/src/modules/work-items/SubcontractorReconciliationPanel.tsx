import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useLocation } from 'react-router-dom';
import { Loader2, RefreshCw, Save, Send } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { api, ApiError, get } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type { BOQItem, SubcontractorReconciliationItem, SubcontractorReconciliationResponse } from '../../lib/types/domain';
import { syncWorkItemMutationTransition, workItemAutoStartedNotice } from './workItemMutationContext';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { EvidenceReferenceViewButton } from '../_shared/EvidenceReferenceViewer';

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
  { value: 'waiting_evidence', label: 'Chờ minh chứng' },
  { value: 'rejected', label: 'Từ chối' },
];

const quantityFormatter = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 3 });

export function SubcontractorReconciliationPanel({
  workItemId,
  projectId,
  clientContext = false,
  readOnly = false,
  externalBusy = false,
  revision,
  onDirtyChange,
  onBusyChange,
}: {
  workItemId: string;
  projectId?: string;
  clientContext?: boolean;
  readOnly?: boolean;
  externalBusy?: boolean;
  revision?: number;
  onDirtyChange?: (dirty: boolean) => void;
  onBusyChange?: (busy: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const location = useLocation();
  const [comment, setComment] = useState('');
  const [rows, setRows] = useState<RowDraft[]>([]);
  const [invalidKey, setInvalidKey] = useState('');
  const [dirty, setDirty] = useState(false);
  const dirtyRef = useRef(false);
  const precondition = useRef<string>();
  const baseline = useRef<SubcontractorReconciliationResponse>();
  const [conflict, setConflict] = useState(false);
  const [historyRevision, setHistoryRevision] = useState<number>();
  const panelRef = useRef<HTMLElement | null>(null);
  const focusedAnchor = useRef<string>();
  const panelAnchor = { ref: panelRef, id: revision === undefined ? 'subcontractor-reconciliation' : undefined, tabIndex: -1 };
  const navigateValidation = useValidationNavigation(panelRef);

  const reconciliation = useQuery({
    queryKey: ['subcontractor-reconciliation', workItemId, revision ?? 'current'],
    queryFn: () => get<SubcontractorReconciliationResponse>(`/work-items/${workItemId}/subcontractor-reconciliation${revision === undefined ? '' : `?revision=${revision}`}`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  function resetFromResponse(response?: SubcontractorReconciliationResponse) {
    const record = response?.reconciliation;
    setComment(record?.comment ?? '');
    setRows((record?.items ?? []).map(rowDraftFromItem));
    precondition.current = response?.precondition;
    baseline.current = response;
    dirtyRef.current = false;
    setDirty(false);
    setConflict(false);
  }

  useEffect(() => {
    // Background invalidation must never overwrite an unsaved ledger draft.
    if (!dirtyRef.current) resetFromResponse(reconciliation.data);
  }, [reconciliation.data]);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  function markDirty() { dirtyRef.current = true; setDirty(true); }

  const record = dirty ? baseline.current?.reconciliation : reconciliation.data?.reconciliation;
  const items = record?.items ?? [];
  const boqItems = revision === undefined ? (dirty ? baseline.current : reconciliation.data)?.boq_items ?? [] : [];
  const rowsByIndex = useMemo(() => new Map(rows.map((row) => [row.item_index, row])), [rows]);
  const locked = readOnly || revision !== undefined || record?.status === 'submitted' || reconciliation.data?.reconciliation?.status === 'submitted' || reconciliation.data?.readonly === true || (clientContext && reconciliation.data?.capabilities?.can_write !== true);
  useEffect(() => {
    if (location.hash !== '#subcontractor-reconciliation') { focusedAnchor.current = undefined; return; }
    if (revision !== undefined || !reconciliation.data?.available || reconciliation.isFetching || reconciliation.error || !record) return;
    const anchor = `${workItemId}:${location.hash}`;
    if (!panelRef.current || focusedAnchor.current === anchor) return;
    focusedAnchor.current = anchor;
    panelRef.current.scrollIntoView({ block: 'start', behavior: 'smooth' });
    panelRef.current.focus({ preventScroll: true });
  }, [location.hash, workItemId, revision, reconciliation.data?.available, reconciliation.isFetching, reconciliation.error, record?.id]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['subcontractor-reconciliation', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item-form', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    if (projectId) queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
  };

  async function writeReconciliation(submit: boolean) {
    if (locked || conflict || externalBusy) return null;
    const payload = buildPayload();
    if (!payload) return null;
    if (clientContext && !precondition.current) throw new Error('Tải lại dữ liệu đối soát trước khi lưu.');
    return api<SubcontractorReconciliationResponse>(`/work-items/${workItemId}/subcontractor-reconciliation${submit ? '/submit' : ''}`, {
      method: submit ? 'POST' : 'PUT',
      headers: clientContext ? { 'If-Match': precondition.current! } : undefined,
      body: JSON.stringify(payload),
    });
  }
  function mutationError(error: unknown, title: string) {
    if (clientContext && error instanceof ApiError && [409, 412, 428].includes(error.status)) {
      setConflict(true);
      toast.warn({ title: 'Dữ liệu đối soát đã thay đổi', description: 'Bản nháp đang nhập được giữ lại. Tải lại dữ liệu để đối chiếu trước khi thử lại.' });
      invalidate();
    } else toast.error(title, error);
  }

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
      const invalidRow = sourceRows.find((row) => {
        const confirmedText = row.confirmed_quantity.trim();
        const confirmedQuantity = confirmedText === '' ? null : Number(confirmedText);
        return (confirmedQuantity !== null && (!Number.isFinite(confirmedQuantity) || confirmedQuantity < 0))
          || (row.decision === 'adjust_quantity' && confirmedQuantity === null);
      });
      const key = invalidRow ? `items.${invalidRow.item_index}.confirmed_quantity` : '';
      setInvalidKey(key);
      navigateValidation(key);
      return null;
    }
    return { comment: comment.trim(), items: payloadRows };
  }

  const saveDraft = useMutation({
    mutationFn: () => writeReconciliation(false),
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      syncWorkItemMutationTransition(queryClient, updated);
      toast.success(`Đã lưu nháp đối soát thầu phụ.${workItemAutoStartedNotice(updated)}`);
    },
    onError: (error) => mutationError(error, 'Không lưu được đối soát thầu phụ'),
  });

  const submitReconciliation = useMutation({
    mutationFn: () => writeReconciliation(true),
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      syncWorkItemMutationTransition(queryClient, updated);
      toast.success(`Đã gửi đối soát thầu phụ.${workItemAutoStartedNotice(updated)}`);
    },
    onError: (error) => mutationError(error, 'Không gửi được đối soát thầu phụ'),
  });
  const ownBusy = reconciliation.isFetching || saveDraft.isPending || submitReconciliation.isPending;
  const pending = ownBusy || externalBusy;
  useEffect(() => { onBusyChange?.(ownBusy); }, [ownBusy, onBusyChange]);

  function updateRow(itemIndex: number, patch: Partial<RowDraft>) {
    markDirty();
    setRows((current) => current.map((row) => (row.item_index === itemIndex ? { ...row, ...patch } : row)));
  }

  if (reconciliation.isLoading) {
    return (
      <section {...panelAnchor}>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát thầu phụ</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (reconciliation.error) {
    return (
      <section {...panelAnchor}>
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
    if (!clientContext && !reason.toLowerCase().includes('submit') && !reason.toLowerCase().includes('gửi')) return null;
    return (
      <section {...panelAnchor}>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát thầu phụ</h3>
        <EmptyState title={reason || 'Chốt checklist thầu phụ trước khi đối soát.'} />
        {clientContext ? <button type="button" className="mt-2 rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={() => reconciliation.refetch()}>Tải lại</button> : null}
        {renderHistory()}
      </section>
    );
  }

  if (!record) return clientContext ? <section {...panelAnchor}><h3 className="text-sm font-semibold">Đối soát thầu phụ</h3><EmptyState title="Chưa có dữ liệu đối soát." /><button type="button" onClick={() => reconciliation.refetch()}>Tải lại</button>{renderHistory()}</section> : null;

  function renderHistory() {
    if (!clientContext || revision !== undefined || !reconciliation.data?.history?.length) return null;
    return <div className="mt-3 rounded-md border border-line bg-white p-3">
      <h4 className="text-sm font-semibold text-ink">Lịch sử đối soát</h4>
      {reconciliation.data.history.map((entry) => <div key={entry.id} className="mt-2 flex flex-wrap items-center justify-between gap-2 text-sm">
        <span>Phiên bản hồ sơ #{entry.form_revision_no} · <StatusBadge status={entry.status} /> {entry.updated_at ? dateText(entry.updated_at) : ''}</span>
        <button type="button" className="rounded-md border border-line px-3 py-1 text-sm font-semibold" onClick={() => setHistoryRevision(historyRevision === entry.form_revision_no ? undefined : entry.form_revision_no)}>{historyRevision === entry.form_revision_no ? 'Đóng' : 'Xem'}</button>
      </div>)}
      {historyRevision !== undefined ? <div className="mt-3 border-t border-line pt-3"><SubcontractorReconciliationPanel key={historyRevision} workItemId={workItemId} clientContext readOnly revision={historyRevision} /></div> : null}
    </div>;
  }

  return (
    <section {...panelAnchor}>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">Đối soát thầu phụ</h3>
          <p className="mt-1 text-xs text-slate-500">
            {[record.subcontractor_name, record.work_package, record.construction_area].filter(Boolean).join(' · ') || 'Checklist thầu phụ'}
          </p>
        </div>
        {clientContext && !record.id ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-700">Chưa lưu đối soát</span> : <StatusBadge status={record.status} />}
      </div>

      <div className="rounded-md border border-line bg-white p-3">
        {conflict ? <div role="alert" className="mb-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">Dữ liệu nguồn đã thay đổi. Bản nháp đang nhập được giữ lại; sao chép nội dung cần giữ trước khi tải lại.<button type="button" disabled={pending} className="mt-2 block rounded-md border border-amber-300 px-3 py-2 font-semibold" onClick={async () => { const result = await reconciliation.refetch(); if (result.data && !result.error) resetFromResponse(result.data); }}>Tải lại dữ liệu đối soát</button></div> : null}
        {clientContext && locked && record.status !== 'submitted' ? <p className="mb-3 rounded-md border border-line bg-slate-50 px-3 py-2 text-sm text-slate-600">Đối soát đang ở chế độ chỉ xem.</p> : null}
        <div className="grid gap-2 text-xs sm:grid-cols-2 xl:grid-cols-5">
          <SummaryChip label="Tổng" value={summaryCount(record.summary_json, 'total', items.length)} />
          <SummaryChip label="Chấp nhận" value={summaryCount(record.summary_json, 'accepted')} tone="emerald" />
          <SummaryChip label="Điều chỉnh" value={summaryCount(record.summary_json, 'adjust_quantity')} tone="blue" />
          <SummaryChip label="Làm lại" value={summaryCount(record.summary_json, 'rework_required')} tone="rose" />
          <SummaryChip label="Chờ minh chứng" value={summaryCount(record.summary_json, 'waiting_evidence')} tone="amber" />
        </div>

        {record.status === 'submitted' ? (
          <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
            Đã gửi bởi {identityReferenceText(record.submitted_by_name, record.submitted_by_account_deleted, 'người dùng')} {record.submitted_at ? `ngày ${dateText(record.submitted_at)}` : ''}.
          </div>
        ) : null}

        <div className="mt-3">
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`subcontractor-reconciliation-comment-${revision ?? 'current'}`}>
            Ghi chú chung
          </label>
          <textarea
            id={`subcontractor-reconciliation-comment-${revision ?? 'current'}`}
            className="mt-1 min-h-16 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={comment}
            onChange={(event) => { markDirty(); setComment(event.target.value); }}
            disabled={locked || pending}
            placeholder="Ghi chú nội bộ khi lưu / gửi đối soát"
          />
        </div>

        {items.length ? (
          <div className="mt-3 divide-y divide-line border-y border-line">
            {items.map((item) => (
              <ReconciliationItemRow
                key={item.item_index}
                item={revision === undefined ? item : { ...item, current_boq_work_item_name: undefined, current_boq_quantity: undefined, current_boq_unit: undefined }}
                draft={rowsByIndex.get(item.item_index) ?? rowDraftFromItem(item)}
                boqItems={boqItems}
                historical={revision !== undefined}
                fieldPrefix={`reconciliation-${revision ?? 'current'}`}
                invalid={invalidKey === `items.${item.item_index}.confirmed_quantity`}
                disabled={locked || pending || conflict}
                onChange={(patch) => {
                  updateRow(item.item_index, patch);
                  if (patch.confirmed_quantity !== undefined) setInvalidKey('');
                }}
              />
            ))}
          </div>
        ) : (
          <EmptyState title="Checklist chưa có dòng đối soát." />
        )}

        <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3">
          <p className="text-xs text-slate-500">
            Phiên bản hồ sơ #{record.form_revision_no ?? 0}
            {record.updated_at ? ` · cập nhật ${dateText(record.updated_at)}` : ''}
          </p>
          <div className="flex flex-wrap justify-end gap-2">
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={locked || pending || conflict || !items.length}
              onClick={() => saveDraft.mutate()}
            >
              {saveDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              Lưu nháp
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={locked || pending || conflict || !items.length}
              onClick={() => submitReconciliation.mutate()}
            >
              {submitReconciliation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              Gửi đối soát
            </button>
          </div>
        </div>
      </div>
      {renderHistory()}
    </section>
  );
}

function ReconciliationItemRow({
  item,
  draft,
  boqItems,
  invalid,
  disabled,
  onChange,
  historical = false,
  fieldPrefix = 'reconciliation-current',
}: {
  item: SubcontractorReconciliationItem;
  draft: RowDraft;
  boqItems: BOQItem[];
  invalid: boolean;
  disabled: boolean;
  onChange: (patch: Partial<RowDraft>) => void;
  historical?: boolean;
  fieldPrefix?: string;
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
        {item.evidence_id ? <div className="mt-2 flex items-center gap-2"><p className="min-w-0 flex-1 truncate text-xs text-slate-500">Minh chứng: {item.evidence_name || item.evidence_id}</p><EvidenceReferenceViewButton reference={{ id: item.evidence_id, name: item.evidence_name || 'Minh chứng đối soát', headerStatus: <StatusBadge status={item.evidence_status ?? 'submitted'} /> }} /></div> : null}
        {item.boq_item_ref ? <p className="mt-1 truncate text-xs text-slate-500">Ref: {item.boq_item_ref}</p> : null}
      </div>

      <div>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`${fieldPrefix}-boq-${item.item_index}`}>
          BOQ
        </label>
        <select
          id={`${fieldPrefix}-boq-${item.item_index}`}
          className="mt-1 w-full rounded-md border border-line bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.boq_item_id}
          onChange={(event) => onChange({ boq_item_id: event.target.value })}
          disabled={disabled}
        >
          <option value="">Chưa gắn BOQ</option>
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
          {historical ? item.boq_work_item_name ?? 'Chưa gắn BOQ trong phiên bản này' : item.current_boq_work_item_name ?? item.boq_work_item_name ?? 'Chưa có BOQ hiện tại'}
        </p>
      </div>

      <div className="grid grid-cols-3 gap-2 lg:block lg:space-y-2">
        <QuantityMetric label="Yêu cầu" value={item.claimed_quantity} unit={item.unit} />
        <label className="block" data-validation-key={`items.${item.item_index}.confirmed_quantity`} data-validation-error={invalid ? 'true' : undefined}>
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Xác nhận</span>
          <input
            className={clsx('mt-1 h-9 w-full rounded-md border px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50', invalid ? 'border-rose-300 bg-rose-50' : 'border-line')}
            name={`items.${item.item_index}.confirmed_quantity`}
            type="number"
            min="0"
            step="0.001"
            value={draft.confirmed_quantity}
            onChange={(event) => onChange({ confirmed_quantity: event.target.value })}
            disabled={disabled}
            aria-invalid={invalid}
          />
        </label>
        <QuantityMetric label="Chênh" value={variance} unit={item.unit} signed />
      </div>

      <div>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`${fieldPrefix}-decision-${item.item_index}`}>
          Quyết định
        </label>
        <select
          id={`${fieldPrefix}-decision-${item.item_index}`}
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
