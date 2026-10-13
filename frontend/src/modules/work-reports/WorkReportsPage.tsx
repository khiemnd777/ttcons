import { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { parseBMGSRevision } from '../_shared/bmgsSourceNavigation';
import { isRecordID, RecordUnavailable } from '../_shared/recordNavigation';
import {
  Download,
  Eye,
  FilePlus2,
  RefreshCw,
  Search,
} from 'lucide-react';
import clsx from 'clsx';
import { DataTable } from '../../components/DataTable';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import { downloadNativeReportExcel, nativeReportListExcelPath } from '../_shared/bmgsExport';
import { useAuth } from '../../lib/auth/AuthContext';
import { dateText } from '../../lib/utils/format';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import type { WorkReportDetail, WorkReportListItem, WorkReportSummary } from '../../lib/types/domain';
import { WorkReportReviewDialog, WorkReportStatusBadge } from './WorkReportReviewDialog';

type WorkReportScope = 'mine' | 'source' | 'review' | 'all';
type ExportFormat = 'pdf' | 'xlsx' | 'csv';

const statusOptions: Array<[string, string]> = [
  ['', 'Tất cả trạng thái'],
  ['draft', 'Nháp tại nguồn'],
  ['submitted', 'Chờ xác nhận'],
  ['needs_changes', 'Yêu cầu bổ sung'],
  ['acknowledged', 'Đã xác nhận'],
  ['recorded', 'Nhật ký lịch sử'],
];

export function WorkReportsPage() {
  const { user, hasPermission } = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const toast = useToast();
  const canReview = hasPermission('work_report.review') || hasPermission('work_report.manage');
  const canManage = hasPermission('work_report.manage');
  const canCreate = hasPermission('work_report.create') && hasPermission('operation.report') && hasPermission('operation.view');
  const canOpenSource = hasPermission('operation.view');
  const canExport = hasPermission('work_report.export');
  const availableScopes = useMemo<Array<{ key: WorkReportScope; label: string }>>(() => [
    { key: 'mine', label: 'Đã gửi' },
    ...(canOpenSource ? [{ key: 'source' as const, label: 'Nháp tại nguồn' }] : []),
    ...(canReview ? [{ key: 'review' as const, label: 'Chờ tôi xem' }] : []),
    ...(canManage ? [{ key: 'all' as const, label: 'Tất cả được phép xem' }] : []),
  ], [canManage, canOpenSource, canReview]);
  const [scope, setScope] = useState<WorkReportScope>(() => {
    const requested = normalizeScope(searchParams.get('scope'));
    return requested && availableScopes.some((item) => item.key === requested) ? requested : availableScopes[0]?.key ?? 'mine';
  });
  const [draftFilters, setDraftFilters] = useState({ projectId: '', status: '', dateFrom: '', dateTo: '', search: '' });
  const [filters, setFilters] = useState(draftFilters);
  const [selectedSourceID, setSelectedSourceID] = useState('');
  const [exportFormat, setExportFormat] = useState<ExportFormat>('xlsx');
  const [exporting, setExporting] = useState(false);
  const [exportingReports, setExportingReports] = useState(false);
  const filtersRef = useRef<HTMLElement | null>(null);
  const navigateFilterValidation = useValidationNavigation(filtersRef);

  const listQueryString = useMemo(() => {
    const params = new URLSearchParams({ scope });
    if (filters.projectId) params.set('project_id', filters.projectId);
    if (filters.status) params.set('status', filters.status);
    if (filters.dateFrom) params.set('date_from', filters.dateFrom);
    if (filters.dateTo) params.set('date_to', filters.dateTo);
    if (filters.search) params.set('search', filters.search.trim());
    return params.toString();
  }, [filters, scope]);

  const summaryQueryString = useMemo(() => {
    const params = new URLSearchParams();
    if (filters.projectId) params.set('project_id', filters.projectId);
    if (filters.status) params.set('status', filters.status);
    if (filters.dateFrom) params.set('date_from', filters.dateFrom);
    if (filters.dateTo) params.set('date_to', filters.dateTo);
    if (filters.search) params.set('search', filters.search.trim());
    return params.toString();
  }, [filters]);

  const reports = useQuery({
    queryKey: ['work-reports', scope, filters],
    queryFn: () => get<WorkReportListItem[]>(`/work-reports?${listQueryString}`),
    retry: false,
  });

  const reportSummary = useQuery({
    queryKey: ['work-report-summary', user?.id, 'page', filters],
    queryFn: () => get<WorkReportSummary>(`/work-reports/summary${summaryQueryString ? `?${summaryQueryString}` : ''}`),
    retry: false,
  });

  const detail = useQuery({
    queryKey: ['work-report-detail', selectedSourceID, user?.id],
    queryFn: () => { if (!isRecordID(selectedSourceID)) throw new RecordUnavailable(); return get<WorkReportDetail>(`/work-reports/daily-reports/${selectedSourceID}`); },
    enabled: Boolean(selectedSourceID),
    retry: false,
  });

  useEffect(() => {
    const requestedScope = normalizeScope(searchParams.get('scope'));
    if (requestedScope && availableScopes.some((item) => item.key === requestedScope) && requestedScope !== scope) {
      setScope(requestedScope);
    } else if (!availableScopes.some((item) => item.key === scope)) {
      setScope(availableScopes[0]?.key ?? 'mine');
    }
    const requestedReport = searchParams.get('report') ?? '';
    if (requestedReport !== selectedSourceID) setSelectedSourceID(requestedReport);
  }, [availableScopes, scope, searchParams, selectedSourceID]);

  function applyFilters() {
    if (draftFilters.dateFrom && draftFilters.dateTo && draftFilters.dateFrom > draftFilters.dateTo) {
      toast.warn({ title: 'Khoảng ngày chưa hợp lệ', description: 'Ngày bắt đầu phải trước hoặc bằng ngày kết thúc.' });
      navigateFilterValidation('date_to');
      return;
    }
    setFilters({ ...draftFilters, search: draftFilters.search.trim() });
  }

  function resetFilters() {
    const empty = { projectId: '', status: '', dateFrom: '', dateTo: '', search: '' };
    setDraftFilters(empty);
    setFilters(empty);
  }

  function changeScope(nextScope: WorkReportScope) {
    setScope(nextScope);
    setFilters((current) => ({ ...current, status: nextScope === 'source' ? 'draft' : current.status === 'draft' ? '' : current.status }));
    setDraftFilters((current) => ({ ...current, status: nextScope === 'source' ? 'draft' : current.status === 'draft' ? '' : current.status }));
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.set('scope', nextScope);
      next.delete('report');
      return next;
    }, { replace: true });
  }

  async function exportRegister() {
    setExporting(true);
    try {
      const params = new URLSearchParams(listQueryString);
      params.set('format', exportFormat);
      const blob = await downloadBlob(`/work-reports/export?${params.toString()}`);
      saveBlob(blob, `so-dang-ky-bao-cao.${exportFormat}`);
      toast.success(`Đã xuất sổ báo cáo dạng ${exportLabel(exportFormat)}`);
    } catch (error) {
      toast.error('Không xuất được sổ báo cáo', error);
    } finally {
      setExporting(false);
    }
  }

  const rows = reports.data ?? [];

  async function exportReports() {
    if (!canExport || exportingReports || reports.isFetching || reports.isError || !rows.length) return;
    setExportingReports(true);
    try {
      await downloadNativeReportExcel(nativeReportListExcelPath('/work-reports', listQueryString));
      toast.success('Đã xuất báo cáo Excel theo bộ lọc đang áp dụng');
    } catch (error) { toast.error('Không xuất được báo cáo Excel', error); }
    finally { setExportingReports(false); }
  }

  function openReport(row: WorkReportListItem) {
    if (!row.work_report_id) {
      navigate(`/projects/${row.project_id}/operations?tab=timeline&report=${encodeURIComponent(row.source_id)}`);
      return;
    }
    setSelectedSourceID(row.source_id);
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.set('scope', scope);
      next.set('report', row.source_id);
      next.delete('revision');
      return next;
    }, { replace: true });
  }

  function closeReport() {
    setSelectedSourceID('');
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.delete('report');
      next.delete('revision');
      return next;
    }, { replace: true });
  }

  return (
    <div className="space-y-5">
      <header className="mb-3 border-b border-line pb-2">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Trao đổi công việc có xác nhận</p>
            <div className="mt-1 min-w-0 md:flex md:items-baseline md:gap-2">
              <h1 className="max-w-full truncate text-lg font-semibold leading-6 tracking-normal text-ink md:max-w-[42%] md:shrink-0" title="Báo cáo">Báo cáo</h1>
              <span className="hidden text-xs text-slate-300 md:inline" aria-hidden="true">·</span>
              <p className="line-clamp-2 min-w-0 text-xs leading-[18px] text-slate-500 md:block md:truncate" title="Nhân sự lập báo cáo từ dữ liệu công việc thực tế, gửi đúng người quản lý và lưu lịch sử từng lần phản hồi.">
                Nhân sự lập báo cáo từ dữ liệu công việc thực tế, gửi đúng người quản lý và lưu lịch sử từng lần phản hồi.
              </p>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {canExport ? <button type="button" disabled={exportingReports || reports.isFetching || reports.isError || !rows.length} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 disabled:opacity-50" onClick={() => void exportReports()}><Download className="h-4 w-4" />{exportingReports ? 'Đang xuất…' : 'Xuất báo cáo'}</button> : null}
            {canExport ? (
              <ExportControl format={exportFormat} exporting={exporting} onFormatChange={setExportFormat} onExport={() => void exportRegister()} />
            ) : null}
            {canCreate ? (
              <button
                type="button"
                className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
                onClick={() => navigate('/operations?tab=timeline')}
              >
                <FilePlus2 className="h-4 w-4" />
                Lập báo cáo tại Thi công
              </button>
            ) : null}
          </div>
        </div>
      </header>

      <section ref={filtersRef} className="rounded-lg border border-line bg-white p-4 shadow-panel" aria-label="Bộ lọc báo cáo">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(15rem,1.4fr)_minmax(10rem,1fr)_9rem_9rem_minmax(14rem,1.2fr)_auto] xl:items-end">
          <AsyncProjectPicker
            value={draftFilters.projectId}
            onChange={(projectId) => setDraftFilters((current) => ({ ...current, projectId }))}
            endpoint="/work-reports/projects"
            queryKey="work-report-project-options"
            emptyLabel="Tất cả công trình được phép xem"
          />
          <FilterSelect label="Trạng thái" value={draftFilters.status} options={statusOptions} onChange={(status) => setDraftFilters((current) => ({ ...current, status }))} />
          <FilterInput fieldKey="date_from" type="date" label="Từ ngày" value={draftFilters.dateFrom} onChange={(dateFrom) => setDraftFilters((current) => ({ ...current, dateFrom }))} />
          <FilterInput fieldKey="date_to" type="date" label="Đến ngày" value={draftFilters.dateTo} onChange={(dateTo) => setDraftFilters((current) => ({ ...current, dateTo }))} />
          <FilterInput label="Tìm kiếm" placeholder="Công trình, người lập, nội dung..." value={draftFilters.search} onChange={(search) => setDraftFilters((current) => ({ ...current, search }))} onEnter={applyFilters} />
          <div className="flex gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={resetFilters}>Đặt lại</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={applyFilters}>
              <Search className="h-4 w-4" /> Áp dụng
            </button>
          </div>
        </div>
      </section>

      <section>
        <div className="mb-3 flex gap-1 overflow-x-auto border-b border-line" role="tablist" aria-label="Phạm vi báo cáo">
          {availableScopes.map((item) => (
            <button
              key={item.key}
              type="button"
              role="tab"
              aria-selected={scope === item.key}
              className={clsx('whitespace-nowrap border-b-2 px-3 py-2 text-sm font-semibold', scope === item.key ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink')}
              onClick={() => changeScope(item.key)}
            >
              {item.label}
              <span className="ml-1 text-xs text-slate-400" aria-label={`${item.label}: ${scopeCount(reportSummary.data, item.key, reportSummary.isLoading, reportSummary.isError)}`}>
                ({scopeCount(reportSummary.data, item.key, reportSummary.isLoading, reportSummary.isError)})
              </span>
            </button>
          ))}
        </div>

        <DataTable<WorkReportListItem>
          rows={rows}
          loading={reports.isLoading}
          error={reports.error}
          onRetry={() => void reports.refetch()}
          empty={scope === 'review' ? 'Không có báo cáo đang chờ anh xem' : 'Chưa có báo cáo trong phạm vi này'}
          emptyAction={canCreate && (scope === 'mine' || scope === 'source') ? (
            <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => navigate('/operations?tab=timeline')}>Lập báo cáo tại Thi công</button>
          ) : undefined}
          pageSize={10}
          defaultSort={{ key: 'report_date', direction: 'desc' }}
          getRowKey={(row) => row.source_id}
          renderMobileCard={(row) => <WorkReportCard row={row} onOpen={() => openReport(row)} />}
          columns={[
            { key: 'action', header: '', action: true, render: (row) => <button type="button" className="inline-flex items-center gap-1 text-sm font-semibold text-blue-700 hover:text-blue-900" onClick={() => openReport(row)}><Eye className="h-4 w-4" />{row.work_report_id ? <><span className="sr-only">Xem </span>Chi tiết</> : 'Mở tại Thi công'}</button> },
            { key: 'report_date', header: 'Ngày báo cáo', sortable: true, render: (row) => <span className="font-semibold text-ink">{dateText(row.report_date)}</span> },
            { key: 'project_name', header: 'Công trình', sortable: true, render: (row) => <div><p className="font-semibold text-ink">{row.project_name}</p><p className="text-xs text-slate-500">{row.project_code}</p></div> },
            { key: 'author_name', header: 'Người lập', sortable: true, render: (row) => identityReferenceText(row.author_name, row.author_account_deleted) },
            { key: 'recipient_name', header: 'Gửi đến', sortable: true, render: (row) => row.recipient_name ? identityReferenceText(row.recipient_name, row.recipient_account_deleted) : 'Chưa gửi' },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <WorkReportStatusBadge status={row.status} /> },
            { key: 'current_revision_no', header: 'Lần gửi', align: 'right', sortable: true, render: (row) => Number(row.current_revision_no) > 0 ? `Lần ${row.current_revision_no}` : '—' },
          ]}
        />
      </section>

      <WorkReportReviewDialog
        key={`${selectedSourceID}:${searchParams.get('revision') ?? ''}:${user?.id}`}
        open={Boolean(selectedSourceID)}
        detail={detail.data ?? null}
        loading={detail.isLoading || detail.isFetching}
        initialRevisionNo={parseBMGSRevision(searchParams.get('revision'))}
        error={detail.error}
        userID={user?.id ?? ''}
        canReview={canReview}
        canManage={canManage}
        canOpenSource={canOpenSource}
        canExport={canExport}
        onRefresh={async () => {
          await Promise.all([
            detail.refetch(),
            queryClient.invalidateQueries({ queryKey: ['work-reports'] }),
            queryClient.invalidateQueries({ queryKey: ['work-report-summary'] }),
          ]);
        }}
        onClose={closeReport}
      />
    </div>
  );
}

