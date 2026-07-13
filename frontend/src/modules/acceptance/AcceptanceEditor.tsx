import type { ReactNode } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import clsx from 'clsx';
import type { AcceptanceRecord, EvidenceRecord, WorkflowFormPDFExport, WorkItem } from '../../lib/types/domain';

export type AcceptanceLineDraft = {
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

export type AcceptanceDraft = {
  id: string;
  work_item_id: string;
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
  checklist: AcceptanceLineDraft[];
};

export type AcceptancePayload = Omit<AcceptanceDraft, 'id' | 'checklist'> & {
  checklist: Array<Omit<AcceptanceLineDraft, 'key'>>;
};

const typeOptions = [
  { value: 'partial', label: 'Một phần' },
  { value: 'final', label: 'Cuối cùng' },
];

const decisionOptions = [
  { value: 'pending', label: 'Chờ kết quả' },
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
  { value: 'waived', label: 'Không yêu cầu chữ ký' },
];

const documentOptions = [
  { value: 'pending', label: 'Chờ PDF' },
  { value: 'linked', label: 'Đã liên kết PDF' },
  { value: 'waived', label: 'Không yêu cầu PDF' },
];

export function AcceptanceEditor({
  draft,
  evidence,
  workItems = [],
  formInstance,
  pdfExports = [],
  fixedWorkItem = false,
  disabled = false,
  onChange,
}: {
  draft: AcceptanceDraft;
  evidence: EvidenceRecord[];
  workItems?: WorkItem[];
  formInstance?: { id: string; title?: string };
  pdfExports?: WorkflowFormPDFExport[];
  fixedWorkItem?: boolean;
  disabled?: boolean;
  onChange: (draft: AcceptanceDraft) => void;
}) {
  const updateDraft = (patch: Partial<AcceptanceDraft>) => onChange({ ...draft, ...patch });
  const updateLine = (key: string, patch: Partial<AcceptanceLineDraft>) => {
    updateDraft({ checklist: draft.checklist.map((line) => line.key === key ? { ...line, ...patch } : line) });
  };
  const addLine = () => updateDraft({ checklist: [...draft.checklist, newAcceptanceLine(draft.checklist.length)] });
  const removeLine = (key: string) => {
    const lines = draft.checklist.filter((line) => line.key !== key);
    updateDraft({ checklist: lines.length ? reindexLines(lines) : [newAcceptanceLine(0)] });
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {!fixedWorkItem ? (
          <EditorField label="Đầu việc nguồn" className="xl:col-span-2">
            <select className={controlClassName} value={draft.work_item_id} onChange={(event) => updateDraft({ work_item_id: event.target.value })} disabled={disabled}>
              <option value="">Không gắn đầu việc</option>
              {workItems.map((item) => <option key={item.id} value={item.id}>{item.title} · {item.status}</option>)}
            </select>
          </EditorField>
        ) : null}
        <EditorField label="Loại biên bản">
          <select className={controlClassName} value={draft.type} onChange={(event) => updateDraft({ type: event.target.value })} disabled={disabled}>
            {typeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </EditorField>
        <EditorField label="Ngày nghiệm thu">
          <input className={controlClassName} type="date" value={draft.acceptance_date} onChange={(event) => updateDraft({ acceptance_date: event.target.value })} disabled={disabled} />
        </EditorField>
        <EditorField label="Tiêu đề" className="md:col-span-2">
          <input className={controlClassName} value={draft.title} onChange={(event) => updateDraft({ title: event.target.value })} disabled={disabled} />
        </EditorField>
        <EditorField label="Khu vực / vị trí" className="md:col-span-2">
          <input className={controlClassName} value={draft.location} onChange={(event) => updateDraft({ location: event.target.value })} disabled={disabled} placeholder="Ví dụ: Khu B · tầng 3" />
        </EditorField>
        <EditorField label="Đại diện chủ đầu tư">
          <input className={controlClassName} value={draft.client_representative} onChange={(event) => updateDraft({ client_representative: event.target.value })} disabled={disabled} />
        </EditorField>
        <EditorField label="Đại diện nhà thầu">
          <input className={controlClassName} value={draft.contractor_representative} onChange={(event) => updateDraft({ contractor_representative: event.target.value })} disabled={disabled} />
        </EditorField>
        <EditorField label="Chữ ký">
          <select className={controlClassName} value={draft.signature_status} onChange={(event) => updateDraft({ signature_status: event.target.value })} disabled={disabled}>
            {signatureOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </EditorField>
        <EditorField label="Tài liệu / PDF">
          <select className={controlClassName} value={draft.document_status} onChange={(event) => updateDraft({ document_status: event.target.value })} disabled={disabled}>
            {documentOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </EditorField>
        {formInstance?.id || draft.form_instance_id ? (
          <EditorField label="Hồ sơ nghiệp vụ" className="md:col-span-2">
            <select className={controlClassName} value={draft.form_instance_id} onChange={(event) => updateDraft({ form_instance_id: event.target.value })} disabled={disabled || !formInstance?.id}>
              <option value="">Không liên kết hồ sơ</option>
              {formInstance?.id ? <option value={formInstance.id}>{formInstance.title || formInstance.id}</option> : <option value={draft.form_instance_id}>{draft.form_instance_id}</option>}
            </select>
          </EditorField>
        ) : null}
        {pdfExports.length || draft.pdf_export_id ? (
          <EditorField label="PDF liên kết" className="md:col-span-2">
            <select className={controlClassName} value={draft.pdf_export_id} onChange={(event) => updateDraft({ pdf_export_id: event.target.value, document_status: event.target.value ? 'linked' : draft.document_status })} disabled={disabled || pdfExports.length === 0}>
              <option value="">Không liên kết PDF</option>
              {pdfExports.map((record) => <option key={record.id} value={record.id}>{record.file_name ?? record.title ?? record.id}</option>)}
              {!pdfExports.length && draft.pdf_export_id ? <option value={draft.pdf_export_id}>{draft.pdf_export_id}</option> : null}
            </select>
          </EditorField>
        ) : null}
      </div>

      <EditorField label="Phạm vi nghiệm thu">
        <textarea className="min-h-20 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50" value={draft.scope_summary} onChange={(event) => updateDraft({ scope_summary: event.target.value })} disabled={disabled} placeholder="Mô tả phạm vi, khu vực và mốc công việc được nghiệm thu" />
      </EditorField>

      <section className="overflow-hidden rounded-md border border-line">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line bg-slate-50 px-3 py-2.5">
          <div>
            <h3 className="text-sm font-semibold text-ink">Checklist nghiệm thu</h3>
            <p className="mt-0.5 text-xs text-slate-500">Mỗi dòng phải có tiêu chí, kết quả và yêu cầu xử lý khi chưa đạt.</p>
          </div>
          <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" disabled={disabled} onClick={addLine}>
            <Plus className="h-4 w-4" /> Thêm tiêu chí
          </button>
        </div>
        <div className="overflow-x-auto">
          <div className="min-w-[1120px]">
            <div className="grid grid-cols-[36px_1.35fr_1.25fr_140px_115px_1.2fr_130px_44px] gap-2 border-b border-line bg-slate-50 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-500">
              <span>#</span><span>Hạng mục</span><span>Tiêu chí / mô tả</span><span>Kết quả</span><span>Mức độ</span><span>Minh chứng / xử lý</span><span>Hạn / ảnh</span><span />
            </div>
            <div className="divide-y divide-line px-3">
              {draft.checklist.map((line, index) => (
                <AcceptanceLine key={line.key} index={index} line={line} evidence={evidence} disabled={disabled} onChange={(patch) => updateLine(line.key, patch)} onRemove={() => removeLine(line.key)} />
              ))}
            </div>
          </div>
        </div>
      </section>

      <EditorField label="Ghi chú chung">
        <textarea className="min-h-16 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50" value={draft.notes} onChange={(event) => updateDraft({ notes: event.target.value })} disabled={disabled} />
      </EditorField>
    </div>
  );
}

function AcceptanceLine({ index, line, evidence, disabled, onChange, onRemove }: {
  index: number;
  line: AcceptanceLineDraft;
  evidence: EvidenceRecord[];
  disabled: boolean;
  onChange: (patch: Partial<AcceptanceLineDraft>) => void;
  onRemove: () => void;
}) {
  const needsEvidence = line.photo_required || line.decision === 'conditional_pass' || line.decision === 'fail';
  return (
    <div className="grid grid-cols-[36px_1.35fr_1.25fr_140px_115px_1.2fr_130px_44px] gap-2 py-3 text-sm">
      <div className="pt-2 text-xs font-semibold text-slate-500">{index + 1}</div>
      <input className={controlClassName} value={line.title} onChange={(event) => onChange({ title: event.target.value })} disabled={disabled} placeholder="Hạng mục kiểm tra" />
      <input className={controlClassName} value={line.description} onChange={(event) => onChange({ description: event.target.value })} disabled={disabled} placeholder="Tiêu chí chấp nhận" />
      <select className={controlClassName} value={line.decision} onChange={(event) => onChange({ decision: event.target.value })} disabled={disabled}>
        {decisionOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
      <select className={controlClassName} value={line.severity} onChange={(event) => onChange({ severity: event.target.value })} disabled={disabled}>
        {severityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
      <div className="grid gap-1">
        <select className={clsx(controlClassName, needsEvidence && !line.evidence_id && !line.photo_url && 'border-amber-300 bg-amber-50')} value={line.evidence_id} onChange={(event) => onChange({ evidence_id: event.target.value })} disabled={disabled}>
          <option value="">{needsEvidence ? 'Cần minh chứng' : 'Chọn minh chứng'}</option>
          {evidence.filter((record) => record.status !== 'rejected').map((record) => <option key={record.id} value={record.id}>{evidenceLabel(record)}</option>)}
        </select>
        <input className={controlClassName} value={line.required_action} onChange={(event) => onChange({ required_action: event.target.value })} disabled={disabled} placeholder="Yêu cầu xử lý" />
      </div>
      <div className="grid gap-1">
        <input className={controlClassName} type="date" value={line.due_date} onChange={(event) => onChange({ due_date: event.target.value })} disabled={disabled} />
        <label className="inline-flex h-9 items-center gap-2 rounded-md border border-line px-2 text-xs font-semibold text-slate-600">
          <input type="checkbox" checked={line.photo_required} onChange={(event) => onChange({ photo_required: event.target.checked })} disabled={disabled} /> Ảnh
        </label>
      </div>
      <button type="button" className="inline-flex h-9 items-center justify-center rounded-md border border-line text-slate-500 hover:bg-slate-100 disabled:opacity-60" disabled={disabled} onClick={onRemove} aria-label="Xóa tiêu chí">
        <Trash2 className="h-4 w-4" />
      </button>
    </div>
  );
}

function EditorField({ label, className = '', children }: { label: string; className?: string; children: ReactNode }) {
  return <label className={clsx('block text-xs font-semibold uppercase tracking-wide text-slate-500', className)}>{label}{children}</label>;
}

export function createAcceptanceDraft({ workItemId = '', formInstanceId = '', pdfExportId = '', hasSignature = false }: { workItemId?: string; formInstanceId?: string; pdfExportId?: string; hasSignature?: boolean } = {}): AcceptanceDraft {
  return {
    id: '', work_item_id: workItemId, type: 'partial', title: 'Nghiệm thu đầu ra',
    acceptance_date: new Date().toISOString().slice(0, 10), location: '', client_representative: '',
    contractor_representative: '', scope_summary: '', form_instance_id: formInstanceId,
    pdf_export_id: pdfExportId, signature_status: hasSignature ? 'signed' : 'pending',
    document_status: pdfExportId ? 'linked' : 'pending', notes: '', checklist: [newAcceptanceLine(0)],
  };
}

export function acceptanceDraftFromRecord(record: AcceptanceRecord): AcceptanceDraft {
  const checklist = (record.checklist ?? []).map((item, index) => ({
    key: String(item.id ?? `${record.id}-${index}`), item_index: Number(item.item_index ?? index),
    title: item.title ?? '', description: item.description ?? '', decision: item.decision || (item.is_passed ? 'pass' : 'pending'),
    severity: item.severity ?? 'medium', evidence_id: item.evidence_id ?? '', required_action: item.required_action ?? '',
    due_date: dateOnly(item.due_date), photo_required: Boolean(item.photo_required), photo_url: item.photo_url ?? '', note: item.note ?? '',
  }));
  return {
    id: String(record.id ?? ''), work_item_id: record.work_item_id ?? '', type: record.type || 'partial',
    title: record.title || 'Nghiệm thu đầu ra', acceptance_date: dateOnly(record.acceptance_date) || new Date().toISOString().slice(0, 10),
    location: record.location ?? '', client_representative: record.client_representative ?? '', contractor_representative: record.contractor_representative ?? '',
    scope_summary: record.scope_summary ?? '', form_instance_id: record.form_instance_id ?? '', pdf_export_id: record.pdf_export_id ?? '',
    signature_status: record.signature_status ?? 'pending', document_status: record.document_status ?? 'pending', notes: record.notes ?? '',
    checklist: checklist.length ? checklist : [newAcceptanceLine(0)],
  };
}

export function acceptanceDraftPayload(draft: AcceptanceDraft, requireComplete: boolean): { payload: AcceptancePayload; invalid: string[] } {
  const invalid: string[] = [];
  if (!draft.title.trim()) invalid.push('Tiêu đề');
  if (requireComplete && !draft.acceptance_date) invalid.push('Ngày nghiệm thu');
  if (requireComplete && draft.signature_status === 'pending') invalid.push('Chữ ký');
  if (requireComplete && draft.document_status === 'pending') invalid.push('Tài liệu/PDF');
  const lines = draft.checklist.filter(hasLineInput);
  if (requireComplete && lines.length === 0) invalid.push('Checklist nghiệm thu');
  const checklist = lines.map((line, index) => {
    if (!line.title.trim()) invalid.push(`Hạng mục dòng ${index + 1}`);
    if (requireComplete && line.decision === 'pending') invalid.push(`Kết quả dòng ${index + 1}`);
    const evidenceRequired = line.photo_required || line.decision === 'conditional_pass' || line.decision === 'fail';
    if (requireComplete && evidenceRequired && !line.evidence_id && !line.photo_url.trim()) invalid.push(`Minh chứng dòng ${index + 1}`);
    if (requireComplete && (line.decision === 'conditional_pass' || line.decision === 'fail') && !line.required_action.trim()) invalid.push(`Yêu cầu xử lý dòng ${index + 1}`);
    return { ...line, item_index: index, title: line.title.trim(), description: line.description.trim(), required_action: line.required_action.trim(), photo_url: line.photo_url.trim(), note: line.note.trim() };
  }).map(({ key: _key, ...line }) => line);
  return {
    invalid,
    payload: {
      work_item_id: draft.work_item_id, type: draft.type, title: draft.title.trim(), acceptance_date: draft.acceptance_date,
      location: draft.location.trim(), client_representative: draft.client_representative.trim(), contractor_representative: draft.contractor_representative.trim(),
      scope_summary: draft.scope_summary.trim(), form_instance_id: draft.form_instance_id, pdf_export_id: draft.pdf_export_id,
      signature_status: draft.signature_status, document_status: draft.document_status, notes: draft.notes.trim(), checklist,
    },
  };
}

function newAcceptanceLine(index: number): AcceptanceLineDraft {
  return { key: `${Date.now()}-${index}-${Math.random().toString(36).slice(2, 8)}`, item_index: index, title: '', description: '', decision: 'pending', severity: 'medium', evidence_id: '', required_action: '', due_date: '', photo_required: false, photo_url: '', note: '' };
}

function reindexLines(lines: AcceptanceLineDraft[]) { return lines.map((line, index) => ({ ...line, item_index: index })); }
function hasLineInput(line: AcceptanceLineDraft) { return Boolean(line.title.trim() || line.description.trim() || line.decision !== 'pending' || line.evidence_id || line.required_action.trim() || line.note.trim()); }
function dateOnly(value?: string) { return value ? value.slice(0, 10) : ''; }
function evidenceLabel(record: EvidenceRecord) { return `${record.name ?? record.title ?? record.id} (${evidenceStatusLabel(record.status)})`; }
function evidenceStatusLabel(status?: string) {
  if (status === 'accepted') return 'đã chấp nhận';
  if (status === 'rejected') return 'đã từ chối';
  if (status === 'pending') return 'đang chờ';
  if (status === 'draft') return 'nháp';
  if (status === 'submitted') return 'đã gửi';
  return status ?? 'đã gửi';
}

const controlClassName = 'mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-50';
