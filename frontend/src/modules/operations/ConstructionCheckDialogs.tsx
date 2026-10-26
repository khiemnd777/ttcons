import { constructionReportWeatherLabel } from './constructionWeatherPresentation';
import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CalendarDays, CheckCircle2, Loader2, Pencil, PlayCircle, Plus, Trash2 } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import { post, put } from '../../lib/api/client';
import type { ConstructionActionItem, ConstructionCheck, ConstructionDailyReportSource, ConstructionWorkspace } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { ConstructionEvidenceButton } from './ConstructionEvidenceButton';
import { FieldBlock, SectionTitle, controlClass, errorSummary, requiredErrors, type FormErrors } from './ConstructionFormControls';
import { createConstructionCheck, uploadConstructionEvidenceItems } from './constructionWorkspaceApi';
import { checkStatusLabel, constructionActionAssigneeLabel, dateTimeText, optionalISO, toDateTimeInput } from './constructionWorkspaceUtils';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { EvidenceGallery } from '../work-items/EvidenceGallery';
import { syncWorkItemMutationTransition, workItemAutoStartedNotice } from '../work-items/workItemMutationContext';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';

export type CheckDialogState =
  | { type: 'template' }
  | { type: 'create'; sourceReport?: ConstructionDailyReportSource }
  | { type: 'edit'; check: ConstructionCheck }
  | { type: 'view'; check: ConstructionCheck }
  | { type: 'complete'; check: ConstructionCheck }
  | null;

type Props = {
  state: CheckDialogState;
  projectId: string;
  workspace: ConstructionWorkspace;
  canEdit: boolean;
  canPerform: boolean;
  onClose: () => void;
  onSaved: () => Promise<void>;
  onEdit: (check: ConstructionCheck) => void;
  onComplete: (check: ConstructionCheck) => void;
  onViewAction: (action: ConstructionActionItem) => void;
  onViewReport?: (reportId: string) => void;
};

type EditableCriterion = { id: string; text: string };

export function ConstructionCheckDialogs(props: Props) {
  if (!props.state) return null;
  if (props.state.type === 'template') return <TemplateDialog {...props} />;
  if (props.state.type === 'create' || props.state.type === 'edit') return <CheckFormDialog {...props} state={props.state} />;
  if (props.state.type === 'complete') return <CompleteCheckDialog {...props} check={props.state.check} />;
  return <CheckViewDialog {...props} check={props.state.check} />;
}

function TemplateDialog({ projectId, onClose, onSaved }: Props) {
  const toast = useToast();
  const [values, setValues] = useState({ name: '', description: '', criteria: '' });
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);

  async function save() {
    const next = requiredErrors(values, [{ name: 'name', label: 'Tên mẫu' }, { name: 'criteria', label: 'Tiêu chí' }]);
    const criteria = values.criteria.split('\n').map((line) => line.trim()).filter(Boolean).map((title) => ({ title, criteria: title, evidence_required: true }));
    if (!criteria.length) next.criteria = 'Cần ít nhất một tiêu chí';
    setErrors(next);
    if (Object.keys(next).length) {
      toast.warn({ title: 'Chưa đủ thông tin', description: errorSummary(next) });
      navigateToValidation({});
      return;
    }
    setSaving(true);
    try {
      await post(`/projects/${projectId}/construction-check-templates`, { name: values.name.trim(), description: values.description.trim(), criteria });
      toast.success('Đã tạo mẫu kiểm tra');
      await onSaved();
    } catch (error) {
      toast.error('Không tạo được mẫu kiểm tra', error);
    } finally {
      setSaving(false);
    }
  }

  return <Dialog open title="Tạo mẫu kiểm tra" size="lg" onClose={() => { if (!saving) onClose(); }} footer={<FormFooter saving={saving} submitLabel="Tạo mẫu" onClose={onClose} onSave={save} />}>
    <fieldset disabled={saving} className="min-w-0"><div ref={formRef} className="grid gap-4 sm:grid-cols-2">
      <FieldBlock label="Tên mẫu" required error={errors.name}><input className={controlClass(errors.name)} value={values.name} onChange={(event) => change(setValues, setErrors, 'name', event.target.value)} autoFocus /></FieldBlock>
      <FieldBlock label="Mô tả ngắn"><input className={controlClass()} value={values.description} onChange={(event) => change(setValues, setErrors, 'description', event.target.value)} /></FieldBlock>
      <FieldBlock className="sm:col-span-2" label="Các tiêu chí" required error={errors.criteria} helper="Mỗi dòng tương ứng một tiêu chí; hệ thống sẽ lưu kèm yêu cầu minh chứng."><textarea rows={8} className={controlClass(errors.criteria)} value={values.criteria} onChange={(event) => change(setValues, setErrors, 'criteria', event.target.value)} placeholder={'Cao độ và độ phẳng — Sai số không quá 3mm\nBề mặt hoàn thiện — Không cong vênh, nứt hoặc hở mối nối'} /></FieldBlock>
    </div></fieldset>
  </Dialog>;
}

