import { useMemo, useState, type ReactNode } from 'react';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import clsx from 'clsx';
import { ArrowRight, ChevronLeft, ChevronRight, RefreshCw, Search } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable, type Column } from '../../components/DataTable';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { statusLabel } from '../../lib/utils/status';
import { reportPath, visibleReportDefinitions } from '../reports/reportRegistry';
import {
  convertDashboardPeriod,
  dashboardPeriodFromSearchParams,
  dashboardPeriodLabel,
  dashboardPeriodQuery,
  moveDashboardPeriod,
} from './dashboardPeriod';
import { CashflowPanel, DashboardError, DashboardLoading, ProjectSnapshotPanel } from './DashboardSummaryPanels';
import type { DashboardData, DashboardPeriod, DashboardPeriodType, DashboardProjectItem, DashboardProjectsPage } from './dashboardTypes';

type ProjectSort = 'name' | 'status' | 'progress' | 'net';
type SortDirection = 'asc' | 'desc';

export function DashboardPage() {
  const { hasAnyPermission } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const period = useMemo(() => dashboardPeriodFromSearchParams(searchParams), [searchParams]);
  const periodQuery = dashboardPeriodQuery(period);
  const [searchDraft, setSearchDraft] = useState('');
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [sort, setSort] = useState<ProjectSort>('name');
  const [direction, setDirection] = useState<SortDirection>('asc');

  const dashboard = useQuery({
    queryKey: ['dashboard-summary', periodQuery],
    queryFn: () => get<DashboardData>(`/reports/dashboard?${periodQuery}`),
    retry: false,
  });

  const portfolioFilters = useMemo(() => ({ periodQuery, search, status, sort, direction }), [direction, periodQuery, search, sort, status]);
  const projects = useInfiniteQuery({
    queryKey: ['dashboard-projects', portfolioFilters],
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams(periodQuery);
      params.set('limit', '25');
      params.set('sort', sort);
      params.set('direction', direction);
      if (search) params.set('search', search);
      if (status) params.set('status', status);
      if (pageParam) params.set('cursor', String(pageParam));
      return get<DashboardProjectsPage>(`/reports/dashboard/projects?${params.toString()}`);
    },
    getNextPageParam: (lastPage) => lastPage.has_more ? lastPage.next_cursor : undefined,
    retry: false,
  });

  const projectRows = useMemo(() => projects.data?.pages.flatMap((page) => page.items) ?? [], [projects.data?.pages]);
  const totalProjects = projects.data?.pages[0]?.total ?? 0;
  const financeVisible = dashboard.data?.capabilities.finance_visible === true;
  const projectColumns = useMemo(() => dashboardProjectColumns(financeVisible), [financeVisible]);
  const reports = visibleReportDefinitions(hasAnyPermission);
  const refreshing = dashboard.isFetching || projects.isFetching;

  function applyPeriod(nextPeriod: DashboardPeriod) {
    setSearchParams(new URLSearchParams(dashboardPeriodQuery(nextPeriod)), { replace: true });
  }

  function refreshDashboard() {
    void Promise.all([dashboard.refetch(), projects.refetch()]);
  }

  function applySearch() {
    setSearch(searchDraft.trim());
  }

  function resetProjectFilters() {
    setSearchDraft('');
    setSearch('');
    setStatus('');
    setSort('name');
    setDirection('asc');
  }

  return (
    <>
      <PageHeader
        title="Dashboard tổng hợp"
        description="Theo dõi toàn cảnh công trình, dòng tiền và các báo cáo quản trị trong phạm vi được phân quyền. Công việc cá nhân tiếp tục nằm tại Bàn làm việc."
        horizontalFrom="xl"
        actions={(
          <>
            <DashboardPeriodToolbar period={period} canMoveForward={dashboard.data?.meta.can_move_forward ?? false} onChange={applyPeriod} />
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={refreshing}
              onClick={refreshDashboard}
            >
              <RefreshCw className={clsx('h-4 w-4', refreshing && 'animate-spin')} />
              <span className="hidden sm:inline">{refreshing ? 'Đang làm mới' : 'Làm mới'}</span>
            </button>
          </>
        )}
      />

      {dashboard.isLoading ? (
        <DashboardLoading />
      ) : dashboard.isError || !dashboard.data ? (
        <DashboardError onRetry={() => void dashboard.refetch()} />
      ) : (
        <div className="space-y-5">
          <ProjectSnapshotPanel snapshot={dashboard.data.project_snapshot} />
          <CashflowPanel data={dashboard.data} />
        </div>
      )}

      <section className="mt-5" aria-labelledby="dashboard-project-list-title">
        <div className="mb-3 flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h2 id="dashboard-project-list-title" className="text-base font-semibold text-ink">Danh mục công trình</h2>
            <p className="mt-1 text-xs leading-5 text-slate-500">Dữ liệu theo phạm vi được xem; số tiền chỉ xuất hiện ở từng công trình có quyền tài chính tương ứng.</p>
          </div>
          <ProjectFilters
            search={searchDraft}
            status={status}
            sort={sort}
            direction={direction}
            financeVisible={financeVisible}
            onSearchChange={setSearchDraft}
            onSearch={applySearch}
            onStatusChange={setStatus}
            onSortChange={setSort}
            onDirectionChange={setDirection}
            onReset={resetProjectFilters}
          />
        </div>

        <DataTable
          rows={projectRows}
          columns={projectColumns}
          loading={projects.isLoading}
          error={projects.error}
          onRetry={() => void projects.refetch()}
          empty="Không có công trình phù hợp với bộ lọc"
          getRowKey={(row) => row.id}
          renderMobileCard={(row) => <ProjectMobileCard project={row} financeVisible={financeVisible} />}
        />
        {!projects.isLoading && !projects.isError ? (
          <div className="mt-3 flex flex-col gap-2 text-xs text-slate-500 sm:flex-row sm:items-center sm:justify-between">
            <span>Đã tải {integerText(projectRows.length)} / {integerText(totalProjects)} công trình</span>
            {projects.hasNextPage ? (
              <button
                type="button"
                className="rounded-md border border-line bg-white px-3 py-2 font-semibold text-blue-700 hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={projects.isFetchingNextPage}
                onClick={() => void projects.fetchNextPage()}
              >
                {projects.isFetchingNextPage ? 'Đang tải thêm...' : 'Tải thêm công trình'}
              </button>
            ) : null}
          </div>
        ) : null}
      </section>

      <ReportLinksPanel reports={reports} />
    </>
  );
}

