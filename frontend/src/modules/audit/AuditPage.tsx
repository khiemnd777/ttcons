import { useDeferredValue, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ChevronLeft, ChevronRight, Clock3, List, RefreshCw, Search, X } from 'lucide-react';
import clsx from 'clsx';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { Timeline } from '../../components/Timeline';
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { IdentityReferenceLabel } from '../../components/IdentityReferenceLabel';
import type { AuditLogPage, AuditLogRecord } from '../../lib/types/domain';
import { enumLabel } from '../../lib/utils/status';

type ViewMode = 'list' | 'timeline';
type DatePreset = 'all' | '7' | '30' | '90' | 'custom';

const fieldClass = 'h-10 rounded-md border border-line bg-white px-3 text-sm text-slate-700 outline-none focus:ring-2 focus:ring-blue-100';

export function AuditPage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const basePath = scoped && id ? `/projects/${id}/audit-logs` : '/audit-logs';
  const [search, setSearch] = useState('');
  const [eventType, setEventType] = useState('');
  const [targetType, setTargetType] = useState('');
  const [actorName, setActorName] = useState('');
  const [projectId, setProjectId] = useState('');
  const [datePreset, setDatePreset] = useState<DatePreset>('all');
  const [customDateFrom, setCustomDateFrom] = useState('');
  const [customDateTo, setCustomDateTo] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [viewMode, setViewMode] = useState<ViewMode>('list');
  const [selected, setSelected] = useState<AuditLogRecord | null>(null);
  const deferredSearch = useDeferredValue(search.trim());

  const dateRange = useMemo(
    () => auditDateRange(datePreset, customDateFrom, customDateTo),
    [customDateFrom, customDateTo, datePreset],
  );
  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    if (deferredSearch) params.set('search', deferredSearch);
    if (eventType) params.set('event_type', eventType);
    if (targetType) params.set('target_type', targetType);
    if (actorName) params.set('actor_name', actorName);
    if (!scoped && projectId) params.set('project_id', projectId);
    if (dateRange.from) params.set('date_from', dateRange.from);
    if (dateRange.to) params.set('date_to', dateRange.to);
    params.set('page', String(page));
    params.set('page_size', String(pageSize));
    return params.toString();
  }, [actorName, dateRange.from, dateRange.to, deferredSearch, eventType, page, pageSize, projectId, scoped, targetType]);

  const logs = useQuery({
    queryKey: ['audit', basePath, queryString],
    queryFn: () => get<AuditLogPage>(`${basePath}?${queryString}`),
    enabled: Boolean(!scoped || id),
    retry: false,
  });
  const data = logs.data;
  const rows = data?.items ?? [];
  const options = data?.options ?? { event_types: [], target_types: [], actors: [], projects: [] };
  const hasFilters = Boolean(search || eventType || targetType || actorName || projectId || datePreset !== 'all');

  const updateFilter = (callback: () => void) => {
    callback();
    setPage(1);
  };
  const clearFilters = () => {
    setSearch('');
    setEventType('');
    setTargetType('');
    setActorName('');
    setProjectId('');
    setDatePreset('all');
    setCustomDateFrom('');
    setCustomDateTo('');
    setPage(1);
  };

  return (
    <>
      <PageHeader
        title={scoped ? 'Audit công trình' : 'Audit toàn hệ thống'}
        description={scoped
          ? 'Tra cứu ai đã thao tác, thời điểm, đối tượng và dữ liệu chi tiết của thay đổi trong công trình.'
          : 'Tra cứu lịch sử thao tác trên các công trình thuộc phạm vi anh có quyền xem.'}
        actions={(
          <>
            {scoped && id ? <ProjectBackLink projectId={id} /> : null}
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
              disabled={logs.isFetching}
              onClick={() => void logs.refetch()}
              type="button"
            >
              <RefreshCw className={clsx('h-4 w-4', logs.isFetching && 'animate-spin')} />
              Làm mới
            </button>
          </>
        )}
      />

      <section className="rounded-lg border border-line bg-white shadow-panel">
        <div className="space-y-3 border-b border-line p-4">
          <label className="relative block w-full xl:max-w-2xl">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              className="h-10 w-full rounded-md border border-line bg-white pl-9 pr-9 text-sm outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Tìm mô tả, người thực hiện hoặc ID..."
              value={search}
              onChange={(event) => updateFilter(() => setSearch(event.target.value))}
            />
            {search ? (
              <button className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-slate-400 hover:bg-slate-100" onClick={() => updateFilter(() => setSearch(''))} type="button" aria-label="Xóa tìm kiếm">
                <X className="h-4 w-4" />
              </button>
            ) : null}
          </label>

          <div className="flex flex-wrap items-center gap-2">
            <select className={fieldClass} value={datePreset} onChange={(event) => updateFilter(() => setDatePreset(event.target.value as DatePreset))}>
              <option value="all">Tất cả thời gian</option>
              <option value="7">7 ngày gần đây</option>
              <option value="30">30 ngày gần đây</option>
              <option value="90">90 ngày gần đây</option>
              <option value="custom">Tùy chọn ngày</option>
            </select>
            {datePreset === 'custom' ? (
              <>
                <input className={fieldClass} type="date" aria-label="Từ ngày" value={customDateFrom} onChange={(event) => updateFilter(() => setCustomDateFrom(event.target.value))} />
                <input className={fieldClass} type="date" aria-label="Đến ngày" value={customDateTo} onChange={(event) => updateFilter(() => setCustomDateTo(event.target.value))} />
              </>
            ) : null}
            {!scoped ? (
              <select className={fieldClass} value={projectId} onChange={(event) => updateFilter(() => setProjectId(event.target.value))}>
                <option value="">Tất cả công trình</option>
                {options.projects.map((project) => <option key={project.id} value={project.id}>{project.code} · {project.name}</option>)}
              </select>
            ) : null}
            <select className={fieldClass} value={eventType} onChange={(event) => updateFilter(() => setEventType(event.target.value))}>
              <option value="">Tất cả sự kiện</option>
              {sortEnumOptions(options.event_types, 'event_type').map((value) => <option key={value} value={value}>{enumLabel('event_type', value)}</option>)}
            </select>
            <select className={fieldClass} value={targetType} onChange={(event) => updateFilter(() => setTargetType(event.target.value))}>
              <option value="">Tất cả đối tượng</option>
              {sortEnumOptions(options.target_types, 'target_type').map((value) => <option key={value} value={value}>{enumLabel('target_type', value)}</option>)}
            </select>
            <select className={fieldClass} value={actorName} onChange={(event) => updateFilter(() => setActorName(event.target.value))}>
              <option value="">Tất cả người thực hiện</option>
              {[...options.actors].sort((left, right) => left.localeCompare(right, 'vi')).map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
            {hasFilters ? (
              <button className="h-10 rounded-md px-3 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={clearFilters} type="button">
                Xóa bộ lọc
              </button>
            ) : null}
          </div>
        </div>

        <div className="flex flex-col gap-3 border-b border-line px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm font-semibold text-ink">{data?.total ?? 0} sự kiện</p>
          <div className="inline-flex self-start rounded-md border border-line bg-slate-50 p-1">
            <ViewButton active={viewMode === 'list'} label="Danh sách" icon={<List className="h-4 w-4" />} onClick={() => setViewMode('list')} />
            <ViewButton active={viewMode === 'timeline'} label="Dòng thời gian" icon={<Clock3 className="h-4 w-4" />} onClick={() => setViewMode('timeline')} />
          </div>
        </div>

        <div className="p-4">
          {logs.isLoading ? <LoadingSpinner label="Đang tải sự kiện audit..." /> : null}
          {logs.error ? (
            <div className="flex flex-col gap-3 rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="font-semibold">Không tải được dữ liệu audit</p>
                <p className="mt-1 text-xs">Vui lòng thử tải lại dữ liệu.</p>
              </div>
              <button className="rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold hover:bg-rose-100" onClick={() => void logs.refetch()} type="button">Thử lại</button>
            </div>
          ) : null}
          {!logs.isLoading && !logs.error && !rows.length ? (
            <EmptyState
              title={hasFilters ? 'Không tìm thấy sự kiện phù hợp' : 'Chưa có sự kiện audit'}
              action={hasFilters ? <button className="text-sm font-semibold text-blue-700 hover:underline" onClick={clearFilters} type="button">Xóa bộ lọc</button> : undefined}
            />
          ) : null}
          {!logs.isLoading && !logs.error && rows.length && viewMode === 'timeline' ? (
            <Timeline events={rows} onSelect={(event) => setSelected(event as AuditLogRecord)} />
          ) : null}
          {!logs.isLoading && !logs.error && rows.length && viewMode === 'list' ? (
            <AuditList rows={rows} scoped={scoped} onSelect={setSelected} />
          ) : null}
        </div>

        {!logs.isLoading && !logs.error && data && data.total > 0 ? (
          <AuditPagination
            page={data.page}
            pageSize={data.page_size}
            total={data.total}
            totalPages={data.total_pages}
            onPageChange={setPage}
            onPageSizeChange={(value) => {
              setPageSize(value);
              setPage(1);
            }}
          />
        ) : null}
      </section>

      <RecordDetailDialog
        open={Boolean(selected)}
        title={selected ? enumLabel('event_type', selected.event_type) : 'Chi tiết sự kiện'}
        record={selected}
        onClose={() => setSelected(null)}
        fields={[
          { key: 'created_at', label: 'Thời gian', render: (row) => auditDateTime(row.created_at) },
          { key: 'project_name', label: 'Công trình', hidden: () => scoped, render: (row) => `${row.project_code ?? '-'} · ${row.project_name ?? '-'}` },
          { key: 'actor_name', label: 'Người thực hiện', render: (row) => <IdentityReferenceLabel name={row.actor_name} deleted={row.actor_account_deleted} /> },
          { key: 'event_type', label: 'Sự kiện', render: (row) => <EnumChip group="event_type" value={row.event_type} /> },
          { key: 'target_type', label: 'Đối tượng', render: (row) => <EnumChip group="target_type" value={row.target_type} /> },
          { key: 'target_id', label: 'ID đối tượng', render: (row) => <code className="select-all break-all text-xs">{row.target_id ?? '-'}</code> },
          { key: 'description', label: 'Mô tả' },
          { key: 'metadata_json', label: 'Dữ liệu kèm theo' },
          { key: 'id', label: 'ID sự kiện', render: (row) => <code className="select-all break-all text-xs">{row.id}</code> },
        ]}
      />
    </>
  );
}