function WorkReportCard({ row, onOpen }: { row: WorkReportListItem; onOpen: () => void }) {
  return (
    <article className="rounded-lg border border-line bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div><p className="text-xs font-semibold text-slate-500">{dateText(row.report_date)}</p><h3 className="mt-1 font-semibold text-ink">{row.project_name}</h3><p className="text-xs text-slate-500">{row.project_code}</p></div>
        <WorkReportStatusBadge status={row.status} />
      </div>
      <dl className="mt-4 grid grid-cols-2 gap-3 text-sm"><div><dt className="text-xs text-slate-500">Người lập</dt><dd className="font-medium text-ink">{identityReferenceText(row.author_name, row.author_account_deleted)}</dd></div><div><dt className="text-xs text-slate-500">Gửi đến</dt><dd className="font-medium text-ink">{row.recipient_name ? identityReferenceText(row.recipient_name, row.recipient_account_deleted) : 'Chưa gửi'}</dd></div></dl>
      <button type="button" className="mt-4 inline-flex items-center gap-1 text-sm font-semibold text-blue-700" onClick={onOpen}><Eye className="h-4 w-4" /> {row.work_report_id ? 'Xem chi tiết' : 'Mở tại Thi công'}</button>
    </article>
  );
}

function FilterInput({ fieldKey, label, value, onChange, type = 'text', placeholder, onEnter }: { fieldKey?: string; label: string; value: string; onChange: (value: string) => void; type?: 'text' | 'date'; placeholder?: string; onEnter?: () => void }) {
  return <label data-validation-key={fieldKey} className="block scroll-mt-24 rounded-md"><span className="text-xs font-semibold text-slate-600">{label}</span><input name={fieldKey} type={type} className="mt-1 h-10 w-full rounded-md border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') onEnter?.(); }} /></label>;
}