function DashboardPeriodToolbar({ period, canMoveForward, onChange }: { period: DashboardPeriod; canMoveForward: boolean; onChange: (period: DashboardPeriod) => void }) {
  return (
    <div className="flex flex-wrap items-center rounded-md border border-line bg-white p-1">
      <select
        aria-label="Loại kỳ Dashboard"
        className="rounded px-2 py-1.5 text-sm font-semibold text-slate-700 outline-none focus:ring-2 focus:ring-blue-100"
        value={period.periodType}
        onChange={(event) => onChange(convertDashboardPeriod(period, event.target.value as DashboardPeriodType))}
      >
        <option value="month">Tháng</option>
        <option value="quarter">Quý</option>
        <option value="year">Năm</option>
      </select>
      <span className="mx-1 h-5 w-px bg-line" />
      <button type="button" className="rounded p-1.5 text-slate-600 hover:bg-slate-100" aria-label="Kỳ trước" onClick={() => onChange(moveDashboardPeriod(period, -1))}><ChevronLeft className="h-4 w-4" /></button>
      <span className="min-w-28 px-2 text-center text-sm font-semibold text-ink">{dashboardPeriodLabel(period)}</span>
      <button
        type="button"
        className="rounded p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-35"
        aria-label="Kỳ sau"
        disabled={!canMoveForward}
        onClick={() => onChange(moveDashboardPeriod(period, 1))}
      ><ChevronRight className="h-4 w-4" /></button>
    </div>
  );
}

function ProjectFilters({
  search,
  status,
  sort,
  direction,
  financeVisible,
  onSearchChange,
  onSearch,
  onStatusChange,
  onSortChange,
  onDirectionChange,
  onReset,
}: {
  search: string;
  status: string;
  sort: ProjectSort;
  direction: SortDirection;
  financeVisible: boolean;
  onSearchChange: (value: string) => void;
  onSearch: () => void;
  onStatusChange: (value: string) => void;
  onSortChange: (value: ProjectSort) => void;
  onDirectionChange: (value: SortDirection) => void;
  onReset: () => void;
}) {
  return (
    <div className="grid gap-2 sm:grid-cols-[minmax(15rem,1fr)_auto_auto] xl:flex">
      <div className="flex min-w-0">
        <label className="relative min-w-0 flex-1">
          <span className="sr-only">Tìm công trình</span>
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            className="w-full rounded-l-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:z-10 focus:ring-2 focus:ring-blue-100"
            placeholder="Mã hoặc tên công trình"
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Enter') onSearch(); }}
          />
        </label>
        <button type="button" className="rounded-r-md border border-l-0 border-line bg-white px-3 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onSearch}>Tìm</button>
      </div>
      <select aria-label="Lọc trạng thái công trình" className="rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-700 outline-none focus:ring-2 focus:ring-blue-100" value={status} onChange={(event) => onStatusChange(event.target.value)}>
        <option value="">Tất cả trạng thái</option>
        {['planning', 'in_progress', 'blocked', 'acceptance', 'handover', 'completed'].map((value) => <option key={value} value={value}>{statusLabel(value)}</option>)}
      </select>
      <div className="flex">
        <select aria-label="Sắp xếp danh mục công trình" className="min-w-0 rounded-l-md border border-line bg-white px-3 py-2 text-sm text-slate-700 outline-none focus:z-10 focus:ring-2 focus:ring-blue-100" value={sort} onChange={(event) => onSortChange(event.target.value as ProjectSort)}>
          <option value="name">Tên công trình</option>
          <option value="status">Trạng thái</option>
          <option value="progress">Tiến độ</option>
          {financeVisible ? <option value="net">Dòng tiền ròng</option> : null}
        </select>
        <select aria-label="Chiều sắp xếp" className="rounded-r-md border border-l-0 border-line bg-white px-2 py-2 text-sm text-slate-700 outline-none focus:z-10 focus:ring-2 focus:ring-blue-100" value={direction} onChange={(event) => onDirectionChange(event.target.value as SortDirection)}>
          <option value="asc">Tăng</option>
          <option value="desc">Giảm</option>
        </select>
      </div>
      <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100" onClick={onReset}>Đặt lại</button>
    </div>
  );
}

