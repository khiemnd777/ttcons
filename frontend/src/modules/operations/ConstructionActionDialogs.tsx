import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { AlertTriangle, CalendarDays, CheckCircle2, GitPullRequestArrow, Loader2, Pencil, ShieldCheck, Wrench } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { post, put } from '../../lib/api/client';
import type { ConstructionActionItem, ConstructionCheck, ConstructionDailyReportSource, ConstructionVariationContext, ConstructionWorkspace } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { ConstructionEvidenceButton } from './ConstructionEvidenceButton';
import { FieldBlock, SectionTitle, controlClass, errorSummary, requiredErrors, type FormErrors } from './ConstructionFormControls';
import { createConstructionAction, uploadConstructionEvidence } from './constructionWorkspaceApi';
import { actionStatusLabel, constructionActionAssigneeLabel, constructionImpactHelperText, dateTimeText, eventLabel, impactLabel, isConstructionActionEditable, optionalISO, toDateTimeInput } from './constructionWorkspaceUtils';
import { syncWorkItemMutationTransition, workItemAutoStartedNotice } from '../work-items/workItemMutationContext';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';

export type ActionDialogState =
  | { type: 'create'; sourceReport?: ConstructionDailyReportSource }
  | { type: 'edit'; action: ConstructionActionItem }
  | { type: 'view'; action: ConstructionActionItem }
  | { type: 'resolve'; action: ConstructionActionItem }
  | { type: 'verify'; action: ConstructionActionItem }
  | null;

type Props = {
  state: ActionDialogState;
  projectId: string;
  workspace: ConstructionWorkspace;
  canEdit: boolean;
  canResolve: boolean;
  canVerify: boolean;
  canApproveImpact: boolean;
  canProposeVariation: boolean;
  variationContext?: ConstructionVariationContext;
  variationContextLoading: boolean;
  variationContextError: boolean;
  canViewContracts: boolean;
  onClose: () => void;
  onSaved: () => Promise<void>;
  onEdit: (action: ConstructionActionItem) => void;
  onResolve: (action: ConstructionActionItem) => void;
  onVerify: (action: ConstructionActionItem) => void;
  onApproveImpact: (action: ConstructionActionItem) => Promise<void>;
  onRetryVariationContext?: () => void;
  onProposeVariation: (action: ConstructionActionItem) => void;
  onViewCheck: (check: ConstructionCheck) => void;
  onViewReport?: (reportId: string) => void;
};

export function ConstructionActionDialogs(props: Props) {
  if (!props.state) return null;
  if (props.state.type === 'create' || props.state.type === 'edit') return <ActionFormDialog {...props} state={props.state} />;
  if (props.state.type === 'resolve') return <ResolutionDialog {...props} action={props.state.action} />;
  if (props.state.type === 'verify') return <VerificationDialog {...props} action={props.state.action} />;
  return <ActionViewDialog {...props} action={props.state.action} />;
}