function FilterSelect({ label, value, options, onChange }: { label: string; value: string; options: Array<[string, string]>; onChange: (value: string) => void }) {
  return <label className="block"><span className="text-xs font-semibold text-slate-600">{label}</span><select className="mt-1 h-10 w-full rounded-md border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={value} onChange={(event) => onChange(event.target.value)}>{options.map(([optionValue, optionLabel]) => <option key={optionValue} value={optionValue}>{optionLabel}</option>)}</select></label>;
}

function ExportControl({ format, exporting, onFormatChange, onExport }: { format: ExportFormat; exporting: boolean; onFormatChange: (format: ExportFormat) => void; onExport: () => void }) {
  return <div className="inline-flex overflow-hidden rounded-md border border-blue-200 bg-white"><label className="sr-only" htmlFor="work-report-export-format">Định dạng xuất sổ báo cáo</label><select id="work-report-export-format" className="border-r border-blue-100 bg-white px-2 py-2 text-sm font-semibold text-blue-700 outline-none" value={format} onChange={(event) => onFormatChange(event.target.value as ExportFormat)} disabled={exporting}><option value="pdf">PDF</option><option value="xlsx">Excel</option><option value="csv">CSV</option></select><button type="button" className="inline-flex items-center gap-2 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" disabled={exporting} onClick={onExport}>{exporting ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}{exporting ? 'Đang xuất…' : 'Xuất sổ đăng ký'}</button></div>;
}

function saveBlob(blob: Blob, filename: string) {
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = filename;
  anchor.click();
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
}

function exportLabel(format: ExportFormat) {
  return format === 'xlsx' ? 'Excel' : format.toUpperCase();
}

function normalizeScope(value: string | null): WorkReportScope | null {
  return value === 'mine' || value === 'source' || value === 'review' || value === 'all' ? value : null;
}

function scopeCount(summary: WorkReportSummary | undefined, scope: WorkReportScope, loading: boolean, error: boolean) {
  if (loading) return '…';
  if (error) return '—';
  return summary?.scopes?.[scope] ?? 0;
}
