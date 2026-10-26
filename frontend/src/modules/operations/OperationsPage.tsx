import { constructionReportWeatherLabel } from './constructionWeatherPresentation';
import { useEffect, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, ClipboardCheck, Download, Eye, GitPullRequestArrow, Plus, ScrollText } from 'lucide-react';
import { downloadNativeReportExcel, nativeReportListExcelPath } from '../_shared/bmgsExport';
import { get, post, postFormWithProgress, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { useToast } from '../../components/ToastProvider';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { dateText } from '../../lib/utils/format';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { WorkspaceTabs, type WorkspaceTabItem } from '../../components/WorkspaceTabs';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ConstructionActionItem, ConstructionCheck, ConstructionVariationContext, ConstructionWorkspace } from '../../lib/types/domain';
import { ConstructionReportDetailDialog } from './ConstructionReportDetailDialog';
import { ConstructionReportFormDialog, type ConstructionDailyReportCreatePayload } from './ConstructionReportFormDialog';
import { ConstructionReportEvidenceDialog } from './ConstructionReportEvidenceDialog';
import { constructionEvidenceFormData, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { latestEvidenceUploadResult, uploadEvidenceDraftBatch } from '../_shared/evidenceUploadSession';
import { type ReportLinkedDraft } from './ConstructionReportLinkedComposer';
import { constructionReportViewModel } from './constructionReportViewModel';
import { createConstructionAction, createConstructionCheck, uploadConstructionEvidenceItems } from './constructionWorkspaceApi';
import { optionalISO } from './constructionWorkspaceUtils';
import { ConstructionWorkspacePanel, type ConstructionVariationSource, type ConstructionWorkspaceDialogRequest } from './ConstructionWorkspacePanel';
import { ConstructionVariationProposalDialog } from './ConstructionVariationProposalDialog';
import { useRecordDeepLink } from '../_shared/useRecordDeepLink';

type OperationsTab = 'timeline' | 'checks' | 'actions';
type OperationsDialog = {
  type: 'report' | 'report-evidence';
  mode: 'view' | 'edit';
  record: Record<string, any>;
  pendingLinkedDrafts?: ReportLinkedDraft[];
  linkedNotice?: string;
};

export function OperationsPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [openReport, setOpenReport] = useState(false);
  const [dialog, setDialog] = useState<OperationsDialog | null>(null);
  const [workspaceDialogRequest, setWorkspaceDialogRequest] = useState<ConstructionWorkspaceDialogRequest | null>(null);
  const [variationSource, setVariationSource] = useState<ConstructionVariationSource | null>(null);
  const [draftDates, setDraftDates] = useState({dateFrom: '', dateTo: ''});
  const [dates, setDates] = useState(draftDates);
  const [exporting, setExporting] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { user, hasAnyPermission } = useAuth();
  const canViewOperations = hasAnyPermission(['operation.view']);
  const canReport = hasAnyPermission(['operation.report']);
  const canSubmitWorkReport = hasAnyPermission(['work_report.submit']);
  const canViewFieldWorkspace = hasAnyPermission(['operation.view', 'quality.view', 'rework.view']);
  const canRequestCheck = hasAnyPermission(['construction.check.request']);
  const canCreateAction = hasAnyPermission(['construction.action.create']);
  const canViewContracts = hasAnyPermission(['contract.view']);
  const canProposeVariation = hasAnyPermission(['contract.variation.propose']);
  const reportSource = useRecordDeepLink({kind: 'daily_report', projectId: id || projectId, onOpen: record => { setProjectId(record.project_id); setDialog({type: 'report', mode: 'view', record}); changeTab('timeline'); }});
  const actionSource = useRecordDeepLink({kind: 'action', projectId: id || projectId, onOpen: record => { setProjectId(record.project_id); openWorkspaceDialog({mode: 'actions', state: {type: 'view', action: record}}); }});
  useEffect(() => { if (id) {setProjectId(id); setDialog(null); setWorkspaceDialogRequest(null);} }, [id]);

  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);

  const reportQueryString = useMemo(() => {
    const params = new URLSearchParams();
    if (dates.dateFrom) params.set('date_from', dates.dateFrom);
    if (dates.dateTo) params.set('date_to', dates.dateTo);
    return params.toString();
  }, [dates]);
  const reports = useQuery({ queryKey: ['daily-reports', projectId, dates], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/daily-reports${reportQueryString ? `?${reportQueryString}` : ''}`), enabled: Boolean(projectId) && canViewOperations });
  function applyDates() {
    if (draftDates.dateFrom && draftDates.dateTo && draftDates.dateFrom > draftDates.dateTo) {
      toast.warn({title: 'Khoảng ngày chưa hợp lệ', description: 'Ngày bắt đầu phải trước hoặc bằng ngày kết thúc.'});
      return;
    }
    setDates({...draftDates});
  }
  async function exportReports() {
    if (!projectId || !canViewOperations || exporting || reports.isFetching || reports.isError || !reports.data?.length) return;
    setExporting(true);
    try {
      await downloadNativeReportExcel(nativeReportListExcelPath(`/projects/${encodeURIComponent(projectId)}/daily-reports`, reportQueryString));
      toast.success('Đã xuất báo cáo Excel theo bộ lọc đang áp dụng');
    } catch (error) { toast.error('Không xuất được báo cáo Excel', error); }
    finally { setExporting(false); }
  }
  const construction = useQuery({ queryKey: ['construction-workspace', projectId], queryFn: () => get<ConstructionWorkspace>(`/projects/${projectId}/construction-workspace`), enabled: Boolean(projectId) && canViewFieldWorkspace, retry: false });
  const variationContext = useQuery({ queryKey: ['construction-variation-context', projectId], queryFn: () => get<ConstructionVariationContext>(`/projects/${projectId}/construction-variation-context`), enabled: Boolean(projectId) && (canViewOperations || canViewContracts || canProposeVariation), retry: false });
  const timelineRows = useMemo(() => [...(reports.data ?? [])].sort((left, right) => String(right.report_date ?? '').localeCompare(String(left.report_date ?? ''))), [reports.data]);
  const pendingChecks = (construction.data?.checks ?? []).filter((row) => row.status === 'pending').length;
  const activeActions = (construction.data?.actions ?? []).filter((row) => ['open', 'waiting_confirmation'].includes(row.status)).length;
  const variationRows = variationContext.data?.variations ?? [];
  const openVariations = variationRows.filter((variation) => !['rejected', 'incorporated'].includes(variation.status));
  const constructionActive = Boolean(construction.data?.runtime.construction_active);
  const tabs = useMemo<WorkspaceTabItem<OperationsTab>[]>(() => [
    ...(canViewOperations ? [{ id: 'timeline' as const, label: 'Nhật ký', icon: ScrollText }] : []),
    ...(canViewFieldWorkspace ? [{ id: 'checks' as const, label: 'Kiểm tra', icon: ClipboardCheck, count: pendingChecks }] : []),
    ...(canViewFieldWorkspace ? [{ id: 'actions' as const, label: 'Cần xử lý', icon: AlertTriangle, count: activeActions }] : []),
  ], [activeActions, canViewFieldWorkspace, canViewOperations, pendingChecks]);
  const requestedTab = normalizeTab(searchParams.get('tab'));
  const activeTab: OperationsTab = requestedTab && tabs.some((tab) => tab.id === requestedTab) ? requestedTab : tabs[0]?.id ?? 'timeline';

  useEffect(() => {
    if (!tabs.length || searchParams.get('tab') === activeTab) return;
    const next = new URLSearchParams(searchParams);
    next.set('tab', activeTab);
    setSearchParams(next, { replace: true });
  }, [activeTab, searchParams, setSearchParams, tabs]);

  useEffect(() => {
    const requestedReportID = searchParams.get('report');
    if (!requestedReportID || !reports.data) return;
    const record = reports.data.find((item) => String(item.id) === requestedReportID);
    const next = new URLSearchParams(searchParams);
    next.delete('report');
    next.set('tab', 'timeline');
    setSearchParams(next, { replace: true });
    if (record) {
      setDialog({ type: 'report', mode: 'view', record });
    } else {
      toast.warn({ title: 'Không mở được báo cáo nguồn', description: 'Báo cáo không còn trong phạm vi truy cập hiện tại.' });
    }
  }, [reports.data, searchParams, setSearchParams, toast]);

  const invalidateReports = () => {
    queryClient.invalidateQueries({ queryKey: ['daily-reports', projectId] });
    queryClient.invalidateQueries({ queryKey: ['project-evidence', projectId] });
    queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
    queryClient.invalidateQueries({ queryKey: ['construction-workspace', projectId] });
    queryClient.invalidateQueries({ queryKey: ['work-item'] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    queryClient.invalidateQueries({ queryKey: ['work-reports'] });
    queryClient.invalidateQueries({ queryKey: ['work-report-summary'] });
    queryClient.invalidateQueries({ queryKey: ['work-report-detail'] });
  };
  const createReport = useMutation({
    mutationFn: async ({ values, uploads, linkedDrafts }: { values: ConstructionDailyReportCreatePayload; uploads: ConstructionEvidenceUploadDraft[]; linkedDrafts: ReportLinkedDraft[] }) => {
      const uploadedEvidenceIDs = uploads.length ? await uploadConstructionEvidenceItems(values.work_item_id, uploads) : [];
      const evidenceIDs = [...new Set([...values.evidence_ids, ...uploadedEvidenceIDs])];
      const payload = { ...values, evidence_ids: evidenceIDs, primary_evidence_id: values.primary_evidence_id || uploadedEvidenceIDs[0] || evidenceIDs[0] || '' };
      const latest = await post<Record<string, any>>(`/projects/${projectId}/daily-reports`, payload);
      const view = constructionReportViewModel(latest);
      const linkedEvidenceIDs = view.evidenceIds;
      const primaryEvidenceID = String(latest.primary_evidence_id ?? payload.primary_evidence_id ?? linkedEvidenceIDs[0] ?? '');
      const results = await Promise.all(linkedDrafts.map(async (draft) => {
        try {
          await createReportLinkedRecord(projectId, String(latest.id), draft, payload.work_item_id, linkedEvidenceIDs, primaryEvidenceID);
          return { draft, error: '' };
        } catch (error) {
          return { draft: { ...draft, last_error: error instanceof Error ? error.message : 'Không tạo được liên kết' }, error: error instanceof Error ? error.message : 'Không tạo được liên kết' };
        }
      }));
      return { report: latest, linkedFailures: results.filter((item) => item.error).map((item) => item.draft), linkedCreated: results.filter((item) => !item.error).length };
    },
    onSuccess: ({ report, linkedFailures, linkedCreated }) => {
      invalidateReports();
      setOpenReport(false);
      const transition = report.work_item_transition ?? {};
      const delivery = report.delivery_transition ?? {};
      const autoStarted = transition.auto_started === true;
      const submitted = delivery.submitted === true;
      const workItemTitle = String(transition.work_item_title ?? '').trim();
      const startedNotice = autoStarted ? `Đã bắt đầu ${workItemTitle ? `“${workItemTitle}”` : 'hạng mục liên quan'}. ` : '';
      const deliveryNotice = submitted ? `Đã gửi cho ${String(delivery.recipient_name ?? 'người quản lý')}. ` : '';
      if (linkedFailures.length) {
        setDialog({ type: 'report', mode: 'view', record: report, pendingLinkedDrafts: linkedFailures, linkedNotice: `Báo cáo đã lưu. ${deliveryNotice}${startedNotice}${linkedFailures.length} kiểm tra/việc xử lý chưa được tạo. Dữ liệu lỗi được giữ lại để thử lại ngay tại đây.` });
        toast.warn({ title: 'Báo cáo đã lưu một phần', description: `${deliveryNotice}${startedNotice}${linkedCreated ? `Đã tạo ${linkedCreated} liên kết. ` : ''}Còn ${linkedFailures.length} liên kết cần thử lại.` });
        return;
      }
      if (submitted) {
        toast.success(`${startedNotice}Đã lưu và gửi báo cáo cho ${String(delivery.recipient_name ?? 'người quản lý')}${linkedCreated ? `; đã tạo ${linkedCreated} liên kết` : ''}`.trim());
        return;
      }
      if (autoStarted) {
        toast.success(`Đã lưu báo cáo và bắt đầu ${workItemTitle ? `“${workItemTitle}”` : 'hạng mục'}${linkedCreated ? `; đã tạo ${linkedCreated} liên kết` : ''}`);
        return;
      }
      toast.success(linkedCreated ? `Đã lưu báo cáo và tạo ${linkedCreated} liên kết` : 'Đã tạo báo cáo thi công hôm nay');
    },
    onError: (error) => toast.error('Không tạo được báo cáo thi công', error),
  });
  const updateReport = useMutation({
    mutationFn: async ({ reportId, values, uploads }: { reportId: string; values: ConstructionDailyReportCreatePayload; uploads: ConstructionEvidenceUploadDraft[] }) => {
      const uploadedEvidenceIDs = uploads.length ? await uploadConstructionEvidenceItems(values.work_item_id, uploads) : [];
      const evidenceIDs = [...new Set([...values.evidence_ids, ...uploadedEvidenceIDs])];
      const payload = { ...values, evidence_ids: evidenceIDs, primary_evidence_id: values.primary_evidence_id || uploadedEvidenceIDs[0] || evidenceIDs[0] || '' };
      return put<Record<string, any>>(`/daily-reports/${reportId}`, payload);
    },
    onSuccess: (report) => {
      invalidateReports();
      setDialog(null);
      const delivery = report.delivery_transition ?? {};
      toast.success(delivery.submitted === true ? `Đã cập nhật và gửi lại báo cáo cho ${String(delivery.recipient_name ?? 'người quản lý')}` : 'Đã cập nhật bản nháp báo cáo thi công');
    },
    onError: (error) => toast.error('Không cập nhật được báo cáo thi công', error),
  });
  const quickSubmitReport = useMutation({
    mutationFn: ({ reportId, recipientId }: { reportId: string; recipientId: string; recipientName: string }) => post<Record<string, any>>(`/work-reports/daily-reports/${reportId}/submit`, {
      recipient_id: recipientId,
      comment: '',
    }),
    onSuccess: (report, variables) => {
      invalidateReports();
      setDialog((current) => {
        if (current?.type !== 'report' || String(current.record.id) !== variables.reportId) return current;
        return {
          ...current,
          record: {
            ...current.record,
            ...report,
            delivery_status: String(report.status ?? 'submitted'),
            delivery_recipient_id: String(report.recipient_id ?? variables.recipientId),
            delivery_recipient_name: String(report.recipient_name ?? variables.recipientName),
            delivery_revision_no: report.current_revision_no,
            delivery_review_comment: report.review_comment,
            delivery_source_changed: false,
          },
        };
      });
      toast.success(String(report.current_revision_no ?? '') === '1'
        ? `Đã gửi báo cáo cho ${variables.recipientName}`
        : `Đã gửi lại báo cáo cho ${variables.recipientName}`);
    },
    onError: (error) => {
      invalidateReports();
      toast.error('Không gửi được báo cáo', error);
    },
  });
  const addReportEvidence = useMutation({
    mutationFn: ({ reportId, workItemId, uploads }: { reportId: string; workItemId: string; uploads: ConstructionEvidenceUploadDraft[]; record: Record<string, any> }) => uploadReportEvidence(reportId, workItemId, uploads),
    onSuccess: (latest, variables) => { invalidateReports(); setDialog({ type: 'report', mode: 'view', record: latest ?? variables.record }); toast.success('Đã thêm minh chứng vào báo cáo'); },
    onError: (error) => toast.error('Không thêm được minh chứng', error),
  });

  async function uploadReportEvidence(reportId: string, workItemId: string, uploads: ConstructionEvidenceUploadDraft[]) {
    const path = `/daily-reports/${reportId}/evidence/upload`;
    const scope = `${path}:${workItemId}`;
    await uploadEvidenceDraftBatch(uploads, scope, (draft, progress) => postFormWithProgress<Record<string, any>>(path, constructionEvidenceFormData(draft, workItemId), ({ percent }) => progress(percent)), 1);
    return latestEvidenceUploadResult<Record<string, any>>(uploads, scope) ?? null;
  }

  function changeTab(tab: OperationsTab) {
    const next = new URLSearchParams(searchParams);
    next.set('tab', tab);
    setSearchParams(next);
  }

  function openWorkspaceDialog(request: ConstructionWorkspaceDialogRequest) {
    setDialog(null);
    setWorkspaceDialogRequest(request);
    changeTab(request.mode);
  }

  function openLinkedReport(reportId: string) {
    const record = reports.data?.find((item) => item.id === reportId);
    if (!record) {
      toast.warn({ title: 'Không mở được báo cáo nguồn', description: 'Báo cáo không còn trong phạm vi truy cập hiện tại.' });
      return;
    }
    setDialog({ type: 'report', mode: 'view', record });
  }

  function openLinkedCheck(check: ConstructionCheck) {
    openWorkspaceDialog({ mode: 'checks', state: { type: 'view', check } });
  }

  function openLinkedAction(action: ConstructionActionItem) {
    openWorkspaceDialog({ mode: 'actions', state: { type: 'view', action } });
  }

  async function variationSaved() {
    setVariationSource(null);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['construction-variation-context', projectId] }),
      queryClient.invalidateQueries({ queryKey: ['construction-workspace', projectId] }),
    ]);
  }

  const selectedReportLocked = Boolean(dialog?.record) && (dialog?.record?.report_delivery_eligible === false || ['recorded', 'submitted', 'acknowledged'].includes(String(dialog?.record?.delivery_status ?? '')));
  const canDeliverSelectedReport = canSubmitWorkReport && String(dialog?.record?.reported_by ?? '') === String(user?.id ?? '');

  return <>
    {searchParams.get('record_type') === 'action' ? actionSource.feedback : reportSource.feedback}
    <PageHeader title="Thi công" actions={<>{!global ? <ProjectBackLink projectId={projectId} /> : null}{global ? <div className="w-full sm:w-80"><AsyncProjectPicker value={projectId} onChange={setProjectId} endpoint="/projects/options" queryKey="project-options" emptyLabel="Chọn công trình" allowEmptySelection={false} /></div> : null}{canReport ? <button disabled={!projectId || !constructionActive} title={!constructionActive ? 'Chỉ lập báo cáo khi quy trình đang ở Node Thi công' : undefined} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => constructionActive && setOpenReport(true)}><Plus className="h-4 w-4" /> Báo cáo hôm nay</button> : null}</>} />
    {construction.data && !constructionActive ? <ConstructionRuntimeBanner workspace={construction.data} /> : null}
    <WorkspaceTabs className="mb-4" tabs={tabs} activeTab={activeTab} onChange={changeTab} ariaLabel="Không gian thi công" />
    {openVariations.length > 0 ? <ConstructionVariationStatus projectId={projectId} openCount={openVariations.length} urgentCount={openVariations.filter((variation) => variation.priority === 'urgent').length} canViewContracts={canViewContracts} /> : null}
    {activeTab === 'timeline' && canViewOperations ? <section aria-label="Bộ lọc nhật ký" className="mb-4 flex flex-wrap items-end gap-3 rounded-lg border border-line bg-white p-3">
      <label className="text-sm">Từ ngày<input type="date" className="mt-1 block rounded-md border border-line px-3 py-2" value={draftDates.dateFrom} onChange={event => setDraftDates(current => ({...current, dateFrom: event.target.value}))} /></label>
      <label className="text-sm">Đến ngày<input type="date" aria-invalid={Boolean(draftDates.dateFrom && draftDates.dateTo && draftDates.dateFrom > draftDates.dateTo)} className="mt-1 block rounded-md border border-line px-3 py-2 aria-[invalid=true]:border-rose-500" value={draftDates.dateTo} onChange={event => setDraftDates(current => ({...current, dateTo: event.target.value}))} /></label>
      <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={applyDates}>Áp dụng</button>
      <button type="button" disabled={exporting || reports.isFetching || reports.isError || !reports.data?.length} className="inline-flex items-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 disabled:opacity-50" onClick={() => void exportReports()}><Download className="h-4 w-4" />{exporting ? 'Đang xuất…' : 'Xuất báo cáo'}</button>
    </section> : null}
    {activeTab === 'timeline' && canViewOperations ? <DailyTimeline rows={timelineRows} workspace={canViewFieldWorkspace ? construction.data : undefined} loading={reports.isLoading} error={reports.error} createEnabled={constructionActive} onRetry={() => reports.refetch()} onCreate={() => constructionActive && setOpenReport(true)} onView={(record) => setDialog({ type: 'report', mode: 'view', record })} /> : null}
    {activeTab === 'checks' && canViewFieldWorkspace ? <ConstructionWorkspacePanel projectId={projectId} mode="checks" dialogRequest={workspaceDialogRequest} onDialogRequestHandled={() => setWorkspaceDialogRequest(null)} onViewReport={canViewOperations ? openLinkedReport : undefined} /> : null}
    {activeTab === 'actions' && canViewFieldWorkspace ? <ConstructionWorkspacePanel projectId={projectId} mode="actions" dialogRequest={workspaceDialogRequest} onDialogRequestHandled={() => setWorkspaceDialogRequest(null)} onViewReport={canViewOperations ? openLinkedReport : undefined} variationContext={variationContext.data} variationContextLoading={variationContext.isLoading} variationContextError={variationContext.isError} canViewContracts={canViewContracts} onRetryVariationContext={() => { void variationContext.refetch(); }} onProposeVariation={canProposeVariation ? setVariationSource : undefined} /> : null}
    <ConstructionReportFormDialog open={openReport} projectId={projectId} projectName={projects.data?.find((project) => project.id === projectId)?.name ?? ''} suggestion={timelineRows[0] ?? null} workspace={construction.data} canCreateCheck={canRequestCheck} canCreateAction={canCreateAction} canSubmitWorkReport={canSubmitWorkReport} onSubmit={(values, uploads, linkedDrafts) => createReport.mutateAsync({ values, uploads, linkedDrafts })} onClose={() => setOpenReport(false)} />
    <ConstructionReportDetailDialog open={dialog?.type === 'report' && dialog.mode === 'view'} projectId={projectId} record={dialog?.record ?? null} initialLinkedDrafts={dialog?.pendingLinkedDrafts} linkedNotice={dialog?.linkedNotice} canManage={canReport && constructionActive && !selectedReportLocked && Boolean(dialog?.record?.runtime_compliant)} canViewLinked={canViewFieldWorkspace} canCreateCheck={canRequestCheck && constructionActive} canCreateAction={canCreateAction && constructionActive} canDeliver={canDeliverSelectedReport} deliveryPending={quickSubmitReport.isPending} onDeliver={(recipientId, recipientName) => dialog?.record ? quickSubmitReport.mutateAsync({ reportId: String(dialog.record.id), recipientId, recipientName }) : Promise.resolve()} onEdit={() => dialog?.record && setDialog({ type: 'report', mode: 'edit', record: dialog.record })} onAddEvidence={() => dialog?.record && setDialog({ type: 'report-evidence', mode: 'edit', record: dialog.record })} onOpenCheck={openLinkedCheck} onOpenAction={openLinkedAction} onClose={() => setDialog(null)} />
    <ConstructionReportFormDialog open={dialog?.type === 'report' && dialog.mode === 'edit'} projectId={projectId} projectName={projects.data?.find((project) => project.id === projectId)?.name ?? ''} mode="edit" record={dialog?.record ?? null} canSubmitWorkReport={canDeliverSelectedReport} onSubmit={(values, uploads) => dialog?.record ? updateReport.mutateAsync({ reportId: dialog.record.id, values, uploads }) : Promise.resolve()} onClose={() => setDialog(null)} />
    <ConstructionReportEvidenceDialog open={dialog?.type === 'report-evidence'} projectId={projectId} record={dialog?.record ?? null} onSubmit={(workItemId, uploads) => dialog?.record ? addReportEvidence.mutateAsync({ reportId: dialog.record.id, workItemId, uploads, record: dialog.record }) : Promise.resolve()} onClose={() => setDialog(null)} />
    <ConstructionVariationProposalDialog open={Boolean(variationSource)} projectId={projectId} context={variationContext.data} contextLoading={variationContext.isLoading} contextError={variationContext.isError} canViewContracts={canViewContracts} workspace={construction.data} sourceWorkItemId={variationSource?.workItemId} sourceAction={variationSource?.action} onRetryContext={() => { void variationContext.refetch(); }} onClose={() => setVariationSource(null)} onSaved={variationSaved} />
  </>;
}

function ConstructionVariationStatus({ projectId, openCount, urgentCount, canViewContracts }: {
  projectId: string;
  openCount: number;
  urgentCount: number;
  canViewContracts: boolean;
}) {
  const query = new URLSearchParams({ tab: 'variations' });
  return (
    <section className="mb-4 flex flex-col gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between" aria-label="Trạng thái phát sinh thương mại">
      <div className="flex min-w-0 items-center gap-2 text-sm">
        <GitPullRequestArrow className="h-4 w-4 shrink-0 text-slate-500" />
        <span className="font-semibold text-slate-800">Phát sinh thương mại</span>
        <span className="text-slate-600">{openCount} đang xử lý</span>
        {urgentCount ? <span className="rounded-full bg-rose-100 px-2 py-0.5 text-xs font-semibold text-rose-700">{urgentCount} gấp</span> : null}
      </div>
      {canViewContracts ? <Link className="inline-flex shrink-0 items-center text-sm font-semibold text-blue-700 hover:text-blue-800" to={`/projects/${projectId}/contracts?${query.toString()}`}>Xem phát sinh</Link> : null}
    </section>
  );
}

function ConstructionRuntimeBanner({ workspace }: { workspace: ConstructionWorkspace }) {
  const nodeName = workspace.runtime.current_node_name || 'chưa xác định';
  return <section className="mb-4 rounded-lg border border-blue-200 bg-blue-50 p-4"><p className="text-sm font-semibold text-blue-900">Không gian Thi công đang ở chế độ theo dõi</p><p className="mt-1 text-xs leading-5 text-blue-800">Quy trình hiện ở <strong>{nodeName}</strong>. Nhật ký cũ vẫn xem được; tạo báo cáo, kiểm tra và việc cần xử lý sẽ mở khi Node Thi công bắt đầu.</p></section>;
}

async function createReportLinkedRecord(projectId: string, reportId: string, draft: ReportLinkedDraft, reportWorkItemId: string, evidenceIDs: string[], primaryEvidenceID: string) {
  const workItemId = reportWorkItemId || draft.work_item_id;
  if (draft.kind === 'check') {
    await createConstructionCheck(projectId, {
      work_item_id: workItemId,
      source_daily_report_id: reportId,
      title: draft.title.trim(),
      location: draft.location.trim(),
      scheduled_at: optionalISO(draft.scheduled_at),
      criteria: draft.criteria_text.split('\n').map((line) => line.trim()).filter(Boolean).map((title) => ({ title, criteria: title, evidence_required: true })),
      context_evidence_ids: evidenceIDs,
    });
    return;
  }
  if (!primaryEvidenceID) throw new Error('Việc xử lý chưa có minh chứng hiện trạng');
  await createConstructionAction(projectId, {
    work_item_id: workItemId,
    source_daily_report_id: reportId,
    title: draft.title.trim(),
    description: draft.description.trim(),
    priority: draft.priority,
    assigned_user_id: draft.assigned_user_id,
    due_at: optionalISO(draft.due_at),
    impact_scope: draft.impact_scope,
    evidence_id: primaryEvidenceID,
  });
}

function normalizeTab(value: string | null): OperationsTab | null {
  if (value === 'quality') return 'checks';
  if (value === 'issues') return 'actions';
  return value === 'timeline' || value === 'checks' || value === 'actions' ? value : null;
}

function DailyTimeline({ rows, workspace, loading, error, createEnabled, onRetry, onCreate, onView }: { rows: Record<string, any>[]; workspace?: ConstructionWorkspace; loading: boolean; error: unknown; createEnabled: boolean; onRetry: () => void; onCreate: () => void; onView: (record: Record<string, any>) => void }) {
  if (loading) return <LoadingSpinner label="Đang tải nhật ký thi công..." />;
  if (error) return <div className="flex items-center justify-between rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"><span>Không tải được nhật ký thi công.</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>;
  if (!rows.length) return <div className="rounded-lg border border-line bg-white p-6"><EmptyState title="Chưa có báo cáo thi công" action={<button disabled={!createEnabled} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50" onClick={onCreate}><Plus className="h-4 w-4" /> Báo cáo hôm nay</button>} /></div>;
  return <div className="relative space-y-4 before:absolute before:bottom-6 before:left-[18px] before:top-6 before:w-px before:bg-blue-200">{rows.map((row) => { const groups = constructionReportViewModel(row); const linkedChecks = (workspace?.checks ?? []).filter((item) => item.source_daily_report_id === row.id); const linkedActions = (workspace?.actions ?? []).filter((item) => item.source_daily_report_id === row.id); return <article key={row.id} className="relative pl-10"><span className="absolute left-2 top-6 z-10 h-5 w-5 rounded-full border-4 border-blue-100 bg-blue-600" /><div className="overflow-hidden rounded-lg border border-line bg-white shadow-sm"><div className="flex flex-col gap-3 border-b border-line px-4 py-3 sm:flex-row sm:items-center sm:justify-between"><div><div className="flex flex-wrap items-center gap-2"><h2 className="font-semibold text-ink">{dateText(row.report_date)}</h2><span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-600">{constructionReportWeatherLabel(row.weather)}</span>{linkedChecks.length ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{linkedChecks.length} kiểm tra</span> : null}{linkedActions.length ? <span className="rounded-md bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-700">{linkedActions.length} cần xử lý</span> : null}{linkedActions.some((item) => item.status === 'done') ? <span className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">Đã khắc phục</span> : null}</div><p className="mt-1 text-xs text-slate-500">{row.manpower_summary || 'Chưa ghi nhân công'} · {row.material_usage_summary || 'Chưa ghi vật tư'}</p></div><div className="flex items-center gap-3"><div className="text-right"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ báo cáo</p><p className="text-lg font-semibold text-blue-700">{groups.hasProgress ? `${groups.progressPercent}%` : 'Chưa ghi tiến độ'}</p></div><button className="rounded-md border border-line p-2 text-slate-600 hover:bg-slate-100" onClick={() => onView(row)} aria-label="Xem báo cáo"><Eye className="h-4 w-4" /></button></div></div><div className="grid divide-y divide-line lg:grid-cols-4 lg:divide-x lg:divide-y-0"><TimelineColumn title="Đã làm" items={groups.done} tone="green" empty="Chưa ghi hạng mục hoàn tất" /><TimelineColumn title="Đang làm" items={groups.current} tone="blue" empty="Không có hạng mục dở dang" /><TimelineColumn title="Sẽ làm" items={groups.next} tone="slate" empty="Chưa ghi kế hoạch tiếp theo" /><TimelineColumn title="Vấn đề phát sinh" items={groups.timelineProblems} tone="red" empty="Không có vấn đề" /></div></div></article>; })}</div>;
}

function TimelineColumn({ title, items, tone, empty }: { title: string; items: string[]; tone: 'green' | 'blue' | 'slate' | 'red'; empty: string }) {
  const tones = { green: 'bg-emerald-50 text-emerald-700', blue: 'bg-blue-50 text-blue-700', slate: 'bg-slate-100 text-slate-700', red: 'bg-red-50 text-red-700' };
  return <section className="min-w-0 p-4"><h3 className={`inline-flex rounded-md px-2 py-1 text-xs font-semibold uppercase tracking-wide ${tones[tone]}`}>{title} · {items.length}</h3>{items.length ? <ul className="mt-3 space-y-2">{items.map((item, index) => <li key={`${item}-${index}`} className="flex gap-2 text-sm leading-5 text-slate-700"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-slate-400" />{item}</li>)}</ul> : <p className="mt-3 text-sm italic text-slate-400">{empty}</p>}</section>;
}