function CheckFormDialog({ state, projectId, workspace, onClose, onSaved }: Props & { state: { type: 'create'; sourceReport?: ConstructionDailyReportSource } | { type: 'edit'; check: ConstructionCheck } }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const check = state.type === 'edit' ? state.check : undefined;
  const sourceReport = state.type === 'create' ? state.sourceReport : undefined;
  const [values, setValues] = useState({ work_item_id: '', template_id: '', title: '', location: '', scheduled_at: '' });
  const [criteria, setCriteria] = useState<EditableCriterion[]>([]);
  const [existingContext, setExistingContext] = useState<ConstructionCheck['context_evidence']>([]);
  const [fallbackSourceEvidenceIDs, setFallbackSourceEvidenceIDs] = useState<string[]>([]);
  const [uploads, setUploads] = useState<ConstructionEvidenceUploadDraft[]>([]);
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);
	const formRef = useRef<HTMLDivElement | null>(null);
	const navigateToValidation = useValidationNavigation(formRef);
	  const availableWorkItems = workspace.work_items.filter((item) => item.report_eligible || item.id === check?.work_item_id || item.id === sourceReport?.work_item_id);

  useEffect(() => {
    setValues({
      work_item_id: check?.work_item_id ?? sourceReport?.work_item_id ?? '',
      template_id: check?.template_id ?? '',
      title: check?.title ?? sourceReport?.suggested_title ?? '',
      location: check?.location ?? '',
      scheduled_at: toDateTimeInput(check?.scheduled_at),
    });
    setCriteria(criteriaRows(check?.criteria_jsonb ?? [], !check));
    setExistingContext(check?.context_evidence ?? sourceReport?.evidence ?? []);
    setFallbackSourceEvidenceIDs(!check && sourceReport?.primary_evidence_id && !sourceReport.evidence.length ? [sourceReport.primary_evidence_id] : []);
    setUploads([]);
    setErrors({});
    setSaving(false);
  }, [check, sourceReport]);

  function changeWorkItem(workItemID: string) {
    if (values.work_item_id && values.work_item_id !== workItemID && (existingContext.length || fallbackSourceEvidenceIDs.length)) {
      setExistingContext([]);
      setFallbackSourceEvidenceIDs([]);
      toast.warn({ title: 'Đã gỡ ảnh hiện trạng cũ', description: 'Ảnh hiện trạng phải thuộc cùng công việc; hãy chọn hoặc chụp lại ảnh phù hợp.' });
    }
    change(setValues, setErrors, 'work_item_id', workItemID);
  }

  function loadTemplate(templateID: string) {
    const template = workspace.templates.find((item) => item.id === templateID);
    setValues((current) => ({
      ...current,
      template_id: templateID,
      title: state.type === 'create' && !current.title.trim() && template ? template.name : current.title,
    }));
    if (template) setCriteria(criteriaRows(template.criteria_jsonb, false));
  }

  function addCriterion() {
    if (criteria.length >= 30) {
      toast.warn({ title: 'Đã đạt giới hạn', description: 'Mỗi kiểm tra có tối đa 30 tiêu chí.' });
      return;
    }
    setCriteria((current) => [...current, editableCriterion()]);
  }

  async function save() {
    const next = requiredErrors(values, [{ name: 'work_item_id', label: 'Công việc liên quan' }, { name: 'title', label: 'Nội dung kiểm tra' }]);
    setErrors(next);
    if (Object.keys(next).length) {
      toast.warn({ title: 'Chưa đủ thông tin', description: errorSummary(next) });
      navigateToValidation({});
      return;
    }
    setSaving(true);
    try {
      const newEvidenceIDs = await uploadConstructionEvidenceItems(values.work_item_id, uploads);
      const contextEvidenceIDs = [...new Set([...existingContext.map((item) => item.id), ...fallbackSourceEvidenceIDs, ...newEvidenceIDs])];
      const body = {
        work_item_id: values.work_item_id,
        source_daily_report_id: sourceReport?.id ?? '',
        template_id: values.template_id,
        title: values.title.trim(),
        location: values.location.trim(),
        scheduled_at: optionalISO(values.scheduled_at),
        criteria: criteria.map((item) => item.text.trim()).filter(Boolean).map((text) => ({ title: text, criteria: text, evidence_required: true })),
        context_evidence_ids: contextEvidenceIDs,
      };
      const result = check ? await put(`/construction-checks/${check.id}`, body) : await createConstructionCheck(projectId, body);
      syncWorkItemMutationTransition(queryClient, result);
      toast.success(`${check ? 'Đã cập nhật kiểm tra' : 'Đã tạo kiểm tra'}.${workItemAutoStartedNotice(result)}`);
      await onSaved();
    } catch (error) {
      toast.error(check ? 'Không cập nhật được kiểm tra' : 'Không tạo được kiểm tra', error);
    } finally {
      setSaving(false);
    }
  }

  return <Dialog open title={check ? 'Chỉnh sửa kiểm tra' : 'Tạo kiểm tra'} description={check ? 'Chỉ kiểm tra đang chờ và được tạo tại màn hình này mới có thể chỉnh sửa.' : sourceReport ? 'Kiểm tra này sẽ được truy vết trực tiếp về báo cáo hiện trường nguồn.' : undefined} size="xl" onClose={() => { if (!saving) onClose(); }} footer={<FormFooter saving={saving} submitLabel={check ? 'Lưu thay đổi' : 'Tạo kiểm tra'} onClose={onClose} onSave={save} />}>
    <fieldset disabled={saving} className="min-w-0"><div ref={formRef} className="space-y-5">
      {sourceReport ? <SourceReportBanner report={sourceReport} /> : null}
      <div><SectionTitle>Thông tin kiểm tra</SectionTitle><div className="mt-3 grid gap-4 sm:grid-cols-2">
	        <FieldBlock label="Công việc liên quan" required error={errors.work_item_id} helper={sourceReport?.work_item_id ? 'Được khóa theo báo cáo nguồn.' : 'Chỉ hiển thị hạng mục thuộc Node Thi công đang hoạt động.'}><select disabled={Boolean(sourceReport?.work_item_id)} className={controlClass(errors.work_item_id, Boolean(sourceReport?.work_item_id))} value={values.work_item_id} onChange={(event) => changeWorkItem(event.target.value)}><option value="">Chọn công việc</option>{availableWorkItems.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></FieldBlock>
        <FieldBlock label="Nội dung kiểm tra" required error={errors.title}><input className={controlClass(errors.title)} value={values.title} onChange={(event) => change(setValues, setErrors, 'title', event.target.value)} /></FieldBlock>
        <FieldBlock label="Vị trí"><input className={controlClass()} value={values.location} onChange={(event) => change(setValues, setErrors, 'location', event.target.value)} placeholder="Ví dụ: Pantry tầng 12" /></FieldBlock>
        <FieldBlock label="Thời điểm dự kiến"><input type="datetime-local" className={controlClass()} value={values.scheduled_at} onChange={(event) => change(setValues, setErrors, 'scheduled_at', event.target.value)} /></FieldBlock>
      </div></div>
      <div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between"><div><SectionTitle>Tiêu chí áp dụng</SectionTitle></div><label className="w-full sm:w-64"><span className="sr-only">Nạp tiêu chí từ mẫu</span><select className={controlClass()} value={values.template_id} onChange={(event) => loadTemplate(event.target.value)}><option value="">Nạp từ mẫu (không bắt buộc)</option>{workspace.templates.filter((item) => item.status === 'active' || item.id === values.template_id).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label></div>
        <div className="mt-3 space-y-2">{criteria.map((item, index) => <div key={item.id} className="flex items-center gap-2"><span className="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-blue-50 text-xs font-bold text-blue-700">{index + 1}</span><input className={controlClass()} value={item.text} onChange={(event) => setCriteria((current) => current.map((row) => row.id === item.id ? { ...row, text: event.target.value } : row))} placeholder="Ví dụ: Cao độ và độ phẳng, sai số không quá 3mm" /><button type="button" className="rounded-md p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600" onClick={() => setCriteria((current) => current.filter((row) => row.id !== item.id))} aria-label={`Xóa tiêu chí ${index + 1}`}><Trash2 className="h-4 w-4" /></button></div>)}</div>
        <button type="button" className="mt-3 inline-flex items-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={addCriterion}><Plus className="h-4 w-4" /> Thêm tiêu chí</button>
      </div>
      <div><SectionTitle>Hình ảnh hiện trạng</SectionTitle><p className="mt-1 text-xs text-slate-500">Không bắt buộc. Ảnh được lưu theo công việc và giúp người kiểm tra hiểu đúng bối cảnh.</p>{existingContext.length ? <ExistingContextEvidence evidence={existingContext} onRemove={(id) => setExistingContext((current) => current.filter((item) => item.id !== id))} /> : null}{fallbackSourceEvidenceIDs.length ? <div className="mt-3 flex items-center justify-between rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700"><span>Đã dùng minh chứng chính từ báo cáo nguồn.</span><button type="button" className="font-semibold hover:underline" onClick={() => setFallbackSourceEvidenceIDs([])}>Gỡ</button></div> : null}<div className="mt-3"><ConstructionEvidenceCapture uploadScope={`/work-items/${values.work_item_id}/evidence/upload`} drafts={uploads} onChange={setUploads} hasSource={Boolean(values.work_item_id)} disabled={saving || existingContext.length + fallbackSourceEvidenceIDs.length >= 10} maxFiles={10 - existingContext.length - fallbackSourceEvidenceIDs.length} hint="Chụp hoặc chọn tối đa 10 ảnh/tệp hiện trạng; có thể bổ sung hoặc gỡ trước khi lưu." /></div></div>
    </div></fieldset>
  </Dialog>;
}