function AuditList({ rows, scoped, onSelect }: { rows: AuditLogRecord[]; scoped: boolean; onSelect: (row: AuditLogRecord) => void }) {
  const columns = [
    { key: 'created_at', header: 'Thời gian', render: (row: AuditLogRecord) => <span className="whitespace-nowrap text-xs leading-5">{auditDateTime(row.created_at)}</span> },
    ...(!scoped ? [{ key: 'project_name', header: 'Công trình', render: (row: AuditLogRecord) => <div className="min-w-40"><p className="font-semibold text-ink">{row.project_code ?? '-'}</p><p className="text-xs text-slate-500">{row.project_name ?? '-'}</p></div> }] : []),
    { key: 'event_type', header: 'Sự kiện', render: (row: AuditLogRecord) => <EnumChip group="event_type" value={row.event_type} /> },
    { key: 'actor_name', header: 'Người thực hiện', render: (row: AuditLogRecord) => <IdentityReferenceLabel name={row.actor_name} deleted={row.actor_account_deleted} /> },
    { key: 'target_type', header: 'Đối tượng', render: (row: AuditLogRecord) => <div className="min-w-32 space-y-1"><EnumChip group="target_type" value={row.target_type} />{row.target_id ? <p className="font-mono text-[11px] text-slate-400">{shortID(row.target_id)}</p> : null}</div> },
    { key: 'description', header: 'Nội dung', render: (row: AuditLogRecord) => <p className="max-w-xl text-sm leading-5 text-slate-700">{row.description || '-'}</p> },
    { key: 'action', header: '', align: 'right' as const, render: (row: AuditLogRecord) => <button className="rounded-md p-2 text-blue-700 hover:bg-blue-50" onClick={() => onSelect(row)} type="button" aria-label="Xem chi tiết"><ChevronRight className="h-4 w-4" /></button> },
  ];
  return (
    <>
      <div className="hidden md:block"><DataTable rows={rows} columns={columns} /></div>
      <div className="space-y-2 md:hidden">
        {rows.map((row) => (
          <button key={row.id} className="w-full rounded-lg border border-line bg-white p-3 text-left hover:border-blue-200 hover:bg-blue-50/40" onClick={() => onSelect(row)} type="button">
            <div className="flex items-start justify-between gap-2"><EnumChip group="event_type" value={row.event_type} /><span className="text-xs text-slate-500">{auditDateTime(row.created_at)}</span></div>
            <p className="mt-2 text-sm leading-5 text-slate-700">{row.description || '-'}</p>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
              <IdentityReferenceLabel name={row.actor_name} deleted={row.actor_account_deleted} /><EnumChip group="target_type" value={row.target_type} />
              {!scoped && row.project_code ? <span>{row.project_code}</span> : null}
            </div>
          </button>
        ))}
      </div>
    </>
  );
}

