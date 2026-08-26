import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, RefreshCw, Save, Send } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type { OwnerClientReconciliationItem, OwnerClientReconciliationResponse, ProjectOperationsRecord } from '../../lib/types/domain';
import { syncWorkItemMutationTransition, workItemAutoStartedNotice } from './workItemMutationContext';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { EvidenceReferenceViewButton } from '../_shared/EvidenceReferenceViewer';

type RowDraft = {
  item_index: number;
  acceptance_id: string;
  decision: string;
  comment: string;
};

type ReconciliationPayload = {
  comment: string;
  items: Array<{
    item_index: number;
    acceptance_id: string;
    decision: string;
    comment: string;
  }>;
};

const decisionOptions = [
  { value: 'accepted', label: 'Chấp nhận' },
  { value: 'conditional_acceptance', label: 'Chấp nhận có điều kiện' },
  { value: 'rework_required', label: 'Yêu cầu sửa lỗi' },
  { value: 'waiting_internal_acceptance', label: 'Chờ nghiệm thu nội bộ' },
  { value: 'rejected', label: 'Từ chối' },
  { value: 'not_applicable', label: 'Không áp dụng' },
];

export function OwnerClientReconciliationPanel({
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
  const [invalidKey, setInvalidKey] = useState('');
  const panelRef = useRef<HTMLElement | null>(null);
  const navigateValidation = useValidationNavigation(panelRef);

  const reconciliation = useQuery({
    queryKey: ['owner-client-reconciliation', workItemId],
    queryFn: () => get<OwnerClientReconciliationResponse>(`/work-items/${workItemId}/owner-client-reconciliation`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  function resetFromResponse(response?: OwnerClientReconciliationResponse) {
    const record = response?.reconciliation;
    setComment(record?.comment ?? '');
    setRows((record?.items ?? []).map(rowDraftFromItem));
  }

  useEffect(() => {
    resetFromResponse(reconciliation.data);
  }, [reconciliation.data]);

  const record = reconciliation.data?.reconciliation;
  const items = record?.items ?? [];
  const acceptances = reconciliation.data?.acceptances ?? [];
  const rowsByIndex = useMemo(() => new Map(rows.map((row) => [row.item_index, row])), [rows]);
  const acceptanceById = useMemo(() => new Map(acceptances.map((row) => [String(row.id), row])), [acceptances]);
  const locked = record?.status === 'submitted';

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['owner-client-reconciliation', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    if (projectId) {
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      queryClient.invalidateQueries({ queryKey: ['acceptances', projectId] });
    }
  };

  function buildPayload(requireComplete: boolean): ReconciliationPayload | null {
    const invalid: string[] = [];
    const sourceRows = items.map((item) => rowsByIndex.get(item.item_index) ?? rowDraftFromItem(item));
    const payloadRows = sourceRows.map((row) => {
      if (requireComplete && ['accepted', 'conditional_acceptance'].includes(row.decision) && !row.acceptance_id) {
        invalid.push(`Dòng ${row.item_index + 1}: nghiệm thu nội bộ`);
      }
      return {
        item_index: row.item_index,
        acceptance_id: row.acceptance_id,
        decision: row.decision,
        comment: row.comment.trim(),
      };
    });
    if (invalid.length) {
      toast.warn({ title: 'Dữ liệu đối soát chưa hợp lệ', description: invalid.join(', ') });
      const invalidRow = sourceRows.find((row) => ['accepted', 'conditional_acceptance'].includes(row.decision) && !row.acceptance_id);
      const key = invalidRow ? `items.${invalidRow.item_index}.acceptance_id` : '';
      setInvalidKey(key);
      navigateValidation(key);
      return null;
    }
    return { comment: comment.trim(), items: payloadRows };
  }

  const saveDraft = useMutation({
    mutationFn: async () => {
      const payload = buildPayload(false);
      if (!payload) return null;
      return put<OwnerClientReconciliationResponse>(`/work-items/${workItemId}/owner-client-reconciliation`, payload);
    },
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      syncWorkItemMutationTransition(queryClient, updated);
      toast.success(`Đã lưu nháp đối soát chủ đầu tư.${workItemAutoStartedNotice(updated)}`);
    },
    onError: (error) => toast.error('Không lưu được đối soát chủ đầu tư', error),
  });

  const submitReconciliation = useMutation({
    mutationFn: async () => {
      const payload = buildPayload(true);
      if (!payload) return null;
      return post<OwnerClientReconciliationResponse>(`/work-items/${workItemId}/owner-client-reconciliation/submit`, payload);
    },
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      syncWorkItemMutationTransition(queryClient, updated);
      toast.success(`Đã gửi đối soát chủ đầu tư.${workItemAutoStartedNotice(updated)}`);
    },
    onError: (error) => toast.error('Không gửi được đối soát chủ đầu tư', error),
  });
  const pending = reconciliation.isFetching || saveDraft.isPending || submitReconciliation.isPending;

  function updateRow(itemIndex: number, patch: Partial<RowDraft>) {
    setRows((current) => current.map((row) => (row.item_index === itemIndex ? { ...row, ...patch } : row)));
  }

  if (reconciliation.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát chủ đầu tư</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (reconciliation.error) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát chủ đầu tư</h3>
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
    if (!reason.toLowerCase().includes('submit') && !reason.toLowerCase().includes('gửi')) return null;
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Đối soát chủ đầu tư</h3>
        <EmptyState title={reason || 'Gửi checklist chủ đầu tư trước khi đối soát.'} />
      </section>
    );
  }

  if (!record) return null;

  return (
    <section ref={panelRef}>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">Đối soát chủ đầu tư</h3>
          <p className="mt-1 text-xs text-slate-500">
            {[record.client_representative, record.handover_scope, record.scope_detail].filter(Boolean).join(' · ') || 'Checklist chủ đầu tư'}
          </p>
        </div>
        <StatusBadge status={record.status} />
      </div>

      <div className="rounded-md border border-line bg-white p-3">
        <div className="grid gap-2 text-xs sm:grid-cols-2 xl:grid-cols-6">
          <SummaryChip label="Tổng" value={summaryCount(record.summary_json, 'total', items.length)} />
          <SummaryChip label="Chấp nhận" value={summaryCount(record.summary_json, 'accepted')} tone="emerald" />
          <SummaryChip label="Có điều kiện" value={summaryCount(record.summary_json, 'conditional_acceptance')} tone="blue" />
          <SummaryChip label="Sửa lỗi" value={summaryCount(record.summary_json, 'rework_required')} tone="rose" />
          <SummaryChip label="Chờ nội bộ" value={summaryCount(record.summary_json, 'waiting_internal_acceptance')} tone="amber" />
          <SummaryChip label="Không áp dụng" value={summaryCount(record.summary_json, 'not_applicable')} />
        </div>

        {locked ? (
          <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
            Đã gửi bởi {identityReferenceText(record.submitted_by_name, record.submitted_by_account_deleted, 'người dùng')} {record.submitted_at ? `ngày ${dateText(record.submitted_at)}` : ''}.
          </div>
        ) : null}

        <div className="mt-3">
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor="owner-client-reconciliation-comment">
            Ghi chú chung
          </label>
          <textarea
            id="owner-client-reconciliation-comment"
            className="mt-1 min-h-16 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={comment}
            onChange={(event) => setComment(event.target.value)}
            disabled={locked || pending}
            placeholder="Ghi chú nội bộ khi lưu / gửi đối soát"
          />
        </div>

        {items.length ? (
          <div className="mt-3 divide-y divide-line border-y border-line">
            {items.map((item) => (
              <ReconciliationItemRow
                key={item.item_index}
                item={item}
                draft={rowsByIndex.get(item.item_index) ?? rowDraftFromItem(item)}
                acceptances={acceptances}
                acceptanceById={acceptanceById}
                invalid={invalidKey === `items.${item.item_index}.acceptance_id`}
                disabled={locked || pending}
                onChange={(patch) => {
                  updateRow(item.item_index, patch);
                  if (patch.acceptance_id) setInvalidKey('');
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
              Gửi đối soát
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
  acceptances,
  acceptanceById,
  invalid,
  disabled,
  onChange,
}: {
  item: OwnerClientReconciliationItem;
  draft: RowDraft;
  acceptances: ProjectOperationsRecord[];
  acceptanceById: Map<string, ProjectOperationsRecord>;
  invalid: boolean;
  disabled: boolean;
  onChange: (patch: Partial<RowDraft>) => void;
}) {
  const selected = acceptanceById.get(draft.acceptance_id);
  const selectedExists = Boolean(selected);

  return (
    <div className="grid gap-3 py-3 lg:grid-cols-[1.2fr_1fr_1fr]">
      <div className="min-w-0">
        <p className="text-sm font-semibold text-ink">{item.checklist_title}</p>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {item.status ? <StatusBadge status={item.status} /> : null}
          {item.client_decision ? <StatusBadge status={item.client_decision} /> : null}
          {item.severity ? <StatusBadge status={item.severity} /> : null}
          {item.evidence_id ? <StatusBadge status={item.evidence_status ?? 'submitted'} /> : null}
        </div>
        {item.evidence_id ? <div className="mt-2 flex items-center gap-2"><p className="min-w-0 flex-1 truncate text-xs text-slate-500">Minh chứng: {item.evidence_name || item.evidence_id}</p><EvidenceReferenceViewButton reference={{ id: item.evidence_id, name: item.evidence_name || 'Minh chứng đối soát', headerStatus: <StatusBadge status={item.evidence_status ?? 'submitted'} /> }} /></div> : null}
        {item.output_ref ? <p className="mt-1 truncate text-xs text-slate-500">Đầu ra: {item.output_ref}</p> : null}
        {item.required_action ? <p className="mt-2 text-xs leading-5 text-slate-600">Yêu cầu: {item.required_action}</p> : null}
      </div>

      <div data-validation-key={`items.${item.item_index}.acceptance_id`} data-validation-error={invalid ? 'true' : undefined}>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`acceptance-${item.item_index}`}>
          Nghiệm thu nội bộ
        </label>
        <select
          id={`acceptance-${item.item_index}`}
          name={`items.${item.item_index}.acceptance_id`}
          className={clsx('mt-1 w-full rounded-md border bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50', invalid ? 'border-rose-300' : 'border-line')}
          value={draft.acceptance_id}
          onChange={(event) => onChange({ acceptance_id: event.target.value })}
          disabled={disabled}
          aria-invalid={invalid}
        >
          <option value="">Chưa liên kết nghiệm thu</option>
          {draft.acceptance_id && !selectedExists ? (
            <option value={draft.acceptance_id}>{item.acceptance_title ?? draft.acceptance_id}</option>
          ) : null}
          {acceptances.map((acceptance) => (
            <option key={acceptance.id} value={acceptance.id}>
              {acceptanceLabel(acceptance)}
            </option>
          ))}
        </select>
        <p className="mt-1 text-xs text-slate-500">
          {selected
            ? `${String(selected.status ?? '-')} / ${String(selected.result ?? '-')}`
            : item.current_acceptance_title ?? item.acceptance_title ?? 'Chưa có nghiệm thu hiện tại'}
        </p>
      </div>

      <div>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`owner-client-decision-${item.item_index}`}>
          Quyết định đối soát
        </label>
        <select
          id={`owner-client-decision-${item.item_index}`}
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

function rowDraftFromItem(item: OwnerClientReconciliationItem): RowDraft {
  return {
    item_index: Number(item.item_index ?? 0),
    acceptance_id: item.acceptance_id ?? '',
    decision: item.decision || 'waiting_internal_acceptance',
    comment: item.comment ?? '',
  };
}

function acceptanceLabel(acceptance: ProjectOperationsRecord) {
  const title = String(acceptance.title ?? acceptance.id);
  const type = acceptance.type ? `${String(acceptance.type)} · ` : '';
  const result = acceptance.result ? ` / ${String(acceptance.result)}` : '';
  return `${type}${title} (${String(acceptance.status ?? '-')}${result})`;
}

function summaryCount(summary: unknown, key: string, fallback = 0) {
  if (!summary || typeof summary !== 'object') return fallback;
  const value = (summary as Record<string, unknown>)[key];
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : fallback;
}
