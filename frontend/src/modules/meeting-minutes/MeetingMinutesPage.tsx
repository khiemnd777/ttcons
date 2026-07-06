import { useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useLocation, useNavigate } from 'react-router-dom';
import { CalendarDays, CheckSquare, ExternalLink, FileText, RotateCcw, Search, Users } from 'lucide-react';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { ListSection } from '../../components/ListSection';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { dateText } from '../../lib/utils/format';
import type { MeetingMinutesItem } from '../../lib/types/domain';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import { compactDateTime, numberValue } from '../work-items/workItemPresentation';

type Filters = {
  projectId: string;
  dateFrom: string;
  dateTo: string;
  chair: string;
  owner: string;
  status: string;
  search: string;
};

const statusOptions = [
  { value: 'all', label: 'Tất cả trạng thái' },
  { value: 'draft', label: 'Nháp' },
  { value: 'submitted', label: 'Đã gửi' },
  { value: 'reopened', label: 'Mở lại' },
];

const attendanceLabels: Record<string, string> = {
  present: 'Có mặt',
  remote: 'Từ xa',
  absent: 'Vắng',
};

const priorityLabels: Record<string, string> = {
  low: 'Thấp',
  normal: 'Bình thường',
  high: 'Cao',
  urgent: 'Khẩn cấp',
};