function ActionFormDialog({ state, projectId, workspace, onClose, onSaved }: Props & { state: { type: 'create'; sourceReport?: ConstructionDailyReportSource } | { type: 'edit'; action: ConstructionActionItem } }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const action = state.type === 'edit' ? state.action : undefined;
  const sourceReport = state.type === 'create' ? state.sourceReport : undefined;
  const [values, setValues] = useState({ work_item_id: '', title: '', description: '', assigned_user_id: '', due_at: '', priority: 'normal', impact_scope: 'none' });
  const [sourceEvidenceId, setSourceEvidenceId] = useState('');
  const [errors, setErrors] = useState<FormErrors>({});
  const [uploads, setUploads] = useState<ConstructionEvidenceUploadDraft[]>([]);
  const [saving, setSaving] = useState(false);
	const formRef = useRef<HTMLDivElement | null>(null);
	const navigateToValidation = useValidationNavigation(formRef);
	  const availableWorkItems = workspace.work_items.filter((item) => item.report_eligible || item.id === action?.work_item_id || item.id === sourceReport?.work_item_id);

  useEffect(() => {
    setValues({ work_item_id: action?.work_item_id ?? sourceReport?.work_item_id ?? '', title: action?.title ?? sourceReport?.suggested_title ?? '', description: action?.description ?? '', assigned_user_id: action?.assigned_user_id ?? '', due_at: toDateTimeInput(action?.due_at), priority: action?.priority ?? 'normal', impact_scope: action?.impact_scope ?? 'none' });
    setSourceEvidenceId(sourceReport?.primary_evidence_id || sourceReport?.evidence[0]?.id || '');
    setErrors({}); setUploads([]); setSaving(false);
  }, [action, sourceReport]);

  async function save() {
    const fields = [{ name: 'work_item_id', label: 'Công việc liên quan' }, { name: 'title', label: 'Việc cần xử lý' }, { name: 'assigned_user_id', label: 'Người xử lý' }, { name: 'due_at', label: 'Hạn xử lý' }];
    const next = requiredErrors(values, fields);
    if (!action && !uploads.length && !sourceEvidenceId) next.evidence = 'Cần ít nhất một ảnh hoặc tệp hiện trạng';
    setErrors(next);
    if (Object.keys(next).length) {
      toast.warn({ title: 'Chưa đủ thông tin', description: errorSummary(next) });
      navigateToValidation({});
      return;
    }
    setSaving(true);
    try {
      if (action) {
        const result = await put(`/construction-actions/${action.id}`, { title: values.title.trim(), description: values.description.trim(), priority: values.priority, assigned_user_id: values.assigned_user_id, due_at: optionalISO(values.due_at) });
        syncWorkItemMutationTransition(queryClient, result);
        toast.success(`Đã cập nhật việc cần xử lý.${workItemAutoStartedNotice(result)}`);
      } else {
        const evidenceId = uploads.length ? await uploadConstructionEvidence(values.work_item_id, uploads) : sourceEvidenceId;
        const result = await createConstructionAction(projectId, { work_item_id: values.work_item_id, source_daily_report_id: sourceReport?.id ?? '', title: values.title.trim(), description: values.description.trim(), priority: values.priority as 'normal' | 'urgent', assigned_user_id: values.assigned_user_id, due_at: optionalISO(values.due_at), impact_scope: values.impact_scope as 'none' | 'timeline' | 'budget' | 'design_scope', evidence_id: evidenceId });
        syncWorkItemMutationTransition(queryClient, result);
        toast.success(`Đã tạo việc cần xử lý.${workItemAutoStartedNotice(result)}`);
      }
      await onSaved();
    } catch (error) {
      toast.error(action ? 'Không cập nhật được việc cần xử lý' : 'Không tạo được việc cần xử lý', error);
    } finally {
      setSaving(false);
    }
  }

  return <Dialog open title={action ? 'Chỉnh sửa việc cần xử lý' : 'Tạo việc cần xử lý'} description={action ? 'Nguồn phát sinh, công việc, ảnh hưởng và minh chứng được khóa để giữ lịch sử.' : sourceReport ? 'Việc này sẽ được truy vết trực tiếp về báo cáo hiện trường nguồn.' : 'Ghi rõ hiện trạng, giao đúng người và đặt hạn xử lý.'} size="xl" onClose={() => { if (!saving) onClose(); }} footer={<FormFooter saving={saving} submitLabel={action ? 'Lưu thay đổi' : 'Tạo việc'} onClose={onClose} onSave={save} />}>
    <fieldset disabled={saving} className="min-w-0"><div ref={formRef} className="space-y-5">
      {sourceReport ? <SourceReportBanner report={sourceReport} /> : null}
      <div><SectionTitle>Thông tin xử lý</SectionTitle><div className="mt-3 grid gap-4 sm:grid-cols-2">
	        <FieldBlock label="Công việc liên quan" required error={errors.work_item_id} helper={action ? 'Không thể đổi sau khi đã tạo.' : sourceReport?.work_item_id ? 'Được khóa theo báo cáo nguồn.' : 'Chỉ hiển thị hạng mục thuộc Node Thi công đang hoạt động.'}><select disabled={Boolean(action || sourceReport?.work_item_id)} className={controlClass(errors.work_item_id, Boolean(action || sourceReport?.work_item_id))} value={values.work_item_id} onChange={(event) => change(setValues, setErrors, 'work_item_id', event.target.value)}><option value="">Chọn công việc</option>{availableWorkItems.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></FieldBlock>
        <FieldBlock label="Việc cần xử lý" required error={errors.title}><input className={controlClass(errors.title)} value={values.title} onChange={(event) => change(setValues, setErrors, 'title', event.target.value)} /></FieldBlock>
        <FieldBlock className="sm:col-span-2" label="Mô tả hiện trạng"><textarea rows={3} className={controlClass()} value={values.description} onChange={(event) => change(setValues, setErrors, 'description', event.target.value)} /></FieldBlock>
        <FieldBlock label="Người xử lý" required error={errors.assigned_user_id}><select className={controlClass(errors.assigned_user_id)} value={values.assigned_user_id} onChange={(event) => change(setValues, setErrors, 'assigned_user_id', event.target.value)}><option value="">Chọn người phụ trách</option>{workspace.participants.map((item) => <option key={item.id} value={item.id}>{item.full_name}</option>)}</select></FieldBlock>
        <FieldBlock label="Hạn xử lý" required error={errors.due_at}><input type="datetime-local" className={controlClass(errors.due_at)} value={values.due_at} onChange={(event) => change(setValues, setErrors, 'due_at', event.target.value)} /></FieldBlock>
        <FieldBlock label="Mức ưu tiên"><select className={controlClass()} value={values.priority} onChange={(event) => change(setValues, setErrors, 'priority', event.target.value)}><option value="normal">Bình thường</option><option value="urgent">Gấp</option></select></FieldBlock>
        <FieldBlock label="Ảnh hưởng" helper={action ? 'Không thể đổi sau khi đã tạo.' : constructionImpactHelperText}><select disabled={Boolean(action)} className={controlClass(undefined, Boolean(action))} value={values.impact_scope} onChange={(event) => change(setValues, setErrors, 'impact_scope', event.target.value)}><option value="none">Không ảnh hưởng</option><option value="timeline">Tiến độ</option><option value="budget">Chi phí</option><option value="design_scope">Thiết kế / phạm vi</option></select></FieldBlock>
      </div></div>
      {action ? <div><SectionTitle>Thông tin khóa</SectionTitle><div className="mt-3 grid gap-3 rounded-lg border border-line bg-slate-50 p-4 sm:grid-cols-3"><Detail label="Nguồn" value={action.source_check_title ? `Kiểm tra: ${action.source_check_title}` : action.source_report_date ? `Báo cáo: ${dateText(action.source_report_date)}` : 'Ghi nhận hiện trường'} /><Detail label="Ảnh hưởng" value={impactLabel(action.impact_scope)} /><Detail label="Minh chứng" value={action.before_evidence_name || 'Đã lưu theo việc'} /></div></div> : <div className="space-y-4">{sourceReport?.evidence.length ? <FieldBlock label="Minh chứng hiện trạng từ báo cáo" helper="Có thể chọn một minh chứng đã có hoặc tải tệp mới ở bên dưới."><select className={controlClass()} value={sourceEvidenceId} onChange={(event) => { setSourceEvidenceId(event.target.value); setErrors((current) => ({ ...current, evidence: '' })); }}><option value="">Không dùng minh chứng báo cáo</option>{sourceReport.evidence.map((item) => <option key={item.id} value={item.id}>{item.name || item.caption || 'Minh chứng báo cáo'}</option>)}</select></FieldBlock> : null}<FieldBlock label={sourceEvidenceId ? 'Ảnh / tệp mới (tùy chọn)' : 'Ảnh / tệp hiện trạng'} required={!sourceEvidenceId} error={errors.evidence}><ConstructionEvidenceCapture maxFiles={1} uploadScope={`/work-items/${values.work_item_id}/evidence/upload`} drafts={uploads} onChange={(next) => { setUploads(next); setErrors((current) => ({ ...current, evidence: '' })); }} hasSource={Boolean(values.work_item_id)} disabled={saving} hint={sourceEvidenceId ? 'Nếu tải tệp mới, tệp đầu tiên sẽ thay minh chứng đã chọn từ báo cáo.' : 'Minh chứng hiện trạng được lưu theo công việc và không thay đổi trong form chỉnh sửa.'} /></FieldBlock></div>}
    </div></fieldset>
  </Dialog>;
}

function ActionViewDialog({ action, projectId, workspace, canEdit, canResolve, canVerify, canApproveImpact, canProposeVariation, variationContext, variationContextLoading, variationContextError, canViewContracts, onClose, onEdit, onResolve, onVerify, onApproveImpact, onRetryVariationContext, onProposeVariation, onViewCheck, onViewReport }: Props & { action: ConstructionActionItem }) {
  const sourceCheck = workspace.checks.find((item) => item.id === action.source_check_id);
  const editable = canEdit && isConstructionActionEditable(action);
  const needsImpactApproval = Boolean(action.technical_replanning_id && action.impact_status !== 'approved' && canApproveImpact);
  const proposalApplicable = canProposeVariation && action.impact_scope !== 'none' && !action.linked_variation_id;
  const canPropose = proposalApplicable && Boolean(variationContext?.available && variationContext.can_propose) && !variationContextLoading && !variationContextError;
  const variationStatus = action.linked_variation_id
    ? `${action.linked_variation_code || 'Đã liên kết'} · ${statusLabel(action.linked_variation_status || 'in_progress')}`
    : action.impact_scope === 'none'
      ? 'Không áp dụng'
      : variationContextLoading
        ? 'Đang kiểm tra điều kiện đề xuất'
        : variationContextError
          ? 'Chưa xác định'
          : variationContext?.available
            ? 'Chưa đề xuất'
            : 'Chưa thể đề xuất — thiếu hợp đồng chủ đầu tư đang hiệu lực';
  const [approving, setApproving] = useState(false);

  async function approve() {
    setApproving(true);
    try { await onApproveImpact(action); } finally { setApproving(false); }
  }

  return <Dialog open title={action.title} description={`${actionStatusLabel(action.status)} · ${action.work_item_title || 'Công việc liên quan'}`} size="xl" onClose={onClose} footer={<div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between"><button type="button" className="rounded-md border border-line px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-white" onClick={onClose}>Đóng</button><div className="flex flex-wrap justify-end gap-2">{canPropose ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-amber-300 bg-amber-50 px-4 py-2 text-sm font-semibold text-amber-800 hover:bg-amber-100" onClick={() => onProposeVariation(action)}><GitPullRequestArrow className="h-4 w-4" /> Đề xuất phát sinh</button> : null}{needsImpactApproval ? <button type="button" disabled={approving} className="inline-flex items-center gap-2 rounded-md border border-amber-300 bg-amber-50 px-4 py-2 text-sm font-semibold text-amber-800 hover:bg-amber-100 disabled:opacity-60" onClick={approve}>{approving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}Xác nhận ảnh hưởng</button> : null}{editable ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => onEdit(action)}><Pencil className="h-4 w-4" /> Chỉnh sửa</button> : null}{action.status === 'open' && canResolve ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => onResolve(action)}><Wrench className="h-4 w-4" /> Gửi khắc phục</button> : null}{action.status === 'waiting_confirmation' && canVerify ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700" onClick={() => onVerify(action)}><ShieldCheck className="h-4 w-4" /> Xác nhận</button> : null}</div></div>}>
    <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_19rem]">
      <div className="space-y-5">
        <div><SectionTitle>Mô tả hiện trạng</SectionTitle><p className="mt-3 whitespace-pre-wrap rounded-lg border border-line bg-slate-50 p-4 text-sm leading-6 text-slate-700">{action.description || 'Không có mô tả.'}</p></div>
        {action.source_daily_report_id ? <div><SectionTitle>Nguồn báo cáo</SectionTitle><button type="button" className="mt-3 flex w-full items-center justify-between rounded-lg border border-blue-200 bg-blue-50 p-4 text-left hover:bg-blue-100" onClick={() => onViewReport?.(action.source_daily_report_id!)}><span><span className="block font-semibold text-blue-800">Báo cáo ngày {dateText(action.source_report_date)}</span><span className="mt-1 block text-sm text-blue-700">{action.source_report_link_kind === 'check' ? 'Truy vết qua kiểm tra không đạt' : 'Liên kết trực tiếp từ báo cáo'}</span></span><span className="text-sm font-semibold text-blue-700">Xem báo cáo</span></button></div> : null}
        {sourceCheck ? <div><SectionTitle>Nguồn phát sinh</SectionTitle><button type="button" className="mt-3 flex w-full items-center justify-between rounded-lg border border-blue-200 bg-blue-50 p-4 text-left hover:bg-blue-100" onClick={() => onViewCheck(sourceCheck)}><span><span className="block font-semibold text-blue-800">{sourceCheck.title}</span><span className="mt-1 block text-sm text-blue-700">Kết quả: Không đạt · {dateTimeText(sourceCheck.checked_at)}</span></span><span className="text-sm font-semibold text-blue-700">Xem kiểm tra</span></button></div> : null}
        <div><SectionTitle>Minh chứng</SectionTitle><div className="mt-3 grid gap-3 sm:grid-cols-2"><ConstructionEvidenceButton evidenceId={action.before_evidence_id} name={action.before_evidence_name} label="Hiện trạng ban đầu" /><ConstructionEvidenceButton evidenceId={action.resolution_evidence_id} name={action.resolution_evidence_name} label="Kết quả khắc phục" /></div>{action.resolution_note ? <p className="mt-3 rounded-md bg-slate-50 px-3 py-2 text-sm text-slate-600">Ghi chú khắc phục: {action.resolution_note}</p> : null}</div>
        <div><SectionTitle>Lịch sử xử lý</SectionTitle><Timeline action={action} /></div>
      </div>
      <aside className="space-y-4 rounded-lg border border-line bg-slate-50 p-4 lg:self-start">
        <Detail label="Người xử lý" value={constructionActionAssigneeLabel(action)} />
        <Detail label="Hạn xử lý" value={dateTimeText(action.due_at)} />
        <Detail label="Mức ưu tiên" value={action.priority === 'urgent' ? 'Gấp' : 'Bình thường'} />
        <Detail label="Ảnh hưởng" value={impactLabel(action.impact_scope)} />
        <Detail label="Trạng thái ảnh hưởng" value={action.impact_scope === 'none' ? 'Không áp dụng' : action.impact_status === 'approved' ? 'Đã xác nhận' : 'Chờ xác nhận'} />
        <Detail label="Phát sinh thương mại" value={variationStatus} />
        {proposalApplicable ? <VariationReadinessNote projectId={projectId} loading={variationContextLoading} error={variationContextError} available={Boolean(variationContext?.available)} canViewContracts={canViewContracts} onRetry={onRetryVariationContext} /> : null}
        <Detail label="Số lần trả lại" value={String(action.retry_count ?? 0)} />
        <Detail label="Ngày tạo" value={dateTimeText(action.created_at)} />
        {!isConstructionActionEditable(action) && action.status === 'open' ? <p className="rounded-md border border-violet-200 bg-violet-50 px-3 py-2 text-xs font-medium text-violet-700">Đồng bộ từ hồ sơ kỹ thuật; chỉnh sửa tại nguồn QA/QC.</p> : null}
      </aside>
    </div>
  </Dialog>;
}

