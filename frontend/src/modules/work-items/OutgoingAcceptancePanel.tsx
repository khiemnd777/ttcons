import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Loader2, Plus, RefreshCw, RotateCcw, Save, Send, Trash2, X } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type {
  AcceptanceRecord,
  EvidenceRecord,
  OutgoingAcceptanceResponse,
  WorkflowFormPDFExport,
} from '../../lib/types/domain';

type LineDraft = {
  key: string;
  item_index: number;
  title: string;
  description: string;
  decision: string;
  severity: string;
  evidence_id: string;
  required_action: string;
  due_date: string;
  photo_required: boolean;
  photo_url: string;
  note: string;
};

type Draft = {
  id: string;
  type: string;
  title: string;
  acceptance_date: string;
  location: string;
  client_representative: string;
  contractor_representative: string;
  scope_summary: string;
  form_instance_id: string;
  pdf_export_id: string;
  signature_status: string;
  document_status: string;
  notes: string;
  checklist: LineDraft[];
};

type AcceptancePayload = Omit<Draft, 'id' | 'checklist'> & {
  work_item_id: string;
  checklist: Array<Omit<LineDraft, 'key'>>;
};

const typeOptions = [
  { value: 'partial', label: 'Một phần' },
  { value: 'final', label: 'Cuối cùng' },
];

const decisionOptions = [
  { value: 'pending', label: 'Đang chờ' },
  { value: 'pass', label: 'Đạt' },
  { value: 'conditional_pass', label: 'Đạt có điều kiện' },
  { value: 'fail', label: 'Không đạt' },
];

const severityOptions = [
  { value: 'low', label: 'Thấp' },
  { value: 'medium', label: 'Trung bình' },
  { value: 'high', label: 'Cao' },
  { value: 'critical', label: 'Nghiêm trọng' },
];

const signatureOptions = [
  { value: 'pending', label: 'Chờ ký' },
  { value: 'signed', label: 'Đã ký' },
  { value: 'waived', label: 'Bỏ qua chữ ký' },
];

const documentOptions = [
  { value: 'pending', label: 'Chờ PDF' },
  { value: 'linked', label: 'Đã liên kết PDF' },
  { value: 'waived', label: 'Bỏ qua tài liệu' },
];

