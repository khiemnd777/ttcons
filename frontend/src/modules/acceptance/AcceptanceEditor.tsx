import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, CheckCircle2, Circle, ClipboardCheck, FileText, Image as ImageIcon, Link2, Pencil, Plus, Trash2, X } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { postForm } from '../../lib/api/client';
import type { AcceptanceChecklistItem, AcceptanceRecord, EvidenceRecord, WorkflowFormPDFExport, WorkItem } from '../../lib/types/domain';
import {
  EvidenceCapture,
  evidenceUploadFormData,
  type EvidenceCaptureIntent,
  type EvidenceUploadDraft,
} from '../_shared/EvidenceCapture';
import { AcceptanceEvidencePicker } from './AcceptanceEvidencePicker';
import { AcceptanceWorkItemPickerDialog } from './AcceptanceWorkItemPickerDialog';

export type AcceptanceLineDraft = {
  key: string;
  item_index: number;
  title: string;
  description: string;
  decision: string;
  severity: string;
  evidence_id: string;
  evidence_ids: string[];
  evidence_records: EvidenceRecord[];
  uploads: EvidenceUploadDraft[];
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
  checklist: Array<Omit<AcceptanceLineDraft, 'key' | 'evidence_records' | 'uploads'>>;
};

const typeOptions = [
  { value: 'partial', label: 'Nghiệm thu một phần' },
  { value: 'final', label: 'Nghiệm thu hoàn thành' },
];