function SourceReportBanner({ report }: { report: ConstructionDailyReportSource }) {
  return <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-4"><span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-white text-blue-600"><CalendarDays className="h-5 w-5" /></span><div><p className="text-xs font-semibold uppercase tracking-wide text-blue-600">Nguồn đã liên kết</p><p className="mt-1 font-semibold text-blue-900">Báo cáo ngày {dateText(report.report_date)}</p><p className="mt-1 text-xs text-blue-700">{report.weather || 'Chưa ghi thời tiết'}{report.evidence.length ? ` · Có ${report.evidence.length} minh chứng có thể tái sử dụng` : ''}</p></div></div>;
}

function VariationReadinessNote({ projectId, loading, error, available, canViewContracts, onRetry }: { projectId: string; loading: boolean; error: boolean; available: boolean; canViewContracts: boolean; onRetry?: () => void }) {
  if (available) return null;
  if (loading) return <p className="rounded-md border border-slate-200 bg-white px-3 py-2 text-xs leading-5 text-slate-600">Đang kiểm tra điều kiện đề xuất…</p>;
  if (error) return <div className="rounded-md border border-slate-200 bg-white px-3 py-2 text-xs leading-5 text-slate-600"><p>Không tải được điều kiện đề xuất.</p>{onRetry ? <button type="button" className="mt-1 font-semibold text-blue-700 hover:text-blue-800" onClick={onRetry}>Thử lại</button> : null}</div>;
  return <div className="rounded-md border border-slate-200 bg-white px-3 py-2 text-xs leading-5 text-slate-600"><p>Cần PM/Thương mại hoàn tất hợp đồng trước khi gửi đề xuất.</p><p className="mt-1">Báo cáo, kiểm tra và xử lý hiện trường vẫn hoạt động bình thường.</p>{canViewContracts ? <Link className="mt-1 inline-flex font-semibold text-blue-700 hover:text-blue-800" to={`/projects/${projectId}/contracts`}>Mở hợp đồng</Link> : null}</div>;
}