function dashboardProjectColumns(financeVisible: boolean): Column<DashboardProjectItem>[] {
  const columns: Column<DashboardProjectItem>[] = [
    {
      key: 'action',
      header: '',
      action: true,
      className: 'w-12',
      render: (project) => (
        <Link className="inline-flex rounded-md border border-line p-2 text-slate-600 hover:bg-blue-50 hover:text-blue-700" to={`/projects/${project.id}`} title={`Mở ${project.name}`} aria-label={`Mở công trình ${project.name}`}><ArrowRight className="h-4 w-4" /></Link>
      ),
    },
    { key: 'code', header: 'Mã', render: (project) => <span className="font-medium text-slate-600">{project.code || '-'}</span> },
    { key: 'name', header: 'Công trình', render: (project) => <span className="font-semibold text-ink">{project.name}</span> },
    { key: 'status', header: 'Trạng thái', render: (project) => <StatusBadge status={project.status} /> },
    { key: 'progress_percent', header: 'Tiến độ', align: 'right', render: (project) => `${decimalText(project.progress_percent)}%` },
    { key: 'open_issues', header: 'Vấn đề mở', align: 'right', render: (project) => integerText(project.open_issues) },
    { key: 'pending_approvals', header: 'Chờ duyệt', align: 'right', render: (project) => integerText(project.pending_approvals) },
  ];
  if (financeVisible) {
    columns.push(
      { key: 'inflow_amount', header: 'Thu', align: 'right', render: (project) => <ProjectMoney project={project} value={project.inflow_amount} /> },
      { key: 'outflow_amount', header: 'Chi', align: 'right', render: (project) => <ProjectMoney project={project} value={project.outflow_amount} /> },
      { key: 'net_amount', header: 'Ròng', align: 'right', render: (project) => <ProjectMoney project={project} value={project.net_amount} signed /> },
    );
  }
  return columns;
}

function ProjectMoney({ project, value, signed = false }: { project: DashboardProjectItem; value?: number; signed?: boolean }) {
  if (!project.finance_visible || value === undefined) return <span className="text-slate-400" title="Không có quyền xem tài chính công trình này">—</span>;
  return <span className={signed ? moneyTone(value) : 'font-medium text-slate-700'}><CurrencyText value={value} /></span>;
}

function ProjectMobileCard({ project, financeVisible }: { project: DashboardProjectItem; financeVisible: boolean }) {
  return (
    <article className="rounded-lg border border-line bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0"><p className="text-xs font-medium text-slate-500">{project.code || 'Chưa có mã'}</p><h3 className="mt-1 text-sm font-semibold text-ink">{project.name}</h3></div>
        <Link className="shrink-0 rounded-md border border-line p-2 text-slate-600" to={`/projects/${project.id}`} aria-label={`Mở công trình ${project.name}`}><ArrowRight className="h-4 w-4" /></Link>
      </div>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2"><StatusBadge status={project.status} /><span className="text-sm font-semibold text-slate-700">{decimalText(project.progress_percent)}%</span></div>
      <div className="mt-3 grid grid-cols-2 gap-2 text-xs"><MobileValue label="Vấn đề mở" value={integerText(project.open_issues)} /><MobileValue label="Chờ duyệt" value={integerText(project.pending_approvals)} /></div>
      {financeVisible ? (
        <div className="mt-2 grid grid-cols-3 gap-2 text-xs">
          <MobileValue label="Thu" value={project.finance_visible ? <CurrencyText value={project.inflow_amount} /> : '—'} />
          <MobileValue label="Chi" value={project.finance_visible ? <CurrencyText value={project.outflow_amount} /> : '—'} />
          <MobileValue label="Ròng" value={project.finance_visible ? <span className={moneyTone(project.net_amount ?? 0)}><CurrencyText value={project.net_amount} /></span> : '—'} />
        </div>
      ) : null}
    </article>
  );
}

function MobileValue({ label, value }: { label: string; value: ReactNode }) {
  return <div className="rounded-md bg-slate-50 p-2"><p className="text-slate-500">{label}</p><div className="mt-1 font-semibold text-slate-700">{value}</div></div>;
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

function moneyTone(value: number) {
  if (value < 0) return 'font-semibold text-rose-700';
  if (value > 0) return 'font-semibold text-emerald-700';
  return 'font-semibold text-slate-600';
}

function integerText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 0 }).format(value);
}

function decimalText(value: number) {
  return new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 }).format(value);
}
