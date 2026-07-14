import { useState, type Dispatch, type SetStateAction } from 'react';
import { AlertTriangle, CalendarClock, CheckCircle2, ClipboardCheck, Loader2, Plus, Trash2, Wrench } from 'lucide-react';
import clsx from 'clsx';
import type { ConstructionWorkspace, EvidenceRecord } from '../../lib/types/domain';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { FieldBlock, controlClass, type FormErrors } from './ConstructionFormControls';

export type ReportLinkedDraft = {
  id: string;
  kind: 'check' | 'action';
  source_blocker_key?: string;
  source_label?: string;
  auto_title?: boolean;
  work_item_id: string;
  title: string;
  criteria_text: string;
  location: string;
  scheduled_at: string;
  description: string;
  assigned_user_id: string;
  due_at: string;
  priority: 'normal' | 'urgent';
  impact_scope: 'none' | 'timeline' | 'budget' | 'design_scope';
  evidence_id: string;
  uploads: ConstructionEvidenceUploadDraft[];
  last_error?: string;
};

export type ReportLinkedComposerMode = 'batch' | 'immediate';

type Props = {
  mode: ReportLinkedComposerMode;
  drafts: ReportLinkedDraft[];
  onChange: Dispatch<SetStateAction<ReportLinkedDraft[]>>;
  workspace?: ConstructionWorkspace;
  sourceWorkItemId?: string;
  reportEvidence?: EvidenceRecord[];
  reportEvidencePending?: boolean;
  primaryEvidenceId?: string;
  canCreateCheck: boolean;
  canCreateAction: boolean;
  blockers?: string[];
  showValidation?: boolean;
  notice?: string;
  onSubmitDraft?: (draft: ReportLinkedDraft) => Promise<void>;
};

export function createReportLinkedDraft(kind: ReportLinkedDraft['kind'], options: { title?: string; workItemId?: string; sourceBlockerKey?: string; sourceLabel?: string } = {}): ReportLinkedDraft {
  return {
    id: linkedDraftID(),
    kind,
    source_blocker_key: options.sourceBlockerKey,
    source_label: options.sourceLabel,
    auto_title: Boolean(options.title),
    work_item_id: options.workItemId ?? '',
    title: options.title ?? '',
    criteria_text: '',
    location: '',
    scheduled_at: '',
    description: '',
    assigned_user_id: '',
    due_at: kind === 'action' ? defaultDueAt() : '',
    priority: 'normal',
    impact_scope: 'none',
    evidence_id: '',
    uploads: [],
  };
}

export function validateReportLinkedDraft(draft: ReportLinkedDraft, sourceWorkItemId: string, hasReportEvidence: boolean): FormErrors {
  const errors: FormErrors = {};
  if (!(sourceWorkItemId || draft.work_item_id).trim()) errors.work_item_id = 'Công việc liên quan là bắt buộc';
  if (!draft.title.trim()) errors.title = draft.kind === 'check' ? 'Nội dung kiểm tra là bắt buộc' : 'Việc cần xử lý là bắt buộc';
  if (draft.kind === 'action') {
    if (!draft.assigned_user_id.trim()) errors.assigned_user_id = 'Người xử lý là bắt buộc';
    if (!draft.due_at.trim()) errors.due_at = 'Hạn xử lý là bắt buộc';
    if (!hasReportEvidence && !draft.evidence_id && !draft.uploads.length) errors.evidence = 'Cần ít nhất một minh chứng hiện trạng';
  }
  return errors;
}

export function validateReportLinkedDrafts(drafts: ReportLinkedDraft[], sourceWorkItemId: string, hasReportEvidence: boolean) {
  return drafts.map((draft) => ({ draft, errors: validateReportLinkedDraft(draft, sourceWorkItemId, hasReportEvidence) })).filter((item) => Object.keys(item.errors).length > 0);
}

export function reportLinkedSubmitLabel(drafts: ReportLinkedDraft[]) {
  if (!drafts.length) return 'Lưu báo cáo';
  const checks = drafts.filter((item) => item.kind === 'check').length;
  const actions = drafts.length - checks;
  const parts = [checks ? `${checks} kiểm tra` : '', actions ? `${actions} việc` : ''].filter(Boolean);
  return `Lưu báo cáo & tạo ${parts.join(', ')}`;
}