function ResolutionDialog({ action, onClose, onSaved }: Props & { action: ConstructionActionItem }) {
  const toast = useToast();
  const [note, setNote] = useState('');
  const [uploads, setUploads] = useState<ConstructionEvidenceUploadDraft[]>([]);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);

  useEffect(() => { setNote(''); setUploads([]); setError(''); setSaving(false); }, [action]);

  async function save() {
    if (!uploads.length) {
      setError('Cần ít nhất một ảnh hoặc tệp minh chứng khắc phục');
      toast.warn({ title: 'Chưa đủ thông tin', description: 'Cần ít nhất một ảnh hoặc tệp minh chứng khắc phục' });
      navigateToValidation({});
      return;
    }
    setSaving(true);
    try {
      const evidenceId = await uploadConstructionEvidence(action.work_item_id, uploads);
      await post(`/construction-actions/${action.id}/submit-resolution`, { evidence_id: evidenceId, note: note.trim() });
      toast.success('Đã gửi kết quả khắc phục để xác nhận');
      await onSaved();
    } catch (saveError) {
      toast.error('Không gửi được kết quả khắc phục', saveError);
    } finally { setSaving(false); }
  }

  return <Dialog open title="Gửi kết quả khắc phục" description={action.title} size="lg" onClose={() => { if (!saving) onClose(); }} footer={<FormFooter saving={saving} submitLabel="Gửi xác nhận" onClose={onClose} onSave={save} />}>
    <fieldset disabled={saving} className="min-w-0"><div ref={formRef} className="space-y-4"><FieldBlock label="Ghi chú khắc phục"><textarea rows={4} className={controlClass()} value={note} onChange={(event) => setNote(event.target.value)} placeholder="Nêu việc đã làm và điểm cần người kiểm tra lưu ý" /></FieldBlock><FieldBlock label="Minh chứng sau khắc phục" required error={error}><ConstructionEvidenceCapture maxFiles={1} uploadScope={`/work-items/${action.work_item_id}/evidence/upload`} drafts={uploads} onChange={(next) => { setUploads(next); setError(''); }} hasSource disabled={saving} hint="Minh chứng này được dùng để người phụ trách xác nhận kết quả." /></FieldBlock></div></fieldset>
  </Dialog>;
}