function CheckViewDialog({ check, workspace, canEdit, canPerform, onClose, onEdit, onComplete, onViewAction, onViewReport }: Props & { check: ConstructionCheck }) {
  const linkedAction = workspace.actions.find((item) => item.source_check_id === check.id);
  const canAct = check.status === 'pending' && canPerform;
  const canChange = check.status === 'pending' && !check.technical_rfi_id && !check.source_handoff_item_id && canEdit;
  return <Dialog open title={check.title} description={`${checkStatusLabel(check)} · ${check.work_item_title || 'Công việc liên quan'}`} size="xl" onClose={onClose} footer={<div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between"><button type="button" className="rounded-md border border-line px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-white" onClick={onClose}>Đóng</button><div className="flex justify-end gap-2">{canChange ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => onEdit(check)}><Pencil className="h-4 w-4" /> Chỉnh sửa</button> : null}{canAct ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => onComplete(check)}><PlayCircle className="h-4 w-4" /> Ghi kết quả</button> : null}</div></div>}>
    <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_19rem]">
      <div className="space-y-5">
        {check.source_daily_report_id ? <div><SectionTitle>Nguồn báo cáo</SectionTitle><button type="button" className="mt-3 flex w-full items-center justify-between rounded-lg border border-blue-200 bg-blue-50 p-4 text-left hover:bg-blue-100" onClick={() => onViewReport?.(check.source_daily_report_id!)}><span><span className="block font-semibold text-blue-800">Báo cáo ngày {dateText(check.source_report_date)}</span><span className="mt-1 block text-sm text-blue-700">{constructionReportWeatherLabel(check.source_report_weather)} · Liên kết trực tiếp</span></span><span className="text-sm font-semibold text-blue-700">Xem báo cáo</span></button></div> : null}
        <div><SectionTitle>Tiêu chí kiểm tra</SectionTitle><CriteriaList criteria={check.criteria_jsonb} emptyText="Kiểm tra này không có tiêu chí định sẵn." /></div>
        <div><SectionTitle>Kết quả và ghi chú</SectionTitle><div className="mt-3 rounded-lg border border-line bg-slate-50 p-4"><p className="font-semibold text-ink">{checkStatusLabel(check)}</p><p className="mt-1 whitespace-pre-wrap text-sm text-slate-600">{check.note || (check.status === 'pending' ? 'Chưa ghi kết quả.' : 'Không có ghi chú.')}</p></div></div>
        <div><SectionTitle>Hình ảnh hiện trạng</SectionTitle><div className="mt-3"><EvidenceGallery evidence={check.context_evidence ?? []} emptyTitle="Chưa có ảnh hiện trạng" showControls={false} /></div></div>
        <div><SectionTitle>Minh chứng kết quả</SectionTitle><div className="mt-3">{check.result_evidence?.length ? <EvidenceGallery evidence={check.result_evidence} emptyTitle="Chưa có minh chứng kết quả" showControls={false} /> : check.status === 'completed' ? <ConstructionEvidenceButton evidenceId={check.evidence_id} name={check.evidence_name} label="Minh chứng kết quả" /> : <EvidenceGallery evidence={[]} emptyTitle="Chưa có minh chứng kết quả" showControls={false} />}</div></div>
        {linkedAction ? <div><SectionTitle>Việc phát sinh</SectionTitle><button type="button" className="mt-3 flex w-full items-center justify-between rounded-lg border border-rose-200 bg-rose-50 p-4 text-left hover:bg-rose-100" onClick={() => onViewAction(linkedAction)}><span><span className="block font-semibold text-rose-800">{linkedAction.title}</span><span className="mt-1 block text-sm text-rose-700">{constructionActionAssigneeLabel(linkedAction, 'Chưa giao người xử lý')} · {dateTimeText(linkedAction.due_at)}</span></span><span className="text-sm font-semibold text-rose-700">Xem việc</span></button></div> : null}
      </div>
      <aside className="space-y-4 rounded-lg border border-line bg-slate-50 p-4 lg:self-start">
        {check.source_handoff_item_id ? <div className="rounded-md border border-blue-200 bg-blue-50 px-3 py-2"><p className="text-xs font-semibold text-blue-800">Từ kế hoạch thi công · revision {check.handoff_revision_no ?? 1}</p><p className="mt-1 text-xs text-blue-700">{check.handoff_title || check.handoff_source_node_name || 'Bàn giao kế hoạch'}</p></div> : null}
        <Detail label="Công việc" value={check.work_item_title || '—'} />
        <Detail label="Vị trí" value={check.location || 'Chưa ghi'} />
        <Detail label="Mẫu" value={check.template_name || 'Không dùng mẫu'} />
        <Detail label="Lịch kiểm tra" value={dateTimeText(check.scheduled_at)} />
        <Detail label="Người yêu cầu" value={identityReferenceText(check.requested_by_name, check.requested_by_account_deleted)} />
        <Detail label="Người kiểm tra" value={identityReferenceText(check.checked_by_name, check.checked_by_account_deleted, 'Chưa thực hiện')} />
        <Detail label="Thời điểm kết quả" value={dateTimeText(check.checked_at)} />
        {check.technical_rfi_id ? <p className="rounded-md border border-violet-200 bg-violet-50 px-3 py-2 text-xs font-medium text-violet-700">Đồng bộ từ RFI QA/QC; chỉnh sửa tại hồ sơ nguồn.</p> : null}
        {check.source_handoff_item_id ? <p className="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs font-medium text-blue-700">Kiểm tra được sinh từ snapshot Planning; thay đổi bằng revision bàn giao mới.</p> : null}
      </aside>
    </div>
  </Dialog>;
}

