import { useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useLocation, useNavigate } from 'react-router-dom';
import { CalendarDays, CheckSquare, ExternalLink, FileText, RotateCcw, Search, Users } from 'lucide-react';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { RecordSummary, RecordSection, RecordMetrics, RecordMetric } from '../../components/RecordDetailLayout';
import { PlanningRecordContext } from '../_shared/PlanningRecordContext';
import { EmptyState } from '../../components/EmptyState';
import { ListSection } from '../../components/ListSection';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { dateText } from '../../lib/utils/format';
import type { MeetingMinutesItem } from '../../lib/types/domain';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import { RecordLink } from '../_shared/RecordLinks';
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
          <AsyncProjectPicker value={filters.projectId} onChange={(value) => updateFilter('projectId', value)} endpoint="/projects/options" queryKey="project-options" emptyLabel="Tất cả công trình" />
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

function MeetingMinutesDetailDialog({ meeting, onClose, onOpenWorkItem }: {
  meeting: MeetingMinutesItem | null; onClose: () => void; onOpenWorkItem: (meeting: MeetingMinutesItem) => void;
}) {
  return <Dialog manageFocus open={Boolean(meeting)} title="Chi tiết biên bản họp" size="xl" onClose={onClose}
    footer={meeting ? <div className="flex flex-wrap justify-end gap-2">
      <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} type="button">Đóng</button>
      {meeting.work_item_id ? <RecordLink primary target={{ kind: 'work_item', id: meeting.work_item_id, projectId: meeting.project_id }}>Mở đầu việc/biểu mẫu</RecordLink> : <span className="text-sm text-slate-500">Chưa có đầu việc nguồn</span>}
    </div> : null}>
    {meeting ? <div className="space-y-6">
      <RecordSummary eyebrow={meeting.project_name} title={meetingTitle(meeting)} subtitle={dateText(meeting.meeting_date)} status={<StatusBadge status={meeting.status} />} />
      <RecordMetrics>
        <RecordMetric label="Chủ trì">{meeting.chair || '—'}</RecordMetric>
        <RecordMetric label="Địa điểm">{meeting.location || '—'}</RecordMetric>
        <RecordMetric label="Người gửi">{identityReferenceText(meeting.submitted_by_name, meeting.submitted_by_account_deleted)}</RecordMetric>
      </RecordMetrics>
      <RecordSection title="Nội dung họp"><p className="whitespace-pre-wrap break-words text-sm leading-7 text-slate-700">{meeting.agenda || 'Chưa có nội dung họp.'}</p></RecordSection>
      <div className="grid gap-6 lg:grid-cols-2">
        <StructuredItems title="Quyết định" items={arrayValue(meeting.decisions)} fallbackText={meeting.decisions_text} empty="Chưa có quyết định." renderMeta={decisionMeta} />
        <StructuredItems title="Đầu việc sau họp" items={arrayValue(meeting.action_items)} fallbackText={meeting.action_items_text} empty="Chưa có đầu việc." renderMeta={actionItemMeta} />
      </div>
      <details className="rounded-lg border border-slate-200 p-4"><summary className="cursor-pointer text-sm font-semibold text-slate-600">Thành phần tham dự · {numberValue(meeting.attendee_count)}</summary><div className="pt-4"><AttendeesList meeting={meeting} /></div></details>
      <PlanningRecordContext record={meeting} />
    </div> : <EmptyState title="Chưa chọn hồ sơ." />}
  </Dialog>;
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
