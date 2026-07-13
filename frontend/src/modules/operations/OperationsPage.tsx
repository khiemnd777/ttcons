import { useEffect, useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Eye, Plus } from 'lucide-react';
import { get, post, postForm, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { useToast } from '../../components/ToastProvider';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { dateText } from '../../lib/utils/format';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ConstructionWorkspace } from '../../lib/types/domain';
import { ConstructionReportDetailDialog } from './ConstructionReportDetailDialog';
import { ConstructionReportFormDialog, type ConstructionDailyReportCreatePayload } from './ConstructionReportFormDialog';
import { ConstructionReportEvidenceDialog } from './ConstructionReportEvidenceDialog';
import { constructionEvidenceFormData, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { constructionReportViewModel } from './constructionReportViewModel';
import { ConstructionWorkspacePanel } from './ConstructionWorkspacePanel';

type OperationsTab = 'timeline' | 'checks' | 'actions';

export function OperationsPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [openReport, setOpenReport] = useState(false);
  const [dialog, setDialog] = useState<{ type: 'report' | 'report-evidence'; mode: 'view' | 'edit'; record: Record<string, any> } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const canViewOperations = hasAnyPermission(['operation.view']);
  const canReport = hasAnyPermission(['operation.report']);
  const canViewFieldWorkspace = hasAnyPermission(['operation.view', 'quality.view', 'rework.view']);

  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);

  const reports = useQuery({ queryKey: ['daily-reports', projectId], queryFn: () => get<Record<string, any>[]>(`/projects/${projectId}/daily-reports`), enabled: Boolean(projectId) && canViewOperations });
  const construction = useQuery({ queryKey: ['construction-workspace', projectId], queryFn: () => get<ConstructionWorkspace>(`/projects/${projectId}/construction-workspace`), enabled: Boolean(projectId) && canViewFieldWorkspace, retry: false });
  const timelineRows = useMemo(() => [...(reports.data ?? [])].sort((left, right) => String(right.report_date ?? '').localeCompare(String(left.report_date ?? ''))), [reports.data]);
  const pendingChecks = (construction.data?.checks ?? []).filter((row) => row.status === 'pending').length;
  const activeActions = (construction.data?.actions ?? []).filter((row) => ['open', 'waiting_confirmation'].includes(row.status)).length;
  const tabs = useMemo(() => [
    canViewOperations ? { key: 'timeline' as const, label: 'Nhật ký' } : null,
    canViewFieldWorkspace ? { key: 'checks' as const, label: `Kiểm tra${pendingChecks ? ` (${pendingChecks})` : ''}` } : null,
    canViewFieldWorkspace ? { key: 'actions' as const, label: `Cần xử lý${activeActions ? ` (${activeActions})` : ''}` } : null,
  ].filter((tab): tab is { key: OperationsTab; label: string } => Boolean(tab)), [activeActions, canViewFieldWorkspace, canViewOperations, pendingChecks]);
  const requestedTab = normalizeTab(searchParams.get('tab'));
  const activeTab: OperationsTab = requestedTab && tabs.some((tab) => tab.key === requestedTab) ? requestedTab : tabs[0]?.key ?? 'timeline';

  useEffect(() => {
    if (!tabs.length || searchParams.get('tab') === activeTab) return;
    const next = new URLSearchParams(searchParams);
    next.set('tab', activeTab);
    setSearchParams(next, { replace: true });
  }, [activeTab, searchParams, setSearchParams, tabs]);

  const invalidateReports = () => {
    queryClient.invalidateQueries({ queryKey: ['daily-reports', projectId] });
    queryClient.invalidateQueries({ queryKey: ['project-evidence', projectId] });
    queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
  };
  const createReport = useMutation({
    mutationFn: async ({ values, uploads }: { values: ConstructionDailyReportCreatePayload; uploads: ConstructionEvidenceUploadDraft[] }) => {
      const report = await post<Record<string, any>>(`/projects/${projectId}/daily-reports`, values);
      const uploaded = await uploadReportEvidence(report.id, values.work_item_id, uploads);
      return { report: uploaded.latest ?? report, failed: uploaded.failed };
    },
    onSuccess: ({ failed }) => { invalidateReports(); setOpenReport(false); if (failed.length) toast.warn({ title: 'Đã lưu báo cáo', description: `Không tải được ${failed.length} tệp: ${failed.join(', ')}` }); else toast.success('Đã tạo báo cáo thi công hôm nay'); },
    onError: (error) => toast.error('Không tạo được báo cáo thi công', error),
  });
  const updateReport = useMutation({
    mutationFn: async ({ reportId, values, uploads }: { reportId: string; values: ConstructionDailyReportCreatePayload; uploads: ConstructionEvidenceUploadDraft[] }) => {
      const report = await put<Record<string, any>>(`/daily-reports/${reportId}`, values);
      const uploaded = await uploadReportEvidence(reportId, values.work_item_id, uploads);
      return { report: uploaded.latest ?? report, failed: uploaded.failed };
    },
    onSuccess: ({ failed }) => { invalidateReports(); setDialog(null); if (failed.length) toast.warn({ title: 'Đã cập nhật báo cáo', description: `Không tải được ${failed.length} tệp: ${failed.join(', ')}` }); else toast.success('Đã cập nhật báo cáo thi công'); },
    onError: (error) => toast.error('Không cập nhật được báo cáo thi công', error),
  });
  const addReportEvidence = useMutation({
    mutationFn: ({ reportId, workItemId, uploads }: { reportId: string; workItemId: string; uploads: ConstructionEvidenceUploadDraft[]; record: Record<string, any> }) => uploadReportEvidence(reportId, workItemId, uploads),
    onSuccess: ({ failed, latest }, variables) => { invalidateReports(); if (failed.length) { if (latest) setDialog({ type: 'report', mode: 'view', record: latest }); toast.warn({ title: 'Chưa tải đủ minh chứng', description: `Không tải được ${failed.length} tệp: ${failed.join(', ')}` }); return; } setDialog({ type: 'report', mode: 'view', record: latest ?? variables.record }); toast.success('Đã thêm minh chứng vào báo cáo'); },
    onError: (error) => toast.error('Không thêm được minh chứng', error),
  });

  async function uploadReportEvidence(reportId: string, workItemId: string, uploads: ConstructionEvidenceUploadDraft[]) {
    const failed: string[] = [];
    let latest: Record<string, any> | null = null;
    for (const draft of uploads) {
      try {
        latest = await postForm<Record<string, any>>(`/daily-reports/${reportId}/evidence/upload`, constructionEvidenceFormData(draft, workItemId));
      } catch (error) {
        failed.push(`${draft.name || draft.file.name} (${error instanceof Error ? error.message : 'không xác định'})`);
      }
    }
    return { failed, latest };
  }

  function changeTab(tab: OperationsTab) {
    const next = new URLSearchParams(searchParams);
    next.set('tab', tab);
    setSearchParams(next);
  }

  return <>
    <PageHeader title="Thi công" description="Ghi nhật ký, kiểm tra chất lượng và xử lý việc phát sinh ngay tại công trường." actions={<>{!global ? <ProjectBackLink projectId={projectId} /> : null}{global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}{canReport ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={() => setOpenReport(true)}><Plus className="h-4 w-4" /> Báo cáo hôm nay</button> : null}</>} />
    <div className="mb-4 flex gap-1 overflow-x-auto rounded-lg border border-line bg-white p-1" role="tablist" aria-label="Không gian thi công">{tabs.map((tab) => <button key={tab.key} role="tab" aria-selected={activeTab === tab.key} className={`shrink-0 rounded-md px-4 py-2 text-sm font-semibold transition ${activeTab === tab.key ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:bg-slate-100'}`} onClick={() => changeTab(tab.key)}>{tab.label}</button>)}</div>
    {activeTab === 'timeline' && canViewOperations ? <DailyTimeline rows={timelineRows} loading={reports.isLoading} error={reports.error} onRetry={() => reports.refetch()} onCreate={() => setOpenReport(true)} onView={(record) => setDialog({ type: 'report', mode: 'view', record })} /> : null}
    {activeTab === 'checks' && canViewFieldWorkspace ? <ConstructionWorkspacePanel projectId={projectId} mode="checks" /> : null}
    {activeTab === 'actions' && canViewFieldWorkspace ? <ConstructionWorkspacePanel projectId={projectId} mode="actions" /> : null}
    <ConstructionReportFormDialog open={openReport} projectId={projectId} onSubmit={(values, uploads) => createReport.mutateAsync({ values, uploads })} onClose={() => setOpenReport(false)} />
    <ConstructionReportDetailDialog open={dialog?.type === 'report' && dialog.mode === 'view'} projectId={projectId} record={dialog?.record ?? null} canManage={canReport} onEdit={() => dialog?.record && setDialog({ type: 'report', mode: 'edit', record: dialog.record })} onAddEvidence={() => dialog?.record && setDialog({ type: 'report-evidence', mode: 'edit', record: dialog.record })} onClose={() => setDialog(null)} />
    <ConstructionReportFormDialog open={dialog?.type === 'report' && dialog.mode === 'edit'} projectId={projectId} mode="edit" record={dialog?.record ?? null} onSubmit={(values, uploads) => dialog?.record ? updateReport.mutateAsync({ reportId: dialog.record.id, values, uploads }) : Promise.resolve()} onClose={() => setDialog(null)} />
    <ConstructionReportEvidenceDialog open={dialog?.type === 'report-evidence'} projectId={projectId} record={dialog?.record ?? null} onSubmit={(workItemId, uploads) => dialog?.record ? addReportEvidence.mutateAsync({ reportId: dialog.record.id, workItemId, uploads, record: dialog.record }) : Promise.resolve()} onClose={() => setDialog(null)} />
  </>;
}