function SourceReportBanner({ report }: { report: ConstructionDailyReportSource }) {
  return <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-4"><span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-white text-blue-600"><CalendarDays className="h-5 w-5" /></span><div><p className="text-xs font-semibold uppercase tracking-wide text-blue-600">Nguồn đã liên kết</p><p className="mt-1 font-semibold text-blue-900">Báo cáo ngày {dateText(report.report_date)}</p><p className="mt-1 text-xs text-blue-700">{constructionReportWeatherLabel(report.weather)}{report.evidence.length ? ` · Dùng lại ${report.evidence.length} minh chứng hiện trạng` : ''}</p></div></div>;
}

function CompleteCheckDialog({ check, workspace, onClose, onSaved }: Props & { check: ConstructionCheck }) {
  const toast = useToast();
  const [values, setValues] = useState({ result: 'pass', note: '', action_title: `Khắc phục: ${check.title}`, assigned_user_id: '', due_at: '', priority: 'normal', impact_scope: 'none' });
  const [errors, setErrors] = useState<FormErrors>({});
  const [uploads, setUploads] = useState<ConstructionEvidenceUploadDraft[]>([]);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);
  const failed = values.result === 'fail';

  useEffect(() => {
    setValues({ result: 'pass', note: '', action_title: `Khắc phục: ${check.title}`, assigned_user_id: '', due_at: '', priority: 'normal', impact_scope: 'none' });
    setErrors({}); setUploads([]); setSaving(false);
  }, [check]);

  async function save() {
    const fields = failed ? [{ name: 'result', label: 'Kết quả' }, { name: 'action_title', label: 'Việc cần xử lý' }, { name: 'assigned_user_id', label: 'Người xử lý' }, { name: 'due_at', label: 'Hạn xử lý' }] : [{ name: 'result', label: 'Kết quả' }];
    const next = requiredErrors(values, fields);
    if (!uploads.length) next.evidence = 'Cần ít nhất một ảnh hoặc tệp minh chứng';
    setErrors(next);
    if (Object.keys(next).length) {
      toast.warn({ title: 'Chưa đủ thông tin', description: errorSummary(next) });
      navigateToValidation({});
      return;
    }
    setSaving(true);
    try {
      const evidenceIDs = await uploadConstructionEvidenceItems(check.work_item_id, uploads);
      await post(`/construction-checks/${check.id}/complete`, { result: values.result, note: values.note.trim(), evidence_id: evidenceIDs[0], evidence_ids: evidenceIDs, action_title: failed ? values.action_title.trim() : '', assigned_user_id: failed ? values.assigned_user_id : '', due_at: failed ? optionalISO(values.due_at) : '', impact_scope: failed ? values.impact_scope : 'none', priority: failed ? values.priority : 'normal' });
      toast.success(failed ? 'Đã ghi nhận không đạt và tạo việc xử lý' : 'Đã ghi nhận kiểm tra đạt');
      await onSaved();
    } catch (error) {
      toast.error('Không lưu được kết quả kiểm tra', error);
    } finally {
      setSaving(false);
    }
  }

  return <Dialog open title="Ghi kết quả kiểm tra" description={`${check.title} · ${check.work_item_title || 'Công việc liên quan'}`} size="xl" onClose={() => { if (!saving) onClose(); }} footer={<FormFooter saving={saving} submitLabel="Lưu kết quả" onClose={onClose} onSave={save} />}>
    <fieldset disabled={saving} className="min-w-0"><div ref={formRef} className="space-y-5">
      <ResultChoice value={values.result} onChange={(result) => change(setValues, setErrors, 'result', result)} />
      <FieldBlock label="Ghi chú kết quả"><textarea rows={3} className={controlClass()} value={values.note} onChange={(event) => change(setValues, setErrors, 'note', event.target.value)} placeholder="Mô tả quan sát hoặc điểm cần lưu ý" /></FieldBlock>
      {failed ? <div><SectionTitle>Việc cần xử lý được tạo tự động</SectionTitle><div className="mt-3 grid gap-4 sm:grid-cols-2">
        <FieldBlock label="Việc cần xử lý" required error={errors.action_title}><input className={controlClass(errors.action_title)} value={values.action_title} onChange={(event) => change(setValues, setErrors, 'action_title', event.target.value)} /></FieldBlock>
        <FieldBlock label="Người xử lý" required error={errors.assigned_user_id}><select className={controlClass(errors.assigned_user_id)} value={values.assigned_user_id} onChange={(event) => change(setValues, setErrors, 'assigned_user_id', event.target.value)}><option value="">Chọn người phụ trách</option>{workspace.participants.map((item) => <option key={item.id} value={item.id}>{item.full_name}</option>)}</select></FieldBlock>
        <FieldBlock label="Hạn xử lý" required error={errors.due_at}><input type="datetime-local" className={controlClass(errors.due_at)} value={values.due_at} onChange={(event) => change(setValues, setErrors, 'due_at', event.target.value)} /></FieldBlock>
        <FieldBlock label="Mức ưu tiên"><select className={controlClass()} value={values.priority} onChange={(event) => change(setValues, setErrors, 'priority', event.target.value)}><option value="normal">Bình thường</option><option value="urgent">Gấp</option></select></FieldBlock>
        <FieldBlock label="Ảnh hưởng"><select className={controlClass()} value={values.impact_scope} onChange={(event) => change(setValues, setErrors, 'impact_scope', event.target.value)}><option value="none">Không ảnh hưởng</option><option value="timeline">Tiến độ</option><option value="budget">Chi phí</option><option value="design_scope">Thiết kế / phạm vi</option></select></FieldBlock>
      </div></div> : null}
      <FieldBlock label="Minh chứng kết quả" required error={errors.evidence}><ConstructionEvidenceCapture uploadScope={`/work-items/${check.work_item_id}/evidence/upload`} drafts={uploads} onChange={(next) => { setUploads(next); setErrors((current) => ({ ...current, evidence: '' })); }} hasSource disabled={saving} maxFiles={10} hint="Chọn từ 1 đến 10 ảnh hoặc tệp; tất cả sẽ được lưu theo công việc và gắn vào kết quả kiểm tra." /></FieldBlock>
    </div></fieldset>
  </Dialog>;
}

