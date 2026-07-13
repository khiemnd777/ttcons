import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CalendarCheck2, ClipboardCheck, Clock3, Eye, Plus, Wrench } from 'lucide-react';
import { get, post, postForm, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { RecordFormDialog } from '../_shared/RecordFormDialog';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { dateText } from '../../lib/utils/format';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useAuth } from '../../lib/auth/AuthContext';
import type { QualityWorkspace } from '../../lib/types/domain';
import { QualityPage } from '../quality/QualityPage';
import { ReworkPage } from '../rework/ReworkPage';
import { ConstructionReportDetailDialog } from './ConstructionReportDetailDialog';
import { ConstructionReportFormDialog, type ConstructionDailyReportCreatePayload } from './ConstructionReportFormDialog';
import { ConstructionReportEvidenceDialog } from './ConstructionReportEvidenceDialog';
import { constructionEvidenceFormData, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { constructionReportViewModel } from './constructionReportViewModel';

type OperationsTab = 'timeline' | 'quality' | 'issues';

export function OperationsPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [issueStatus, setIssueStatus] = useState('all');
  const [open, setOpen] = useState<'report' | 'issue' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'report' | 'report-evidence' | 'issue'; mode: 'view' | 'edit'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const canViewOperations = hasAnyPermission(['operation.view']);
  const canReport = hasAnyPermission(['operation.report']);
  const canViewQuality = hasAnyPermission(['quality.view']);
  const canViewRework = hasAnyPermission(['rework.view']);
  const canViewReplanning = hasAnyPermission(['replanning.view']);

  const tabs = useMemo(() => [
    canViewOperations ? { key: 'timeline' as const, label: 'Nhật ký & tiến độ' } : null,
    canViewQuality ? { key: 'quality' as const, label: 'Checklist chất lượng' } : null,
    (canViewOperations || canViewQuality || canViewRework || canViewReplanning) ? { key: 'issues' as const, label: 'Vấn đề & khắc phục' } : null,
  ].filter((tab): tab is { key: OperationsTab; label: string } => Boolean(tab)), [canViewOperations, canViewQuality, canViewReplanning, canViewRework]);
  const requestedTab = searchParams.get('tab') as OperationsTab | null;
  const activeTab = tabs.some((tab) => tab.key === requestedTab) ? requestedTab as OperationsTab : tabs[0]?.key ?? 'timeline';

  useEffect(() => {
    if (!tabs.length || requestedTab === activeTab) return;
    const next = new URLSearchParams(searchParams);
    next.set('tab', activeTab);
    setSearchParams(next, { replace: true });
  }, [activeTab, requestedTab, searchParams, setSearchParams, tabs]);

  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);

  const reports = useQuery({ queryKey: ['daily-reports', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/daily-reports`), enabled: Boolean(projectId) && canViewOperations });
  const issues = useQuery({ queryKey: ['issues', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/issues`), enabled: Boolean(projectId) && canViewOperations });
  const quality = useQuery({ queryKey: ['quality', projectId], queryFn: () => get<QualityWorkspace>(`/projects/${projectId}/quality`), enabled: Boolean(projectId) && canViewQuality });
  const reworks = useQuery({ queryKey: ['reworks', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/reworks`), enabled: Boolean(projectId) && canViewRework });
  const reportRows = reports.data ?? [];
  const timelineRows = useMemo(() => [...reportRows].sort((left, right) => String(right.report_date ?? '').localeCompare(String(left.report_date ?? ''))), [reportRows]);
  const issueRows = issues.data ?? [];
  const reworkRows = reworks.data ?? [];
  const filteredIssues = useMemo(() => issueStatus === 'all' ? issueRows : issueRows.filter((row) => row.status === issueStatus), [issueRows, issueStatus]);
  const issueFilterOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: issueRows.length },
    { value: 'open', label: 'Mở', count: countBy(issueRows, 'status', 'open') },
    { value: 'in_progress', label: 'Đang xử lý', count: countBy(issueRows, 'status', 'in_progress') },
    { value: 'closed', label: 'Đã đóng', count: countBy(issueRows, 'status', 'closed') },
  ], [issueRows]);
  const openIssueCount = issueRows.filter((row) => ['open', 'in_progress'].includes(row.status)).length;
  const openNCRCount = (quality.data?.ncrs ?? []).filter((row) => row.status !== 'closed').length;
  const openReworkCount = reworkRows.filter((row) => row.status !== 'done').length;
  const pendingInspectionCount = (quality.data?.inspection_requests ?? []).filter((row) => ['draft', 'submitted', 'scheduled'].includes(row.status)).length
    + (quality.data?.recurring_occurrences ?? []).filter((row) => ['pending', 'in_progress', 'overdue'].includes(row.effective_status)).length;
  const today = new Date().toISOString().slice(0, 10);
  const hasTodayReport = reportRows.some((row) => String(row.report_date ?? '').slice(0, 10) === today);
  const latestTimeline = timelineRows.length ? constructionReportViewModel(timelineRows[0]) : null;

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['daily-reports', projectId] });
    queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
    queryClient.invalidateQueries({ queryKey: ['reworks', projectId] });
    queryClient.invalidateQueries({ queryKey: ['quality', projectId] });
    queryClient.invalidateQueries({ queryKey: ['project-evidence', projectId] });
    queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
  };
  const createReport = useMutation({
    mutationFn: async ({ values, uploads }: { values: ConstructionDailyReportCreatePayload; uploads: ConstructionEvidenceUploadDraft[] }) => {
      const report = await post<Record<string, any>>(`/projects/${projectId}/daily-reports`, values);
      const uploaded = await uploadReportEvidence(report.id, values.work_item_id, uploads);
      return { report: uploaded.latest ?? report, failed: uploaded.failed };
    },
    onSuccess: ({ failed }) => { invalidate(); setOpen(null); if (failed.length) toast.warn({ title: 'Đã lưu báo cáo', description: `Không tải được ${failed.length} tệp: ${failed.join(', ')}` }); else toast.success('Đã tạo báo cáo thi công hôm nay'); },
    onError: (error) => toast.error('Không tạo được báo cáo thi công', error),
  });
  const updateReport = useMutation({
    mutationFn: async ({ reportId, values, uploads }: { reportId: string; values: ConstructionDailyReportCreatePayload; uploads: ConstructionEvidenceUploadDraft[] }) => {
      const report = await put<Record<string, any>>(`/daily-reports/${reportId}`, values);
      const uploaded = await uploadReportEvidence(reportId, values.work_item_id, uploads);
      return { report: uploaded.latest ?? report, failed: uploaded.failed };
    },
    onSuccess: ({ failed }) => { invalidate(); setDialog(null); if (failed.length) toast.warn({ title: 'Đã cập nhật báo cáo', description: `Không tải được ${failed.length} tệp: ${failed.join(', ')}` }); else toast.success('Đã cập nhật báo cáo thi công'); },
    onError: (error) => toast.error('Không cập nhật được báo cáo thi công', error),
  });
  const addReportEvidence = useMutation({
    mutationFn: ({ reportId, workItemId, uploads }: { reportId: string; workItemId: string; uploads: ConstructionEvidenceUploadDraft[]; record: Record<string, any> }) => uploadReportEvidence(reportId, workItemId, uploads),
    onSuccess: ({ failed, latest }, variables) => { invalidate(); if (failed.length) { if (latest) setDialog({ type: 'report', mode: 'view', record: latest }); toast.warn({ title: 'Chưa tải đủ minh chứng', description: `Không tải được ${failed.length} tệp: ${failed.join(', ')}` }); return; } setDialog({ type: 'report', mode: 'view', record: latest ?? variables.record }); toast.success('Đã thêm minh chứng vào báo cáo'); },
    onError: (error) => toast.error('Không thêm được minh chứng', error),
  });

  async function uploadReportEvidence(reportId: string, workItemId: string, uploads: ConstructionEvidenceUploadDraft[]) {
    const failed: string[] = [];
    let latest: Record<string, any> | null = null;
    for (const draft of uploads) {
      try {
        latest = await postForm<Record<string, any>>(`/daily-reports/${reportId}/evidence/upload`, constructionEvidenceFormData(draft, workItemId));
      } catch (error) {
        const reason = error instanceof Error ? error.message : 'không xác định';
        failed.push(`${draft.name || draft.file.name} (${reason})`);
      }
    }
    return { failed, latest };
  }
  const createIssue = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/issues`, values),
    onSuccess: () => { invalidate(); setOpen(null); toast.success('Đã ghi nhận vấn đề hiện trường'); },
    onError: (error) => toast.error('Không ghi nhận được vấn đề', error),
  });
  const closeIssue = useMutation({
    mutationFn: (issueId: string) => put(`/issues/${issueId}`, { status: 'closed' }),
    onSuccess: () => { invalidate(); toast.success('Đã đóng vấn đề'); },
    onError: (error) => toast.error('Không đóng được vấn đề', error),
  });
  const updateIssue = useMutation({
    mutationFn: ({ issueId, values }: { issueId: string; values: Record<string, any> }) => put(`/issues/${issueId}`, values),
    onSuccess: () => { invalidate(); setDialog(null); toast.success('Đã cập nhật vấn đề'); },
    onError: (error) => toast.error('Không cập nhật được vấn đề', error),
  });

  const issueFields = (record?: Record<string, any>) => [
    { name: 'title', label: 'Tiêu đề', defaultValue: record?.title ?? 'Vấn đề phát sinh', required: true, fullWidth: true, helperText: 'Tên ngắn gọn để đội thi công nhận diện vấn đề.' },
    { name: 'description', label: 'Mô tả', type: 'textarea' as const, defaultValue: record?.description ?? '', helperText: 'Ghi rõ vị trí, ảnh hưởng và bước xử lý tạm thời nếu có.' },
    { name: 'severity', label: 'Mức độ', type: 'select' as const, defaultValue: record?.severity ?? 'medium', required: true, options: enumOptions('severity', ['low', 'medium', 'high', 'critical']) },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'open', required: true, options: enumOptions('status', ['open', 'in_progress', 'closed']) },
    { name: 'requires_rework', label: 'Cần sửa lỗi', type: 'checkbox' as const, defaultValue: Boolean(record?.requires_rework), helperText: 'Bật khi cần mở luồng sửa lỗi.' },
    { name: 'requires_replanning', label: 'Cần lập kế hoạch lại', type: 'checkbox' as const, defaultValue: Boolean(record?.requires_replanning), helperText: 'Bật khi vấn đề ảnh hưởng tiến độ hoặc thứ tự thi công.' },
  ];

  function changeTab(tab: OperationsTab) {
    const next = new URLSearchParams(searchParams);
    next.set('tab', tab);
    setSearchParams(next);
  }

  return (
    <>
      <PageHeader
        title="Thi công"
        description="Theo dõi nhịp thi công hằng ngày, kiểm soát checklist chất lượng và xử lý vấn đề trong một luồng tập trung."
        actions={<>
          {!global ? <ProjectBackLink projectId={projectId} /> : null}
          {global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
          {canReport ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:opacity-50" onClick={() => setOpen('issue')}><AlertTriangle className="h-4 w-4" /> Ghi nhận vấn đề</button> : null}
          {canReport ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={() => setOpen('report')}><Plus className="h-4 w-4" /> Báo cáo hôm nay</button> : null}
        </>}
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatusCard icon={<CalendarCheck2 className="h-4 w-4" />} label="Cần báo cáo" value={canViewOperations ? (hasTodayReport ? 'Đã có' : 'Chưa có') : '—'} detail={canViewOperations ? 'Nhật ký của ngày hôm nay' : 'Không có quyền nhật ký'} tone={canViewOperations && !hasTodayReport ? 'amber' : 'slate'} />
        <StatusCard icon={<Clock3 className="h-4 w-4" />} label="Đang thi công" value={canViewOperations ? latestTimeline?.current.length ?? 0 : '—'} detail="Hạng mục đang thực hiện gần nhất" tone="blue" />
        <StatusCard icon={<ClipboardCheck className="h-4 w-4" />} label="Chờ kiểm tra" value={canViewQuality ? pendingInspectionCount : '—'} detail="RFI và kiểm tra định kỳ" tone={pendingInspectionCount ? 'blue' : 'slate'} />
        <StatusCard icon={<Wrench className="h-4 w-4" />} label="Cần khắc phục" value={(canViewOperations || canViewQuality) ? openIssueCount + openNCRCount : '—'} detail={`${openReworkCount} yêu cầu sửa lỗi đang mở`} tone={(openIssueCount + openNCRCount) ? 'red' : 'slate'} />
      </div>

      <div className="mb-4 flex gap-1 overflow-x-auto rounded-lg border border-line bg-white p-1" role="tablist" aria-label="Không gian thi công">
        {tabs.map((tab) => <button key={tab.key} role="tab" aria-selected={activeTab === tab.key} className={`shrink-0 rounded-md px-4 py-2 text-sm font-semibold transition ${activeTab === tab.key ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:bg-slate-100'}`} onClick={() => changeTab(tab.key)}>{tab.label}</button>)}
      </div>

      {activeTab === 'timeline' && canViewOperations ? (
        <DailyTimeline rows={timelineRows} loading={reports.isLoading} error={reports.error} onRetry={() => reports.refetch()} onCreate={() => setOpen('report')} onView={(record) => setDialog({ type: 'report', mode: 'view', record })} />
      ) : null}

      {activeTab === 'quality' && canViewQuality ? <QualityPage embedded projectId={projectId} section="checklist" /> : null}

      {activeTab === 'issues' ? (
        <div className="space-y-4">
          {canViewOperations ? (
            <section className="overflow-hidden rounded-lg border border-line bg-white">
              <div className="border-b border-line px-4 py-3"><h2 className="font-semibold text-ink">Vấn đề hiện trường</h2><p className="mt-1 text-xs text-slate-500">Issue chính thức được quản lý riêng với blocker ghi trong báo cáo ngày.</p></div>
              <div className="p-4"><FilterBar label="Trạng thái vấn đề" value={issueStatus} options={issueFilterOptions} onChange={setIssueStatus} disabled={!projectId} /></div>
              <DataTable rows={filteredIssues} loading={issues.isLoading} error={issues.error} onRetry={() => issues.refetch()} searchPlaceholder="Tìm vấn đề, mô tả, mức độ..." pageSize={10} defaultSort={{ key: 'severity', direction: 'desc' }} columns={[
                { key: 'title', header: 'Vấn đề', sortable: true, filterValue: (row) => [row.title, row.description], render: (row) => <div><p className="font-semibold text-ink">{row.title}</p><p className="mt-1 text-xs text-slate-500">{row.description || 'Không có mô tả'}</p></div> },
                { key: 'severity', header: 'Mức độ', sortable: true, render: (row) => <EnumChip group="severity" value={row.severity} /> },
                { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
                { key: 'requires_replanning', header: 'Lập kế hoạch lại', align: 'center', render: (row) => row.requires_replanning ? 'Có' : 'Không' },
                { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setDialog({ type: 'issue', mode: 'view', record: row })} onEdit={() => setDialog({ type: 'issue', mode: 'edit', record: row })}><button disabled={row.status === 'closed'} className="text-sm font-semibold text-blue-700 disabled:text-slate-400" onClick={() => closeIssue.mutate(row.id)}>Đóng</button></RowActions> },
              ]} empty="Chưa có vấn đề trong bộ lọc này" />
            </section>
          ) : null}
          {canViewQuality ? <QualityPage embedded projectId={projectId} section="issues" /> : null}
          {(canViewRework || canViewReplanning) ? <ReworkPage embedded projectIdOverride={projectId} /> : null}
        </div>
      ) : null}

      <ConstructionReportFormDialog open={open === 'report'} projectId={projectId} onSubmit={(values, uploads) => createReport.mutateAsync({ values, uploads })} onClose={() => setOpen(null)} />
      <RecordFormDialog open={open === 'issue'} title="Ghi nhận vấn đề hiện trường" submitLabel="Tạo vấn đề" layout="two-column" fields={issueFields()} onSubmit={(values) => createIssue.mutateAsync(values)} onClose={() => setOpen(null)} />
      <ConstructionReportDetailDialog open={dialog?.type === 'report' && dialog.mode === 'view'} projectId={projectId} record={dialog?.record ?? null} canManage={canReport} onEdit={() => dialog?.record && setDialog({ type: 'report', mode: 'edit', record: dialog.record })} onAddEvidence={() => dialog?.record && setDialog({ type: 'report-evidence', mode: 'edit', record: dialog.record })} onClose={() => setDialog(null)} />
      <ConstructionReportFormDialog open={dialog?.type === 'report' && dialog.mode === 'edit'} projectId={projectId} mode="edit" record={dialog?.record ?? null} onSubmit={(values, uploads) => dialog?.record ? updateReport.mutateAsync({ reportId: dialog.record.id, values, uploads }) : Promise.resolve()} onClose={() => setDialog(null)} />
      <ConstructionReportEvidenceDialog open={dialog?.type === 'report-evidence'} projectId={projectId} record={dialog?.record ?? null} onSubmit={(workItemId, uploads) => dialog?.record ? addReportEvidence.mutateAsync({ reportId: dialog.record.id, workItemId, uploads, record: dialog.record }) : Promise.resolve()} onClose={() => setDialog(null)} />
      <RecordDetailDialog open={dialog?.type === 'issue' && dialog.mode === 'view'} title="Chi tiết vấn đề" record={dialog?.record ?? null} onClose={() => setDialog(null)} fields={[
        { key: 'title', label: 'Tiêu đề' }, { key: 'description', label: 'Mô tả' }, { key: 'severity', label: 'Mức độ', render: (row) => <EnumChip group="severity" value={row.severity} /> },
        { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> }, { key: 'requires_rework', label: 'Cần sửa lỗi' }, { key: 'requires_replanning', label: 'Cần lập kế hoạch lại' },
      ]} />
      <RecordFormDialog open={dialog?.type === 'issue' && dialog.mode === 'edit'} title="Chỉnh sửa vấn đề" submitLabel="Lưu thay đổi" layout="two-column" fields={dialog?.record ? issueFields(dialog.record) : []} onSubmit={(values) => dialog?.record ? updateIssue.mutateAsync({ issueId: dialog.record.id, values }) : undefined} onClose={() => setDialog(null)} />
    </>
  );
}

function DailyTimeline({ rows, loading, error, onRetry, onCreate, onView }: { rows: Record<string, any>[]; loading: boolean; error: unknown; onRetry: () => void; onCreate: () => void; onView: (record: Record<string, any>) => void }) {
  if (loading) return <div className="grid min-h-64 place-items-center rounded-lg border border-line bg-white"><LoadingSpinner /></div>;
  if (error) return <div className="flex items-center justify-between rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"><span>Không tải được nhật ký thi công.</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>;
  if (!rows.length) return <div className="rounded-lg border border-line bg-white p-6"><EmptyState title="Chưa có báo cáo thi công" action={<button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={onCreate}><Plus className="h-4 w-4" /> Báo cáo hôm nay</button>} /></div>;
  return (
    <div className="relative space-y-4 before:absolute before:bottom-6 before:left-[18px] before:top-6 before:w-px before:bg-blue-200">
      {rows.map((row) => {
        const groups = constructionReportViewModel(row);
        return <article key={row.id} className="relative pl-10"><span className="absolute left-2 top-6 z-10 h-5 w-5 rounded-full border-4 border-blue-100 bg-blue-600" />
          <div className="overflow-hidden rounded-lg border border-line bg-white shadow-sm">
            <div className="flex flex-col gap-3 border-b border-line px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <div><div className="flex flex-wrap items-center gap-2"><h2 className="font-semibold text-ink">{dateText(row.report_date)}</h2><span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-600">{row.weather || 'Chưa ghi thời tiết'}</span></div><p className="mt-1 text-xs text-slate-500">{row.manpower_summary || 'Chưa ghi nhân công'} · {row.material_usage_summary || 'Chưa ghi vật tư'}</p></div>
              <div className="flex items-center gap-3"><div className="text-right"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ báo cáo</p><p className="text-lg font-semibold text-blue-700">{Number(row.progress_percent ?? 0)}%</p></div><button className="rounded-md border border-line p-2 text-slate-600 hover:bg-slate-100" onClick={() => onView(row)} aria-label="Xem báo cáo"><Eye className="h-4 w-4" /></button></div>
            </div>
            <div className="grid divide-y divide-line lg:grid-cols-4 lg:divide-x lg:divide-y-0">
              <TimelineColumn title="Đã làm" items={groups.done} tone="green" empty="Chưa ghi hạng mục hoàn tất" />
              <TimelineColumn title="Đang làm" items={groups.current} tone="blue" empty="Không có hạng mục dở dang" />
              <TimelineColumn title="Sẽ làm" items={groups.next} tone="slate" empty="Chưa ghi kế hoạch tiếp theo" />
              <TimelineColumn title="Vấn đề phát sinh" items={groups.timelineProblems} tone="red" empty="Không có vấn đề" />
            </div>
          </div>
        </article>;
      })}
    </div>
  );
}

function TimelineColumn({ title, items, tone, empty }: { title: string; items: string[]; tone: 'green' | 'blue' | 'slate' | 'red'; empty: string }) {
  const tones = { green: 'bg-emerald-50 text-emerald-700', blue: 'bg-blue-50 text-blue-700', slate: 'bg-slate-100 text-slate-700', red: 'bg-red-50 text-red-700' };
  return <section className="min-w-0 p-4"><h3 className={`inline-flex rounded-md px-2 py-1 text-xs font-semibold uppercase tracking-wide ${tones[tone]}`}>{title} · {items.length}</h3>{items.length ? <ul className="mt-3 space-y-2">{items.map((item, index) => <li key={`${item}-${index}`} className="flex gap-2 text-sm leading-5 text-slate-700"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-slate-400" />{item}</li>)}</ul> : <p className="mt-3 text-sm italic text-slate-400">{empty}</p>}</section>;
}

function StatusCard({ icon, label, value, detail, tone = 'slate' }: { icon: ReactNode; label: string; value: ReactNode; detail: string; tone?: 'slate' | 'blue' | 'amber' | 'red' }) {
  const tones = { slate: 'border-line bg-white text-ink', blue: 'border-blue-200 bg-blue-50 text-blue-800', amber: 'border-amber-200 bg-amber-50 text-amber-800', red: 'border-red-200 bg-red-50 text-red-800' };
  return <div className={`rounded-lg border px-4 py-3 ${tones[tone]}`}><div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide opacity-75">{icon}{label}</div><div className="mt-2 flex items-end justify-between gap-3"><span className="text-2xl font-semibold">{value}</span><span className="max-w-[150px] text-right text-xs leading-4 opacity-75">{detail}</span></div></div>;
}

function countBy(rows: Record<string, any>[], key: string, value: string) { return rows.filter((row) => row[key] === value).length; }
