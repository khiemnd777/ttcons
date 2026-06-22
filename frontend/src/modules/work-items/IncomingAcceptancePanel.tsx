import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Plus, RefreshCw, Save, Send, Trash2 } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type {
  EvidenceRecord,
  IncomingAcceptanceRecord,
  IncomingAcceptanceResponse,
  MaterialCatalogItem,
  ProcurementRequestItemOption,
  ProcurementRequestOption,
  Warehouse,
} from '../../lib/types/domain';

type LineDraft = {
  key: string;
  item_index: number;
  material_id: string;
  material_name: string;
  procurement_request_item_id: string;
  delivered_quantity: string;
  accepted_quantity: string;
  rejected_quantity: string;
  unit: string;
  condition: string;
  evidence_id: string;
  defect_note: string;
  decision: string;
  comment: string;
};

type Draft = {
  id: string;
  source_type: string;
  supplier_name: string;
  subcontractor_name: string;
  procurement_request_id: string;
  warehouse_id: string;
  delivery_reference: string;
  received_at: string;
  result: string;
  comment: string;
  items: LineDraft[];
};

type IncomingAcceptancePayload = {
  work_item_id: string;
  source_type: string;
  supplier_name: string;
  subcontractor_name: string;
  procurement_request_id: string;
  warehouse_id: string;
  delivery_reference: string;
  received_at: string;
  result: string;
  comment: string;
  items: Array<{
    item_index: number;
    material_id: string;
    material_name: string;
    procurement_request_item_id: string;
    delivered_quantity: number;
    accepted_quantity: number;
    rejected_quantity: number;
    unit: string;
    condition: string;
    evidence_id: string;
    defect_note: string;
    decision: string;
    comment: string;
  }>;
};

const sourceOptions = [
  { value: 'supplier', label: 'Nhà cung cấp' },
  { value: 'subcontractor', label: 'Thầu phụ' },
  { value: 'procurement', label: 'Theo yêu cầu mua' },
  { value: 'other', label: 'Nguồn khác' },
];

const resultOptions = [
  { value: 'pass', label: 'Pass' },
  { value: 'conditional_pass', label: 'Conditional pass' },
  { value: 'fail', label: 'Fail' },
];

const conditionOptions = [
  { value: 'usable', label: 'Dùng được' },
  { value: 'damaged', label: 'Hư hỏng' },
  { value: 'mixed', label: 'Lẫn tình trạng' },
  { value: 'unknown', label: 'Chưa rõ' },
];

const decisionOptions = [
  { value: 'pending', label: 'Pending' },
  { value: 'accepted', label: 'Accepted' },
  { value: 'conditional_acceptance', label: 'Conditional' },
  { value: 'rejected', label: 'Rejected' },
];