function normalizeTab(value: string | null): OperationsTab | null {
  if (value === 'quality') return 'checks';
  if (value === 'issues') return 'actions';
  return value === 'timeline' || value === 'checks' || value === 'actions' ? value : null;
}

function DailyTimeline({ rows, loading, error, onRetry, onCreate, onView }: { rows: Record<string, any>[]; loading: boolean; error: unknown; onRetry: () => void; onCreate: () => void; onView: (record: Record<string, any>) => void }) {
  if (loading) return <LoadingSpinner label="Đang tải nhật ký thi công..." />;
  if (error) return <div className="flex items-center justify-between rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"><span>Không tải được nhật ký thi công.</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>;
  if (!rows.length) return <div className="rounded-lg border border-line bg-white p-6"><EmptyState title="Chưa có báo cáo thi công" action={<button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={onCreate}><Plus className="h-4 w-4" /> Báo cáo hôm nay</button>} /></div>;
  return <div className="relative space-y-4 before:absolute before:bottom-6 before:left-[18px] before:top-6 before:w-px before:bg-blue-200">{rows.map((row) => { const groups = constructionReportViewModel(row); return <article key={row.id} className="relative pl-10"><span className="absolute left-2 top-6 z-10 h-5 w-5 rounded-full border-4 border-blue-100 bg-blue-600" /><div className="overflow-hidden rounded-lg border border-line bg-white shadow-sm"><div className="flex flex-col gap-3 border-b border-line px-4 py-3 sm:flex-row sm:items-center sm:justify-between"><div><div className="flex flex-wrap items-center gap-2"><h2 className="font-semibold text-ink">{dateText(row.report_date)}</h2><span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-600">{row.weather || 'Chưa ghi thời tiết'}</span></div><p className="mt-1 text-xs text-slate-500">{row.manpower_summary || 'Chưa ghi nhân công'} · {row.material_usage_summary || 'Chưa ghi vật tư'}</p></div><div className="flex items-center gap-3"><div className="text-right"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ báo cáo</p><p className="text-lg font-semibold text-blue-700">{Number(row.progress_percent ?? 0)}%</p></div><button className="rounded-md border border-line p-2 text-slate-600 hover:bg-slate-100" onClick={() => onView(row)} aria-label="Xem báo cáo"><Eye className="h-4 w-4" /></button></div></div><div className="grid divide-y divide-line lg:grid-cols-4 lg:divide-x lg:divide-y-0"><TimelineColumn title="Đã làm" items={groups.done} tone="green" empty="Chưa ghi hạng mục hoàn tất" /><TimelineColumn title="Đang làm" items={groups.current} tone="blue" empty="Không có hạng mục dở dang" /><TimelineColumn title="Sẽ làm" items={groups.next} tone="slate" empty="Chưa ghi kế hoạch tiếp theo" /><TimelineColumn title="Vấn đề phát sinh" items={groups.timelineProblems} tone="red" empty="Không có vấn đề" /></div></div></article>; })}</div>;
}

function TimelineColumn({ title, items, tone, empty }: { title: string; items: string[]; tone: 'green' | 'blue' | 'slate' | 'red'; empty: string }) {
  const tones = { green: 'bg-emerald-50 text-emerald-700', blue: 'bg-blue-50 text-blue-700', slate: 'bg-slate-100 text-slate-700', red: 'bg-red-50 text-red-700' };
  return <section className="min-w-0 p-4"><h3 className={`inline-flex rounded-md px-2 py-1 text-xs font-semibold uppercase tracking-wide ${tones[tone]}`}>{title} · {items.length}</h3>{items.length ? <ul className="mt-3 space-y-2">{items.map((item, index) => <li key={`${item}-${index}`} className="flex gap-2 text-sm leading-5 text-slate-700"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-slate-400" />{item}</li>)}</ul> : <p className="mt-3 text-sm italic text-slate-400">{empty}</p>}</section>;
}