function VerificationDialog({ action, onClose, onSaved }: Props & { action: ConstructionActionItem }) {
  const toast = useToast();
  const [values, setValues] = useState({ result: 'pass', comment: '' });
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);

  useEffect(() => { setValues({ result: 'pass', comment: '' }); setErrors({}); setSaving(false); }, [action]);

  async function save() {
    const next = values.result === 'fail' ? requiredErrors(values, [{ name: 'comment', label: 'Lý do chưa đạt' }]) : {};
    setErrors(next);
    if (Object.keys(next).length) {
      toast.warn({ title: 'Chưa đủ thông tin', description: errorSummary(next) });
      navigateToValidation({});
      return;
    }
    setSaving(true);
    try {
      await post(`/construction-actions/${action.id}/verify`, { result: values.result, comment: values.comment.trim() });
      toast.success(values.result === 'pass' ? 'Đã xác nhận việc xử lý đạt' : 'Đã trả lại để xử lý tiếp');
      await onSaved();
    } catch (error) {
      toast.error('Không xác nhận được kết quả', error);
    } finally { setSaving(false); }
  }

  return <Dialog open title="Xác nhận kết quả xử lý" description={action.title} size="lg" onClose={() => { if (!saving) onClose(); }} footer={<FormFooter saving={saving} submitLabel="Xác nhận" onClose={onClose} onSave={save} />}>
    <fieldset disabled={saving} className="min-w-0"><div ref={formRef} className="space-y-4"><fieldset><legend className="text-sm font-semibold text-slate-700">Kết luận <span className="text-rose-600">*</span></legend><div className="mt-2 grid grid-cols-2 gap-3"><button type="button" className={`rounded-lg border px-4 py-3 text-sm font-semibold ${values.result === 'pass' ? 'border-emerald-500 bg-emerald-50 text-emerald-700' : 'border-line text-slate-600'}`} onClick={() => change(setValues, setErrors, 'result', 'pass')}><CheckCircle2 className="mx-auto mb-1 h-5 w-5" />Đạt</button><button type="button" className={`rounded-lg border px-4 py-3 text-sm font-semibold ${values.result === 'fail' ? 'border-rose-500 bg-rose-50 text-rose-700' : 'border-line text-slate-600'}`} onClick={() => change(setValues, setErrors, 'result', 'fail')}><AlertTriangle className="mx-auto mb-1 h-5 w-5" />Chưa đạt</button></div></fieldset><FieldBlock label={values.result === 'fail' ? 'Lý do chưa đạt' : 'Nhận xét'} required={values.result === 'fail'} error={errors.comment}><textarea rows={4} className={controlClass(errors.comment)} value={values.comment} onChange={(event) => change(setValues, setErrors, 'comment', event.target.value)} placeholder={values.result === 'fail' ? 'Nêu rõ điểm cần khắc phục tiếp' : 'Nhận xét sau khi xác nhận'} /></FieldBlock></div></fieldset>
  </Dialog>;
}

