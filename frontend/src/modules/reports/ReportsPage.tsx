import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import { useIsFetching } from '@tanstack/react-query';
import { Navigate, NavLink, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import clsx from 'clsx';
import { ChevronRight, ShieldAlert } from 'lucide-react';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { OperationalReportKind } from './OperationalReportPanel';
import { ReportExportButton, ReportScopeActions } from './ReportFilters';
import { ReportProjectPicker } from './ReportProjectPicker';
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
  const [searchParams, setSearchParams] = useSearchParams();
  const appliedProjectId = searchParams.get('project_id') ?? '';
  const [draftProjectId, setDraftProjectId] = useState(appliedProjectId);
  const [exporting, setExporting] = useState(false);
  const toast = useToast();
  const visibleReports = visibleReportDefinitions(hasAnyPermission);
  const activeReport = isReportKey(reportKey)
    ? visibleReports.find((definition) => definition.key === reportKey)
    : undefined;
  const searchSuffix = searchParams.size ? `?${searchParams.toString()}` : '';

  useEffect(() => {
    setDraftProjectId(appliedProjectId);
  }, [appliedProjectId]);

  const activeReportFetching = useIsFetching({
    queryKey: ['operational-report', activeReport?.key ?? 'unavailable', appliedProjectId || 'all'],
  }) > 0;

  const exportQueryString = useMemo(() => {
    const params = new URLSearchParams();
    if (appliedProjectId) params.set('project_id', appliedProjectId);
    return params.toString();
  }, [appliedProjectId]);

  function applyProjectScope() {
    const params = new URLSearchParams(searchParams);
    if (draftProjectId) params.set('project_id', draftProjectId);
    else params.delete('project_id');
    setSearchParams(params, { replace: true });
  }

  function resetProjectScope() {
    const params = new URLSearchParams(searchParams);
    params.delete('project_id');
    setDraftProjectId('');
    setSearchParams(params, { replace: true });
  }

  async function exportCSV() {
    if (!activeReport) return;
    setExporting(true);
    try {
      const blob = await downloadBlob(`/reports/${activeReport.key}/export${exportQueryString ? `?${exportQueryString}` : ''}`);
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = `${activeReport.fileName}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success(`Đã xuất CSV ${activeReport.label.toLocaleLowerCase('vi-VN')}`);
    } catch (error) {
      toast.error(`Không xuất được CSV ${activeReport.label.toLocaleLowerCase('vi-VN')}`, error);
    } finally {
      setExporting(false);
    }
  }

  if (!visibleReports.length) {
    return (
      <div className="rounded-lg border border-amber-200 bg-amber-50 p-6 text-amber-800">
        <ShieldAlert className="h-6 w-6" />
        <h1 className="mt-3 text-lg font-semibold">Chưa có quyền xem báo cáo</h1>
        <p className="mt-1 text-sm">Liên hệ quản trị viên để được cấp quyền báo cáo phù hợp.</p>
      </div>
    );
  }

  if (!activeReport) return <Navigate to={`${reportPath(visibleReports[0].key)}${searchSuffix}`} replace />;

  return (
    <>
      <header className="mb-4 border-b border-line pb-4 md:mb-5 md:pb-5">
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_36rem] xl:items-end xl:gap-8">
          <div className="min-w-0">
            <h1 className="text-xl font-semibold tracking-normal text-ink md:text-2xl">Báo cáo & phân tích</h1>
            <p className="mt-1 max-w-3xl text-sm leading-6 text-slate-600">
              <span className="md:hidden">Số liệu và CSV dùng cùng phạm vi lọc và quyền truy cập.</span>
              <span className="hidden md:inline">Chọn một báo cáo để tập trung phân tích; số liệu và CSV luôn dùng cùng phạm vi lọc và quyền truy cập.</span>
            </p>
          </div>

          <section className="min-w-0 border-t border-line pt-4 xl:border-0 xl:pt-0" aria-label="Bộ lọc công trình báo cáo">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <ReportProjectPicker
                value={draftProjectId}
                onChange={setDraftProjectId}
              />
              <ReportScopeActions
                applying={activeReportFetching}
                resetDisabled={activeReportFetching || (!draftProjectId && !appliedProjectId)}
                onApply={applyProjectScope}
                onReset={resetProjectScope}
              />
            </div>
          </section>
        </div>
      </header>

      <div className="mb-4 lg:hidden">
        <label className="block">
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Loại báo cáo</span>
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
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">{reportGroupLabels[activeReport.group]}</p>
                <h1 className="mt-1 text-xl font-semibold text-ink">{activeReport.label}</h1>
                <p className="mt-1 max-w-3xl text-sm leading-6 text-slate-600">{activeReport.basis}</p>
              </div>
              <ReportExportButton exporting={exporting} disabled={activeReportFetching} onExport={() => void exportCSV()} />
            </div>
          </div>

          <ActiveReport reportKey={activeReport.key} projectId={appliedProjectId} />
        </main>
      </div>
    </>
  );
}

function ActiveReport({ reportKey, projectId }: { reportKey: ReportKey; projectId: string }) {
  return (
    <Suspense fallback={<div className="rounded-lg border border-line bg-white p-8"><LoadingSpinner /></div>}>
      <OperationalReportPanel kind={reportKey as OperationalReportKind} projectId={projectId} />
    </Suspense>
  );
}
