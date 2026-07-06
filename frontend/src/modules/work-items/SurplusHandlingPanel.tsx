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
import type {
  EvidenceRecord,
  ProjectMaterialBalance,
  SurplusHandlingRecord,
  SurplusHandlingResponse,
  Warehouse,
} from '../../lib/types/domain';

type Draft = {
  id: string;
  site_balance_id: string;
  handling_type: string;
  quantity: string;
  unit: string;
  warehouse_id: string;
  custody_location: string;
  condition: string;
  evidence_id: string;
  comment: string;
};

type SurplusPayload = {
  site_balance_id: string;
  handling_type: string;
  quantity: number;
  unit: string;
  warehouse_id: string;
  custody_location: string;
  condition: string;
  evidence_id: string;
  comment: string;
};

const handlingOptions = [
  { value: 'return_to_warehouse', label: 'Lưu kho' },
  { value: 'return_to_subcontractor', label: 'Trả thầu phụ' },
];

const conditionOptions = [
  { value: 'usable', label: 'Dùng được' },
  { value: 'damaged', label: 'Hư hỏng' },
  { value: 'mixed', label: 'Lẫn tình trạng' },
  { value: 'unknown', label: 'Chưa rõ' },
];

export function SurplusHandlingPanel({
  workItemId,
  projectId,
  evidence,
}: {
  workItemId: string;
  projectId?: string;
  evidence: EvidenceRecord[];
}) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});

  const handlingQuery = useQuery({
    queryKey: ['surplus-handlings', workItemId],
    queryFn: () => get<SurplusHandlingResponse>(`/work-items/${workItemId}/surplus-handlings`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  const balances = handlingQuery.data?.material_balances ?? [];
  const warehouses = handlingQuery.data?.warehouses ?? [];
  const handlings = handlingQuery.data?.handlings ?? [];
  const evidenceOptions = useMemo(() => evidence.filter((item) => item.status !== 'rejected'), [evidence]);
  const pendingBalances = useMemo(
    () => balances.filter((balance) => Number(balance.surplus_quantity ?? 0) > 0),
    [balances],
  );
  const submitted = handlings.filter((handling) => handling.status === 'submitted');

  useEffect(() => {
    const next: Record<string, Draft> = {};
    const draftByBalance = new Map(
      handlings
        .filter((handling) => handling.status === 'draft')
        .map((handling) => [String(handling.site_balance_id), handling]),
    );
    for (const balance of pendingBalances) {
      const saved = draftByBalance.get(String(balance.id));
      next[String(balance.id)] = saved ? draftFromRecord(saved) : defaultDraft(balance);
    }
    setDrafts(next);
  }, [handlings, pendingBalances]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['surplus-handlings', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    queryClient.invalidateQueries({ queryKey: ['stock'] });
    if (projectId) {
      queryClient.invalidateQueries({ queryKey: ['material-balances', projectId] });
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
    }
  };

  function updateDraft(siteBalanceId: string, patch: Partial<Draft>) {
    setDrafts((current) => ({
      ...current,
      [siteBalanceId]: { ...current[siteBalanceId], ...patch },
    }));
  }

  function buildPayload(balance: ProjectMaterialBalance, draft: Draft, requireComplete: boolean): SurplusPayload | null {
    const invalid: string[] = [];
    const quantity = numberInput(draft.quantity);
    const surplus = Number(balance.surplus_quantity ?? 0);
    const unit = (draft.unit || balance.unit || '').trim();
    if (!Number.isFinite(quantity) || quantity <= 0) invalid.push('Số lượng');
    if (quantity > surplus + 0.000001) invalid.push('Vượt dư');
    if (!unit) invalid.push('Đơn vị');
    if (!draft.warehouse_id) invalid.push('Kho');
    if (requireComplete && !draft.custody_location.trim()) invalid.push('Vị trí lưu/trả');
    if (requireComplete && !draft.evidence_id) invalid.push('Minh chứng');
    if (invalid.length) {
      toast.warn({ title: 'Dữ liệu xử lý vật tư dư chưa hợp lệ', description: invalid.join(', ') });
      return null;
    }
    return {
      site_balance_id: String(balance.id),
      handling_type: draft.handling_type,
      quantity,
      unit,
      warehouse_id: draft.warehouse_id,
      custody_location: draft.custody_location.trim(),
      condition: draft.condition,
      evidence_id: draft.evidence_id,
      comment: draft.comment.trim(),
    };
  }

  async function saveDraftRecord(balance: ProjectMaterialBalance, draft: Draft, requireComplete: boolean) {
    const payload = buildPayload(balance, draft, requireComplete);
    if (!payload) return null;
    if (draft.id) {
      return put<SurplusHandlingRecord>(`/surplus-handlings/${draft.id}`, payload);
    }
    return post<SurplusHandlingRecord>(`/work-items/${workItemId}/surplus-handlings`, payload);
  }

  const saveDraft = useMutation({
    mutationFn: async ({ balance, draft }: { balance: ProjectMaterialBalance; draft: Draft }) => saveDraftRecord(balance, draft, false),
    onSuccess: (record) => {
      if (!record) return;
      invalidate();
      toast.success('Đã lưu nháp xử lý vật tư dư');
    },
    onError: (error) => toast.error('Không lưu được xử lý vật tư dư', error),
  });

  const submitDraft = useMutation({
    mutationFn: async ({ balance, draft }: { balance: ProjectMaterialBalance; draft: Draft }) => {
      const record = await saveDraftRecord(balance, draft, true);
      if (!record) return null;
      return post<SurplusHandlingRecord>(`/surplus-handlings/${record.id}/submit`);
    },
    onSuccess: (record) => {
      if (!record) return;
      invalidate();
      toast.success('Đã gửi xử lý vật tư dư');
    },
    onError: (error) => toast.error('Không gửi được xử lý vật tư dư', error),
  });
  const actionPending = saveDraft.isPending || submitDraft.isPending || handlingQuery.isFetching;

  if (handlingQuery.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Lưu kho vật tư tồn</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (handlingQuery.error) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Lưu kho vật tư tồn</h3>
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-semibold text-amber-900">Không tải được dữ liệu vật tư tồn</p>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-amber-300 px-3 py-2 text-sm font-semibold text-amber-900 hover:bg-amber-100"
              onClick={() => handlingQuery.refetch()}
            >
              <RefreshCw className="h-4 w-4" />
              Tải lại
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (!handlingQuery.data?.available) return null;

  return (
    <section>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">Lưu kho vật tư tồn</h3>
          <p className="mt-1 text-xs text-slate-500">
            {pendingBalances.length ? `${pendingBalances.length} vật tư còn dư` : 'Không còn vật tư dư cần xử lý'}
          </p>
        </div>
        <StatusBadge status={pendingBalances.length ? 'pending' : 'handled'} />
      </div>

      <div className="rounded-md border border-line bg-white p-3">
        {pendingBalances.length ? (
          <div className="divide-y divide-line border-y border-line">
            {pendingBalances.map((balance) => {
              const draft = drafts[String(balance.id)] ?? defaultDraft(balance);
              return (
                <SurplusRow
                  key={balance.id}
                  balance={balance}
                  draft={draft}
                  warehouses={warehouses}
                  evidence={evidenceOptions}
                  disabled={actionPending}
                  onChange={(patch) => updateDraft(String(balance.id), patch)}
                  onSave={() => saveDraft.mutate({ balance, draft })}
                  onSubmit={() => submitDraft.mutate({ balance, draft })}
                  saving={saveDraft.isPending}
                  submitting={submitDraft.isPending}
                />
              );
            })}
          </div>
        ) : (
          <EmptyState title="Chưa có vật tư dư cần xử lý cho đầu việc này" />
        )}

        <div className="mt-4">
          <h4 className="mb-2 text-sm font-semibold text-ink">Hồ sơ đã gửi</h4>
          {submitted.length ? (
            <div className="space-y-2">
              {submitted.map((handling) => (
                <div key={handling.id} className="grid gap-2 rounded-md border border-line px-3 py-2 text-sm md:grid-cols-[1fr_auto_auto]">
                  <div className="min-w-0">
                    <p className="truncate font-semibold text-ink">
                      {handling.material_name ?? handling.material_id} · {handlingLabel(handling.handling_type)}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      {handling.stock_movement_code ?? handling.handling_code}
                      {handling.submitted_at ? ` · ${dateText(handling.submitted_at)}` : ''}
                    </p>
                  </div>
                  <span className="text-slate-700">{formatQty(Number(handling.quantity ?? 0))} {handling.unit}</span>
                  <StatusBadge status={handling.status} />
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="Chưa có hồ sơ xử lý vật tư dư đã gửi" />
          )}
        </div>
      </div>
    </section>
  );
}

function SurplusRow({
  balance,
  draft,
  warehouses,
  evidence,
  disabled,
  onChange,
  onSave,
  onSubmit,
  saving,
  submitting,
}: {
  balance: ProjectMaterialBalance;
  draft: Draft;
  warehouses: Warehouse[];
  evidence: EvidenceRecord[];
  disabled: boolean;
  onChange: (patch: Partial<Draft>) => void;
  onSave: () => void;
  onSubmit: () => void;
  saving: boolean;
  submitting: boolean;
}) {
  const surplus = Number(balance.surplus_quantity ?? 0);
  const quantity = numberInput(draft.quantity);
  const over = quantity > surplus + 0.000001;

  return (
    <div className="grid gap-3 py-3 xl:grid-cols-[1fr_1fr_1.2fr_auto]">
      <div className="min-w-0">
        <p className="truncate text-sm font-semibold text-ink">{balance.material_name ?? balance.material_id}</p>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {balance.code ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{balance.code}</span> : null}
          <StatusBadge status={balance.surplus_status ?? 'pending'} />
          {over ? <span className="rounded-md bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700">Vượt dư</span> : null}
        </div>
        <p className="mt-2 text-xs text-slate-500">
          Dư {formatQty(surplus)} {balance.unit ?? draft.unit}
        </p>
      </div>

      <div className="grid grid-cols-2 gap-2">
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
          Xử lý
          <select
            className="mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={draft.handling_type}
            onChange={(event) => onChange({ handling_type: event.target.value })}
            disabled={disabled}
          >
            {handlingOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
          Số lượng
          <input
            type="number"
            min="0"
            step="0.001"
            className={clsx(
              'mt-1 h-9 w-full rounded-md border px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50',
              over ? 'border-rose-300 bg-rose-50' : 'border-line',
            )}
            value={draft.quantity}
            onChange={(event) => onChange({ quantity: event.target.value })}
            disabled={disabled}
          />
        </label>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
          Đơn vị
          <input
            className="mt-1 h-9 w-full rounded-md border border-line px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={draft.unit}
            onChange={(event) => onChange({ unit: event.target.value })}
            disabled={disabled}
          />
        </label>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
          Tình trạng
          <select
            className="mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={draft.condition}
            onChange={(event) => onChange({ condition: event.target.value })}
            disabled={disabled}
          >
            {conditionOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>
      </div>

      <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-1">
        <select
          className="h-9 rounded-md border border-line bg-white px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.warehouse_id}
          onChange={(event) => onChange({ warehouse_id: event.target.value })}
          disabled={disabled}
          aria-label="Kho trung chuyển"
        >
          <option value="">Chọn kho</option>
          {warehouses.map((warehouse) => (
            <option key={warehouse.id} value={warehouse.id}>{warehouseLabel(warehouse)}</option>
          ))}
        </select>
        <input
          className="h-9 rounded-md border border-line px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.custody_location}
          onChange={(event) => onChange({ custody_location: event.target.value })}
          disabled={disabled}
          placeholder="Vị trí lưu / bàn giao"
        />
        <select
          className="h-9 rounded-md border border-line bg-white px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.evidence_id}
          onChange={(event) => onChange({ evidence_id: event.target.value })}
          disabled={disabled}
          aria-label="Minh chứng vật tư dư"
        >
          <option value="">Chọn minh chứng</option>
          {evidence.map((item) => (
            <option key={item.id} value={item.id}>{evidenceLabel(item)}</option>
          ))}
        </select>
        <input
          className="h-9 rounded-md border border-line px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.comment}
          onChange={(event) => onChange({ comment: event.target.value })}
          disabled={disabled}
          placeholder="Ghi chú"
        />
      </div>

      <div className="flex flex-wrap items-start justify-end gap-2">
        <button
          className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={disabled}
          onClick={onSave}
        >
          {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          Lưu nháp
        </button>
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={disabled}
          onClick={onSubmit}
        >
          {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          Gửi
        </button>
      </div>
    </div>
  );
}

function defaultDraft(balance: ProjectMaterialBalance): Draft {
  return {
    id: '',
    site_balance_id: String(balance.id),
    handling_type: 'return_to_warehouse',
    quantity: String(balance.surplus_quantity ?? 0),
    unit: balance.unit ?? '',
    warehouse_id: '',
    custody_location: '',
    condition: 'usable',
    evidence_id: '',
    comment: '',
  };
}

function draftFromRecord(record: SurplusHandlingRecord): Draft {
  return {
    id: String(record.id),
    site_balance_id: String(record.site_balance_id),
    handling_type: record.handling_type || 'return_to_warehouse',
    quantity: String(record.quantity ?? 0),
    unit: record.unit ?? '',
    warehouse_id: record.warehouse_id ?? '',
    custody_location: record.custody_location ?? '',
    condition: record.condition || 'usable',
    evidence_id: record.evidence_id ?? '',
    comment: record.comment ?? '',
  };
}

function numberInput(value: string) {
  if (value.trim() === '') return 0;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : Number.NaN;
}

function formatQty(value: number) {
  if (!Number.isFinite(value)) return '-';
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 3 }).format(value);
}

function handlingLabel(value: string) {
  return handlingOptions.find((option) => option.value === value)?.label ?? value;
}

function warehouseLabel(warehouse: Warehouse) {
  const location = warehouse.location ? ` · ${warehouse.location}` : '';
  return `${warehouse.name}${location}`;
}

function evidenceLabel(evidence: EvidenceRecord) {
  return `${evidence.name ?? evidence.title ?? evidence.id} (${evidenceStatusLabel(evidence.status)})`;
}

function evidenceStatusLabel(status?: string) {
  if (status === 'accepted') return 'đã chấp nhận';
  if (status === 'rejected') return 'đã từ chối';
  if (status === 'pending') return 'đang chờ';
  if (status === 'draft') return 'nháp';
  if (status === 'submitted') return 'đã gửi';
  return status ?? 'đã gửi';
}
