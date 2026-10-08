import { useMemo } from 'react';
import { useIsFetching, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import clsx from 'clsx';
import { ArrowRight, ChevronLeft, ChevronRight, RefreshCw } from 'lucide-react';
import { PageHeader } from '../../components/PageHeader';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { reportPath, visibleReportDefinitions } from '../reports/reportRegistry';
import { convertDashboardPeriod, dashboardPeriodFromSearchParams, dashboardPeriodLabel, dashboardPeriodQuery, moveDashboardPeriod } from './dashboardPeriod';
import { DashboardError, DashboardLoading, ProjectSnapshotPanel } from './DashboardSummaryPanels';
import { DashboardActivityPanel } from './DashboardActivityPanel';
import type { DashboardData, DashboardPeriod, DashboardPeriodType } from './dashboardTypes';

export function DashboardPage() {
  const { user, hasAnyPermission } = useAuth();
  const actorID = user?.id ?? '';
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const period = useMemo(() => dashboardPeriodFromSearchParams(searchParams), [searchParams]);
  const periodQuery = dashboardPeriodQuery(period);
  const dashboard = useQuery({
    queryKey: ['dashboard-summary', actorID, periodQuery],
    queryFn: () => get<DashboardData>(`/reports/dashboard?${periodQuery}`),
    enabled: Boolean(actorID),
    retry: false,
  });
  const activityFetching = useIsFetching({ queryKey: ['dashboard-activities', actorID] });
  const refreshing = dashboard.isFetching || activityFetching > 0;
  const reports = visibleReportDefinitions(hasAnyPermission);

  function applyPeriod(nextPeriod: DashboardPeriod) {
    setSearchParams(new URLSearchParams(dashboardPeriodQuery(nextPeriod)), { replace: true });
  }

  function refreshDashboard() {
    void Promise.all([dashboard.refetch(), queryClient.invalidateQueries({ queryKey: ['dashboard-activities', actorID] })]);
  }

  return (
    <>
      <PageHeader title="Dashboard tổng hợp" description="Theo dõi toàn cảnh, diễn biến công trình và báo cáo vận hành trong phạm vi được phân quyền." horizontalFrom="xl" actions={(
        <>
          <DashboardPeriodToolbar period={period} canMoveForward={dashboard.data?.meta.can_move_forward ?? false} onChange={applyPeriod} />
          <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60" disabled={refreshing} onClick={refreshDashboard}>
            <RefreshCw className={clsx('h-4 w-4', refreshing && 'animate-spin')} /><span className="hidden sm:inline">{refreshing ? 'Đang làm mới' : 'Làm mới'}</span>
          </button>
        </>
      )} />
      {dashboard.isLoading ? <DashboardLoading /> : dashboard.isError || !dashboard.data ? (
        <DashboardError onRetry={() => void dashboard.refetch()} />
      ) : (
        <div className="space-y-5"><ProjectSnapshotPanel snapshot={dashboard.data.project_snapshot} /></div>
      )}
      <DashboardActivityPanel key={actorID} actorID={actorID} />
      <ReportLinksPanel reports={reports} />
    </>
  );
}

function DashboardPeriodToolbar({ period, canMoveForward, onChange }: { period: DashboardPeriod; canMoveForward: boolean; onChange: (period: DashboardPeriod) => void }) {
  return (
    <div className="flex flex-wrap items-center rounded-md border border-line bg-white p-1">
      <select aria-label="Loại kỳ Dashboard" className="rounded px-2 py-1.5 text-sm font-semibold text-slate-700 outline-none focus:ring-2 focus:ring-blue-100" value={period.periodType} onChange={(event) => onChange(convertDashboardPeriod(period, event.target.value as DashboardPeriodType))}>
        <option value="month">Tháng</option><option value="quarter">Quý</option><option value="year">Năm</option>
      </select>
      <span className="mx-1 h-5 w-px bg-line" />
      <button type="button" className="rounded p-1.5 text-slate-600 hover:bg-slate-100" aria-label="Kỳ trước" onClick={() => onChange(moveDashboardPeriod(period, -1))}><ChevronLeft className="h-4 w-4" /></button>
      <span className="min-w-28 px-2 text-center text-sm font-semibold text-ink">{dashboardPeriodLabel(period)}</span>
      <button type="button" className="rounded p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-35" aria-label="Kỳ sau" disabled={!canMoveForward} onClick={() => onChange(moveDashboardPeriod(period, 1))}><ChevronRight className="h-4 w-4" /></button>
    </div>
  );
}

function ReportLinksPanel({ reports }: { reports: ReturnType<typeof visibleReportDefinitions> }) {
  if (!reports.length) return null;
  return (
    <section className="mt-5 rounded-lg border border-line bg-white p-4 shadow-panel" aria-labelledby="dashboard-reports-title">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div><h2 id="dashboard-reports-title" className="text-base font-semibold text-ink">Báo cáo chuyên sâu</h2><p className="mt-1 text-xs leading-5 text-slate-500">Mở báo cáo nghiệp vụ để phân tích chi tiết hoặc xuất dữ liệu.</p></div>
        <Link className="inline-flex items-center gap-1 text-sm font-semibold text-blue-700 hover:text-blue-800" to="/reports">Tất cả báo cáo <ArrowRight className="h-4 w-4" /></Link>
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {reports.map((report) => (
          <Link key={report.key} className="group rounded-md border border-line p-3 hover:border-blue-200 hover:bg-blue-50" to={reportPath(report.key)}>
            <div className="flex items-start justify-between gap-3"><h3 className="text-sm font-semibold text-ink group-hover:text-blue-700">{report.label}</h3><ArrowRight className="h-4 w-4 shrink-0 text-slate-400 group-hover:text-blue-700" /></div>
            <p className="mt-2 text-xs leading-5 text-slate-500">{report.description}</p>
          </Link>
        ))}
      </div>
    </section>
  );
}