const decisionOptions = [
  { value: 'pending', label: 'Chờ kết quả', description: 'Chưa kiểm tra hoặc chưa đủ thông tin.' },
  { value: 'pass', label: 'Đạt', description: 'Đáp ứng tiêu chí chấp nhận.' },
  { value: 'conditional_pass', label: 'Đạt có điều kiện', description: 'Có phần việc phải hoàn tất sau nghiệm thu.' },
  { value: 'fail', label: 'Không đạt', description: 'Cần xử lý và nghiệm thu lại.' },
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
  projectId = '',
  evidence,
  workItems = [],
  formInstance,
  pdfExports = [],
  fixedWorkItem = false,
  workItemsLoading = false,
  workItemsError = false,
  disabled = false,
  onReloadWorkItems,
  onChange,
}: {
  draft: AcceptanceDraft;
  projectId?: string;
  evidence: EvidenceRecord[];
  workItems?: WorkItem[];
  formInstance?: { id: string; title?: string };
  pdfExports?: WorkflowFormPDFExport[];
  fixedWorkItem?: boolean;
  workItemsLoading?: boolean;
  workItemsError?: boolean;
  disabled?: boolean;
  onReloadWorkItems?: () => void;
  onChange: (draft: AcceptanceDraft) => void;
}) {
  const toast = useToast();
  const [selectedKey, setSelectedKey] = useState(draft.checklist[0]?.key ?? '');
  const [evidencePickerOpen, setEvidencePickerOpen] = useState(false);
  const [workItemPickerOpen, setWorkItemPickerOpen] = useState(false);
  const [workItemPickerMode, setWorkItemPickerMode] = useState<'capture' | 'change'>('change');
  const pendingCaptureResume = useRef<(() => void) | null>(null);
  const evidenceCatalog = useMemo(() => new Map(evidence.map((record) => [String(record.id), record])), [evidence]);
  const selectedWorkItem = useMemo(() => workItems.find((item) => item.id === draft.work_item_id), [draft.work_item_id, workItems]);
  const evidenceWorkItemIDs = useMemo(() => new Set(
    draft.checklist.flatMap((line) => line.evidence_records.map((record) => String(record.work_item_id ?? '').trim()).filter(Boolean)),
  ), [draft.checklist]);
  const evidenceSourceWorkItemId = evidenceWorkItemIDs.size === 1 ? Array.from(evidenceWorkItemIDs)[0] : '';
  const hasMixedEvidenceSources = evidenceWorkItemIDs.size > 1;
  const hasQueuedUploads = draft.checklist.some((line) => line.uploads.length > 0);

  useEffect(() => {
    if (draft.checklist.some((line) => line.key === selectedKey)) return;
    setSelectedKey(draft.checklist[0]?.key ?? '');
  }, [draft.checklist, selectedKey]);

  const selectedLine = draft.checklist.find((line) => line.key === selectedKey) ?? draft.checklist[0];
  const updateDraft = (patch: Partial<AcceptanceDraft>) => onChange({ ...draft, ...patch });
  const updateLine = (key: string, patch: Partial<AcceptanceLineDraft>) => {
    updateDraft({ checklist: draft.checklist.map((line) => line.key === key ? { ...line, ...patch } : line) });
  };
  const addLine = () => {
    const line = newAcceptanceLine(draft.checklist.length);
    updateDraft({ checklist: [...draft.checklist, line] });
    setSelectedKey(line.key);
  };
  const removeLine = (key: string) => {
    const lines = reindexLines(draft.checklist.filter((line) => line.key !== key));
    updateDraft({ checklist: lines });
    if (key === selectedKey) setSelectedKey(lines[0]?.key ?? '');
  };

  const selectedEvidence = selectedLine ? mergeEvidenceRecords(selectedLine, evidenceCatalog) : [];
  const selectedEvidenceCount = selectedLine ? selectedLine.evidence_ids.length + selectedLine.uploads.length : 0;

  function openWorkItemPicker(mode: 'capture' | 'change', resume?: () => void) {
    if (hasMixedEvidenceSources) {
      toast.warn({ title: 'Minh chứng thuộc nhiều công việc', description: 'Bỏ các minh chứng khác nguồn trước khi gắn công việc hoặc tải ảnh/tệp mới.' });
      return;
    }
    pendingCaptureResume.current = resume ?? null;
    setWorkItemPickerMode(mode);
    setWorkItemPickerOpen(true);
  }

  function closeWorkItemPicker() {
    pendingCaptureResume.current = null;
    setWorkItemPickerOpen(false);
  }

  function selectWorkItem(workItemId: string) {
    const resume = pendingCaptureResume.current;
    pendingCaptureResume.current = null;
    updateDraft({ work_item_id: workItemId });
    setWorkItemPickerOpen(false);
    resume?.();
  }

  function clearWorkItem() {
    pendingCaptureResume.current = null;
    updateDraft({ work_item_id: '' });
    setWorkItemPickerOpen(false);
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-line bg-white p-4">
        <div className="mb-4 flex items-start gap-3">
          <span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-blue-50 text-blue-700"><FileText className="h-4 w-4" /></span>
          <div><h3 className="text-sm font-semibold text-ink">Thông tin biên bản</h3><p className="mt-1 text-xs leading-5 text-slate-500">Xác định phạm vi nghiệm thu trước khi lập các tiêu chí kiểm tra.</p></div>
        </div>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          {draft.work_item_id ? (
            <div className="flex flex-col gap-3 rounded-lg border border-blue-200 bg-blue-50/60 p-3 md:col-span-2 md:flex-row md:items-center md:justify-between xl:col-span-4" aria-label="Nguồn công việc">
              <div className="min-w-0">
                <p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-blue-700"><Link2 className="h-3.5 w-3.5" /> Nguồn công việc</p>
                <p className="mt-1 truncate text-sm font-semibold text-ink">{selectedWorkItem?.title || selectedWorkItem?.name || 'Công việc đã liên kết'}</p>
                {selectedWorkItem ? <div className="mt-1 flex flex-wrap items-center gap-2"><StatusBadge status={selectedWorkItem.status} />{selectedWorkItem.run_node_name ? <span className="text-xs text-slate-600">Giai đoạn: {selectedWorkItem.run_node_name}</span> : null}{selectedWorkItem.assignee_name ? <span className="text-xs text-slate-600">Phụ trách: {identityReferenceText(selectedWorkItem.assignee_name, selectedWorkItem.assignee_account_deleted)}{selectedWorkItem.assignee_account_deleted ? ' · Cần phân công lại' : ''}</span> : null}</div> : null}
              </div>
              {!fixedWorkItem && !evidenceSourceWorkItemId ? <button type="button" disabled={disabled} className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-100 disabled:opacity-50" onClick={() => openWorkItemPicker('change')}><Pencil className="h-4 w-4" /> Thay đổi</button> : null}
              {!fixedWorkItem && evidenceSourceWorkItemId ? <span className="shrink-0 text-xs font-semibold text-slate-500">Đã khóa theo minh chứng</span> : null}
            </div>
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
          <EditorField label="Phạm vi nghiệm thu" className="md:col-span-2 xl:col-span-4">
            <textarea className={textareaClassName} value={draft.scope_summary} onChange={(event) => updateDraft({ scope_summary: event.target.value })} disabled={disabled} placeholder="Mô tả khu vực, hạng mục và mốc công việc được nghiệm thu" />
          </EditorField>
        </div>
      </section>

      <section className="overflow-hidden rounded-lg border border-line bg-white">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-slate-50 px-4 py-3">
          <div>
            <h3 className="flex items-center gap-2 text-sm font-semibold text-ink"><ClipboardCheck className="h-4 w-4 text-blue-600" /> Tiêu chí nghiệm thu</h3>
            <p className="mt-1 text-xs leading-5 text-slate-500">Mỗi tiêu chí có kết quả, yêu cầu xử lý và tối đa 10 minh chứng riêng. Công việc, hồ sơ và PDF chỉ là liên kết tham chiếu.</p>
          </div>
          <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" disabled={disabled} onClick={addLine}><Plus className="h-4 w-4" /> Thêm tiêu chí</button>
        </div>

        {!draft.checklist.length ? <div className="p-5"><EmptyState title="Chưa có tiêu chí nghiệm thu" /><p className="mt-2 text-center text-sm text-slate-500">Thêm tiêu chí đầu tiên để bắt đầu lập checklist.</p><div className="mt-3 flex justify-center"><button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={addLine}><Plus className="h-4 w-4" /> Thêm tiêu chí đầu tiên</button></div></div> : (
          <div className="grid min-h-[520px] lg:grid-cols-[310px_minmax(0,1fr)]">
            <aside className="border-b border-line bg-slate-50/70 lg:border-b-0 lg:border-r">
              <div className="border-b border-line px-3 py-2 text-xs font-semibold uppercase tracking-wide text-slate-500">Danh sách · {draft.checklist.length}</div>
              <div className="max-h-[320px] divide-y divide-line overflow-y-auto lg:max-h-[620px]">
                {draft.checklist.map((line, index) => <CriterionListItem key={line.key} line={line} index={index} active={line.key === selectedLine?.key} onClick={() => setSelectedKey(line.key)} />)}
              </div>
            </aside>

            {selectedLine ? (
              <div className="min-w-0 space-y-5 p-4 sm:p-5">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiêu chí #{selectedLine.item_index + 1}</p><h4 className="mt-1 font-semibold text-ink">{selectedLine.title || 'Tiêu chí chưa đặt tên'}</h4></div>
                  <button type="button" disabled={disabled} className="inline-flex items-center gap-2 self-start rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-60" onClick={() => removeLine(selectedLine.key)}><Trash2 className="h-4 w-4" /> Xóa tiêu chí</button>
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <EditorField label="Hạng mục / tiêu chí">
                    <input className={controlClassName} value={selectedLine.title} onChange={(event) => updateLine(selectedLine.key, { title: event.target.value })} disabled={disabled} placeholder="Ví dụ: Cao độ trần khu B" />
                  </EditorField>
                  <EditorField label="Mô tả / tiêu chuẩn chấp nhận">
                    <input className={controlClassName} value={selectedLine.description} onChange={(event) => updateLine(selectedLine.key, { description: event.target.value })} disabled={disabled} placeholder="Điều kiện để kết luận đạt" />
                  </EditorField>
                </div>

                <fieldset>
                  <legend className="text-xs font-semibold uppercase tracking-wide text-slate-500">Kết quả kiểm tra</legend>
                  <div className="mt-2 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
                    {decisionOptions.map((option) => {
                      const active = selectedLine.decision === option.value;
                      return <button key={option.value} type="button" disabled={disabled} className={clsx('rounded-lg border p-3 text-left disabled:opacity-60', active ? decisionTone(option.value) : 'border-line bg-white hover:border-blue-200 hover:bg-slate-50')} onClick={() => updateLine(selectedLine.key, { decision: option.value })}><span className="flex items-center gap-2 text-sm font-semibold">{active ? <CheckCircle2 className="h-4 w-4" /> : <Circle className="h-4 w-4 text-slate-300" />}{option.label}</span><span className="mt-1 block text-xs leading-5 opacity-75">{option.description}</span></button>;
                    })}
                  </div>
                </fieldset>

                {selectedLine.decision === 'conditional_pass' || selectedLine.decision === 'fail' ? (
                  <div className="grid gap-3 rounded-lg border border-amber-200 bg-amber-50/50 p-4 md:grid-cols-[180px_minmax(0,1fr)_180px]">
                    <EditorField label="Mức độ">
                      <select className={controlClassName} value={selectedLine.severity} onChange={(event) => updateLine(selectedLine.key, { severity: event.target.value })} disabled={disabled}>{severityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
                    </EditorField>
                    <EditorField label="Yêu cầu xử lý">
                      <input className={controlClassName} value={selectedLine.required_action} onChange={(event) => updateLine(selectedLine.key, { required_action: event.target.value })} disabled={disabled} placeholder="Nêu rõ phần việc cần hoàn tất hoặc khắc phục" />
                    </EditorField>
                    <EditorField label="Hạn xử lý">
                      <input className={controlClassName} type="date" value={selectedLine.due_date} onChange={(event) => updateLine(selectedLine.key, { due_date: event.target.value })} disabled={disabled} />
                    </EditorField>
                  </div>
                ) : null}

                <section className="space-y-3 rounded-lg border border-line p-4">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div><h5 className="flex items-center gap-2 text-sm font-semibold text-ink"><ImageIcon className="h-4 w-4 text-blue-600" /> Hình ảnh / minh chứng</h5><p className="mt-1 text-xs leading-5 text-slate-500">Chụp ảnh, chọn tệp mới hoặc dùng hồ sơ đã có trong công trình.</p></div>
                    <span className={clsx('w-fit rounded-md px-2 py-1 text-xs font-semibold', selectedEvidenceCount >= 10 ? 'bg-amber-50 text-amber-700' : 'bg-slate-100 text-slate-600')}>{selectedEvidenceCount}/10</span>
                  </div>

                  <label className="flex items-start gap-2 rounded-md border border-line bg-slate-50 px-3 py-2.5 text-sm text-slate-700"><input type="checkbox" className="mt-0.5" checked={selectedLine.photo_required} onChange={(event) => updateLine(selectedLine.key, { photo_required: event.target.checked })} disabled={disabled} /><span><strong className="font-semibold">Bắt buộc có minh chứng</strong><span className="mt-0.5 block text-xs text-slate-500">Tự động bắt buộc khi kết quả là Đạt có điều kiện hoặc Không đạt.</span></span></label>

                  {selectedEvidence.length ? <div className="grid gap-2 sm:grid-cols-2">{selectedEvidence.map((record) => <SelectedEvidenceCard key={String(record.id)} record={record} mismatch={Boolean(draft.work_item_id && record.work_item_id && record.work_item_id !== draft.work_item_id)} disabled={disabled} onRemove={() => removeEvidence(selectedLine, String(record.id), (patch) => updateLine(selectedLine.key, patch))} />)}</div> : null}
                  {selectedLine.photo_url ? <div className="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-800">Tiêu chí này còn một liên kết ảnh từ dữ liệu cũ. Liên kết sẽ tiếp tục được giữ khi lưu.</div> : null}

                  <div className="flex flex-wrap gap-2">
                    <button type="button" disabled={disabled || !projectId || selectedLine.evidence_ids.length + selectedLine.uploads.length >= 10} className="inline-flex items-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" onClick={() => setEvidencePickerOpen(true)}><FileText className="h-4 w-4" /> Chọn từ hồ sơ công trình</button>
                  </div>

                  <EvidenceCapture
                    drafts={selectedLine.uploads}
                    onChange={(uploads) => updateLine(selectedLine.key, { uploads })}
                    hasSource={Boolean(draft.work_item_id)}
                    onSourceRequired={(_intent: EvidenceCaptureIntent, resume) => openWorkItemPicker('capture', resume)}
                    disabled={disabled || selectedLine.evidence_ids.length >= 10}
                    compact
                    maxFiles={Math.max(0, 10 - selectedLine.evidence_ids.length)}
                    hint="Ảnh/tệp mới được giữ tạm tại đây và chỉ tải lên khi lưu biên bản. Mỗi tiêu chí tối đa 10 minh chứng."
                  />
                </section>

                <EditorField label="Ghi chú tiêu chí">
                  <textarea className={textareaClassName} value={selectedLine.note} onChange={(event) => updateLine(selectedLine.key, { note: event.target.value })} disabled={disabled} placeholder="Thông tin bổ sung cho người kiểm tra hoặc người kết luận" />
                </EditorField>
              </div>
            ) : null}
          </div>
        )}
      </section>

      <details className="rounded-lg border border-line bg-white" open={Boolean(draft.form_instance_id || draft.pdf_export_id || draft.notes)}>
        <summary className="cursor-pointer px-4 py-3 text-sm font-semibold text-ink">Hồ sơ, chữ ký và ghi chú</summary>
        <div className="grid gap-3 border-t border-line p-4 md:grid-cols-2 xl:grid-cols-4">
          <EditorField label="Chữ ký">
            <select className={controlClassName} value={draft.signature_status} onChange={(event) => updateDraft({ signature_status: event.target.value })} disabled={disabled}>{signatureOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
          </EditorField>
          <EditorField label="Tài liệu / PDF">
            <select className={controlClassName} value={draft.document_status} onChange={(event) => updateDraft({ document_status: event.target.value })} disabled={disabled}>{documentOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
          </EditorField>
          {formInstance?.id || draft.form_instance_id ? (
            <EditorField label="Hồ sơ nghiệp vụ" className="md:col-span-2">
              <select className={controlClassName} value={draft.form_instance_id} onChange={(event) => updateDraft({ form_instance_id: event.target.value })} disabled={disabled || !formInstance?.id}><option value="">Không liên kết hồ sơ</option>{formInstance?.id ? <option value={formInstance.id}>{formInstance.title || formInstance.id}</option> : <option value={draft.form_instance_id}>{draft.form_instance_id}</option>}</select>
            </EditorField>
          ) : null}
          {pdfExports.length || draft.pdf_export_id ? (
            <EditorField label="PDF liên kết" className="md:col-span-2">
              <select className={controlClassName} value={draft.pdf_export_id} onChange={(event) => updateDraft({ pdf_export_id: event.target.value, document_status: event.target.value ? 'linked' : draft.document_status })} disabled={disabled || pdfExports.length === 0}><option value="">Không liên kết PDF</option>{pdfExports.map((record) => <option key={record.id} value={record.id}>{record.file_name ?? record.title ?? record.id}</option>)}{!pdfExports.length && draft.pdf_export_id ? <option value={draft.pdf_export_id}>{draft.pdf_export_id}</option> : null}</select>
            </EditorField>
          ) : null}
          <EditorField label="Ghi chú chung" className="md:col-span-2 xl:col-span-4">
            <textarea className={textareaClassName} value={draft.notes} onChange={(event) => updateDraft({ notes: event.target.value })} disabled={disabled} />
          </EditorField>
        </div>
      </details>

      {selectedLine ? (
        <AcceptanceEvidencePicker
          open={evidencePickerOpen}
          projectId={projectId}
          workItemId={draft.work_item_id || undefined}
          selected={selectedEvidence}
          maxSelected={Math.max(0, 10 - selectedLine.uploads.length)}
          onClose={() => setEvidencePickerOpen(false)}
          onConfirm={(records) => {
            const normalized = uniqueEvidenceRecords(records);
            updateLine(selectedLine.key, { evidence_records: normalized, evidence_ids: normalized.map((record) => String(record.id)), evidence_id: String(normalized[0]?.id ?? '') });
            setEvidencePickerOpen(false);
          }}
        />
      ) : null}

      <AcceptanceWorkItemPickerDialog
        open={workItemPickerOpen}
        items={workItems}
        selectedId={draft.work_item_id}
        requiredWorkItemId={evidenceSourceWorkItemId}
        loading={workItemsLoading}
        error={workItemsError}
        confirmLabel={workItemPickerMode === 'capture' ? 'Chọn và tiếp tục' : 'Dùng công việc này'}
        canClear={workItemPickerMode === 'change' && Boolean(draft.work_item_id) && !hasQueuedUploads && !evidenceSourceWorkItemId}
        onClose={closeWorkItemPicker}
        onRetry={onReloadWorkItems}
        onClear={clearWorkItem}
        onConfirm={selectWorkItem}
      />
    </div>
  );
}

function CriterionListItem({ line, index, active, onClick }: { line: AcceptanceLineDraft; index: number; active: boolean; onClick: () => void }) {
  const issue = lineIssue(line);
  return <button type="button" className={clsx('w-full px-3 py-3 text-left transition', active ? 'bg-white shadow-[inset_3px_0_0_#2563eb]' : 'hover:bg-white/70')} onClick={onClick}><span className="flex items-start gap-3"><span className={clsx('mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full text-xs font-semibold', active ? 'bg-blue-600 text-white' : 'bg-white text-slate-500 ring-1 ring-line')}>{index + 1}</span><span className="min-w-0 flex-1"><span className="block truncate text-sm font-semibold text-ink">{line.title || 'Tiêu chí chưa đặt tên'}</span><span className="mt-1 flex flex-wrap items-center gap-2 text-xs text-slate-500"><span>{decisionLabel(line.decision)}</span><span>·</span><span>{line.evidence_ids.length + line.uploads.length} minh chứng</span></span>{issue ? <span className="mt-1 inline-flex items-center gap-1 text-xs font-medium text-amber-700"><AlertTriangle className="h-3.5 w-3.5" /> {issue}</span> : null}</span></span></button>;
}

function SelectedEvidenceCard({ record, mismatch, disabled, onRemove }: { record: EvidenceRecord; mismatch: boolean; disabled: boolean; onRemove: () => void }) {
  const Icon = record.evidence_type === 'photo' ? ImageIcon : FileText;
  return <article className={clsx('flex min-w-0 items-start gap-3 rounded-md border p-3', mismatch ? 'border-amber-300 bg-amber-50' : 'border-line bg-white')}><span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-slate-100 text-slate-500"><Icon className="h-4 w-4" /></span><div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold text-ink">{record.name || record.title || String(record.id)}</p><p className="mt-1 truncate text-xs text-slate-500">{record.work_item_title || 'Hồ sơ công trình'}</p><div className="mt-2 flex flex-wrap items-center gap-2"><StatusBadge status={record.status} />{mismatch ? <span className="text-xs font-semibold text-amber-800">Khác nguồn công việc</span> : null}</div></div><button type="button" disabled={disabled} className="rounded-md p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600 disabled:opacity-50" onClick={onRemove} aria-label="Bỏ minh chứng"><X className="h-4 w-4" /></button></article>;
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
  const checklist = (record.checklist ?? []).map((item, index) => acceptanceLineFromRecord(item, record, index));
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
  if (!draft.work_item_id && draft.checklist.some((line) => line.uploads.length > 0)) invalid.push('Nguồn công việc cho tệp chờ tải');
  const lines = draft.checklist.filter(hasLineInput);
  if (requireComplete && lines.length === 0) invalid.push('Tiêu chí nghiệm thu');
  const checklist = lines.map((line, index) => {
    const evidenceIDs = uniqueStrings([line.evidence_id, ...line.evidence_ids]);
    const evidenceCount = evidenceIDs.length + line.uploads.length;
    if (!line.title.trim()) invalid.push(`Hạng mục tiêu chí ${index + 1}`);
    if (requireComplete && line.decision === 'pending') invalid.push(`Kết quả tiêu chí ${index + 1}`);
    if (evidenceCount > 10) invalid.push(`Tối đa 10 minh chứng ở tiêu chí ${index + 1}`);
    if (draft.work_item_id && line.evidence_records.some((record) => record.work_item_id && record.work_item_id !== draft.work_item_id)) invalid.push(`Minh chứng khác nguồn công việc ở tiêu chí ${index + 1}`);
    const evidenceRequired = line.photo_required || line.decision === 'conditional_pass' || line.decision === 'fail';
    if (requireComplete && evidenceRequired && evidenceCount === 0 && !line.photo_url.trim()) invalid.push(`Minh chứng tiêu chí ${index + 1}`);
    if (requireComplete && (line.decision === 'conditional_pass' || line.decision === 'fail') && !line.required_action.trim()) invalid.push(`Yêu cầu xử lý tiêu chí ${index + 1}`);
    const { key: _key, evidence_records: _records, uploads: _uploads, ...payloadLine } = line;
    return {
      ...payloadLine,
      item_index: index,
      title: line.title.trim(),
      description: line.description.trim(),
      required_action: line.required_action.trim(),
      photo_url: line.photo_url.trim(),
      note: line.note.trim(),
      evidence_id: evidenceIDs[0] ?? '',
      evidence_ids: evidenceIDs,
    };
  });
  return {
    invalid: uniqueStrings(invalid),
    payload: {
      work_item_id: draft.work_item_id, type: draft.type, title: draft.title.trim(), acceptance_date: draft.acceptance_date,
      location: draft.location.trim(), client_representative: draft.client_representative.trim(), contractor_representative: draft.contractor_representative.trim(),
      scope_summary: draft.scope_summary.trim(), form_instance_id: draft.form_instance_id, pdf_export_id: draft.pdf_export_id,
      signature_status: draft.signature_status, document_status: draft.document_status, notes: draft.notes.trim(), checklist,
    },
  };
}

export async function uploadAcceptanceDraftEvidence(draft: AcceptanceDraft, onProgress?: (next: AcceptanceDraft) => void): Promise<AcceptanceDraft> {
  if (!draft.checklist.some((line) => line.uploads.length)) return draft;
  if (!draft.work_item_id) throw new Error('Chọn nguồn công việc trước khi tải ảnh hoặc tệp minh chứng.');
  let next = draft;
  for (const sourceLine of draft.checklist) {
    for (const upload of sourceLine.uploads) {
      const record = await postForm<EvidenceRecord>(`/work-items/${draft.work_item_id}/evidence/upload`, evidenceUploadFormData(upload, draft.work_item_id));
      next = {
        ...next,
        checklist: next.checklist.map((line) => line.key === sourceLine.key ? {
          ...line,
          evidence_ids: uniqueStrings([...line.evidence_ids, String(record.id)]),
          evidence_id: line.evidence_id || String(record.id),
          evidence_records: uniqueEvidenceRecords([...line.evidence_records, record]),
          uploads: line.uploads.filter((draftUpload) => draftUpload.id !== upload.id),
        } : line),
      };
      onProgress?.(next);
    }
  }
  return next;
}

function acceptanceLineFromRecord(item: AcceptanceChecklistItem, acceptance: AcceptanceRecord, index: number): AcceptanceLineDraft {
  const records = uniqueEvidenceRecords(item.evidence_records ?? legacyEvidenceRecords(item, acceptance));
  const evidenceIDs = uniqueStrings([item.evidence_id ?? '', ...(item.evidence_ids ?? []), ...records.map((record) => String(record.id))]);
  return {
    key: String(item.id ?? `${acceptance.id}-${index}`), item_index: Number(item.item_index ?? index),
    title: item.title ?? '', description: item.description ?? '', decision: item.decision || (item.is_passed ? 'pass' : 'pending'),
    severity: item.severity ?? 'medium', evidence_id: evidenceIDs[0] ?? '', evidence_ids: evidenceIDs, evidence_records: records, uploads: [],
    required_action: item.required_action ?? '', due_date: dateOnly(item.due_date), photo_required: Boolean(item.photo_required), photo_url: item.photo_url ?? '', note: item.note ?? '',
  };
}

function legacyEvidenceRecords(item: AcceptanceChecklistItem, acceptance: AcceptanceRecord): EvidenceRecord[] {
  if (!item.evidence_id) return [];
  return [{
    id: item.evidence_id,
    work_item_id: acceptance.work_item_id ?? '',
    evidence_type: 'other',
    name: item.evidence_name || item.evidence_id,
    status: item.evidence_status || 'submitted',
  } as EvidenceRecord];
}

function newAcceptanceLine(index: number): AcceptanceLineDraft {
  return { key: `${Date.now()}-${index}-${Math.random().toString(36).slice(2, 8)}`, item_index: index, title: '', description: '', decision: 'pending', severity: 'medium', evidence_id: '', evidence_ids: [], evidence_records: [], uploads: [], required_action: '', due_date: '', photo_required: false, photo_url: '', note: '' };
}

function mergeEvidenceRecords(line: AcceptanceLineDraft, catalog: Map<string, EvidenceRecord>) {
  const known = new Map(line.evidence_records.map((record) => [String(record.id), record]));
  line.evidence_ids.forEach((id) => { if (!known.has(id) && catalog.has(id)) known.set(id, catalog.get(id)!); });
  return line.evidence_ids.map((id) => known.get(id)).filter((record): record is EvidenceRecord => Boolean(record));
}

function removeEvidence(line: AcceptanceLineDraft, id: string, onChange: (patch: Partial<AcceptanceLineDraft>) => void) {
  const evidenceIDs = line.evidence_ids.filter((value) => value !== id);
  onChange({ evidence_ids: evidenceIDs, evidence_id: evidenceIDs[0] ?? '', evidence_records: line.evidence_records.filter((record) => String(record.id) !== id) });
}

function uniqueEvidenceRecords(records: EvidenceRecord[]) {
  const seen = new Set<string>();
  return records.filter((record) => {
    const id = String(record.id ?? '');
    if (!id || seen.has(id) || record.status === 'rejected') return false;
    seen.add(id);
    return true;
  });
}

function reindexLines(lines: AcceptanceLineDraft[]) { return lines.map((line, index) => ({ ...line, item_index: index })); }
function hasLineInput(line: AcceptanceLineDraft) { return Boolean(line.title.trim() || line.description.trim() || line.decision !== 'pending' || line.evidence_ids.length || line.uploads.length || line.required_action.trim() || line.note.trim()); }
function dateOnly(value?: string) { return value ? value.slice(0, 10) : ''; }
function uniqueStrings(values: string[]) { return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean))); }
function decisionLabel(value: string) { return decisionOptions.find((option) => option.value === value)?.label ?? value; }
function lineIssue(line: AcceptanceLineDraft) {
  if (!line.title.trim()) return 'Thiếu tên tiêu chí';
  if (line.decision === 'pending') return 'Chưa có kết quả';
  if ((line.decision === 'conditional_pass' || line.decision === 'fail') && !line.required_action.trim()) return 'Thiếu yêu cầu xử lý';
  const evidenceRequired = line.photo_required || line.decision === 'conditional_pass' || line.decision === 'fail';
  if (evidenceRequired && !line.evidence_ids.length && !line.uploads.length && !line.photo_url) return 'Thiếu minh chứng';
  return '';
}
function decisionTone(value: string) {
  if (value === 'pass') return 'border-emerald-300 bg-emerald-50 text-emerald-800';
  if (value === 'conditional_pass') return 'border-amber-300 bg-amber-50 text-amber-800';
  if (value === 'fail') return 'border-rose-300 bg-rose-50 text-rose-800';
  return 'border-blue-300 bg-blue-50 text-blue-800';
}

const controlClassName = 'mt-1 h-10 w-full rounded-md border border-line bg-white px-3 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-50';
const textareaClassName = 'mt-1 min-h-20 w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-normal normal-case tracking-normal outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-50';
