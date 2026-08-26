import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Plus, RefreshCw, Save, Send, Trash2 } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type {
  EvidenceRecord,
  MaterialCatalogItem,
  MaterialSnapshotItem,
  MaterialSnapshotResponse,
  ProjectMaterialBalance,
} from '../../lib/types/domain';
import { syncWorkItemMutationTransition, workItemAutoStartedNotice } from './workItemMutationContext';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { SelectedEvidenceViewButton } from '../_shared/EvidenceReferenceViewer';

type RowDraft = {
  item_index: number;
  material_id: string;
  material_name: string;
  unit: string;
  source: string;
  before_quantity: string;
  added_quantity: string;
  after_quantity: string;
  waste_quantity: string;
  affects_site_balance: boolean;
  before_evidence_id: string;
  after_evidence_id: string;
  construction_area: string;
  condition: string;
  comment: string;
};

type MaterialSnapshotPayload = {
  snapshot_date: string;
  construction_area: string;
  comment: string;
  items: Array<{
    item_index: number;
    material_id: string;
    material_name: string;
    unit: string;
    source: string;
    before_quantity: number;
    added_quantity: number;
    after_quantity: number;
    waste_quantity: number;
    affects_site_balance: boolean;
    before_evidence_id: string;
    after_evidence_id: string;
    construction_area: string;
    condition: string;
    comment: string;
  }>;
};

const sourceOptions = [
  { value: 'site_balance', label: 'Tồn công trình' },
  { value: 'warehouse_issue', label: 'Xuất kho' },
  { value: 'subcontractor', label: 'Thầu phụ' },
  { value: 'other', label: 'Khác' },
];

const conditionOptions = [
  { value: 'usable', label: 'Dùng được' },
  { value: 'damaged', label: 'Hư hỏng' },
  { value: 'mixed', label: 'Lẫn tình trạng' },
  { value: 'unknown', label: 'Chưa rõ' },
];