function AuditPagination({ page, pageSize, total, totalPages, onPageChange, onPageSizeChange }: { page: number; pageSize: number; total: number; totalPages: number; onPageChange: (page: number) => void; onPageSizeChange: (size: number) => void }) {
  const start = (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);
  return (
    <div className="flex flex-col gap-3 border-t border-line bg-slate-50 px-4 py-3 text-xs font-medium text-slate-600 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-2"><span>Hiển thị {start}-{end} / {total}</span><select className="rounded-md border border-line bg-white px-2 py-1" value={pageSize} onChange={(event) => onPageSizeChange(Number(event.target.value))}><option value={25}>25 dòng</option><option value={50}>50 dòng</option><option value={100}>100 dòng</option></select></div>
      <div className="flex items-center justify-end gap-2"><button className="rounded-md border border-line bg-white p-1.5 disabled:opacity-40" disabled={page <= 1} onClick={() => onPageChange(page - 1)} type="button" aria-label="Trang trước"><ChevronLeft className="h-4 w-4" /></button><span>Trang {page} / {Math.max(totalPages, 1)}</span><button className="rounded-md border border-line bg-white p-1.5 disabled:opacity-40" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)} type="button" aria-label="Trang sau"><ChevronRight className="h-4 w-4" /></button></div>
    </div>
  );
}

function ViewButton({ active, label, icon, onClick }: { active: boolean; label: string; icon: React.ReactNode; onClick: () => void }) {
  return <button className={clsx('inline-flex items-center gap-2 rounded px-3 py-1.5 text-sm font-semibold', active ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-500 hover:text-ink')} onClick={onClick} type="button">{icon}{label}</button>;
}

function auditDateRange(preset: DatePreset, customFrom: string, customTo: string) {
  if (preset === 'custom') return { from: customFrom, to: customTo };
  if (preset === 'all') return { from: '', to: '' };
  const to = new Date();
  const from = new Date(to);
  from.setDate(from.getDate() - Number(preset) + 1);
  return { from: localDate(from), to: localDate(to) };
}

function localDate(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function auditDateTime(value: unknown) {
  if (!value) return '-';
  const date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat('vi-VN', { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit', year: 'numeric' }).format(date);
}

function shortID(value: string) {
  return value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-4)}` : value;
}

function sortEnumOptions(values: string[], group: 'event_type' | 'target_type') {
  return [...values].sort((left, right) => enumLabel(group, left).localeCompare(enumLabel(group, right), 'vi'));
}
