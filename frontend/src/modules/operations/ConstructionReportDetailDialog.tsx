import { useEffect, useMemo, useState, type Dispatch, type SetStateAction } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  CalendarDays,
  CheckCircle2,
  ClipboardList,
  CloudSun,
  ExternalLink,
  FileText,
  Hammer,
  Link2,
  Package,
  Pencil,
  RefreshCw,
  Send,
  Upload,
  Users,
  Wrench,
} from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get } from '../../lib/api/client';
import type { ConstructionActionItem, ConstructionCheck, ConstructionWorkspace, EvidenceRecord, ProjectEvidencePage, ProjectOperationsResponse, WorkItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { EvidenceGallery } from '../work-items/EvidenceGallery';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import { ConstructionReportLinkedComposer, type ReportLinkedDraft } from './ConstructionReportLinkedComposer';
import { createConstructionAction, createConstructionCheck, uploadConstructionEvidenceItems } from './constructionWorkspaceApi';
import { constructionReportViewModel } from './constructionReportViewModel';
import { actionStatusLabel, checkStatusLabel, constructionActionAssigneeLabel, dateTimeText, optionalISO } from './constructionWorkspaceUtils';
import { useWorkReportRecipients } from './useWorkReportRecipients';

type ConstructionReportDetailDialogProps = {
  open: boolean;
  projectId: string;
  record: Record<string, any> | null;
  canManage?: boolean;
  canViewLinked?: boolean;
  canCreateCheck?: boolean;
  canCreateAction?: boolean;
  initialLinkedDrafts?: ReportLinkedDraft[];
  linkedNotice?: string;
  onEdit?: () => void;
  onAddEvidence?: () => void;
  onOpenCheck?: (check: ConstructionCheck) => void;
  onOpenAction?: (action: ConstructionActionItem) => void;
  canDeliver?: boolean;
  deliveryPending?: boolean;
  onDeliver?: (recipientId: string, recipientName: string) => Promise<unknown>;
  onClose: () => void;
};

export function ConstructionReportDetailDialog({ open, projectId, record, canManage = false, canViewLinked = false, canCreateCheck = false, canCreateAction = false, initialLinkedDrafts, linkedNotice, onEdit, onAddEvidence, onOpenCheck, onOpenAction, canDeliver = false, deliveryPending = false, onDeliver, onClose }: ConstructionReportDetailDialogProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [linkedDrafts, setLinkedDrafts] = useState<ReportLinkedDraft[]>([]);
  const report = useMemo(() => constructionReportViewModel(record ?? {}), [record]);
  const evidenceKey = report.evidenceIds.join(',');
  const sourceExists = Boolean(report.workItemId || report.workflowFormInstanceId);
  const manualReport = !report.workflowFormInstanceId;
  const deliveryStatus = String(record?.delivery_status ?? '');
  const sourceChangedAfterReview = record?.delivery_source_changed === true;
  const needsSourceChange = deliveryStatus === 'needs_changes' && !sourceChangedAfterReview;
  const deliveryCandidate = record?.report_delivery_eligible !== false
    && canDeliver
    && Boolean(onDeliver)
    && (deliveryStatus === '' || deliveryStatus === 'draft' || (deliveryStatus === 'needs_changes' && sourceChangedAfterReview));
  const recipients = useWorkReportRecipients(projectId, open && deliveryCandidate);
  const [recipientId, setRecipientId] = useState('');
  const selectedRecipient = recipients.data?.find((recipient) => recipient.id === recipientId);
  const deliveryRecipientLabel = selectedRecipient?.recommendation_reason === 'project_pm'
    ? `Quản lý dự án · ${selectedRecipient.full_name}`
    : selectedRecipient?.full_name ?? 'người quản lý';
  const deliveryButtonLabel = deliveryStatus === 'needs_changes'
    ? `Gửi lại cho ${deliveryRecipientLabel}`
    : `Gửi cho ${deliveryRecipientLabel}`;

  useEffect(() => {
    if (!open) return;
    setLinkedDrafts(initialLinkedDrafts ?? []);
  }, [initialLinkedDrafts, open, record?.id]);

  useEffect(() => {
    if (!open || !deliveryCandidate) {
      setRecipientId('');
      return;
    }
    setRecipientId((current) => {
      if (current && recipients.data?.some((recipient) => recipient.id === current)) return current;
      return recipients.recommendedRecipient?.id ?? '';
    });
  }, [deliveryCandidate, open, record?.id, recipients.data, recipients.recommendedRecipient?.id]);

  const operations = useQuery({
    queryKey: ['project-operations', projectId, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${projectId}/operations`),
    enabled: open && Boolean(projectId) && Boolean(report.workItemId),
    retry: false,
  });
  const workItem = useMemo(
    () => operations.data?.work_items?.find((item) => item.id === report.workItemId),
    [operations.data?.work_items, report.workItemId],
  );

  const evidence = useQuery({
    queryKey: ['construction-report-evidence', projectId, record?.id, evidenceKey],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (report.workItemId) params.set('work_item_id', report.workItemId);
      params.set('limit', '50');
      const page = await get<ProjectEvidencePage>(`/projects/${projectId}/evidence?${params.toString()}`);
      const requestedIds = new Set(report.evidenceIds);
      return page.items.filter((item) => requestedIds.has(item.id));
    },
    enabled: open && Boolean(projectId) && report.evidenceIds.length > 0,
    retry: false,
  });
  const workspace = useQuery({
    queryKey: ['construction-workspace', projectId],
    queryFn: () => get<ConstructionWorkspace>(`/projects/${projectId}/construction-workspace`),
    enabled: open && Boolean(projectId) && canViewLinked,
    retry: false,
  });
  const relatedChecks = useMemo(
    () => (workspace.data?.checks ?? []).filter((item) => item.source_daily_report_id === record?.id),
    [record?.id, workspace.data?.checks],
  );
  const relatedActions = useMemo(
    () => (workspace.data?.actions ?? []).filter((item) => item.source_daily_report_id === record?.id),
    [record?.id, workspace.data?.actions],
  );
  async function createLinkedDraft(draft: ReportLinkedDraft) {
    const workItemId = report.workItemId || draft.work_item_id;
    try {
      const uploadedEvidenceIDs = draft.uploads.length ? await uploadConstructionEvidenceItems(workItemId, draft.uploads) : [];
      const reportEvidenceIDs = report.evidenceIds;
      if (draft.kind === 'check') {
        await createConstructionCheck(projectId, {
          work_item_id: workItemId,
          source_daily_report_id: String(record?.id ?? ''),
          title: draft.title.trim(),
          location: draft.location.trim(),
          scheduled_at: optionalISO(draft.scheduled_at),
          criteria: draft.criteria_text.split('\n').map((line) => line.trim()).filter(Boolean).map((title) => ({ title, criteria: title, evidence_required: true })),
          context_evidence_ids: [...new Set([...reportEvidenceIDs, ...uploadedEvidenceIDs])],
        });
        toast.success('Đã tạo kiểm tra ngay trong báo cáo');
      } else {
        const evidenceId = uploadedEvidenceIDs[0] || draft.evidence_id || String(record?.primary_evidence_id ?? '') || reportEvidenceIDs[0] || '';
        if (!evidenceId) throw new Error('Việc xử lý chưa có minh chứng hiện trạng');
        await createConstructionAction(projectId, {
          work_item_id: workItemId,
          source_daily_report_id: String(record?.id ?? ''),
          title: draft.title.trim(),
          description: draft.description.trim(),
          priority: draft.priority,
          assigned_user_id: draft.assigned_user_id,
          due_at: optionalISO(draft.due_at),
          impact_scope: draft.impact_scope,
          evidence_id: evidenceId,
        });
        toast.success('Đã tạo việc xử lý ngay trong báo cáo');
      }
      await queryClient.invalidateQueries({ queryKey: ['construction-workspace', projectId] });
      await workspace.refetch();
    } catch (error) {
      toast.error(draft.kind === 'check' ? 'Không tạo được kiểm tra' : 'Không tạo được việc xử lý', error);
      throw error;
    }
  }

  function openSource() {
    if (!report.workItemId) return;
    onClose();
    navigate(workItemDetailPath(report.workItemId, currentReturnTo(location), projectId));
  }

  async function deliverReport() {
    if (!onDeliver || !selectedRecipient) return;
    try {
      await onDeliver(selectedRecipient.id, selectedRecipient.full_name);
    } catch {
      // The mutation owner displays the API error and refreshes stale delivery state.
    }
  }

  return (
    <Dialog
      open={open}
      title="Chi tiết báo cáo thi công"
      description="Tổng hợp tiến độ, công việc, nguồn lực và minh chứng của một ngày thi công."
      size="report"
      onClose={onClose}
      footer={(
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
          {manualReport && canManage && onEdit && !needsSourceChange ? <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 bg-white px-4 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onEdit}><Pencil className="h-4 w-4" /> Sửa báo cáo</button> : null}
          {report.workItemId ? <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 bg-white px-4 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={openSource}><ExternalLink className="h-4 w-4" /> Mở công việc liên quan</button> : null}
          {needsSourceChange && manualReport && canManage && onEdit ? <button type="button" className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 sm:w-auto" onClick={onEdit}><Pencil className="h-4 w-4" /> Sửa theo yêu cầu</button> : null}
          {deliveryCandidate && recipients.data && recipients.data.length > 1 ? <select aria-label="Người nhận báo cáo" className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-700 sm:w-auto" value={recipientId} onChange={(event) => setRecipientId(event.target.value)}><option value="">Chọn người nhận</option>{recipients.data.map((recipient) => <option key={recipient.id} value={recipient.id}>{recipient.full_name}{recipient.position ? ` · ${recipient.position}` : ''}{recipient.is_recommended ? ' · Đề xuất' : ''}</option>)}</select> : null}
          {deliveryCandidate && recipients.isLoading ? <button type="button" disabled className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white opacity-60 sm:w-auto"><RefreshCw className="h-4 w-4 animate-spin" /> Đang xác định người nhận…</button> : null}
          {deliveryCandidate && recipients.isError ? <button type="button" className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-amber-300 bg-amber-50 px-4 py-2 text-sm font-semibold text-amber-800 hover:bg-amber-100 sm:w-auto" onClick={() => void recipients.refetch()}><RefreshCw className="h-4 w-4" /> Không tải được người nhận · Thử lại</button> : null}
          {deliveryCandidate && recipients.isSuccess && !recipients.data.length ? <span className="inline-flex items-center justify-center rounded-md border border-amber-200 bg-amber-50 px-4 py-2 text-sm font-medium text-amber-800">Chưa có người nhận hợp lệ</span> : null}
          {deliveryCandidate && recipients.isSuccess && recipients.data.length ? <button type="button" disabled={!selectedRecipient || deliveryPending} className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 sm:w-auto" onClick={() => void deliverReport()}>{deliveryPending ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />} {deliveryPending ? 'Đang gửi…' : selectedRecipient ? deliveryButtonLabel : 'Chọn người nhận để gửi'}</button> : null}
          {deliveryStatus === 'submitted' ? <span className="inline-flex items-center justify-center rounded-md bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700">Đã gửi cho {String(record?.delivery_recipient_name ?? 'người quản lý')}</span> : null}
          {deliveryStatus === 'acknowledged' ? <span className="inline-flex items-center justify-center rounded-md bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-700">Người quản lý đã xác nhận</span> : null}
        </div>
      )}
    >
      <div className="space-y-5">
        <section className="overflow-hidden rounded-lg border border-blue-200 bg-gradient-to-r from-blue-50 to-white">
          <div className="grid gap-4 p-4 sm:grid-cols-[minmax(0,1fr)_240px] sm:items-center">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="inline-flex items-center gap-1.5 rounded-md bg-white px-2.5 py-1 text-sm font-semibold text-ink ring-1 ring-inset ring-blue-100"><CalendarDays className="h-4 w-4 text-blue-600" /> {dateText(report.reportDate)}</span>
                <span className="inline-flex items-center gap-1.5 rounded-md bg-white px-2.5 py-1 text-sm text-slate-700 ring-1 ring-inset ring-blue-100"><CloudSun className="h-4 w-4 text-amber-500" /> {report.weather || 'Chưa ghi thời tiết'}</span>
                <span className="rounded-md bg-blue-100 px-2.5 py-1 text-xs font-semibold uppercase tracking-wide text-blue-700">{report.sourceKind === 'workflow' ? 'Báo cáo từ quy trình' : report.sourceKind === 'structured' ? 'Báo cáo hiện trường' : 'Báo cáo tóm tắt'}</span>
              </div>
              <p className="mt-3 text-sm text-slate-600">Nhịp thi công trong ngày được tách rõ thành kết quả, việc đang tiếp diễn, kế hoạch tiếp theo và blocker cần xử lý.</p>
            </div>
            <div>
              <div className="mb-2 flex items-end justify-between gap-3"><span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ báo cáo</span><strong className="text-2xl text-blue-700">{report.progressPercent}%</strong></div>
              <div className="h-2.5 overflow-hidden rounded-full bg-blue-100" role="progressbar" aria-label="Tiến độ báo cáo" aria-valuemin={0} aria-valuemax={100} aria-valuenow={report.progressPercent}><div className="h-full rounded-full bg-blue-600 transition-[width]" style={{ width: `${report.progressPercent}%` }} /></div>
            </div>
          </div>
        </section>

        <div className={canViewLinked ? 'grid gap-5 xl:grid-cols-[minmax(0,1fr)_23rem]' : ''}>
          <div className="space-y-5">
            <section aria-labelledby="report-work-heading">
              <SectionHeading id="report-work-heading" icon={<Hammer className="h-4 w-4" />} title="Công việc trong ngày" description="Đọc từ trái sang phải để nắm kết quả và bước tiếp theo." />
              <div className="mt-3 grid gap-3 md:grid-cols-2"><WorkGroup title="Đã làm" items={report.done} tone="green" empty="Chưa ghi hạng mục hoàn tất" /><WorkGroup title="Đang làm" items={report.current} tone="blue" empty="Không có hạng mục dở dang" /><WorkGroup title="Sẽ làm" items={report.next} tone="slate" empty="Chưa ghi kế hoạch tiếp theo" /><WorkGroup title="Blocker cần xử lý" items={report.blockers} tone="red" empty="Không có blocker cần xử lý" /></div>
            </section>

            <section aria-labelledby="report-resource-heading"><SectionHeading id="report-resource-heading" icon={<Users className="h-4 w-4" />} title="Nguồn lực sử dụng" description="Nhân công và vật tư đã ghi nhận trong ca thi công." /><div className="mt-3 grid gap-3 md:grid-cols-2"><ResourceCard icon={<Users className="h-4 w-4" />} title="Nhân công / tổ đội" items={report.crew} empty="Chưa ghi nhân công hoặc tổ đội" /><ResourceCard icon={<Package className="h-4 w-4" />} title="Vật tư sử dụng" items={report.materials} empty="Chưa ghi vật tư sử dụng" /></div></section>

            <section aria-labelledby="report-note-heading"><SectionHeading id="report-note-heading" icon={<FileText className="h-4 w-4" />} title="Ghi chú hiện trường" description="Thông tin bổ sung; không được tính là blocker nếu báo cáo không khai báo blocker có cấu trúc." /><div className="mt-3 rounded-lg border border-line bg-slate-50 p-4">{report.fieldNotes.length ? <ItemList items={report.fieldNotes} bulletClass="bg-slate-400" /> : <p className="text-sm italic text-slate-500">Không có ghi chú hiện trường.</p>}</div></section>

            <section aria-labelledby="report-evidence-heading"><div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><SectionHeading id="report-evidence-heading" icon={<ClipboardList className="h-4 w-4" />} title={`Minh chứng · ${report.evidenceIds.length}`} description="Ảnh và tài liệu được liên kết trực tiếp với báo cáo." />{manualReport && canManage && onAddEvidence ? <button type="button" className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onAddEvidence}><Upload className="h-4 w-4" /> Thêm minh chứng</button> : null}</div><div className="mt-3">{!report.evidenceIds.length ? <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 p-5"><EmptyState title="Chưa có minh chứng liên kết với báo cáo này" /></div> : null}{report.evidenceIds.length && evidence.isLoading ? <LoadingSpinner label="Đang tải minh chứng báo cáo..." /> : null}{report.evidenceIds.length && evidence.isError ? <InlineError message="Không tải được minh chứng đã liên kết." onRetry={() => evidence.refetch()} /> : null}{report.evidenceIds.length && evidence.isSuccess ? <EvidenceGallery evidence={evidence.data} showControls={false} showWorkItem variant="cards" emptyTitle="Không tìm thấy minh chứng trong phạm vi truy cập" /> : null}</div></section>

            {sourceExists ? <section aria-labelledby="report-source-heading"><SectionHeading id="report-source-heading" icon={<ClipboardList className="h-4 w-4" />} title="Nguồn báo cáo" description="Truy vết báo cáo về đầu việc và biểu mẫu quy trình đã tạo ra dữ liệu." /><div className="mt-3 rounded-lg border border-line bg-white p-4">{report.workItemId && operations.isLoading ? <LoadingSpinner label="Đang tải công việc liên quan..." /> : null}{report.workItemId && operations.isError ? <InlineError message="Không tải được thông tin công việc; vẫn có thể mở công việc bằng liên kết đã lưu." onRetry={() => operations.refetch()} /> : null}{!operations.isLoading && !operations.isError ? <SourceSummary workItem={workItem} workItemId={report.workItemId} workflowFormInstanceId={report.workflowFormInstanceId} onOpen={report.workItemId ? openSource : undefined} /> : null}</div></section> : null}
          </div>

          {canViewLinked ? <RelatedConstructionSection loading={workspace.isLoading} error={workspace.error} workspace={workspace.data} checks={relatedChecks} actions={relatedActions} blockers={report.blockers} drafts={linkedDrafts} sourceWorkItemId={report.workItemId} reportEvidence={evidence.data ?? []} reportEvidencePending={Boolean(report.evidenceIds.length && !evidence.data?.length)} primaryEvidenceId={String(record?.primary_evidence_id ?? '')} canCreateCheck={canCreateCheck} canCreateAction={canCreateAction} linkedNotice={linkedDrafts.length ? linkedNotice : undefined} onDraftsChange={setLinkedDrafts} onSubmitDraft={createLinkedDraft} onRetry={() => workspace.refetch()} onOpenCheck={(check) => onOpenCheck?.(check)} onOpenAction={(action) => onOpenAction?.(action)} /> : null}
        </div>
      </div>
    </Dialog>
  );
}

function RelatedConstructionSection({ loading, error, workspace, checks, actions, blockers, drafts, sourceWorkItemId, reportEvidence, reportEvidencePending, primaryEvidenceId, canCreateCheck, canCreateAction, linkedNotice, onDraftsChange, onSubmitDraft, onRetry, onOpenCheck, onOpenAction }: {
  loading: boolean;
  error: unknown;
  workspace?: ConstructionWorkspace;
  checks: ConstructionCheck[];
  actions: ConstructionActionItem[];
  blockers: string[];
  drafts: ReportLinkedDraft[];
  sourceWorkItemId: string;
  reportEvidence: EvidenceRecord[];
  reportEvidencePending: boolean;
  primaryEvidenceId: string;
  canCreateCheck: boolean;
  canCreateAction: boolean;
  linkedNotice?: string;
  onDraftsChange: Dispatch<SetStateAction<ReportLinkedDraft[]>>;
  onSubmitDraft: (draft: ReportLinkedDraft) => Promise<void>;
  onRetry: () => void;
  onOpenCheck: (check: ConstructionCheck) => void;
  onOpenAction: (action: ConstructionActionItem) => void;
}) {
  const checkIDs = new Set(checks.map((item) => item.id));
  const directActions = actions.filter((item) => !item.source_check_id || !checkIDs.has(item.source_check_id));
  return <aside className="space-y-4 xl:sticky xl:top-4 xl:self-start">
    <section aria-labelledby="report-linked-construction-heading" className="rounded-lg border border-line bg-slate-50/70 p-4">
      <SectionHeading id="report-linked-construction-heading" icon={<Link2 className="h-4 w-4" />} title="Chuỗi liên quan" description="Kiểm tra, xử lý và kết quả khắc phục của báo cáo." />
      <div className="mt-3">
      {loading ? <LoadingSpinner label="Đang tải kiểm tra và việc xử lý liên quan..." /> : null}
      {error ? <InlineError message="Không tải được chuỗi kiểm tra và xử lý liên quan." onRetry={onRetry} /> : null}
      {!loading && !error && !checks.length && !actions.length ? <div className="rounded-lg border border-dashed border-slate-300 bg-white p-4"><EmptyState title="Chưa có kiểm tra hoặc việc xử lý" /></div> : null}
      {!loading && !error && (checks.length || actions.length) ? <div className="space-y-3">
        {checks.map((check) => {
          const childActions = actions.filter((item) => item.source_check_id === check.id);
          return <article key={check.id} className="overflow-hidden rounded-lg border border-blue-200 bg-white">
            <button type="button" className="flex w-full items-start justify-between gap-4 bg-blue-50 px-4 py-3 text-left hover:bg-blue-100" onClick={() => onOpenCheck(check)}><span className="min-w-0"><span className="block font-semibold text-blue-900">{check.title}</span><span className="mt-1 block text-xs text-blue-700">{check.location || check.work_item_title || 'Kiểm tra hiện trường'} · {dateTimeText(check.scheduled_at || check.checked_at)}</span></span><span className={`shrink-0 rounded-full px-2 py-1 text-xs font-semibold ${check.result === 'fail' ? 'bg-rose-100 text-rose-700' : check.result === 'pass' ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'}`}>{checkStatusLabel(check)}</span></button>
            {childActions.length ? <div className="space-y-2 border-t border-blue-100 p-3">{childActions.map((action) => <LinkedActionRow key={action.id} action={action} onOpen={() => onOpenAction(action)} nested />)}</div> : null}
          </article>;
        })}
        {directActions.map((action) => <LinkedActionRow key={action.id} action={action} onOpen={() => onOpenAction(action)} />)}
      </div> : null}
      </div>
    </section>
    <ConstructionReportLinkedComposer mode="immediate" drafts={drafts} onChange={onDraftsChange} workspace={workspace} sourceWorkItemId={sourceWorkItemId} reportEvidence={reportEvidence} reportEvidencePending={reportEvidencePending} primaryEvidenceId={primaryEvidenceId} canCreateCheck={canCreateCheck} canCreateAction={canCreateAction} blockers={blockers} notice={linkedNotice} onSubmitDraft={onSubmitDraft} />
  </aside>;
}

function LinkedActionRow({ action, onOpen, nested = false }: { action: ConstructionActionItem; onOpen: () => void; nested?: boolean }) {
  return <button type="button" className={`flex w-full items-start justify-between gap-4 rounded-lg border p-3 text-left hover:bg-slate-50 ${nested ? 'border-slate-200 bg-slate-50/60' : 'border-amber-200 bg-white'}`} onClick={onOpen}><span className="flex min-w-0 gap-3"><span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full bg-amber-100 text-amber-700"><Wrench className="h-4 w-4" /></span><span className="min-w-0"><span className="block font-semibold text-ink">{action.title}</span><span className="mt-1 block text-xs text-slate-500">{constructionActionAssigneeLabel(action, 'Chưa giao người xử lý')} · {resolutionState(action)}</span></span></span><span className={`shrink-0 rounded-full px-2 py-1 text-xs font-semibold ${action.status === 'done' ? 'bg-emerald-100 text-emerald-700' : action.status === 'waiting_confirmation' ? 'bg-blue-100 text-blue-700' : 'bg-amber-100 text-amber-700'}`}>{actionStatusLabel(action.status)}</span></button>;
}

function resolutionState(action: ConstructionActionItem) {
  const latest = [...(action.events ?? [])].reverse().find((item) => ['resolution_submitted', 'verified_pass', 'verified_fail'].includes(item.event_type));
  if (!latest) return 'Chưa gửi khắc phục';
  if (latest.event_type === 'verified_pass') return 'Đã xác nhận khắc phục đạt';
  if (latest.event_type === 'verified_fail') return 'Đã trả lại để khắc phục tiếp';
  return 'Đã gửi minh chứng khắc phục';
}

function SectionHeading({ id, icon, title, description }: { id: string; icon: React.ReactNode; title: string; description: string }) {
  return <div className="flex items-start gap-2"><span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-md bg-slate-100 text-slate-600">{icon}</span><div><h3 id={id} className="font-semibold text-ink">{title}</h3><p className="mt-0.5 text-xs leading-5 text-slate-500">{description}</p></div></div>;
}

function WorkGroup({ title, items, tone, empty }: { title: string; items: string[]; tone: 'green' | 'blue' | 'slate' | 'red'; empty: string }) {
  const tones = {
    green: { border: 'border-emerald-200', badge: 'bg-emerald-100 text-emerald-700', bullet: 'bg-emerald-500', empty: 'text-emerald-700' },
    blue: { border: 'border-blue-200', badge: 'bg-blue-100 text-blue-700', bullet: 'bg-blue-500', empty: 'text-slate-500' },
    slate: { border: 'border-line', badge: 'bg-slate-100 text-slate-700', bullet: 'bg-slate-400', empty: 'text-slate-500' },
    red: { border: items.length ? 'border-red-200' : 'border-emerald-200', badge: items.length ? 'bg-red-100 text-red-700' : 'bg-emerald-100 text-emerald-700', bullet: 'bg-red-500', empty: 'text-emerald-700' },
  }[tone];
  return <article className={`min-h-32 rounded-lg border bg-white p-4 ${tones.border}`}><div className="flex items-center justify-between gap-3"><h4 className="text-sm font-semibold text-ink">{title}</h4><span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${tones.badge}`}>{items.length}</span></div>{items.length ? <div className="mt-3"><ItemList items={items} bulletClass={tones.bullet} /></div> : <p className={`mt-4 flex items-center gap-2 text-sm italic ${tones.empty}`}>{tone === 'red' ? <CheckCircle2 className="h-4 w-4" /> : null}{empty}</p>}</article>;
}

function ResourceCard({ icon, title, items, empty }: { icon: React.ReactNode; title: string; items: string[]; empty: string }) {
  return <article className="rounded-lg border border-line bg-white p-4"><h4 className="flex items-center gap-2 text-sm font-semibold text-ink"><span className="text-blue-600">{icon}</span>{title}</h4>{items.length ? <div className="mt-3"><ItemList items={items} bulletClass="bg-blue-400" /></div> : <p className="mt-3 text-sm italic text-slate-500">{empty}</p>}</article>;
}

function ItemList({ items, bulletClass }: { items: string[]; bulletClass: string }) {
  return <ul className="space-y-2">{items.map((item, index) => <li key={`${item}-${index}`} className="flex gap-2 text-sm leading-5 text-slate-700"><span className={`mt-2 h-1.5 w-1.5 shrink-0 rounded-full ${bulletClass}`} /> <span>{item}</span></li>)}</ul>;
}

function InlineError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className="flex flex-col gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 sm:flex-row sm:items-center sm:justify-between"><span className="flex items-start gap-2"><AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />{message}</span><button type="button" className="inline-flex items-center gap-1.5 self-start font-semibold hover:underline" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" /> Thử lại</button></div>;
}

function SourceSummary({ workItem, workItemId, workflowFormInstanceId, onOpen }: { workItem?: WorkItem; workItemId: string; workflowFormInstanceId: string; onOpen?: () => void }) {
  if (!workItemId && !workflowFormInstanceId) return <EmptyState title="Báo cáo không có nguồn quy trình" />;
  return <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><p className="font-semibold text-ink">{workItem?.title || 'Đầu việc đã liên kết'}</p>{workItem?.status ? <StatusBadge status={workItem.status} /> : null}</div><p className="mt-1 text-xs text-slate-500">{workItem?.run_step_name || workItem?.run_node_name || 'Nguồn quy trình thi công'}{workflowFormInstanceId ? ' · Có biểu mẫu báo cáo' : ''}</p></div>{onOpen ? <button type="button" className="inline-flex shrink-0 items-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onOpen}><ExternalLink className="h-4 w-4" /> Mở đầu việc</button> : null}</div>;
}
