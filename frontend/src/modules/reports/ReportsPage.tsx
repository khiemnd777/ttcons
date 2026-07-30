import { lazy, Suspense } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Navigate, NavLink, useLocation, useNavigate, useParams } from 'react-router-dom';
import clsx from 'clsx';
import { ChevronRight, ShieldAlert } from 'lucide-react';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ReportOptions } from '../../lib/types/domain';
import type { OperationalReportKind } from './OperationalReportPanel';
import {
  isReportKey,
  reportGroupLabels,
  reportPath,
  type ReportGroup,
  type ReportKey,
  visibleReportDefinitions,
} from './reportRegistry';

const OperationalReportPanel = lazy(() => import('./OperationalReportPanel').then((module) => ({ default: module.OperationalReportPanel })));

export function ReportsPage() {
  const { reportKey } = useParams<{ reportKey?: string }>();
  const { hasAnyPermission } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const visibleReports = visibleReportDefinitions(hasAnyPermission);
  const activeReport = isReportKey(reportKey)
    ? visibleReports.find((definition) => definition.key === reportKey)
    : undefined;

  const options = useQuery({
    queryKey: ['report-options'],
    queryFn: () => get<ReportOptions>('/reports/options'),
    staleTime: 5 * 60 * 1000,
  });

  if (!visibleReports.length) {
    return (
      <div className="rounded-lg border border-amber-200 bg-amber-50 p-6 text-amber-800">
        <ShieldAlert className="h-6 w-6" />
        <h1 className="mt-3 text-lg font-semibold">Chưa có quyền xem báo cáo</h1>
        <p className="mt-1 text-sm">Liên hệ quản trị viên để được cấp quyền báo cáo phù hợp.</p>
      </div>
    );
  }

  if (!activeReport) return <Navigate to={reportPath(visibleReports[0].key)} replace />;

  const projectOptions = options.data?.projects ?? [];
  const searchSuffix = location.search;

  return (
    <>
      <PageHeader
        title="Báo cáo & phân tích"
        description="Chọn một báo cáo để tập trung phân tích; số liệu và CSV luôn dùng cùng phạm vi lọc và quyền truy cập."
      />

      <div className="mb-4 lg:hidden">
        <label className="block">
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Báo cáo đang xem</span>
          <select
            className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2.5 text-sm font-semibold text-ink outline-none focus:ring-2 focus:ring-blue-100"
            value={activeReport.key}
            onChange={(event) => navigate(`${reportPath(event.target.value as ReportKey)}${searchSuffix}`)}
          >
            {visibleReports.map((report) => <option key={report.key} value={report.key}>{report.label}</option>)}
          </select>
        </label>
      </div>

      <div className="grid gap-5 lg:grid-cols-[16rem_minmax(0,1fr)]">
        <aside className="hidden lg:block">
          <div className="sticky top-20 rounded-lg border border-line bg-white p-3 shadow-panel">
            <p className="px-2 pb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">Danh mục báo cáo</p>
            {(['operations', 'resources'] as ReportGroup[]).map((group) => {
              const reports = visibleReports.filter((report) => report.group === group);
              if (!reports.length) return null;
              return (
                <div key={group} className="mb-3 last:mb-0">
                  <p className="px-2 py-1 text-xs font-semibold text-slate-500">{reportGroupLabels[group]}</p>
                  <div className="space-y-1">
                    {reports.map((report) => (
                      <NavLink
                        key={report.key}
                        to={`${reportPath(report.key)}${searchSuffix}`}
                        className={({ isActive }) => clsx(
                          'group flex items-start justify-between gap-2 rounded-md px-2.5 py-2 text-sm transition-colors',
                          isActive ? 'bg-blue-50 font-semibold text-blue-700' : 'text-slate-600 hover:bg-slate-50 hover:text-ink',
                        )}
                      >
                        <span>{report.label}</span>
                        <ChevronRight className="mt-0.5 h-4 w-4 shrink-0 opacity-50 group-hover:opacity-100" />
                      </NavLink>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        </aside>

        <main className="min-w-0">
          <div className="mb-4 rounded-lg border border-line bg-white px-4 py-3 shadow-panel">
            <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">{reportGroupLabels[activeReport.group]}</p>
            <h1 className="mt-1 text-xl font-semibold text-ink">{activeReport.label}</h1>
            <p className="mt-1 text-sm text-slate-600">{activeReport.description}</p>
          </div>

          {options.isError ? (
            <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
              Không tải được danh sách công trình. Báo cáo vẫn hiển thị theo toàn bộ phạm vi được phép xem.
            </div>
          ) : null}

          <ActiveReport reportKey={activeReport.key} projectOptions={projectOptions} />
        </main>
      </div>
    </>
  );
}

function ActiveReport({ reportKey, projectOptions }: { reportKey: ReportKey; projectOptions: ReportOptions['projects'] }) {
  return (
    <Suspense fallback={<div className="rounded-lg border border-line bg-white p-8"><LoadingSpinner /></div>}>
      <OperationalReportPanel kind={reportKey as OperationalReportKind} projectOptions={projectOptions} />
    </Suspense>
  );
}