function Timeline({ action }: { action: ConstructionActionItem }) {
  if (!action.events?.length) return <p className="mt-3 rounded-lg border border-dashed border-line p-4 text-sm text-slate-500">Chưa có sự kiện xử lý.</p>;
  return <ol className="mt-3 space-y-0">{action.events.map((event, index) => <li key={event.id} className="relative grid grid-cols-[1.25rem_minmax(0,1fr)] gap-3 pb-4 last:pb-0"><span className="relative z-10 mt-1 h-3 w-3 rounded-full bg-blue-600 ring-4 ring-blue-50" />{index < action.events.length - 1 ? <span className="absolute left-[5px] top-4 h-full w-px bg-slate-200" /> : null}<div><p className="text-sm font-semibold text-ink">{eventLabel(event.event_type)}</p><p className="mt-0.5 text-xs text-slate-500">{identityReferenceText(event.actor_name, event.actor_account_deleted, 'Hệ thống')} · {dateTimeText(event.created_at)}</p>{event.comment ? <p className="mt-1 whitespace-pre-wrap text-sm text-slate-600">{event.comment}</p> : null}{event.evidence_id ? <div className="mt-2"><ConstructionEvidenceButton evidenceId={event.evidence_id} name={event.evidence_name} label="Minh chứng sự kiện" /></div> : null}</div></li>)}</ol>;
}

function Detail({ label, value }: { label: string; value: string }) { return <div><p className="text-[11px] font-semibold uppercase tracking-wide text-slate-400">{label}</p><p className="mt-1 text-sm font-medium text-slate-700">{value}</p></div>; }

function FormFooter({ saving, submitLabel, onClose, onSave }: { saving: boolean; submitLabel: string; onClose: () => void; onSave: () => void }) {
  return <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><button type="button" disabled={saving} className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Hủy</button><button type="button" disabled={saving} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" onClick={onSave}>{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}{saving ? 'Đang lưu...' : submitLabel}</button></div>;
}

function change<T extends Record<string, string>>(setValues: Dispatch<SetStateAction<T>>, setErrors: Dispatch<SetStateAction<FormErrors>>, name: keyof T & string, value: string) {
  setValues((current) => ({ ...current, [name]: value }));
  setErrors((current) => ({ ...current, [name]: '' }));
}