export function MeetingMinutesPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [filters, setFilters] = useState<Filters>({
    projectId: '',
    dateFrom: '',
    dateTo: '',
    chair: '',
    owner: '',
    status: 'all',
    search: '',
  });
  const [detailMeeting, setDetailMeeting] = useState<MeetingMinutesItem | null>(null);

  const queryPath = useMemo(() => meetingMinutesPath(filters), [filters]);
  const meetings = useQuery({
    queryKey: ['planning-meeting-minutes', filters],
    queryFn: () => get<MeetingMinutesItem[]>(queryPath),
  });
  const rows = meetings.data ?? [];
  const metrics = useMemo(() => meetingMetrics(rows), [rows]);

  function updateFilter(key: keyof Filters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }));
  }

  function resetFilters() {
    setFilters({
      projectId: '',
      dateFrom: '',
      dateTo: '',
      chair: '',
      owner: '',
      status: 'all',
      search: '',
    });
  }

  function openWorkItem(meeting: MeetingMinutesItem) {
    if (meeting.work_item_id) navigate(workItemDetailPath(meeting.work_item_id, currentReturnTo(location), meeting.project_id));
  }

  return (
    <>
      <PageHeader
        title="Biên bản họp"
        description="Theo dõi biên bản họp từ biểu mẫu quy trình đang chạy, đầu việc và liên kết công trình/quy trình."
        actions={
          <button
            className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            onClick={resetFilters}
            type="button"
          >
            <RotateCcw className="h-4 w-4" />
            Xóa lọc
          </button>
        }
      />

      <section className="mb-4 rounded-lg border border-line bg-white p-4 shadow-panel">
        <div className="grid gap-3 xl:grid-cols-[220px_160px_160px_minmax(0,1fr)_minmax(0,1fr)_170px]">
          <ProjectSelector value={filters.projectId} placeholder="Tất cả công trình" onChange={(value) => updateFilter('projectId', value)} />
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Từ ngày
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              type="date"
              value={filters.dateFrom}
              onChange={(event) => updateFilter('dateFrom', event.target.value)}
            />
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Đến ngày
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              type="date"
              value={filters.dateTo}
              onChange={(event) => updateFilter('dateTo', event.target.value)}
            />
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Chủ trì
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Tên người chủ trì"
              value={filters.chair}
              onChange={(event) => updateFilter('chair', event.target.value)}
            />
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Phụ trách
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Người phụ trách quyết định/đầu việc"
              value={filters.owner}
              onChange={(event) => updateFilter('owner', event.target.value)}
            />
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Trạng thái
            <select
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              value={filters.status}
              onChange={(event) => updateFilter('status', event.target.value)}
            >
              {statusOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
        </div>
        <label className="relative mt-3 block">
          <span className="sr-only">Tìm biên bản họp</span>
          <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
          <input
            className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
            placeholder="Tìm công trình, node, bước, tiêu đề, nội dung họp, quyết định, đầu việc"
            value={filters.search}
            onChange={(event) => updateFilter('search', event.target.value)}
          />
        </label>
      </section>

      <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard icon={<FileText className="h-5 w-5" />} label="Biên bản" value={metrics.total} sub={`${metrics.needsSubmit} cần gửi`} />
        <MetricCard icon={<Users className="h-5 w-5" />} label="Người tham dự" value={metrics.attendees} sub={`${metrics.submitted} đã gửi`} />
        <MetricCard icon={<CheckSquare className="h-5 w-5" />} label="Quyết định" value={metrics.decisions} sub="dòng quyết định" />
        <MetricCard icon={<CalendarDays className="h-5 w-5" />} label="Đầu việc" value={metrics.actionItems} sub={`${metrics.generatedItems} đầu việc đã sinh`} />
      </div>

      <ListSection title="Biên bản họp" description={`${rows.length} biên bản khớp bộ lọc hiện tại`}>
        <DataTable
          rows={rows}
          loading={meetings.isLoading}
          error={meetings.error}
          onRetry={() => meetings.refetch()}
          pageSize={12}
          defaultSort={{ key: 'meeting_date', direction: 'desc' }}
          getRowKey={(row) => row.form_instance_id}
          empty="Chưa có biên bản họp khớp bộ lọc."
          columns={[
            { key: 'meeting_date', header: 'Ngày', sortable: true, render: (row) => dateText(row.meeting_date) },
            {
              key: 'project',
              header: 'Công trình',
              sortable: true,
              sortValue: (row) => row.project_name,
              filterValue: (row) => [row.project_name, row.run_node_name, row.run_step_name, row.work_item_title],
              render: (row) => (
                <div className="min-w-0">
                  <p className="font-semibold text-ink">{row.project_name}</p>
                  <p className="mt-1 text-xs text-slate-500">{row.run_node_name ?? '-'} · {row.run_step_name ?? row.work_item_title ?? '-'}</p>
                </div>
              ),
            },
            {
              key: 'meeting_title',
              header: 'Biên bản',
              sortable: true,
              sortValue: (row) => meetingTitle(row),
              filterValue: (row) => [meetingTitle(row), row.chair, row.location],
              render: (row) => (
                <div className="min-w-0">
                  <p className="break-words font-semibold text-ink">{meetingTitle(row)}</p>
                  <p className="mt-1 text-xs text-slate-500">Chủ trì: {row.chair || '-'}</p>
                </div>
              ),
            },
            { key: 'attendee_count', header: 'Tham dự', align: 'center', sortable: true, render: (row) => numberValue(row.attendee_count) },
            {
              key: 'decision_action',
              header: 'Q/ĐV',
              align: 'center',
              sortValue: (row) => numberValue(row.decision_count) + numberValue(row.action_item_count),
              render: (row) => (
                <span className="whitespace-nowrap text-xs font-semibold text-slate-700">
                  {numberValue(row.decision_count)} / {numberValue(row.action_item_count)}
                </span>
              ),
            },
            { key: 'generated_work_item_count', header: 'Đã sinh', align: 'center', sortable: true, render: (row) => numberValue(row.generated_work_item_count) },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            {
              key: 'actions',
              header: 'Thao tác',
              render: (row) => (
                <RowActions onView={() => setDetailMeeting(row)}>
                  <button
                    className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:text-slate-300"
                    disabled={!row.work_item_id}
                    onClick={() => openWorkItem(row)}
                    title={row.work_item_id ? 'Mở đầu việc/biểu mẫu' : 'Biên bản chưa có đầu việc'}
                    type="button"
                  >
                    <ExternalLink className="h-4 w-4" />
                  </button>
                </RowActions>
              ),
            },
          ]}
        />
      </ListSection>

      <MeetingMinutesDetailDialog meeting={detailMeeting} onClose={() => setDetailMeeting(null)} onOpenWorkItem={openWorkItem} />
    </>
  );
}

function MetricCard({ icon, label, value, sub }: { icon: ReactNode; label: string; value: number; sub: string }) {
  return (
    <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
          <p className="mt-2 text-2xl font-black text-blue-700">{value}</p>
        </div>
        <div className="grid h-10 w-10 shrink-0 place-items-center rounded-md bg-blue-50 text-blue-700">{icon}</div>
      </div>
      <p className="mt-2 text-xs text-slate-500">{sub}</p>
    </div>
  );
}