export function OutgoingAcceptancePanel({
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
    queryKey: ['outgoing-acceptance', workItemId],
    queryFn: () => get<OutgoingAcceptanceResponse>(`/work-items/${workItemId}/outgoing-acceptance`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  const records = acceptanceQuery.data?.records ?? [];
  const draftRecord = records.find((record) => record.status === 'draft');
  const draftDetailQuery = useQuery({
    queryKey: ['acceptance', draftRecord?.id],
    queryFn: () => get<AcceptanceRecord>(`/acceptances/${draftRecord?.id}`),
    enabled: Boolean(draftRecord?.id),
    retry: false,
  });

  const form = acceptanceQuery.data?.form;
  const formInstance = form?.instance;
  const pdfExports = (form?.pdf_exports ?? []) as WorkflowFormPDFExport[];
  const signatures = form?.signatures ?? [];
  const evidenceOptions = useMemo(() => evidence.filter((item) => item.status !== 'rejected'), [evidence]);
  const submitted = records.filter((record) => record.status === 'submitted' || record.status === 'completed');

  useEffect(() => {
    if (draftDetailQuery.data) {
      setDraft(draftFromRecord(draftDetailQuery.data));
      return;
    }
    if (!draftRecord) {
      setDraft(defaultDraft(formInstance?.id, pdfExports[0]?.id, signatures.length > 0));
    }
  }, [draftDetailQuery.data, draftRecord?.id, formInstance?.id, pdfExports[0]?.id, signatures.length]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['outgoing-acceptance', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    queryClient.invalidateQueries({ queryKey: ['acceptances', projectId] });
    if (draft.id) {
      queryClient.invalidateQueries({ queryKey: ['acceptance', draft.id] });
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
      checklist: current.checklist.map((item) => (item.key === key ? { ...item, ...patch } : item)),
    }));
  }

  function addLine() {
    setDraft((current) => ({ ...current, checklist: [...current.checklist, newLineDraft(current.checklist.length)] }));
  }

  function removeLine(key: string) {
    setDraft((current) => {
      const checklist = current.checklist.filter((item) => item.key !== key);
      return { ...current, checklist: checklist.length ? reindexLines(checklist) : [newLineDraft(0)] };
    });
  }

  function buildPayload(requireComplete: boolean): AcceptancePayload | null {
    const invalid: string[] = [];
    if (!projectId) invalid.push('Công trình');
    if (!draft.title.trim()) invalid.push('Tiêu đề');
    if (requireComplete && !draft.acceptance_date) invalid.push('Ngày nghiệm thu');
    if (requireComplete && draft.signature_status === 'pending') invalid.push('Chữ ký/dấu PDF');
    if (requireComplete && draft.document_status === 'pending') invalid.push('Tài liệu/PDF');
    const lines = draft.checklist.filter(hasLineInput);
    if (requireComplete && lines.length === 0) invalid.push('Checklist nghiệm thu');
    const checklist = lines.map((line, index) => {
      if (!line.title.trim()) invalid.push(`Tên checklist dòng ${index + 1}`);
      if (requireComplete && line.decision === 'pending') invalid.push(`Quyết định dòng ${index + 1}`);
      const evidenceRequired = line.photo_required || line.decision === 'conditional_pass' || line.decision === 'fail';
      if (requireComplete && evidenceRequired && !line.evidence_id && !line.photo_url.trim()) invalid.push(`Minh chứng dòng ${index + 1}`);
      if (requireComplete && (line.decision === 'conditional_pass' || line.decision === 'fail') && !line.required_action.trim()) {
        invalid.push(`Yêu cầu xử lý dòng ${index + 1}`);
      }
      return {
        item_index: index,
        title: line.title.trim(),
        description: line.description.trim(),
        decision: line.decision,
        severity: line.severity,
        evidence_id: line.evidence_id,
        required_action: line.required_action.trim(),
        due_date: line.due_date,
        photo_required: line.photo_required,
        photo_url: line.photo_url.trim(),
        note: line.note.trim(),
      };
    });
    if (invalid.length) {
      toast.warn({ title: 'Dữ liệu nghiệm thu đầu ra chưa hợp lệ', description: invalid.slice(0, 8).join(', ') });
      return null;
    }
    return {
      work_item_id: workItemId,
      type: draft.type,
      title: draft.title.trim(),
      acceptance_date: draft.acceptance_date,
      location: draft.location.trim(),
      client_representative: draft.client_representative.trim(),
      contractor_representative: draft.contractor_representative.trim(),
      scope_summary: draft.scope_summary.trim(),
      form_instance_id: draft.form_instance_id,
      pdf_export_id: draft.pdf_export_id,
      signature_status: draft.signature_status,
      document_status: draft.document_status,
      notes: draft.notes.trim(),
      checklist,
    };
  }

  async function saveDraftRecord(requireComplete: boolean) {
    const payload = buildPayload(requireComplete);
    if (!payload || !projectId) return null;
    if (draft.id) {
      return put<AcceptanceRecord>(`/acceptances/${draft.id}`, payload);
    }
    return post<AcceptanceRecord>(`/projects/${projectId}/acceptances`, payload);
  }

  const saveDraft = useMutation({
    mutationFn: () => saveDraftRecord(false),
    onSuccess: (record) => {
      if (!record) return;
      setDraft(draftFromRecord(record));
      invalidate();
      toast.success('Đã lưu nháp nghiệm thu đầu ra');
    },
    onError: (error) => toast.error('Không lưu được nghiệm thu đầu ra', error),
  });

  const submitDraft = useMutation({
    mutationFn: async () => {
      const record = await saveDraftRecord(true);
      if (!record) return null;
      return post<AcceptanceRecord>(`/acceptances/${record.id}/submit`);
    },
    onSuccess: (record) => {
      if (!record) return;
      setDraft(defaultDraft(formInstance?.id, pdfExports[0]?.id, signatures.length > 0));
      invalidate();
      toast.success('Đã gửi nghiệm thu đầu ra');
    },
    onError: (error) => toast.error('Không gửi được nghiệm thu đầu ra', error),
  });

  const actionMutation = useMutation({
    mutationFn: ({ acceptanceId, action }: { acceptanceId: string; action: 'pass' | 'conditional-pass' | 'fail' }) =>
      post<AcceptanceRecord>(`/acceptances/${acceptanceId}/${action}`),
    onSuccess: (_record, variables) => {
      invalidate();
      toast.success(actionToastTitle(variables.action));
    },
    onError: (error) => toast.error('Không chốt được nghiệm thu đầu ra', error),
  });

  const actionPending = saveDraft.isPending || submitDraft.isPending || actionMutation.isPending || acceptanceQuery.isFetching || draftDetailQuery.isFetching;

  if (acceptanceQuery.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Nghiệm thu đầu ra</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (acceptanceQuery.error) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Nghiệm thu đầu ra</h3>
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-semibold text-amber-900">Không tải được dữ liệu nghiệm thu đầu ra</p>
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
          <h3 className="text-sm font-semibold text-ink">Nghiệm thu đầu ra</h3>
          <p className="mt-1 text-xs text-slate-500">
            {submitted.length ? `${submitted.length} hồ sơ đã gửi/chốt` : 'Chưa có hồ sơ đã gửi'}
          </p>
        </div>
        <StatusBadge status={draft.id ? 'draft' : 'new'} />
      </div>

      <div className="space-y-4 rounded-md border border-line bg-white p-3">
        <div className="grid gap-2 lg:grid-cols-4">
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Loại
            <select className={controlClassName} value={draft.type} onChange={(event) => updateDraft({ type: event.target.value })} disabled={actionPending}>
              {typeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Tiêu đề
            <input className={controlClassName} value={draft.title} onChange={(event) => updateDraft({ title: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Ngày nghiệm thu
            <input className={controlClassName} type="date" value={draft.acceptance_date} onChange={(event) => updateDraft({ acceptance_date: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Khu vực
            <input className={controlClassName} value={draft.location} onChange={(event) => updateDraft({ location: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Đại diện CĐT
            <input className={controlClassName} value={draft.client_representative} onChange={(event) => updateDraft({ client_representative: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Đại diện nhà thầu
            <input className={controlClassName} value={draft.contractor_representative} onChange={(event) => updateDraft({ contractor_representative: event.target.value })} disabled={actionPending} />
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Chữ ký
            <select className={controlClassName} value={draft.signature_status} onChange={(event) => updateDraft({ signature_status: event.target.value })} disabled={actionPending}>
              {signatureOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Tài liệu
            <select className={controlClassName} value={draft.document_status} onChange={(event) => updateDraft({ document_status: event.target.value })} disabled={actionPending}>
              {documentOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500 lg:col-span-2">
            Biểu mẫu quy trình
            <select className={controlClassName} value={draft.form_instance_id} onChange={(event) => updateDraft({ form_instance_id: event.target.value })} disabled={actionPending || !formInstance?.id}>
              <option value="">{formInstance?.id ? 'Không liên kết biểu mẫu' : 'Chưa có biểu mẫu'}</option>
              {formInstance?.id ? <option value={formInstance.id}>{formInstance.title}</option> : null}
            </select>
          </label>
          <label className="text-xs font-semibold uppercase tracking-wide text-slate-500 lg:col-span-2">
            PDF
            <select
              className={controlClassName}
              value={draft.pdf_export_id}
              onChange={(event) => updateDraft({ pdf_export_id: event.target.value, document_status: event.target.value ? 'linked' : draft.document_status })}
              disabled={actionPending || pdfExports.length === 0}
            >
              <option value="">{pdfExports.length ? 'Không liên kết PDF' : 'Chưa có PDF'}</option>
              {pdfExports.map((record) => (
                <option key={record.id} value={record.id}>{record.file_name ?? record.title ?? record.id}</option>
              ))}
            </select>
          </label>
        </div>

        <textarea
          className="min-h-20 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50"
          value={draft.scope_summary}
          onChange={(event) => updateDraft({ scope_summary: event.target.value })}
          disabled={actionPending}
          placeholder="Phạm vi nghiệm thu"
        />

        <div className="overflow-x-auto border-y border-line">
          <div className="min-w-[1120px] divide-y divide-line">
            {draft.checklist.map((line, index) => (
              <AcceptanceLine
                key={line.key}
                index={index}
                line={line}
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
            value={draft.notes}
            onChange={(event) => updateDraft({ notes: event.target.value })}
            disabled={actionPending}
            placeholder="Ghi chú nghiệm thu"
          />
          <div className="flex flex-wrap justify-end gap-2">
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60" disabled={actionPending} onClick={addLine}>
              <Plus className="h-4 w-4" />
              Thêm dòng
            </button>
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60" disabled={actionPending} onClick={() => saveDraft.mutate()}>
              {saveDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              Lưu nháp
            </button>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={actionPending} onClick={() => submitDraft.mutate()}>
              {submitDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              Gửi
            </button>
          </div>
        </div>

        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink">Hồ sơ đã gửi/chốt</h4>
          {submitted.length ? (
            <div className="space-y-2">
              {submitted.map((record) => (
                <div key={record.id} className="grid gap-2 rounded-md border border-line px-3 py-2 text-sm xl:grid-cols-[1fr_auto_auto]">
                  <div className="min-w-0">
                    <p className="truncate font-semibold text-ink">
                      {record.acceptance_code ?? record.id} · {record.title}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      {record.acceptance_date ? dateText(record.acceptance_date) : record.type}
                      {record.submitted_at ? ` · Gửi ${dateText(record.submitted_at)}` : ''}
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <StatusBadge status={record.result ?? record.status} />
                    <StatusBadge status={record.signature_status ?? 'pending'} />
                    <StatusBadge status={record.document_status ?? 'pending'} />
                  </div>
                  {record.status === 'submitted' ? (
                    <div className="flex flex-wrap justify-end gap-2">
                      <button className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:opacity-60" disabled={actionPending} title="Đạt" onClick={() => actionMutation.mutate({ acceptanceId: String(record.id), action: 'pass' })}><Check className="h-4 w-4" /></button>
                      <button className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:opacity-60" disabled={actionPending} title="Đạt có điều kiện" onClick={() => actionMutation.mutate({ acceptanceId: String(record.id), action: 'conditional-pass' })}><RotateCcw className="h-4 w-4" /></button>
                      <button className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:opacity-60" disabled={actionPending} title="Không đạt" onClick={() => actionMutation.mutate({ acceptanceId: String(record.id), action: 'fail' })}><X className="h-4 w-4" /></button>
                    </div>
                  ) : (
                    <span />
                  )}
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="Chưa có hồ sơ nghiệm thu đầu ra đã gửi" />
          )}
        </div>
      </div>
    </section>
  );
}

function AcceptanceLine({
  index,
  line,
  evidence,
  disabled,
  onChange,
  onRemove,
}: {
  index: number;
  line: LineDraft;
  evidence: EvidenceRecord[];
  disabled: boolean;
  onChange: (patch: Partial<LineDraft>) => void;
  onRemove: () => void;
}) {
  const needsEvidence = line.photo_required || line.decision === 'conditional_pass' || line.decision === 'fail';
  return (
    <div className="grid grid-cols-[32px_1.4fr_1fr_130px_110px_1.2fr_130px_44px] gap-2 py-3 text-sm">
      <div className="pt-2 text-xs font-semibold text-slate-500">{index + 1}</div>
      <input className={controlClassName} value={line.title} onChange={(event) => onChange({ title: event.target.value })} disabled={disabled} placeholder="Hạng mục" />
      <input className={controlClassName} value={line.description} onChange={(event) => onChange({ description: event.target.value })} disabled={disabled} placeholder="Mô tả" />
      <select className={controlClassName} value={line.decision} onChange={(event) => onChange({ decision: event.target.value })} disabled={disabled}>
        {decisionOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
      <select className={controlClassName} value={line.severity} onChange={(event) => onChange({ severity: event.target.value })} disabled={disabled}>
        {severityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
      <div className="grid gap-1">
        <select className={clsx(controlClassName, needsEvidence && !line.evidence_id && !line.photo_url && 'border-amber-300 bg-amber-50')} value={line.evidence_id} onChange={(event) => onChange({ evidence_id: event.target.value })} disabled={disabled}>
          <option value="">{needsEvidence ? 'Cần minh chứng' : 'Minh chứng'}</option>
          {evidence.map((record) => (
            <option key={record.id} value={record.id}>{evidenceLabel(record)}</option>
          ))}
        </select>
        <input className={controlClassName} value={line.required_action} onChange={(event) => onChange({ required_action: event.target.value })} disabled={disabled} placeholder="Yêu cầu xử lý" />
      </div>
      <div className="grid gap-1">
        <input className={controlClassName} type="date" value={line.due_date} onChange={(event) => onChange({ due_date: event.target.value })} disabled={disabled} />
        <label className="inline-flex h-9 items-center gap-2 rounded-md border border-line px-2 text-xs font-semibold text-slate-600">
          <input type="checkbox" checked={line.photo_required} onChange={(event) => onChange({ photo_required: event.target.checked })} disabled={disabled} />
          Ảnh
        </label>
      </div>
      <button className="inline-flex h-9 items-center justify-center rounded-md border border-line text-slate-500 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60" disabled={disabled} onClick={onRemove} aria-label="Xóa dòng nghiệm thu">
        <Trash2 className="h-4 w-4" />
      </button>
    </div>
  );
}

const controlClassName = 'mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50';

function defaultDraft(formInstanceId = '', pdfExportId = '', hasSignature = false): Draft {
  return {
    id: '',
    type: 'partial',
    title: 'Nghiệm thu đầu ra',
    acceptance_date: new Date().toISOString().slice(0, 10),
    location: '',
    client_representative: '',
    contractor_representative: '',
    scope_summary: '',
    form_instance_id: formInstanceId,
    pdf_export_id: pdfExportId,
    signature_status: hasSignature ? 'signed' : 'pending',
    document_status: pdfExportId ? 'linked' : 'pending',
    notes: '',
    checklist: [newLineDraft(0)],
  };
}

function newLineDraft(index: number): LineDraft {
  return {
    key: `${Date.now()}-${index}-${Math.random().toString(36).slice(2, 8)}`,
    item_index: index,
    title: '',
    description: '',
    decision: 'pending',
    severity: 'medium',
    evidence_id: '',
    required_action: '',
    due_date: '',
    photo_required: false,
    photo_url: '',
    note: '',
  };
}

function draftFromRecord(record: AcceptanceRecord): Draft {
  const checklist = (record.checklist ?? []).map((item, index) => ({
    key: String(item.id ?? `${record.id}-${index}`),
    item_index: Number(item.item_index ?? index),
    title: item.title ?? '',
    description: item.description ?? '',
    decision: item.decision || (item.is_passed ? 'pass' : 'pending'),
    severity: item.severity ?? 'medium',
    evidence_id: item.evidence_id ?? '',
    required_action: item.required_action ?? '',
    due_date: dateOnly(item.due_date),
    photo_required: Boolean(item.photo_required),
    photo_url: item.photo_url ?? '',
    note: item.note ?? '',
  }));
  return {
    id: String(record.id ?? ''),
    type: record.type || 'partial',
    title: record.title || 'Nghiệm thu đầu ra',
    acceptance_date: dateOnly(record.acceptance_date) || new Date().toISOString().slice(0, 10),
    location: record.location ?? '',
    client_representative: record.client_representative ?? '',
    contractor_representative: record.contractor_representative ?? '',
    scope_summary: record.scope_summary ?? '',
    form_instance_id: record.form_instance_id ?? '',
    pdf_export_id: record.pdf_export_id ?? '',
    signature_status: record.signature_status ?? 'pending',
    document_status: record.document_status ?? 'pending',
    notes: record.notes ?? '',
    checklist: checklist.length ? checklist : [newLineDraft(0)],
  };
}

function reindexLines(lines: LineDraft[]) {
  return lines.map((line, index) => ({ ...line, item_index: index }));
}

function hasLineInput(line: LineDraft) {
  return Boolean(
    line.title.trim()
      || line.description.trim()
      || line.decision !== 'pending'
      || line.evidence_id
      || line.required_action.trim()
      || line.note.trim(),
  );
}

function dateOnly(value?: string) {
  if (!value) return '';
  return value.slice(0, 10);
}

function evidenceLabel(evidence: EvidenceRecord) {
  return `${evidence.name ?? evidence.title ?? evidence.id} (${evidenceStatusLabel(evidence.status)})`;
}

function actionToastTitle(action: string) {
  if (action === 'pass') return 'Đã chốt nghiệm thu đầu ra đạt';
  if (action === 'conditional-pass') return 'Đã chốt nghiệm thu đầu ra đạt có điều kiện';
  return 'Đã chốt nghiệm thu đầu ra không đạt';
}

function evidenceStatusLabel(status?: string) {
  if (status === 'accepted') return 'đã chấp nhận';
  if (status === 'rejected') return 'đã từ chối';
  if (status === 'pending') return 'đang chờ';
  if (status === 'draft') return 'nháp';
  if (status === 'submitted') return 'đã gửi';
  return status ?? 'đã gửi';
}