export function MaterialSnapshotPanel({
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
  const [snapshotDate, setSnapshotDate] = useState('');
  const [constructionArea, setConstructionArea] = useState('');
  const [comment, setComment] = useState('');
  const [rows, setRows] = useState<RowDraft[]>([]);
  const [invalidKey, setInvalidKey] = useState('');
  const panelRef = useRef<HTMLElement | null>(null);
  const navigateValidation = useValidationNavigation(panelRef);

  const snapshotQuery = useQuery({
    queryKey: ['material-snapshot', workItemId],
    queryFn: () => get<MaterialSnapshotResponse>(`/work-items/${workItemId}/material-snapshot`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  function resetFromResponse(response?: MaterialSnapshotResponse) {
    const record = response?.snapshot;
    setSnapshotDate(dateInputValue(record?.snapshot_date));
    setConstructionArea(record?.construction_area ?? '');
    setComment(record?.comment ?? '');
    setRows((record?.items ?? []).map(rowDraftFromItem));
  }

  useEffect(() => {
    resetFromResponse(snapshotQuery.data);
  }, [snapshotQuery.data]);

  const record = snapshotQuery.data?.snapshot;
  const materials = snapshotQuery.data?.materials ?? [];
  const balances = snapshotQuery.data?.material_balances ?? [];
  const materialById = useMemo(() => new Map(materials.map((material) => [String(material.id), material])), [materials]);
  const balanceByMaterialId = useMemo(() => new Map(balances.map((balance) => [String(balance.material_id), balance])), [balances]);
  const evidenceOptions = useMemo(() => evidence.filter((item) => item.status !== 'rejected'), [evidence]);
  const locked = record?.status === 'submitted' || Boolean(record?.applied_to_site_balance);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['material-snapshot', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    if (projectId) {
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      queryClient.invalidateQueries({ queryKey: ['material-balances', projectId] });
    }
  };

  function buildPayload(requireComplete: boolean): MaterialSnapshotPayload | null {
    const invalid: string[] = [];
    if (requireComplete && !snapshotDate) invalid.push('Ngày ghi nhận');
    if (requireComplete && rows.length === 0) invalid.push('Ít nhất một dòng vật tư');

    const payloadRows = rows.map((row, index) => {
      const material = materialById.get(row.material_id);
      const materialName = (material?.name ?? row.material_name).trim();
      const unit = (row.unit || material?.unit || '').trim();
      const before = numberInput(row.before_quantity);
      const added = numberInput(row.added_quantity);
      const after = numberInput(row.after_quantity);
      const waste = numberInput(row.waste_quantity);
      const used = computedUsed(row);
      const label = `Dòng ${index + 1}`;

      if (row.affects_site_balance && !row.material_id) invalid.push(`${label}: vật tư`);
      if (!row.affects_site_balance && !materialName) invalid.push(`${label}: tên vật tư`);
      if (!unit) invalid.push(`${label}: đơn vị`);
      if ([before, added, after, waste].some((value) => value < 0 || !Number.isFinite(value))) invalid.push(`${label}: số lượng`);
      if (used < -0.000001) invalid.push(`${label}: KL dùng âm`);
      if (row.affects_site_balance && !balanceByMaterialId.has(row.material_id)) invalid.push(`${label}: chưa có tồn công trình`);
      if (requireComplete && row.affects_site_balance) {
        const available = balanceAvailable(balanceByMaterialId.get(row.material_id));
        if (used > available + 0.000001) invalid.push(`${label}: vượt tồn công trình`);
      }

      return {
        item_index: row.item_index,
        material_id: row.material_id,
        material_name: materialName,
        unit,
        source: row.source,
        before_quantity: before,
        added_quantity: added,
        after_quantity: after,
        waste_quantity: waste,
        affects_site_balance: row.affects_site_balance,
        before_evidence_id: row.before_evidence_id,
        after_evidence_id: row.after_evidence_id,
        construction_area: row.construction_area.trim(),
        condition: row.condition,
        comment: row.comment.trim(),
      };
    });

    if (invalid.length) {
      toast.warn({ title: 'Dữ liệu ghi nhận vật tư chưa hợp lệ', description: invalid.slice(0, 6).join(', ') });
      const key = firstMaterialSnapshotInvalidKey({
        snapshotDate,
        rows,
        requireComplete,
        materialById,
        balanceByMaterialId,
      });
      setInvalidKey(key ?? '');
      navigateValidation(key);
      return null;
    }
    return {
      snapshot_date: snapshotDate,
      construction_area: constructionArea.trim(),
      comment: comment.trim(),
      items: payloadRows,
    };
  }

  const saveDraft = useMutation({
    mutationFn: async () => {
      const payload = buildPayload(false);
      if (!payload) return null;
      return put<MaterialSnapshotResponse>(`/work-items/${workItemId}/material-snapshot`, payload);
    },
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      syncWorkItemMutationTransition(queryClient, updated);
      toast.success(`Đã lưu nháp ghi nhận vật tư.${workItemAutoStartedNotice(updated)}`);
    },
    onError: (error) => toast.error('Không lưu được ghi nhận vật tư', error),
  });

  const submitSnapshot = useMutation({
    mutationFn: async () => {
      const payload = buildPayload(true);
      if (!payload) return null;
      return post<MaterialSnapshotResponse>(`/work-items/${workItemId}/material-snapshot/submit`, payload);
    },
    onSuccess: (updated) => {
      if (!updated) return;
      resetFromResponse(updated);
      invalidate();
      syncWorkItemMutationTransition(queryClient, updated);
      toast.success(`Đã gửi ghi nhận vật tư.${workItemAutoStartedNotice(updated)}`);
    },
    onError: (error) => toast.error('Không gửi được ghi nhận vật tư', error),
  });

  const actionPending = snapshotQuery.isFetching || saveDraft.isPending || submitSnapshot.isPending;

  function addRow() {
    const nextIndex = rows.length ? Math.max(...rows.map((row) => row.item_index)) + 1 : 0;
    setRows((current) => [...current, defaultRowDraft(nextIndex)]);
  }

  function updateRow(itemIndex: number, patch: Partial<RowDraft>) {
    setRows((current) => current.map((row) => (row.item_index === itemIndex ? { ...row, ...patch } : row)));
  }

  function removeRow(itemIndex: number) {
    setRows((current) => current.filter((row) => row.item_index !== itemIndex));
  }

  if (snapshotQuery.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Vật tư trước/sau</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (snapshotQuery.error) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Vật tư trước/sau</h3>
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-semibold text-amber-900">Không tải được ghi nhận vật tư</p>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-amber-300 px-3 py-2 text-sm font-semibold text-amber-900 hover:bg-amber-100"
              onClick={() => snapshotQuery.refetch()}
            >
              <RefreshCw className="h-4 w-4" />
              Tải lại
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (!snapshotQuery.data?.available || !record) return null;

  return (
    <section ref={panelRef}>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">Vật tư trước/sau</h3>
          <p className="mt-1 text-xs text-slate-500">
            {record.snapshot_date ? dateText(record.snapshot_date) : 'Chưa chốt ngày'} · {record.construction_area || 'Chưa ghi khu vực'}
          </p>
        </div>
        <StatusBadge status={record.status} />
      </div>

      <div className="rounded-md border border-line bg-white p-3">
        <div className="grid gap-2 text-xs sm:grid-cols-2 xl:grid-cols-5">
          <SummaryChip label="Dòng" value={summaryCount(record.summary_json, 'total', rows.length)} />
          <SummaryChip label="Ảnh hưởng tồn công trình" value={summaryCount(record.summary_json, 'affects_site_balance')} tone="blue" />
          <SummaryChip label="Đủ minh chứng" value={summaryCount(record.summary_json, 'complete_evidence')} tone="emerald" />
          <SummaryChip label="Hư hao" value={summaryCount(record.summary_json, 'with_waste_or_damage')} tone="amber" />
          <SummaryChip label="KL dùng" value={summaryNumber(record.summary_json, 'site_balance_used_total')} />
        </div>

        {locked ? (
          <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
            Đã gửi bởi {identityReferenceText(record.submitted_by_name, record.submitted_by_account_deleted, 'người dùng')} {record.submitted_at ? `ngày ${dateText(record.submitted_at)}` : ''}.
          </div>
        ) : null}

        <div className="mt-3 grid gap-3 lg:grid-cols-[180px_1fr]">
          <div data-validation-key="snapshot_date" data-validation-error={invalidKey === 'snapshot_date' ? 'true' : undefined}>
            <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor="material-snapshot-date">
              Ngày ghi nhận
            </label>
            <input
              id="material-snapshot-date"
              name="snapshot_date"
              type="date"
              className={clsx('mt-1 h-10 w-full rounded-md border px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50', invalidKey === 'snapshot_date' ? 'border-rose-300 bg-rose-50' : 'border-line')}
              value={snapshotDate}
              onChange={(event) => setSnapshotDate(event.target.value)}
              disabled={locked || actionPending}
              aria-invalid={invalidKey === 'snapshot_date'}
            />
          </div>
          <div>
            <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor="material-snapshot-area">
              Khu vực
            </label>
            <input
              id="material-snapshot-area"
              className="mt-1 h-10 w-full rounded-md border border-line px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
              value={constructionArea}
              onChange={(event) => setConstructionArea(event.target.value)}
              disabled={locked || actionPending}
              placeholder="Khu vực thi công / tầng / trục"
            />
          </div>
        </div>

        <div className="mt-3">
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor="material-snapshot-comment">
            Ghi chú chung
          </label>
          <textarea
            id="material-snapshot-comment"
            className="mt-1 min-h-16 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={comment}
            onChange={(event) => setComment(event.target.value)}
            disabled={locked || actionPending}
            placeholder="Ghi chú khi lưu / gửi ghi nhận vật tư"
          />
        </div>

        <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3">
          <p className="text-xs text-slate-500">Khi gửi, hệ thống sẽ cộng khối lượng dùng vào tồn công trình và không tạo phiếu xuất kho.</p>
          <button
            className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={locked || actionPending}
            onClick={addRow}
          >
            <Plus className="h-4 w-4" />
            Thêm dòng
          </button>
        </div>

        {rows.length ? (
          <div className="mt-3 divide-y divide-line border-y border-line" data-validation-key="items" data-validation-error={invalidKey === 'items' ? 'true' : undefined}>
            {rows.map((row, index) => (
              <MaterialSnapshotRow
                key={row.item_index}
                row={row}
                index={index}
                materials={materials}
                materialById={materialById}
                balance={balanceByMaterialId.get(row.material_id)}
                evidence={evidenceOptions}
                invalidKey={invalidKey}
                disabled={locked || actionPending}
                onChange={(patch) => {
                  updateRow(row.item_index, patch);
                  setInvalidKey('');
                }}
                onRemove={() => removeRow(row.item_index)}
              />
            ))}
          </div>
        ) : (
          <div className="mt-3">
            <EmptyState title="Chưa có dòng ghi nhận vật tư." />
          </div>
        )}

        <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3">
          <p className="text-xs text-slate-500">{record.updated_at ? `Cập nhật ${dateText(record.updated_at)}` : 'Chưa lưu ghi nhận vật tư'}</p>
          <div className="flex flex-wrap justify-end gap-2">
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={locked || actionPending}
              onClick={() => saveDraft.mutate()}
            >
              {saveDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              Lưu nháp
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={locked || actionPending || !rows.length}
              onClick={() => submitSnapshot.mutate()}
            >
              {submitSnapshot.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              Gửi ghi nhận
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}

function MaterialSnapshotRow({
  row,
  index,
  materials,
  materialById,
  balance,
  evidence,
  invalidKey,
  disabled,
  onChange,
  onRemove,
}: {
  row: RowDraft;
  index: number;
  materials: MaterialCatalogItem[];
  materialById: Map<string, MaterialCatalogItem>;
  balance?: ProjectMaterialBalance;
  evidence: EvidenceRecord[];
  invalidKey: string;
  disabled: boolean;
  onChange: (patch: Partial<RowDraft>) => void;
  onRemove: () => void;
}) {
  const used = computedUsed(row);
  const available = balanceAvailable(balance);
  const overAvailable = row.affects_site_balance && used > available + 0.000001;
  const material = materialById.get(row.material_id);
  const keyFor = (field: string) => `items.${row.item_index}.${field}`;
  const isInvalid = (field: string) => invalidKey === keyFor(field);

  return (
    <div className="grid gap-3 py-3 xl:grid-cols-[44px_1.2fr_1fr_1.2fr]">
      <div className="flex items-start">
        <button
          className="inline-flex h-10 w-10 items-center justify-center rounded-md border border-line text-slate-500 hover:bg-rose-50 hover:text-rose-700 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={disabled}
          onClick={onRemove}
          title="Xóa dòng"
          aria-label="Xóa dòng vật tư"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </div>
      <div className="min-w-0">
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-semibold text-ink">Dòng {index + 1}</p>
          {overAvailable ? <span className="rounded-md bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700">Vượt tồn công trình</span> : null}
        </div>
        <label className="mt-2 block text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`material-${row.item_index}`}>
          Vật tư
        </label>
        <select
          id={`material-${row.item_index}`}
          name={keyFor('material_id')}
          data-validation-key={keyFor('material_id')}
          data-validation-error={isInvalid('material_id') ? 'true' : undefined}
          className={clsx('mt-1 w-full rounded-md border bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50', isInvalid('material_id') ? 'border-rose-300' : 'border-line')}
          value={row.material_id}
          onChange={(event) => {
            const selected = materialById.get(event.target.value);
            onChange({
              material_id: event.target.value,
              material_name: selected?.name ?? '',
              unit: selected?.unit ?? row.unit,
            });
          }}
          disabled={disabled}
          aria-invalid={isInvalid('material_id')}
        >
          <option value="">Chọn vật tư</option>
          {materials.map((item) => (
            <option key={item.id} value={item.id}>
              {materialLabel(item)}
            </option>
          ))}
        </select>
        {!row.affects_site_balance ? (
          <input
            name={keyFor('material_name')}
            data-validation-key={keyFor('material_name')}
            data-validation-error={isInvalid('material_name') ? 'true' : undefined}
            className={clsx('mt-2 h-9 w-full rounded-md border px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50', isInvalid('material_name') ? 'border-rose-300 bg-rose-50' : 'border-line')}
            value={row.material_name}
            onChange={(event) => onChange({ material_name: event.target.value })}
            disabled={disabled}
            placeholder="Tên vật tư ngoài tồn công trình"
            aria-invalid={isInvalid('material_name')}
          />
        ) : null}
        <div className="mt-2 grid grid-cols-2 gap-2">
          <select
            className="h-9 rounded-md border border-line bg-white px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={row.source}
            onChange={(event) => {
              const source = event.target.value;
              onChange({
                source,
                affects_site_balance: source === 'site_balance' || source === 'warehouse_issue',
              });
            }}
            disabled={disabled}
            aria-label="Nguồn vật tư"
          >
            {sourceOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <select
            className="h-9 rounded-md border border-line bg-white px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={row.condition}
            onChange={(event) => onChange({ condition: event.target.value })}
            disabled={disabled}
            aria-label="Tình trạng"
          >
            {conditionOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
        <label className="mt-2 inline-flex items-center gap-2 text-xs font-semibold text-slate-600">
          <input
            type="checkbox"
            checked={row.affects_site_balance}
            onChange={(event) => onChange({ affects_site_balance: event.target.checked })}
            disabled={disabled}
          />
          Cộng khối lượng dùng vào tồn công trình khi gửi
        </label>
        <p className="mt-1 text-xs text-slate-500">
          {row.affects_site_balance
            ? `${material?.code ?? 'Tồn công trình'} · còn ${formatQty(available)} ${row.unit || material?.unit || ''}`
            : 'Dòng ghi nhận báo cáo, không ảnh hưởng tồn công trình'}
        </p>
      </div>

      <div>
        <div className="grid grid-cols-2 gap-2">
          <QuantityInput fieldKey={keyFor('before_quantity')} invalid={isInvalid('before_quantity')} label="Trước" value={row.before_quantity} disabled={disabled} onChange={(value) => onChange({ before_quantity: value })} />
          <QuantityInput fieldKey={keyFor('added_quantity')} invalid={isInvalid('added_quantity')} label="Bổ sung" value={row.added_quantity} disabled={disabled} onChange={(value) => onChange({ added_quantity: value })} />
          <QuantityInput fieldKey={keyFor('after_quantity')} invalid={isInvalid('after_quantity')} label="Sau" value={row.after_quantity} disabled={disabled} onChange={(value) => onChange({ after_quantity: value })} />
          <QuantityInput fieldKey={keyFor('waste_quantity')} invalid={isInvalid('waste_quantity')} label="Hư hao" value={row.waste_quantity} disabled={disabled} onChange={(value) => onChange({ waste_quantity: value })} />
        </div>
        <div className={clsx('mt-2 rounded-md px-3 py-2 text-sm font-semibold', used < 0 ? 'bg-rose-50 text-rose-700' : 'bg-slate-50 text-slate-700')}>
          KL dùng: {formatQty(Math.max(used, 0))} {row.unit || material?.unit || ''}
        </div>
        <input
          name={keyFor('unit')}
          data-validation-key={keyFor('unit')}
          data-validation-error={isInvalid('unit') ? 'true' : undefined}
          className={clsx('mt-2 h-9 w-full rounded-md border px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50', isInvalid('unit') ? 'border-rose-300 bg-rose-50' : 'border-line')}
          value={row.unit}
          onChange={(event) => onChange({ unit: event.target.value })}
          disabled={disabled}
          placeholder="Đơn vị"
          aria-invalid={isInvalid('unit')}
        />
      </div>

      <div>
        <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`before-evidence-${row.item_index}`}>
          Minh chứng trước
        </label>
        <div className="mt-1 flex items-center gap-2">
          <select
            id={`before-evidence-${row.item_index}`}
            className="min-w-0 flex-1 rounded-md border border-line bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={row.before_evidence_id}
            onChange={(event) => onChange({ before_evidence_id: event.target.value })}
            disabled={disabled}
          >
            <option value="">Chưa chọn</option>
            {evidence.map((item) => (
              <option key={item.id} value={item.id}>
                {evidenceLabel(item)}
              </option>
            ))}
          </select>
          <SelectedEvidenceViewButton evidenceId={row.before_evidence_id} evidence={evidence} />
        </div>
        <label className="mt-2 block text-xs font-semibold uppercase tracking-wide text-slate-500" htmlFor={`after-evidence-${row.item_index}`}>
          Minh chứng sau
        </label>
        <div className="mt-1 flex items-center gap-2">
          <select
            id={`after-evidence-${row.item_index}`}
            className="min-w-0 flex-1 rounded-md border border-line bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={row.after_evidence_id}
            onChange={(event) => onChange({ after_evidence_id: event.target.value })}
            disabled={disabled}
          >
            <option value="">Chưa chọn</option>
            {evidence.map((item) => (
              <option key={item.id} value={item.id}>
                {evidenceLabel(item)}
              </option>
            ))}
          </select>
          <SelectedEvidenceViewButton evidenceId={row.after_evidence_id} evidence={evidence} />
        </div>
        <div className="mt-2 grid gap-2 sm:grid-cols-2 xl:grid-cols-1">
          <input
            className="h-9 w-full rounded-md border border-line px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={row.construction_area}
            onChange={(event) => onChange({ construction_area: event.target.value })}
            disabled={disabled}
            placeholder="Khu vực dòng"
          />
          <input
            className="h-9 w-full rounded-md border border-line px-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
            value={row.comment}
            onChange={(event) => onChange({ comment: event.target.value })}
            disabled={disabled}
            placeholder="Ghi chú dòng"
          />
        </div>
      </div>

    </div>
  );
}

function QuantityInput({
  fieldKey,
  invalid,
  label,
  value,
  disabled,
  onChange,
}: {
  fieldKey: string;
  invalid: boolean;
  label: string;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <label className="text-xs font-semibold uppercase tracking-wide text-slate-500" data-validation-key={fieldKey} data-validation-error={invalid ? 'true' : undefined}>
      {label}
      <input
        type="number"
        name={fieldKey}
        min="0"
        step="0.001"
        className={clsx('mt-1 h-9 w-full rounded-md border px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50', invalid ? 'border-rose-300 bg-rose-50' : 'border-line')}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        aria-invalid={invalid}
      />
    </label>
  );
}

function SummaryChip({
  label,
  value,
  tone = 'slate',
}: {
  label: string;
  value: number | string;
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

function firstMaterialSnapshotInvalidKey({
  snapshotDate,
  rows,
  requireComplete,
  materialById,
  balanceByMaterialId,
}: {
  snapshotDate: string;
  rows: RowDraft[];
  requireComplete: boolean;
  materialById: Map<string, MaterialCatalogItem>;
  balanceByMaterialId: Map<string, ProjectMaterialBalance>;
}) {
  if (requireComplete && !snapshotDate) return 'snapshot_date';
  if (requireComplete && !rows.length) return 'items';
  for (const row of rows) {
    const keyFor = (field: string) => `items.${row.item_index}.${field}`;
    const material = materialById.get(row.material_id);
    const materialName = (material?.name ?? row.material_name).trim();
    const unit = (row.unit || material?.unit || '').trim();
    const numericFields = [
      ['before_quantity', numberInput(row.before_quantity)],
      ['added_quantity', numberInput(row.added_quantity)],
      ['after_quantity', numberInput(row.after_quantity)],
      ['waste_quantity', numberInput(row.waste_quantity)],
    ] as const;
    if (row.affects_site_balance && !row.material_id) return keyFor('material_id');
    if (!row.affects_site_balance && !materialName) return keyFor('material_name');
    if (!unit) return keyFor('unit');
    const invalidNumber = numericFields.find(([, value]) => value < 0 || !Number.isFinite(value));
    if (invalidNumber) return keyFor(invalidNumber[0]);
    if (computedUsed(row) < -0.000001) return keyFor('after_quantity');
    if (row.affects_site_balance && !balanceByMaterialId.has(row.material_id)) return keyFor('material_id');
    if (requireComplete && row.affects_site_balance && computedUsed(row) > balanceAvailable(balanceByMaterialId.get(row.material_id)) + 0.000001) return keyFor('after_quantity');
  }
  return undefined;
}

function defaultRowDraft(itemIndex: number): RowDraft {
  return {
    item_index: itemIndex,
    material_id: '',
    material_name: '',
    unit: '',
    source: 'site_balance',
    before_quantity: '0',
    added_quantity: '0',
    after_quantity: '0',
    waste_quantity: '0',
    affects_site_balance: true,
    before_evidence_id: '',
    after_evidence_id: '',
    construction_area: '',
    condition: 'usable',
    comment: '',
  };
}

function rowDraftFromItem(item: MaterialSnapshotItem): RowDraft {
  return {
    item_index: Number(item.item_index ?? 0),
    material_id: item.material_id ?? '',
    material_name: item.material_name ?? '',
    unit: item.unit ?? '',
    source: item.source || 'site_balance',
    before_quantity: String(item.before_quantity ?? 0),
    added_quantity: String(item.added_quantity ?? 0),
    after_quantity: String(item.after_quantity ?? 0),
    waste_quantity: String(item.waste_quantity ?? 0),
    affects_site_balance: Boolean(item.affects_site_balance),
    before_evidence_id: item.before_evidence_id ?? '',
    after_evidence_id: item.after_evidence_id ?? '',
    construction_area: item.construction_area ?? '',
    condition: item.condition || 'usable',
    comment: item.comment ?? '',
  };
}

function numberInput(value: string) {
  if (value.trim() === '') return 0;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : Number.NaN;
}

function computedUsed(row: RowDraft) {
  return roundQty(numberInput(row.before_quantity) + numberInput(row.added_quantity) - numberInput(row.after_quantity) - numberInput(row.waste_quantity));
}

function roundQty(value: number) {
  return Math.round(value * 1000) / 1000;
}

function balanceAvailable(balance?: ProjectMaterialBalance) {
  if (!balance) return 0;
  const issuedMinusUsed = Number(balance.issued_quantity ?? 0) - Number(balance.used_quantity ?? 0);
  const surplus = Number(balance.surplus_quantity ?? 0);
  return Math.max(issuedMinusUsed, surplus, 0);
}

function materialLabel(material: MaterialCatalogItem) {
  const code = material.code ? `${material.code} · ` : '';
  const category = material.category ? ` · ${material.category}` : '';
  return `${code}${material.name} (${material.unit})${category}`;
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

function formatQty(value: number) {
  if (!Number.isFinite(value)) return '-';
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 3 }).format(value);
}

function summaryCount(summary: unknown, key: string, fallback = 0) {
  if (!summary || typeof summary !== 'object') return fallback;
  const value = (summary as Record<string, unknown>)[key];
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : fallback;
}

function summaryNumber(summary: unknown, key: string) {
  return formatQty(summaryCount(summary, key));
}

function dateInputValue(value?: string) {
  if (!value) return '';
  return value.slice(0, 10);
}