function MeetingMinutesDetailDialog({
  meeting,
  onClose,
  onOpenWorkItem,
}: {
  meeting: MeetingMinutesItem | null;
  onClose: () => void;
  onOpenWorkItem: (meeting: MeetingMinutesItem) => void;
}) {
  return (
    <Dialog
      open={Boolean(meeting)}
      title="Chi tiết biên bản họp"
      description={meeting ? `${meeting.project_name} · ${dateText(meeting.meeting_date)} · ${meeting.chair || '-'}` : undefined}
      size="xl"
      onClose={onClose}
      footer={meeting ? (
        <div className="flex flex-wrap justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} type="button">Đóng</button>
          <button
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-200 disabled:text-slate-500"
            disabled={!meeting.work_item_id}
            onClick={() => onOpenWorkItem(meeting)}
            type="button"
          >
            <ExternalLink className="h-4 w-4" />
            Mở đầu việc/biểu mẫu
          </button>
        </div>
      ) : null}
    >
      {meeting ? (
        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <DetailPill label="Trạng thái" value={<StatusBadge status={meeting.status} />} />
            <DetailPill label="Ngày họp" value={dateText(meeting.meeting_date)} />
            <DetailPill label="Chủ trì" value={meeting.chair || '-'} />
            <DetailPill label="Cập nhật" value={compactDateTime(meeting.updated_at)} />
          </div>
          <section className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-sm font-semibold text-ink">{meetingTitle(meeting)}</p>
            <div className="mt-2 grid gap-2 text-xs text-slate-600 md:grid-cols-2">
              <p>Địa điểm: <span className="font-semibold text-ink">{meeting.location || '-'}</span></p>
              <p>Người gửi: <span className="font-semibold text-ink">{meeting.submitted_by_name || '-'}</span></p>
              <p>Node: <span className="font-semibold text-ink">{meeting.run_node_name ?? '-'}</span></p>
              <p>Bước: <span className="font-semibold text-ink">{meeting.run_step_name ?? '-'}</span></p>
            </div>
          </section>
          <div className="grid gap-4 xl:grid-cols-3">
            <AttendeesList meeting={meeting} />
            <StructuredItems title="Quyết định" items={arrayValue(meeting.decisions)} fallbackText={meeting.decisions_text} empty="Chưa có quyết định." renderMeta={decisionMeta} />
            <StructuredItems title="Đầu việc" items={arrayValue(meeting.action_items)} fallbackText={meeting.action_items_text} empty="Chưa có đầu việc." renderMeta={actionItemMeta} />
          </div>
          <section className="rounded-md border border-line bg-white p-3">
            <h3 className="text-sm font-semibold text-ink">Nội dung họp</h3>
            <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{meeting.agenda || '-'}</p>
          </section>
          <div className="grid gap-2 rounded-md border border-line bg-slate-50 p-3 text-xs text-slate-600 md:grid-cols-2">
            <p>Đầu việc đã sinh: <span className="font-semibold text-ink">{numberValue(meeting.generated_work_item_count)}</span></p>
            <p>Tài liệu: <span className="font-semibold text-ink">{numberValue(meeting.signature_count)} chữ ký · {numberValue(meeting.pdf_export_count)} PDF · {numberValue(meeting.evidence_count)} minh chứng</span></p>
          </div>
        </div>
      ) : <EmptyState title="Chưa chọn biên bản họp." />}
    </Dialog>
  );
}

