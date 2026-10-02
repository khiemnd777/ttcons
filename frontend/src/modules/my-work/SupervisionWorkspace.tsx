import { useState } from 'react';
import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { RefreshCw } from 'lucide-react';
import { DataTable, type Column } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { WorkItem, WorkReportDetail, WorkReportListItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { WorkItemDetailDialog } from '../work-items/WorkItemDetailDialog';
import { WorkReportReviewDialog, WorkReportStatusBadge } from '../work-reports/WorkReportReviewDialog';

type Options = { projects: Array<{ id: string; code: string; name: string }>; supervisors: Array<{ id: string; full_name: string }> };
type WorkPage = { items: WorkItem[]; has_more: boolean };
const PAGE_SIZE = 25;
const emptyFilters = { project: '', supervisor: '', status: '', read: '', search: '' };

// Purpose: company-wide Supervisor oversight. Source APIs retain every scope and
// decision check; this workspace adds no Work Item mutation capabilities.
export function SupervisionWorkspace({ reports }: { reports: boolean }) {
  const { user, hasPermission } = useAuth();
  const queryClient = useQueryClient();
  const allowed = Boolean(user?.id) && hasPermission('project.supervision.view');
  const [draft, setDraft] = useState(emptyFilters);
  const [filters, setFilters] = useState(emptyFilters);
  const [selectedWork, setSelectedWork] = useState<WorkItem | null>(null);
  const [sourceID, setSourceID] = useState('');
  const options = useQuery({ queryKey: ['supervision-options', user?.id], queryFn: () => get<Options>('/me/supervision-options'), enabled: allowed, retry: false });
  function params(offset: number) {
    const query = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) });
    if (filters.project) query.set('project_id', filters.project);
    if (filters.supervisor) query.set('supervisor_id', filters.supervisor);
    if (filters.search.trim()) query.set('search', filters.search.trim());
    if (reports) { query.set('scope', 'supervision'); if (filters.read) query.set('read_status', filters.read); }
    else if (filters.status) query.set('status', filters.status);
    return query;
  }
  const work = useInfiniteQuery({
    queryKey: ['supervision-work', user?.id, filters], initialPageParam: 0,
    queryFn: ({ pageParam }) => get<WorkPage>(`/me/supervision-work?${params(pageParam)}`),
    getNextPageParam: (page, pages) => page.has_more ? pages.length * PAGE_SIZE : undefined,
    enabled: allowed && !reports, retry: false,
  });
  const reportList = useInfiniteQuery({
    queryKey: ['work-reports', 'supervision', user?.id, filters], initialPageParam: 0,
    queryFn: ({ pageParam }) => get<WorkReportListItem[]>(`/work-reports?${params(pageParam)}`),
    getNextPageParam: (page, pages) => page.length === PAGE_SIZE ? pages.length * PAGE_SIZE : undefined,
    enabled: allowed && reports, retry: false,
  });
  const detail = useQuery({ queryKey: ['work-report-detail', user?.id, sourceID], queryFn: () => get<WorkReportDetail>(`/work-reports/daily-reports/${sourceID}`), enabled: allowed && Boolean(sourceID), retry: false });
  const workRows = Array.from(new Map((work.data?.pages.flatMap((page) => page.items) ?? []).map((item) => [item.id, item])).values());
  const reportRows = Array.from(new Map((reportList.data?.pages.flat() ?? []).map((item) => [item.source_id, item])).values());
  const activeQuery = reports ? reportList : work;
  const actionClass = 'rounded border border-line px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-50';
  const workColumns: Column<WorkItem>[] = [
    { key: 'actions', header: 'Thao tác', action: true, render: (item) => <div className="flex flex-wrap gap-2"><button className={actionClass} onClick={() => setSelectedWork(item)}>Xem công việc</button><Link className={actionClass} to={`/projects/${item.project_id}/discussion`}>Trao đổi</Link></div> },
    { key: 'title', header: 'Công việc' },
    { key: 'project', header: 'Công trình', render: (item) => `${item.project_code ?? ''} · ${item.project_name ?? ''}` },
    { key: 'assignee_name', header: 'Giám sát', render: (item) => item.assignee_name || 'Chưa phân công' },
    { key: 'status', header: 'Trạng thái', render: (item) => <StatusBadge status={item.status} /> },
    { key: 'due_at', header: 'Hạn xử lý', render: (item) => dateText(item.due_at) },
  ];
  const reportColumns: Column<WorkReportListItem>[] = [
    { key: 'actions', header: 'Thao tác', action: true, render: (item) => <button className={actionClass} onClick={() => setSourceID(item.source_id)}>Xem báo cáo</button> },
    { key: 'report_date', header: 'Ngày báo cáo', render: (item) => dateText(item.report_date) },
    { key: 'project', header: 'Công trình', render: (item) => `${item.project_code} · ${item.project_name}` },
    { key: 'author_name', header: 'Giám sát' },
    { key: 'current_revision_no', header: 'Lần gửi' },
    { key: 'status', header: 'Phản hồi', render: (item) => <WorkReportStatusBadge status={item.status} /> },
    { key: 'viewer_acknowledged', header: 'Tôi đã xem', render: (item) => item.viewer_acknowledged ? 'Đã xem' : 'Chưa xem' },
  ];
  if (!allowed) return null;
  return <section className="mt-4 space-y-3" aria-label={reports ? 'Báo cáo Giám sát' : 'Công việc Giám sát'}>
    <p className="text-sm text-slate-600">{reports ? 'Theo dõi báo cáo của Giám sát trên toàn công ty. Xác nhận đã xem được ghi riêng cho từng người và từng lần gửi.' : 'Theo dõi công việc Giám sát của toàn bộ công trình và trao đổi khi cần.'}</p>
    <form className="grid gap-3 rounded-lg border border-line bg-white p-3 sm:grid-cols-2 xl:grid-cols-5" onSubmit={(event) => { event.preventDefault(); setFilters({ ...draft }); }}>
      <Filter label="Công trình" value={draft.project} onChange={(value) => setDraft({ ...draft, project: value })} options={(options.data?.projects ?? []).map((item) => [item.id, `${item.code} · ${item.name}`])} />
      <Filter label="Giám sát" value={draft.supervisor} onChange={(value) => setDraft({ ...draft, supervisor: value })} options={(options.data?.supervisors ?? []).map((item) => [item.id, item.full_name])} />
      {reports ? <Filter label="Tôi đã xem" value={draft.read} onChange={(value) => setDraft({ ...draft, read: value })} options={[[ 'unread', 'Chưa xem' ], [ 'read', 'Đã xem' ]]} /> : <Filter label="Trạng thái" value={draft.status} onChange={(value) => setDraft({ ...draft, status: value })} options={[[ 'pending', 'Chưa bắt đầu' ], [ 'ready', 'Sẵn sàng' ], [ 'in_progress', 'Đang làm' ], [ 'blocked', 'Bị chặn' ], [ 'needs_changes', 'Cần bổ sung' ], [ 'waiting_approval', 'Chờ duyệt' ], [ 'completed', 'Hoàn thành' ]]} />}
      <label className="text-xs font-semibold text-slate-600">Tìm kiếm<input className="mt-1 h-10 w-full rounded border border-line px-3 text-sm font-normal" value={draft.search} onChange={(event) => setDraft({ ...draft, search: event.target.value })} placeholder="Từ khóa..." /></label>
      <div className="flex items-end gap-2"><button className="h-10 rounded bg-blue-600 px-4 text-sm font-semibold text-white" type="submit">Lọc</button><button className="h-10 rounded border border-line px-3 text-sm" type="button" onClick={() => void activeQuery.refetch()} aria-label="Làm mới danh sách Giám sát"><RefreshCw className="h-4 w-4" /></button></div>
    </form>
    {options.isLoading ? <p className="text-xs text-slate-500">Đang tải bộ lọc...</p> : options.isError ? <p className="text-sm text-rose-700">Không tải được bộ lọc. <button className="underline" onClick={() => void options.refetch()}>Thử lại</button></p> : null}
    {reports ? <DataTable rows={reportRows} columns={reportColumns} getRowKey={(item) => item.source_id} loading={reportList.isLoading} error={reportList.error} onRetry={() => void reportList.refetch()} empty="Chưa có báo cáo Giám sát phù hợp bộ lọc." /> : <DataTable rows={workRows} columns={workColumns} getRowKey={(item) => item.id} loading={work.isLoading} error={work.error} onRetry={() => void work.refetch()} empty="Chưa có công việc Giám sát phù hợp bộ lọc." />}
    {activeQuery.hasNextPage ? <button className="rounded border border-line bg-white px-4 py-2 text-sm font-semibold disabled:opacity-60" disabled={activeQuery.isFetchingNextPage} onClick={() => void activeQuery.fetchNextPage()}>{activeQuery.isFetchingNextPage ? 'Đang tải...' : 'Xem thêm'}</button> : null}
    <WorkItemDetailDialog open={Boolean(selectedWork)} item={selectedWork} readOnly onClose={() => setSelectedWork(null)} />
    <WorkReportReviewDialog open={Boolean(sourceID)} detail={detail.data ?? null} loading={detail.isLoading} error={detail.error} userID={user?.id ?? ''} canReview={hasPermission('work_report.review')} canManage={hasPermission('work_report.manage')} canOpenSource={hasPermission('operation.view')} canExport={hasPermission('work_report.export')} onClose={() => setSourceID('')} onRefresh={async () => { await Promise.all([detail.refetch(), queryClient.invalidateQueries({ queryKey: ['work-reports'] }), queryClient.invalidateQueries({ queryKey: ['work-report-summary'] })]); }} />
  </section>;
}

function Filter({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: Array<[string, string]> }) {
  return <label className="text-xs font-semibold text-slate-600">{label}<select className="mt-1 h-10 w-full rounded border border-line bg-white px-2 text-sm font-normal" value={value} onChange={(event) => onChange(event.target.value)}><option value="">Tất cả</option>{options.map(([key, text]) => <option value={key} key={key}>{text}</option>)}</select></label>;
}
