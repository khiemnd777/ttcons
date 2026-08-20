import { useDeferredValue, useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Bug, ChevronLeft, ChevronRight, Copy, Eye, RefreshCw, Search, ServerCrash, ShieldAlert, X } from 'lucide-react';
import clsx from 'clsx';
import { get } from '../../lib/api/client';
import type { EventLogPage, EventLogRecord } from '../../lib/types/domain';
import { PageHeader } from '../../components/PageHeader';
import { StatCard } from '../../components/StatCard';
import { DataTable } from '../../components/DataTable';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import {
  eventLogCategoryLabel,
  eventLogCategoryTone,
  eventLogDateRange,
  eventLogDateTime,
  shortEventLogID,
  type EventLogDatePreset,
} from './eventLogPresentation';

const fieldClass = 'h-10 rounded-md border border-line bg-white px-3 text-sm text-slate-700 outline-none focus:ring-2 focus:ring-blue-100';

export function EventLogsPage() {
  const toast = useToast();
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('');
  const [httpStatus, setHTTPStatus] = useState('');
  const [method, setMethod] = useState('');
  const [route, setRoute] = useState('');
  const [actorEmail, setActorEmail] = useState('');
  const [projectId, setProjectId] = useState('');
  const [datePreset, setDatePreset] = useState<EventLogDatePreset>('7');
  const [customDateFrom, setCustomDateFrom] = useState('');
  const [customDateTo, setCustomDateTo] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [selectedId, setSelectedId] = useState('');
  const deferredSearch = useDeferredValue(search.trim());

  const dateRange = useMemo(
    () => eventLogDateRange(datePreset, customDateFrom, customDateTo),
    [customDateFrom, customDateTo, datePreset],
  );
  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    if (deferredSearch) params.set('search', deferredSearch);
    if (category) params.set('category', category);
    if (httpStatus) params.set('http_status', httpStatus);
    if (method) params.set('method', method);
    if (route) params.set('route', route);
    if (actorEmail) params.set('actor_email', actorEmail);
    if (projectId) params.set('project_id', projectId);
    if (dateRange.from) params.set('date_from', dateRange.from);
    if (dateRange.to) params.set('date_to', dateRange.to);
    params.set('page', String(page));
    params.set('page_size', String(pageSize));
    return params.toString();
  }, [actorEmail, category, dateRange.from, dateRange.to, deferredSearch, httpStatus, method, page, pageSize, projectId, route]);

  const logs = useQuery({
    queryKey: ['event-logs', queryString],
    queryFn: () => get<EventLogPage>(`/event-logs?${queryString}`),
    retry: false,
  });
  const detail = useQuery({
    queryKey: ['event-log', selectedId],
    queryFn: () => get<EventLogRecord>(`/event-logs/${selectedId}`),
    enabled: Boolean(selectedId),
    retry: false,
  });
  const data = logs.data;
  const rows = data?.items ?? [];
  const summary = data?.summary ?? { unexpected_5xx: 0, panic: 0, controlled_503: 0 };
  const options = data?.options ?? { categories: [], statuses: [], methods: [], routes: [], actors: [], projects: [] };
  const hasFilters = Boolean(search || category || httpStatus || method || route || actorEmail || projectId || datePreset !== '7');

  const updateFilter = (callback: () => void) => {
    callback();
    setPage(1);
  };
  const clearFilters = () => {
    setSearch('');
    setCategory('');
    setHTTPStatus('');
    setMethod('');
    setRoute('');
    setActorEmail('');
    setProjectId('');
    setDatePreset('7');
    setCustomDateFrom('');
    setCustomDateTo('');
    setPage(1);
  };
  const copyRequestID = async (requestID: string) => {
    try {
      await navigator.clipboard.writeText(requestID);
      toast.success('Đã sao chép mã tra cứu');
    } catch (error) {
      toast.error('Không sao chép được mã tra cứu', error);
    }
  };

  return (
    <>
      <PageHeader
        title="Nhật ký sự kiện"
        description="Tra cứu lỗi 5xx phát sinh khi người dùng thao tác trên backend bằng mã request và ngữ cảnh đã lọc dữ liệu nhạy cảm."
        actions={(
          <button
            className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
            disabled={logs.isFetching}
            onClick={() => void logs.refetch()}
            type="button"
          >
            <RefreshCw className={clsx('h-4 w-4', logs.isFetching && 'animate-spin')} />
            Làm mới
          </button>
        )}
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-3">
        <StatCard label="500 bất ngờ" value={summary.unexpected_5xx} sub="Lỗi backend chưa được kiểm soát" icon={<ServerCrash className="h-5 w-5 text-rose-600" />} />
        <StatCard label="Panic" value={summary.panic} sub="Lỗi runtime có stack trace" icon={<Bug className="h-5 w-5 text-violet-600" />} />
        <StatCard label="503 có kiểm soát" value={summary.controlled_503} sub="Backend chủ động từ chối tạm thời" icon={<ShieldAlert className="h-5 w-5 text-amber-600" />} />
      </div>

      <section className="rounded-lg border border-line bg-white shadow-panel">
        <div className="space-y-3 border-b border-line p-4">
          <div className="rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-xs leading-5 text-blue-800">
            Event Logs không lưu request body, token, cookie, query string hoặc địa chỉ IP thô. Log Caddy và lỗi gateway 502/504 không xuất hiện tại đây.
          </div>
          <label className="relative block w-full xl:max-w-2xl">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              className="h-10 w-full rounded-md border border-line bg-white pl-9 pr-9 text-sm outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Tìm request ID, mã lỗi, endpoint hoặc fingerprint..."
              value={search}
              onChange={(event) => updateFilter(() => setSearch(event.target.value))}
            />
            {search ? <button className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-slate-400 hover:bg-slate-100" onClick={() => updateFilter(() => setSearch(''))} type="button" aria-label="Xóa tìm kiếm"><X className="h-4 w-4" /></button> : null}
          </label>

          <div className="flex flex-wrap items-center gap-2">
            <select className={fieldClass} value={datePreset} onChange={(event) => updateFilter(() => setDatePreset(event.target.value as EventLogDatePreset))}>
              <option value="7">7 ngày gần đây</option><option value="30">30 ngày gần đây</option><option value="90">90 ngày gần đây</option><option value="custom">Tùy chọn ngày</option>
            </select>
            {datePreset === 'custom' ? <><input className={fieldClass} type="date" aria-label="Từ ngày" value={customDateFrom} onChange={(event) => updateFilter(() => setCustomDateFrom(event.target.value))} /><input className={fieldClass} type="date" aria-label="Đến ngày" value={customDateTo} onChange={(event) => updateFilter(() => setCustomDateTo(event.target.value))} /></> : null}
            <select className={fieldClass} value={category} onChange={(event) => updateFilter(() => setCategory(event.target.value))}>
              <option value="">Tất cả loại lỗi</option>{options.categories.map((value) => <option key={value} value={value}>{eventLogCategoryLabel(value)}</option>)}
            </select>
            <select className={fieldClass} value={httpStatus} onChange={(event) => updateFilter(() => setHTTPStatus(event.target.value))}>
              <option value="">Tất cả HTTP</option>{[...options.statuses].sort().map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
            <select className={fieldClass} value={method} onChange={(event) => updateFilter(() => setMethod(event.target.value))}>
              <option value="">Tất cả method</option>{[...options.methods].sort().map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
            <select className={fieldClass} value={route} onChange={(event) => updateFilter(() => setRoute(event.target.value))}>
              <option value="">Tất cả endpoint</option>{[...options.routes].sort().map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
            <select className={fieldClass} value={actorEmail} onChange={(event) => updateFilter(() => setActorEmail(event.target.value))}>
              <option value="">Tất cả người dùng</option>{[...options.actors].sort().map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
            <select className={fieldClass} value={projectId} onChange={(event) => updateFilter(() => setProjectId(event.target.value))}>
              <option value="">Tất cả công trình</option>{options.projects.map((project) => <option key={project.id} value={project.id}>{project.code} · {project.name}</option>)}
            </select>
            {hasFilters ? <button className="h-10 rounded-md px-3 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={clearFilters} type="button">Xóa bộ lọc</button> : null}
          </div>
        </div>

        <div className="border-b border-line px-4 py-3 text-sm font-semibold text-ink">{data?.total ?? 0} lỗi</div>
        <div className="p-4">
          {logs.isLoading ? <LoadingSpinner label="Đang tải nhật ký sự kiện..." /> : null}
          {logs.error ? <LoadError title="Không tải được nhật ký sự kiện" onRetry={() => void logs.refetch()} /> : null}
          {!logs.isLoading && !logs.error && !rows.length ? <EmptyState title={hasFilters ? 'Không tìm thấy lỗi phù hợp' : 'Không có lỗi trong 7 ngày gần đây'} action={hasFilters ? <button className="text-sm font-semibold text-blue-700 hover:underline" onClick={clearFilters} type="button">Xóa bộ lọc</button> : undefined} /> : null}
          {!logs.isLoading && !logs.error && rows.length ? <EventLogList rows={rows} onSelect={setSelectedId} /> : null}
        </div>
        {!logs.isLoading && !logs.error && data && data.total > 0 ? <Pagination page={data.page} pageSize={data.page_size} total={data.total} totalPages={data.total_pages} onPageChange={setPage} onPageSizeChange={(value) => { setPageSize(value); setPage(1); }} /> : null}
      </section>

      <Dialog open={Boolean(selectedId)} title="Chi tiết lỗi" onClose={() => setSelectedId('')} size="xl">
        {detail.isLoading ? <LoadingSpinner label="Đang tải chi tiết lỗi..." /> : null}
        {detail.error ? <LoadError title="Không tải được chi tiết lỗi" onRetry={() => void detail.refetch()} /> : null}
        {detail.data ? <EventLogDetail record={detail.data} onCopyRequestID={copyRequestID} /> : null}
      </Dialog>
    </>
  );
}

function EventLogList({ rows, onSelect }: { rows: EventLogRecord[]; onSelect: (id: string) => void }) {
  const columns = [
    { key: 'created_at', header: 'Thời gian', render: (row: EventLogRecord) => <span className="whitespace-nowrap text-xs">{eventLogDateTime(row.created_at)}</span> },
    { key: 'category', header: 'HTTP / Loại', render: (row: EventLogRecord) => <div className="space-y-1"><CategoryChip value={row.category} /><p className="text-xs font-semibold text-slate-500">HTTP {row.http_status}</p></div> },
    { key: 'route_template', header: 'Thao tác / Endpoint', render: (row: EventLogRecord) => <div className="max-w-md"><p className="font-mono text-xs font-semibold text-ink">{row.method} {row.route_template}</p><p className="mt-1 line-clamp-2 text-xs text-slate-500">{row.error_code}{row.message_preview ? ` · ${row.message_preview}` : ''}</p></div> },
    { key: 'actor_email', header: 'Người dùng', render: (row: EventLogRecord) => <div><p>{row.actor_email || 'Chưa xác định'}</p>{row.project_code ? <p className="mt-1 text-xs text-slate-500">{row.project_code}</p> : null}</div> },
    { key: 'request_id', header: 'Request ID', render: (row: EventLogRecord) => <code className="select-all text-xs text-slate-600" title={row.request_id}>{shortEventLogID(row.request_id)}</code> },
    { key: 'action', header: '', align: 'right' as const, render: (row: EventLogRecord) => <button className="rounded-md p-2 text-blue-700 hover:bg-blue-50" onClick={() => onSelect(row.id)} type="button" aria-label="Xem chi tiết lỗi"><Eye className="h-4 w-4" /></button> },
  ];
  return <><div className="hidden md:block"><DataTable rows={rows} columns={columns} /></div><div className="space-y-2 md:hidden">{rows.map((row) => <button key={row.id} className="w-full rounded-lg border border-line bg-white p-3 text-left hover:border-blue-200 hover:bg-blue-50/40" onClick={() => onSelect(row.id)} type="button"><div className="flex items-start justify-between gap-2"><CategoryChip value={row.category} /><span className="text-xs text-slate-500">{eventLogDateTime(row.created_at)}</span></div><p className="mt-2 break-all font-mono text-xs font-semibold text-ink">{row.method} {row.route_template}</p><p className="mt-2 text-xs text-slate-500">{row.actor_email || 'Chưa xác định'} · {shortEventLogID(row.request_id)}</p></button>)}</div></>;
}

function EventLogDetail({ record, onCopyRequestID }: { record: EventLogRecord; onCopyRequestID: (value: string) => void }) {
  return <div className="space-y-4"><div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 p-3 sm:flex-row sm:items-center sm:justify-between"><div><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Mã tra cứu</p><code className="mt-1 block select-all break-all text-sm text-ink">{record.request_id}</code></div><button className="inline-flex self-start items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => onCopyRequestID(record.request_id)} type="button"><Copy className="h-4 w-4" />Sao chép</button></div><DetailSection title="Tổng quan"><DetailValue label="Thời gian" value={eventLogDateTime(record.created_at)} /><DetailValue label="Loại lỗi" value={<CategoryChip value={record.category} />} /><DetailValue label="HTTP status" value={record.http_status} /><DetailValue label="Mã lỗi" value={record.error_code} /><DetailValue label="Method / route" value={<code className="break-all text-xs">{record.method} {record.route_template}</code>} /><DetailValue label="Thời lượng" value={`${record.duration_ms ?? 0} ms`} /><DetailValue label="Request class" value={record.request_class} /></DetailSection><DetailSection title="Người thực hiện"><DetailValue label="Email" value={record.actor_email || 'Chưa xác định'} /><DetailValue label="User ID" value={<code className="break-all text-xs">{record.actor_id || '-'}</code>} /><DetailValue label="Công trình" value={record.project_code ? `${record.project_code} · ${record.project_name ?? ''}` : 'Không xác định chắc chắn'} /></DetailSection><DetailSection title="Thông tin kỹ thuật"><DetailValue label="Error type" value={record.error_type || '-'} /><DetailValue label="Fingerprint" value={<code className="select-all break-all text-xs">{record.error_fingerprint}</code>} /><DetailValue label="Nội dung đã lọc" value={<pre className="whitespace-pre-wrap break-words text-xs leading-5">{record.internal_message || '-'}</pre>} full />{record.stack_trace ? <DetailValue label="Stack trace đã lọc" value={<pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded bg-slate-950 p-3 text-xs leading-5 text-slate-100">{record.stack_trace}</pre>} full /> : null}</DetailSection><p className="text-xs text-slate-500">Không lưu request body, header xác thực, query string hoặc địa chỉ IP thô.</p></div>;
}

function DetailSection({ title, children }: { title: string; children: ReactNode }) { return <section><h3 className="mb-2 text-sm font-semibold text-ink">{title}</h3><dl className="grid gap-3 md:grid-cols-2">{children}</dl></section>; }
function DetailValue({ label, value, full = false }: { label: string; value: ReactNode; full?: boolean }) { return <div className={clsx('rounded-md border border-line bg-slate-50 px-3 py-2', full && 'md:col-span-2')}><dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</dt><dd className="mt-1 break-words text-sm text-ink">{value}</dd></div>; }
function CategoryChip({ value }: { value: string }) { return <span className={clsx('inline-flex rounded-full border px-2 py-1 text-xs font-semibold', eventLogCategoryTone(value))}>{eventLogCategoryLabel(value)}</span>; }
function LoadError({ title, onRetry }: { title: string; onRetry: () => void }) { return <div className="flex flex-col gap-3 rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700 sm:flex-row sm:items-center sm:justify-between"><div className="flex items-start gap-2"><AlertTriangle className="mt-0.5 h-4 w-4" /><div><p className="font-semibold">{title}</p><p className="mt-1 text-xs">Vui lòng thử tải lại dữ liệu.</p></div></div><button className="rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold hover:bg-rose-100" onClick={onRetry} type="button">Thử lại</button></div>; }
function Pagination({ page, pageSize, total, totalPages, onPageChange, onPageSizeChange }: { page: number; pageSize: number; total: number; totalPages: number; onPageChange: (page: number) => void; onPageSizeChange: (size: number) => void }) { const start = (page - 1) * pageSize + 1; const end = Math.min(page * pageSize, total); return <div className="flex flex-col gap-3 border-t border-line bg-slate-50 px-4 py-3 text-xs font-medium text-slate-600 sm:flex-row sm:items-center sm:justify-between"><div className="flex items-center gap-2"><span>Hiển thị {start}-{end} / {total}</span><select className="rounded-md border border-line bg-white px-2 py-1" value={pageSize} onChange={(event) => onPageSizeChange(Number(event.target.value))}><option value={25}>25 dòng</option><option value={50}>50 dòng</option><option value={100}>100 dòng</option></select></div><div className="flex items-center justify-end gap-2"><button className="rounded-md border border-line bg-white p-1.5 disabled:opacity-40" disabled={page <= 1} onClick={() => onPageChange(page - 1)} type="button" aria-label="Trang trước"><ChevronLeft className="h-4 w-4" /></button><span>Trang {page} / {Math.max(totalPages, 1)}</span><button className="rounded-md border border-line bg-white p-1.5 disabled:opacity-40" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)} type="button" aria-label="Trang sau"><ChevronRight className="h-4 w-4" /></button></div></div>; }