function DetailPill({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <div className="mt-1 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

function AttendeesList({ meeting }: { meeting: MeetingMinutesItem }) {
  const attendees = arrayValue(meeting.attendees);
  return (
    <section className="min-w-0">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">Thành phần tham dự</h3>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{numberValue(meeting.attendee_count)}</span>
      </div>
      {attendees.length ? (
        <div className="space-y-2">
          {attendees.map((item, index) => (
            <article key={`attendee-${index}`} className="rounded-md border border-line bg-white px-3 py-2">
              <p className="break-words text-sm font-semibold text-ink">{textOrDash(item.name)}</p>
              <p className="mt-1 break-words text-xs text-slate-500">
                {[item.role, attendanceLabel(item.attendance)].filter(Boolean).join(' · ') || '-'}
              </p>
            </article>
          ))}
        </div>
      ) : meeting.attendees_text ? (
        <LegacyText text={meeting.attendees_text} />
      ) : (
        <div className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-6 text-center text-sm text-slate-500">Chưa có người tham dự.</div>
      )}
    </section>
  );
}

function StructuredItems({
  title,
  items,
  fallbackText,
  empty,
  renderMeta,
}: {
  title: string;
  items: Record<string, any>[];
  fallbackText?: string;
  empty: string;
  renderMeta: (item: Record<string, any>) => string;
}) {
  return (
    <section className="min-w-0">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">{title}</h3>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{items.length || (fallbackText ? 1 : 0)}</span>
      </div>
      {items.length ? (
        <div className="space-y-2">
          {items.map((item, index) => (
            <article key={`${title}-${index}`} className="rounded-md border border-line bg-white px-3 py-2">
              <p className="break-words text-sm font-semibold text-ink">{itemTitle(item, title)}</p>
              <p className="mt-1 break-words text-xs text-slate-500">{renderMeta(item)}</p>
              {item.description ? <p className="mt-2 break-words text-xs leading-5 text-slate-600">{String(item.description)}</p> : null}
            </article>
          ))}
        </div>
      ) : fallbackText ? (
        <LegacyText text={fallbackText} />
      ) : (
        <div className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-6 text-center text-sm text-slate-500">{empty}</div>
      )}
    </section>
  );
}

function LegacyText({ text }: { text: string }) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <p className="whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{text}</p>
    </div>
  );
}

function meetingMinutesPath(filters: Filters) {
  const params = new URLSearchParams();
  if (filters.projectId) params.set('project_id', filters.projectId);
  if (filters.dateFrom) params.set('date_from', filters.dateFrom);
  if (filters.dateTo) params.set('date_to', filters.dateTo);
  if (filters.chair.trim()) params.set('chair', filters.chair.trim());
  if (filters.owner.trim()) params.set('owner', filters.owner.trim());
  if (filters.status && filters.status !== 'all') params.set('status', filters.status);
  if (filters.search.trim()) params.set('search', filters.search.trim());
  const query = params.toString();
  return query ? `/planning/meeting-minutes?${query}` : '/planning/meeting-minutes';
}

function meetingMetrics(rows: MeetingMinutesItem[]) {
  return {
    total: rows.length,
    needsSubmit: rows.filter((row) => row.status !== 'submitted').length,
    submitted: rows.filter((row) => row.status === 'submitted').length,
    attendees: rows.reduce((sum, row) => sum + numberValue(row.attendee_count), 0),
    decisions: rows.reduce((sum, row) => sum + numberValue(row.decision_count), 0),
    actionItems: rows.reduce((sum, row) => sum + numberValue(row.action_item_count), 0),
    generatedItems: rows.reduce((sum, row) => sum + numberValue(row.generated_work_item_count), 0),
  };
}

function arrayValue(value: unknown): Record<string, any>[] {
  return Array.isArray(value) ? value.filter((item): item is Record<string, any> => Boolean(item) && typeof item === 'object' && !Array.isArray(item)) : [];
}

function meetingTitle(meeting: MeetingMinutesItem) {
  const title = String(meeting.meeting_title ?? meeting.title ?? '').trim();
  return title || 'Biên bản họp';
}

function itemTitle(item: Record<string, any>, fallback: string) {
  const title = String(item.title ?? item.content ?? '').trim();
  return title || fallback;
}

function decisionMeta(item: Record<string, any>) {
  return [
    item.owner ? `Phụ trách: ${item.owner}` : '',
    item.target ? `Mốc: ${dateText(item.target)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function actionItemMeta(item: Record<string, any>) {
  return [
    priorityLabels[String(item.priority ?? '')] ?? item.priority,
    item.owner_role || item.owner_name ? `Phụ trách: ${[item.owner_role, item.owner_name].filter(Boolean).join(' / ')}` : '',
    item.due_date ? `Hạn: ${dateText(item.due_date)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function attendanceLabel(value: unknown) {
  const key = String(value ?? '').trim();
  return attendanceLabels[key] ?? key;
}

function textOrDash(value: unknown) {
  const text = String(value ?? '').trim();
  return text || '-';
}