export function IncomingAcceptancePanel({
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
  const [draft, setDraft] = useState<Draft>(() => defaultDraft());

  const acceptanceQuery = useQuery({
    queryKey: ['incoming-acceptances', workItemId],
    queryFn: () => get<IncomingAcceptanceResponse>(`/work-items/${workItemId}/incoming-acceptances`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  const records = acceptanceQuery.data?.records ?? [];
  const draftRecord = records.find((record) => record.status === 'draft');
  const draftDetailQuery = useQuery({
    queryKey: ['incoming-acceptance', draftRecord?.id],
    queryFn: () => get<IncomingAcceptanceRecord>(`/incoming-acceptances/${draftRecord?.id}`),
    enabled: Boolean(draftRecord?.id),
    retry: false,
  });

  const materials = acceptanceQuery.data?.materials ?? [];
  const warehouses = acceptanceQuery.data?.warehouses ?? [];
  const procurementRequests = acceptanceQuery.data?.procurement_requests ?? [];
  const procurementItems = acceptanceQuery.data?.procurement_request_items ?? [];
  const evidenceOptions = useMemo(() => evidence.filter((item) => item.status !== 'rejected'), [evidence]);
  const submitted = records.filter((record) => record.status === 'submitted');

  useEffect(() => {
    if (draftDetailQuery.data) {
      setDraft(draftFromRecord(draftDetailQuery.data));
      return;
    }
    if (!draftRecord) {
      setDraft(defaultDraft());
    }
  }, [draftDetailQuery.data, draftRecord?.id]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['incoming-acceptances', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    if (draft.id) {
      queryClient.invalidateQueries({ queryKey: ['incoming-acceptance', draft.id] });
    }
    if (projectId) {
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
    }
  };

  function updateDraft(patch: Partial<Draft>) {
    setDraft((current) => ({ ...current, ...patch }));
  }

  function updateLine(key: string, patch: Partial<LineDraft>) {
    setDraft((current) => ({
      ...current,
      items: current.items.map((item) => (item.key === key ? { ...item, ...patch } : item)),
    }));
  }

  function addLine() {
    setDraft((current) => ({ ...current, items: [...current.items, newLineDraft(current.items.length)] }));
  }

  function removeLine(key: string) {
    setDraft((current) => {
      const items = current.items.filter((item) => item.key !== key);
      return { ...current, items: items.length ? reindexLines(items) : [newLineDraft(0)] };
    });
  }

  function buildPayload(requireComplete: boolean): IncomingAcceptancePayload | null {
    const invalid: string[] = [];
    if (!projectId) invalid.push('Project');
    if (requireComplete && !draft.received_at) invalid.push('Ngày nhận');
    const materialByID = new Map(materials.map((item) => [String(item.id), item]));
    const procurementItemByID = new Map(procurementItems.map((item) => [String(item.id), item]));
    const lines = draft.items.filter(hasLineInput);
    if (requireComplete && lines.length === 0) invalid.push('Dòng vật tư');

    const items = lines.map((line, index) => {
      const material = materialByID.get(line.material_id);
      const procurementItem = procurementItemByID.get(line.procurement_request_item_id);
      const delivered = numberInput(line.delivered_quantity);
      const accepted = numberInput(line.accepted_quantity);
      const rejected = numberInput(line.rejected_quantity);
      const materialName = (material?.name ?? line.material_name ?? procurementItem?.material_name ?? '').trim();
      const unit = (line.unit || material?.unit || procurementItem?.unit || '').trim();
      if (!materialName) invalid.push(`Tên vật tư dòng ${index + 1}`);
      if (!unit) invalid.push(`Đơn vị dòng ${index + 1}`);
      if (requireComplete && (!Number.isFinite(delivered) || delivered <= 0)) invalid.push(`SL giao dòng ${index + 1}`);
      if (!Number.isFinite(delivered) || delivered < 0 || !Number.isFinite(accepted) || accepted < 0 || !Number.isFinite(rejected) || rejected < 0) {
        invalid.push(`Số lượng dòng ${index + 1}`);
      }
      if (accepted + rejected > delivered + 0.000001) invalid.push(`Vượt SL giao dòng ${index + 1}`);
      if (requireComplete && line.decision === 'pending') invalid.push(`Quyết định dòng ${index + 1}`);
      const evidenceRequired = line.decision !== 'accepted' || line.condition !== 'usable' || rejected > 0;
      if (requireComplete && evidenceRequired && !line.evidence_id) invalid.push(`Evidence dòng ${index + 1}`);
      if (requireComplete && evidenceRequired && !line.defect_note.trim()) invalid.push(`Mô tả lỗi dòng ${index + 1}`);
      return {
        item_index: index,
        material_id: line.material_id,
        material_name: materialName,
        procurement_request_item_id: line.procurement_request_item_id,
        delivered_quantity: Number.isFinite(delivered) ? delivered : 0,
        accepted_quantity: Number.isFinite(accepted) ? accepted : 0,
        rejected_quantity: Number.isFinite(rejected) ? rejected : 0,
        unit,
        condition: line.condition,
        evidence_id: line.evidence_id,
        defect_note: line.defect_note.trim(),
        decision: line.decision,
        comment: line.comment.trim(),
      };
    });

    if (invalid.length) {
      toast.warn({ title: 'Dữ liệu nghiệm thu đầu vào chưa hợp lệ', description: invalid.slice(0, 8).join(', ') });
      return null;
    }
    return {
      work_item_id: workItemId,
      source_type: draft.source_type,
      supplier_name: draft.supplier_name.trim(),
      subcontractor_name: draft.subcontractor_name.trim(),
      procurement_request_id: draft.procurement_request_id,
      warehouse_id: draft.warehouse_id,
      delivery_reference: draft.delivery_reference.trim(),
      received_at: draft.received_at,
      result: draft.result,
      comment: draft.comment.trim(),
      items,
    };
  }

  async function saveDraftRecord(requireComplete: boolean) {
    const payload = buildPayload(requireComplete);
    if (!payload || !projectId) return null;
    if (draft.id) {
      return put<IncomingAcceptanceRecord>(`/incoming-acceptances/${draft.id}`, payload);
    }
    return post<IncomingAcceptanceRecord>(`/projects/${projectId}/incoming-acceptances`, payload);
  }

  const saveDraft = useMutation({
    mutationFn: () => saveDraftRecord(false),
    onSuccess: (record) => {
      if (!record) return;
      setDraft(draftFromRecord(record));
      invalidate();
      toast.success('Đã lưu nháp nghiệm thu đầu vào');
    },
    onError: (error) => toast.error('Không lưu được nghiệm thu đầu vào', error),
  });

  const submitDraft = useMutation({
    mutationFn: async () => {
      const record = await saveDraftRecord(true);
      if (!record) return null;
      return post<IncomingAcceptanceRecord>(`/incoming-acceptances/${record.id}/submit`);
    },
    onSuccess: (record) => {
      if (!record) return;
      setDraft(defaultDraft());
      invalidate();
      toast.success('Đã submit nghiệm thu đầu vào');
    },
    onError: (error) => toast.error('Không submit được nghiệm thu đầu vào', error),
  });

  const actionPending = saveDraft.isPending || submitDraft.isPending || acceptanceQuery.isFetching || draftDetailQuery.isFetching;

  if (acceptanceQuery.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Nghiệm thu đầu vào</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (acceptanceQuery.error) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Nghiệm thu đầu vào</h3>
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-semibold text-amber-900">Không tải được dữ liệu nghiệm thu đầu vào</p>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-amber-300 px-3 py-2 text-sm font-semibold text-amber-900 hover:bg-amber-100"
              onClick={() => acceptanceQuery.refetch()}
            >
              <RefreshCw className="h-4 w-4" />
              Tải lại
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (!acceptanceQuery.data?.available) return null;

  return (
    <section>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">Nghiệm thu đầu vào</h3>
          <p className="mt-1 text-xs text-slate-500">
            {submitted.length ? `${submitted.length} hồ sơ đã submit` : 'Chưa có hồ sơ đã submit'}
          </p>
        </div>
        <StatusBadge status={draft.id ? 'draft' : 'new'} />
      </div>

      <div className="space-y-4 rounded-md border border-line bg-white p-3">
        <div className="grid gap-2 lg:grid-cols-4">
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Nguồn
            <select
              className={controlClassName}
              value={draft.source_type}
              onChange={(event) => updateDraft({ source_type: event.target.value })}
              disabled={actionPending}
            >
              {sourceOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Yêu cầu mua
            <select
              className={controlClassName}
              value={draft.procurement_request_id}
              onChange={(event) => updateDraft({ procurement_request_id: event.target.value, source_type: event.target.value ? 'procurement' : draft.source_type })}
              disabled={actionPending}
            >
              <option value="">Không liên kết</option>
              {procurementRequests.map((request) => (
                <option key={request.id} value={request.id}>{procurementRequestLabel(request)}</option>
              ))}
            </select>
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Kho nhận
            <select className={controlClassName} value={draft.warehouse_id} onChange={(event) => updateDraft({ warehouse_id: event.target.value })} disabled={actionPending}>
              <option value="">Chọn kho</option>
              {warehouses.map((warehouse) => (
                <option key={warehouse.id} value={warehouse.id}>{warehouseLabel(warehouse)}</option>
              ))}
            </select>
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Ngày nhận
            <input className={controlClassName} type="date" value={draft.received_at} onChange={(event) => updateDraft({ received_at: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Nhà cung cấp
            <input className={controlClassName} value={draft.supplier_name} onChange={(event) => updateDraft({ supplier_name: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Thầu phụ
            <input className={controlClassName} value={draft.subcontractor_name} onChange={(event) => updateDraft({ subcontractor_name: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Phiếu giao
            <input className={controlClassName} value={draft.delivery_reference} onChange={(event) => updateDraft({ delivery_reference: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Kết quả
            <select className={controlClassName} value={draft.result} onChange={(event) => updateDraft({ result: event.target.value })} disabled={actionPending}>
              <option value="">Tự xác định</option>
              {resultOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </label>
        </div>

        <div className="overflow-x-auto border-y border-line">
          <div className="min-w-[1180px] divide-y divide-line">
            {draft.items.map((line, index) => (
              <IncomingLine
                key={line.key}
                index={index}
                line={line}
                materials={materials}
                procurementItems={procurementItems.filter((item) => !draft.procurement_request_id || item.request_id === draft.procurement_request_id)}
                evidence={evidenceOptions}
                disabled={actionPending}
                onChange={(patch) => updateLine(line.key, patch)}
                onRemove={() => removeLine(line.key)}
              />
            ))}
          </div>
        </div>

        <div className="grid gap-2 md:grid-cols-[1fr_auto]">
          <input
            className={controlClassName}
            value={draft.comment}
            onChange={(event) => updateDraft({ comment: event.target.value })}
            disabled={actionPending}
            placeholder="Ghi chú nghiệm thu đầu vào"
          />
          <div className="flex flex-wrap justify-end gap-2">
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={actionPending}
              onClick={addLine}
            >
              <Plus className="h-4 w-4" />
              Thêm dòng
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={actionPending}
              onClick={() => saveDraft.mutate()}
            >
              {saveDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              Lưu nháp
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={actionPending}
              onClick={() => submitDraft.mutate()}
            >
              {submitDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              Submit
            </button>
          </div>
        </div>

        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink">Hồ sơ đã submit</h4>
          {submitted.length ? (
            <div className="space-y-2">
              {submitted.map((record) => (
                <div key={record.id} className="grid gap-2 rounded-md border border-line px-3 py-2 text-sm md:grid-cols-[1fr_auto_auto]">
                  <div className="min-w-0">
                    <p className="truncate font-semibold text-ink">
                      {record.acceptance_code} · {resultLabel(record.result)}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      {record.procurement_request_code ?? sourceLabel(record.source_type)}
                      {record.submitted_at ? ` · ${dateText(record.submitted_at)}` : ''}
                    </p>
                  </div>
                  <span className="text-slate-700">{Number(record.item_count ?? 0)} dòng</span>
                  <StatusBadge status={record.status} />
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="Chưa có hồ sơ nghiệm thu đầu vào đã submit" />
          )}
        </div>
      </div>
    </section>
  );
}

function IncomingLine({
  index,
  line,
  materials,
  procurementItems,
  evidence,
  disabled,
  onChange,
  onRemove,
}: {
  index: number;
  line: LineDraft;
  materials: MaterialCatalogItem[];
  procurementItems: ProcurementRequestItemOption[];
  evidence: EvidenceRecord[];
  disabled: boolean;
  onChange: (patch: Partial<LineDraft>) => void;
  onRemove: () => void;
}) {
  const delivered = numberInput(line.delivered_quantity);
  const accepted = numberInput(line.accepted_quantity);
  const rejected = numberInput(line.rejected_quantity);
  const over = Number.isFinite(delivered) && accepted + rejected > delivered + 0.000001;

  function selectMaterial(materialID: string) {
    const material = materials.find((item) => item.id === materialID);
    onChange({
      material_id: materialID,
      material_name: material?.name ?? line.material_name,
      unit: material?.unit ?? line.unit,
    });
  }

  function selectProcurementItem(itemID: string) {
    const item = procurementItems.find((candidate) => candidate.id === itemID);
    onChange({
      procurement_request_item_id: itemID,
      material_id: item?.material_id ?? line.material_id,
      material_name: item?.material_name ?? line.material_name,
      unit: item?.unit ?? line.unit,
      delivered_quantity: item?.quantity_requested ? String(item.quantity_requested) : line.delivered_quantity,
    });
  }

  return (
    <div className="grid grid-cols-[32px_1.5fr_1.3fr_90px_90px_90px_80px_118px_120px_1.2fr_44px] gap-2 py-3 text-sm">
      <div className="pt-2 text-xs font-semibold text-slate-500">{index + 1}</div>
      <select className={controlClassName} value={line.material_id} onChange={(event) => selectMaterial(event.target.value)} disabled={disabled}>
        <option value="">Chọn vật tư</option>
        {materials.map((material) => (
          <option key={material.id} value={material.id}>{materialLabel(material)}</option>
        ))}
      </select>
      <select className={controlClassName} value={line.procurement_request_item_id} onChange={(event) => selectProcurementItem(event.target.value)} disabled={disabled}>
        <option value="">Dòng mua sắm</option>
        {procurementItems.map((item) => (
          <option key={item.id} value={item.id}>{procurementItemLabel(item)}</option>
        ))}
      </select>
      <input
        className={clsx(controlClassName, over && 'border-rose-300 bg-rose-50')}
        type="number"
        min="0"
        step="0.001"
        value={line.delivered_quantity}
        onChange={(event) => onChange({ delivered_quantity: event.target.value })}
        disabled={disabled}
        aria-label="Số lượng giao"
      />
      <input
        className={clsx(controlClassName, over && 'border-rose-300 bg-rose-50')}
        type="number"
        min="0"
        step="0.001"
        value={line.accepted_quantity}
        onChange={(event) => onChange({ accepted_quantity: event.target.value })}
        disabled={disabled}
        aria-label="Số lượng nhận"
      />
      <input
        className={clsx(controlClassName, over && 'border-rose-300 bg-rose-50')}
        type="number"
        min="0"
        step="0.001"
        value={line.rejected_quantity}
        onChange={(event) => onChange({ rejected_quantity: event.target.value })}
        disabled={disabled}
        aria-label="Số lượng từ chối"
      />
      <input className={controlClassName} value={line.unit} onChange={(event) => onChange({ unit: event.target.value })} disabled={disabled} aria-label="Đơn vị" />
      <select className={controlClassName} value={line.condition} onChange={(event) => onChange({ condition: event.target.value })} disabled={disabled}>
        {conditionOptions.map((option) => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
      <select className={controlClassName} value={line.decision} onChange={(event) => onChange({ decision: event.target.value })} disabled={disabled}>
        {decisionOptions.map((option) => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
      <div className="grid gap-1">
        <select className={controlClassName} value={line.evidence_id} onChange={(event) => onChange({ evidence_id: event.target.value })} disabled={disabled}>
          <option value="">Evidence</option>
          {evidence.map((record) => (
            <option key={record.id} value={record.id}>{evidenceLabel(record)}</option>
          ))}
        </select>
        <input className={controlClassName} value={line.defect_note} onChange={(event) => onChange({ defect_note: event.target.value })} disabled={disabled} placeholder="Mô tả lỗi" />
      </div>
      <button
        className="inline-flex h-9 items-center justify-center rounded-md border border-line text-slate-500 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
        disabled={disabled}
        onClick={onRemove}
        aria-label="Xóa dòng nghiệm thu"
      >
        <Trash2 className="h-4 w-4" />
      </button>
    </div>
  );
}

const controlClassName = 'mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50';

function defaultDraft(): Draft {
  return {
    id: '',
    source_type: 'supplier',
    supplier_name: '',
    subcontractor_name: '',
    procurement_request_id: '',
    warehouse_id: '',
    delivery_reference: '',
    received_at: new Date().toISOString().slice(0, 10),
    result: '',
    comment: '',
    items: [newLineDraft(0)],
  };
}

function newLineDraft(index: number): LineDraft {
  return {
    key: `${Date.now()}-${index}-${Math.random().toString(36).slice(2, 8)}`,
    item_index: index,
    material_id: '',
    material_name: '',
    procurement_request_item_id: '',
    delivered_quantity: '',
    accepted_quantity: '',
    rejected_quantity: '',
    unit: '',
    condition: 'usable',
    evidence_id: '',
    defect_note: '',
    decision: 'pending',
    comment: '',
  };
}

function draftFromRecord(record: IncomingAcceptanceRecord): Draft {
  const items = (record.items ?? []).map((item, index) => ({
    key: String(item.id ?? `${record.id}-${index}`),
    item_index: Number(item.item_index ?? index),
    material_id: item.material_id ?? '',
    material_name: item.material_name ?? '',
    procurement_request_item_id: item.procurement_request_item_id ?? '',
    delivered_quantity: String(item.delivered_quantity ?? 0),
    accepted_quantity: String(item.accepted_quantity ?? 0),
    rejected_quantity: String(item.rejected_quantity ?? 0),
    unit: item.unit ?? '',
    condition: item.condition || 'usable',
    evidence_id: item.evidence_id ?? '',
    defect_note: item.defect_note ?? '',
    decision: item.decision || 'pending',
    comment: item.comment ?? '',
  }));
  return {
    id: String(record.id ?? ''),
    source_type: record.source_type || 'supplier',
    supplier_name: record.supplier_name ?? '',
    subcontractor_name: record.subcontractor_name ?? '',
    procurement_request_id: record.procurement_request_id ?? '',
    warehouse_id: record.warehouse_id ?? '',
    delivery_reference: record.delivery_reference ?? '',
    received_at: dateOnly(record.received_at) || new Date().toISOString().slice(0, 10),
    result: record.result ?? '',
    comment: record.comment ?? '',
    items: items.length ? items : [newLineDraft(0)],
  };
}

function reindexLines(lines: LineDraft[]) {
  return lines.map((line, index) => ({ ...line, item_index: index }));
}

function hasLineInput(line: LineDraft) {
  return Boolean(
    line.material_id
      || line.material_name.trim()
      || line.procurement_request_item_id
      || line.delivered_quantity.trim()
      || line.accepted_quantity.trim()
      || line.rejected_quantity.trim()
      || line.evidence_id
      || line.defect_note.trim()
      || line.comment.trim(),
  );
}

function numberInput(value: string) {
  if (value.trim() === '') return 0;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : Number.NaN;
}

function dateOnly(value?: string) {
  if (!value) return '';
  return value.slice(0, 10);
}

function sourceLabel(value?: string) {
  return sourceOptions.find((option) => option.value === value)?.label ?? value ?? '-';
}

function resultLabel(value?: string) {
  return resultOptions.find((option) => option.value === value)?.label ?? value ?? '-';
}

function warehouseLabel(warehouse: Warehouse) {
  const location = warehouse.location ? ` · ${warehouse.location}` : '';
  return `${warehouse.name}${location}`;
}

function materialLabel(material: MaterialCatalogItem) {
  const code = material.code ? `${material.code} · ` : '';
  return `${code}${material.name} (${material.unit})`;
}

function procurementRequestLabel(request: ProcurementRequestOption) {
  const code = request.request_code ?? request.id;
  return `${code}${request.title ? ` · ${request.title}` : ''}`;
}

function procurementItemLabel(item: ProcurementRequestItemOption) {
  const quantity = item.quantity_requested ? ` · ${item.quantity_requested} ${item.unit ?? ''}` : '';
  return `${item.material_name ?? item.work_item_name ?? item.id}${quantity}`;
}

function evidenceLabel(evidence: EvidenceRecord) {
  return `${evidence.name ?? evidence.title ?? evidence.id} (${evidence.status ?? 'submitted'})`;
}