export function ConstructionReportLinkedComposer({ mode, drafts, onChange, workspace, sourceWorkItemId = '', reportEvidence = [], reportEvidencePending = false, primaryEvidenceId = '', canCreateCheck, canCreateAction, blockers = [], showValidation = false, notice, onSubmitDraft }: Props) {
  const [attempted, setAttempted] = useState<Record<string, boolean>>({});
  const [saving, setSaving] = useState<Record<string, boolean>>({});
  const [submitErrors, setSubmitErrors] = useState<Record<string, string>>({});
  const hasReportEvidence = reportEvidence.length > 0 || reportEvidencePending;

  function add(kind: ReportLinkedDraft['kind'], title = '', sourceBlockerKey?: string) {
    onChange((current) => [...current, createReportLinkedDraft(kind, { title, workItemId: sourceWorkItemId, sourceBlockerKey, sourceLabel: title })]);
  }

  function patch(id: string, values: Partial<ReportLinkedDraft>) {
    onChange((current) => current.map((draft) => draft.id === id ? { ...draft, ...values, last_error: undefined } : draft));
    setSubmitErrors((current) => ({ ...current, [id]: '' }));
  }

  function remove(id: string) {
    onChange((current) => current.filter((draft) => draft.id !== id));
  }

  async function submit(draft: ReportLinkedDraft) {
    setAttempted((current) => ({ ...current, [draft.id]: true }));
    const errors = validateReportLinkedDraft(draft, sourceWorkItemId, hasReportEvidence);
    if (Object.keys(errors).length || !onSubmitDraft) return;
    setSaving((current) => ({ ...current, [draft.id]: true }));
    setSubmitErrors((current) => ({ ...current, [draft.id]: '' }));
    try {
      await onSubmitDraft(draft);
      remove(draft.id);
    } catch (error) {
      setSubmitErrors((current) => ({ ...current, [draft.id]: errorMessage(error) }));
    } finally {
      setSaving((current) => ({ ...current, [draft.id]: false }));
    }
  }

  const canAdd = Boolean(workspace);
  return <aside className={clsx('rounded-lg border border-line bg-slate-50/70 p-4', mode === 'batch' && 'xl:sticky xl:top-4 xl:self-start')}>
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between xl:flex-col">
      <div>
        <h3 className="font-semibold text-ink">Kiểm tra & xử lý</h3>
        <p className="mt-1 text-xs leading-5 text-slate-500">Chuẩn bị việc phát sinh ngay tại báo cáo, không cần đổi màn hình.</p>
      </div>
      <div className="flex flex-wrap gap-2">
        {canCreateCheck ? <button type="button" disabled={!canAdd} className="inline-flex items-center gap-1.5 rounded-md border border-blue-200 bg-white px-2.5 py-2 text-xs font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" onClick={() => add('check')}><ClipboardCheck className="h-3.5 w-3.5" /> Kiểm tra</button> : null}
        {canCreateAction ? <button type="button" disabled={!canAdd} className="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-2.5 py-2 text-xs font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={() => add('action')}><Plus className="h-3.5 w-3.5" /> Việc xử lý</button> : null}
      </div>
    </div>

    {!workspace && (canCreateCheck || canCreateAction) ? <div className="mt-3 flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2 text-xs text-slate-500"><Loader2 className="h-3.5 w-3.5 animate-spin" /> Đang tải công việc và người phụ trách...</div> : null}
    {notice ? <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs font-medium leading-5 text-amber-800">{notice}</div> : null}

    {mode === 'immediate' && blockers.length && workspace ? <div className="mt-4 space-y-2"><p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">Tạo nhanh từ blocker</p>{blockers.map((blocker, index) => {
      const key = `detail-${index}`;
      const linked = drafts.some((draft) => draft.source_blocker_key === key);
      return <div key={`${blocker}-${index}`} className="rounded-md border border-amber-200 bg-amber-50 p-2.5"><p className="text-xs font-medium leading-5 text-amber-900">{blocker}</p><div className="mt-2 flex gap-2">{canCreateCheck ? <button type="button" disabled={linked} className="rounded-md border border-amber-300 bg-white px-2 py-1 text-[11px] font-semibold text-amber-800 disabled:opacity-40" onClick={() => add('check', blocker, key)}>Kiểm tra</button> : null}{canCreateAction ? <button type="button" disabled={linked} className="rounded-md bg-amber-700 px-2 py-1 text-[11px] font-semibold text-white disabled:opacity-40" onClick={() => add('action', blocker, key)}>Giao xử lý</button> : null}</div></div>;
    })}</div> : null}

    <div className="mt-4 space-y-3">
      {!drafts.length ? <div className="rounded-lg border border-dashed border-slate-300 bg-white p-5 text-center"><CheckCircle2 className="mx-auto h-5 w-5 text-emerald-500" /><p className="mt-2 text-sm font-semibold text-slate-700">Chưa có phát sinh cần tạo</p><p className="mt-1 text-xs text-slate-500">Có thể chỉ lưu báo cáo hoặc thêm nhanh khi cần.</p></div> : null}
      {drafts.map((draft, index) => {
        const errors = showValidation || attempted[draft.id] ? validateReportLinkedDraft(draft, sourceWorkItemId, hasReportEvidence) : {};
        const selectedEvidenceId = draft.evidence_id || primaryEvidenceId || reportEvidence[0]?.id || '';
        return <article key={draft.id} className={clsx('rounded-lg border bg-white p-3', draft.kind === 'check' ? 'border-blue-200' : 'border-amber-200', Object.keys(errors).length && 'ring-1 ring-rose-200')}>
          <div className="flex items-start justify-between gap-3">
            <div className="flex min-w-0 items-start gap-2"><span className={clsx('grid h-7 w-7 shrink-0 place-items-center rounded-md', draft.kind === 'check' ? 'bg-blue-50 text-blue-700' : 'bg-amber-50 text-amber-700')}>{draft.kind === 'check' ? <ClipboardCheck className="h-4 w-4" /> : <Wrench className="h-4 w-4" />}</span><div><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{draft.kind === 'check' ? `Kiểm tra ${index + 1}` : `Việc xử lý ${index + 1}`}</p>{draft.source_label ? <p className="mt-0.5 line-clamp-2 text-[11px] text-slate-500">Từ blocker: {draft.source_label}</p> : null}</div></div>
            <button type="button" disabled={saving[draft.id]} className="rounded-md p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600 disabled:opacity-50" onClick={() => remove(draft.id)} aria-label="Xóa mục liên kết"><Trash2 className="h-4 w-4" /></button>
          </div>

          <div className="mt-3 space-y-3">
            {!sourceWorkItemId ? <FieldBlock label="Công việc liên quan" required error={errors.work_item_id}><select className={controlClass(errors.work_item_id)} value={draft.work_item_id} onChange={(event) => patch(draft.id, { work_item_id: event.target.value })}><option value="">Chọn công việc</option>{workspace?.work_items.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></FieldBlock> : <p className="rounded-md bg-slate-50 px-2.5 py-2 text-xs text-slate-600">Dùng công việc của báo cáo.</p>}
            <FieldBlock label={draft.kind === 'check' ? 'Nội dung kiểm tra' : 'Việc cần xử lý'} required error={errors.title}><input className={controlClass(errors.title)} value={draft.title} onChange={(event) => patch(draft.id, { title: event.target.value, auto_title: false })} placeholder={draft.kind === 'check' ? 'Điểm cần kiểm tra' : 'Nội dung cần khắc phục'} /></FieldBlock>

            {draft.kind === 'check' ? <FieldBlock label="Tiêu chí" helper="Mỗi dòng là một tiêu chí; có thể để trống."><textarea rows={3} className={controlClass()} value={draft.criteria_text} onChange={(event) => patch(draft.id, { criteria_text: event.target.value })} placeholder={'Cao độ và độ phẳng\nBề mặt hoàn thiện'} /></FieldBlock> : <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
              <FieldBlock label="Người xử lý" required error={errors.assigned_user_id}><select className={controlClass(errors.assigned_user_id)} value={draft.assigned_user_id} onChange={(event) => patch(draft.id, { assigned_user_id: event.target.value })}><option value="">Chọn người phụ trách</option>{workspace?.participants.map((item) => <option key={item.id} value={item.id}>{item.full_name}</option>)}</select></FieldBlock>
              <FieldBlock label="Hạn xử lý" required error={errors.due_at}><input type="datetime-local" className={controlClass(errors.due_at)} value={draft.due_at} onChange={(event) => patch(draft.id, { due_at: event.target.value })} /></FieldBlock>
            </div>}

            {hasReportEvidence ? <FieldBlock label="Minh chứng báo cáo" required={draft.kind === 'action'}>{reportEvidence.length > 1 && mode === 'immediate' ? <select className={controlClass()} value={selectedEvidenceId} onChange={(event) => patch(draft.id, { evidence_id: event.target.value })}>{reportEvidence.map((item) => <option key={item.id} value={item.id}>{item.name || item.title || 'Minh chứng báo cáo'}</option>)}</select> : <div className="mt-1 flex items-center gap-2 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs font-medium text-emerald-700"><CheckCircle2 className="h-4 w-4" /> Tự động dùng minh chứng của báo cáo</div>}</FieldBlock> : mode === 'batch' ? <div className={clsx('rounded-md border px-3 py-2 text-xs leading-5', errors.evidence ? 'border-rose-200 bg-rose-50 text-rose-700' : 'border-dashed border-slate-300 bg-slate-50 text-slate-500')}><AlertTriangle className="mr-1 inline h-3.5 w-3.5" /> {draft.kind === 'action' ? 'Thêm ảnh ở phần Minh chứng báo cáo để tạo việc xử lý.' : 'Chưa có ảnh hiện trạng; kiểm tra vẫn có thể được tạo.'}</div> : <FieldBlock label={draft.kind === 'action' ? 'Minh chứng hiện trạng' : 'Hình ảnh hiện trạng'} required={draft.kind === 'action'} error={errors.evidence}><ConstructionEvidenceCapture drafts={draft.uploads} onChange={(uploads) => patch(draft.id, { uploads })} hasSource={Boolean(sourceWorkItemId || draft.work_item_id)} disabled={saving[draft.id]} compact maxFiles={draft.kind === 'check' ? 10 : 1} hint={draft.kind === 'check' ? 'Không bắt buộc; ảnh sẽ được gắn làm bối cảnh kiểm tra.' : 'Chụp hoặc chọn một ảnh/tệp hiện trạng.'} /></FieldBlock>}

            <details className="rounded-md border border-line bg-slate-50 px-3 py-2">
              <summary className="cursor-pointer text-xs font-semibold text-slate-600">Thêm thông tin</summary>
              <div className="mt-3 space-y-3">
                {draft.kind === 'check' ? <><FieldBlock label="Vị trí"><input className={controlClass()} value={draft.location} onChange={(event) => patch(draft.id, { location: event.target.value })} placeholder="Ví dụ: Pantry tầng 12" /></FieldBlock><FieldBlock label="Lịch dự kiến"><input type="datetime-local" className={controlClass()} value={draft.scheduled_at} onChange={(event) => patch(draft.id, { scheduled_at: event.target.value })} /></FieldBlock></> : <><FieldBlock label="Mô tả hiện trạng"><textarea rows={3} className={controlClass()} value={draft.description} onChange={(event) => patch(draft.id, { description: event.target.value })} /></FieldBlock><div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-1"><FieldBlock label="Mức ưu tiên"><select className={controlClass()} value={draft.priority} onChange={(event) => patch(draft.id, { priority: event.target.value as ReportLinkedDraft['priority'] })}><option value="normal">Bình thường</option><option value="urgent">Gấp</option></select></FieldBlock><FieldBlock label="Ảnh hưởng"><select className={controlClass()} value={draft.impact_scope} onChange={(event) => patch(draft.id, { impact_scope: event.target.value as ReportLinkedDraft['impact_scope'] })}><option value="none">Không ảnh hưởng</option><option value="timeline">Tiến độ</option><option value="budget">Chi phí</option><option value="design_scope">Thiết kế / phạm vi</option></select></FieldBlock></div></>}
              </div>
            </details>

            {draft.last_error || submitErrors[draft.id] ? <p className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-medium text-rose-700">{draft.last_error || submitErrors[draft.id]}</p> : null}
            {mode === 'immediate' ? <button type="button" disabled={saving[draft.id]} className={clsx('inline-flex w-full items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold text-white disabled:opacity-60', draft.kind === 'check' ? 'bg-blue-600 hover:bg-blue-700' : 'bg-amber-700 hover:bg-amber-800')} onClick={() => submit(draft)}>{saving[draft.id] ? <Loader2 className="h-4 w-4 animate-spin" /> : draft.kind === 'check' ? <ClipboardCheck className="h-4 w-4" /> : <Wrench className="h-4 w-4" />}{saving[draft.id] ? 'Đang tạo...' : draft.kind === 'check' ? 'Tạo kiểm tra' : 'Tạo việc xử lý'}</button> : <div className="flex items-center gap-2 rounded-md bg-blue-50 px-3 py-2 text-xs font-medium text-blue-700"><CalendarClock className="h-4 w-4" /> Sẽ tạo cùng lúc khi lưu báo cáo.</div>}
          </div>
        </article>;
      })}
    </div>
  </aside>;
}

function linkedDraftID() {
  return globalThis.crypto?.randomUUID?.() ?? `linked-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function defaultDueAt() {
  const date = new Date();
  date.setDate(date.getDate() + 1);
  date.setHours(17, 0, 0, 0);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Không tạo được liên kết. Vui lòng thử lại.';
}