function ResultChoice({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return <fieldset><legend className="text-sm font-semibold text-slate-700">Kết quả <span className="text-rose-600">*</span></legend><div className="mt-2 grid grid-cols-2 gap-3"><button type="button" className={`rounded-lg border px-4 py-3 text-sm font-semibold ${value === 'pass' ? 'border-emerald-500 bg-emerald-50 text-emerald-700' : 'border-line text-slate-600'}`} onClick={() => onChange('pass')}><CheckCircle2 className="mx-auto mb-1 h-5 w-5" />Đạt</button><button type="button" className={`rounded-lg border px-4 py-3 text-sm font-semibold ${value === 'fail' ? 'border-rose-500 bg-rose-50 text-rose-700' : 'border-line text-slate-600'}`} onClick={() => onChange('fail')}><AlertTriangle className="mx-auto mb-1 h-5 w-5" />Không đạt</button></div></fieldset>;
}

function ExistingContextEvidence({ evidence, onRemove }: { evidence: ConstructionCheck['context_evidence']; onRemove: (id: string) => void }) {
  return <div className="mt-3 grid gap-2 sm:grid-cols-2">{evidence.map((item, index) => <div key={item.id} className="relative pr-10"><ConstructionEvidenceButton record={item} evidenceId={item.id} name={item.name} label={`Ảnh hiện trạng ${index + 1}`} /><button type="button" className="absolute right-2 top-2 rounded-md bg-white p-1.5 text-slate-400 shadow-sm ring-1 ring-inset ring-line hover:bg-rose-50 hover:text-rose-600" onClick={() => onRemove(item.id)} aria-label={`Gỡ ${item.name || `ảnh hiện trạng ${index + 1}`}`}><Trash2 className="h-4 w-4" /></button></div>)}</div>;
}

function criteriaRows(items: ConstructionCheck['criteria_jsonb'], includeEmpty: boolean) {
  const rows = items.map((item) => editableCriterion(item.criteria && item.criteria !== item.title ? `${item.title} — ${item.criteria}` : item.title));
  return rows.length || !includeEmpty ? rows : [editableCriterion()];
}

function editableCriterion(text = ''): EditableCriterion {
  return { id: `criterion-${Date.now()}-${Math.random().toString(16).slice(2)}`, text };
}

function CriteriaList({ criteria, emptyText }: { criteria: ConstructionCheck['criteria_jsonb']; emptyText: string }) {
  if (!criteria.length) return <p className="mt-3 rounded-lg border border-dashed border-line px-4 py-5 text-sm text-slate-500">{emptyText}</p>;
  return <ol className="mt-3 divide-y divide-line overflow-hidden rounded-lg border border-line">{criteria.map((item, index) => <li key={`${item.title}-${index}`} className="flex gap-3 px-4 py-3"><span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-blue-50 text-xs font-bold text-blue-700">{index + 1}</span><span className="text-sm text-slate-700"><strong className="font-semibold text-ink">{item.title}</strong>{item.criteria && item.criteria !== item.title ? <span className="mt-0.5 block">{item.criteria}</span> : null}{item.evidence_required ? <span className="mt-1 block text-xs text-blue-700">Yêu cầu minh chứng</span> : null}</span></li>)}</ol>;
}

function Detail({ label, value }: { label: string; value: string }) { return <div><p className="text-[11px] font-semibold uppercase tracking-wide text-slate-400">{label}</p><p className="mt-1 text-sm font-medium text-slate-700">{value}</p></div>; }

function FormFooter({ saving, submitLabel, onClose, onSave }: { saving: boolean; submitLabel: string; onClose: () => void; onSave: () => void }) {
  return <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><button type="button" disabled={saving} className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Hủy</button><button type="button" disabled={saving} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" onClick={onSave}>{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}{saving ? 'Đang lưu...' : submitLabel}</button></div>;
}

function change<T extends Record<string, string>>(setValues: Dispatch<SetStateAction<T>>, setErrors: Dispatch<SetStateAction<FormErrors>>, name: keyof T & string, value: string) {
  setValues((current) => ({ ...current, [name]: value }));
  setErrors((current) => ({ ...current, [name]: '' }));
}
