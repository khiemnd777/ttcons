import { useMemo, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { CalendarDays, ClipboardList, ExternalLink, FileText, RotateCcw, Search } from 'lucide-react';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { ListSection } from '../../components/ListSection';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { dateText, percent } from '../../lib/utils/format';
import type { PlanningDailyReportItem, WorkItem } from '../../lib/types/domain';
import { WorkItemDetailDialog } from '../work-items/WorkItemDetailDialog';
import { compactDateTime, numberValue } from '../work-items/workItemPresentation';

type Filters = {
  projectId: string;
  dateFrom: string;
  dateTo: string;
  reportedBy: string;
  owner: string;
  status: string;
  search: string;
};

const statusOptions = [
  { value: 'all', label: 'Tất cả trạng thái' },
  { value: 'draft', label: 'Nháp' },
  { value: 'submitted', label: 'Đã submit' },
  { value: 'reopened', label: 'Mở lại' },
];

const planningScopeLabels: Record<string, string> = {
  survey: 'Khảo sát',
  design: 'Thiết kế',
  boq: 'BOQ / Dự toán',
  budget: 'Ngân sách',
  procurement: 'Mua sắm',
  site_preparation: 'Chuẩn bị thi công',
  other: 'Khác',
};

const resultLabels: Record<string, string> = {
  done: 'Done',
  partial: 'Partial',
  blocked: 'Blocked',
};

const priorityLabels: Record<string, string> = {
  low: 'Low',
  normal: 'Normal',
  high: 'High',
  urgent: 'Urgent',
};

const severityLabels: Record<string, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  critical: 'Critical',
};

export function PlanningDailyReportsPage() {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<Filters>({
    projectId: '',
    dateFrom: '',
    dateTo: '',
    reportedBy: '',
    owner: '',
    status: 'all',
    search: '',
  });
  const [detailReport, setDetailReport] = useState<PlanningDailyReportItem | null>(null);
  const [selectedWorkItem, setSelectedWorkItem] = useState<WorkItem | null>(null);

  const queryPath = useMemo(() => planningDailyReportPath(filters), [filters]);
  const reports = useQuery({
    queryKey: ['planning-daily-reports', filters],
    queryFn: () => get<PlanningDailyReportItem[]>(queryPath),
  });
  const rows = reports.data ?? [];
  const metrics = useMemo(() => dailyReportMetrics(rows), [rows]);

  function updateFilter(key: keyof Filters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }));
  }

  function resetFilters() {
    setFilters({
      projectId: '',
      dateFrom: '',
      dateTo: '',
      reportedBy: '',
      owner: '',
      status: 'all',
      search: '',
    });
  }

  function openWorkItem(report: PlanningDailyReportItem) {
    const item = reportToWorkItem(report);
    if (item) setSelectedWorkItem(item);
  }

  return (
    <>
      <PageHeader
        title="Daily report kế hoạch"
        description="Theo dõi báo cáo ngày trong giai đoạn kế hoạch từ workflow form runtime."
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
            Người report
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Tên hoặc submitter"
              value={filters.reportedBy}
              onChange={(event) => updateFilter('reportedBy', event.target.value)}
            />
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Owner
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Role hoặc tên owner"
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
          <span className="sr-only">Tìm daily report kế hoạch</span>
          <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
          <input
            className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
            placeholder="Tìm công trình, node, step, đã làm, sẽ làm, blocker"
            value={filters.search}
            onChange={(event) => updateFilter('search', event.target.value)}
          />
        </label>
      </section>

      <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard icon={<FileText className="h-5 w-5" />} label="Reports" value={metrics.total} sub={`${metrics.needsSubmit} cần submit`} />
        <MetricCard icon={<ClipboardList className="h-5 w-5" />} label="Đã làm" value={metrics.completedItems} sub="completed items" />
        <MetricCard icon={<CalendarDays className="h-5 w-5" />} label="Sẽ làm" value={metrics.nextPlanItems} sub={`${metrics.generatedItems} work item đã sinh`} />
        <MetricCard icon={<FileText className="h-5 w-5" />} label="Blocker" value={metrics.blockers} sub={`${metrics.submitted} đã submit`} />
      </div>

      <ListSection title="Daily reports planning" description={`${rows.length} report khớp bộ lọc hiện tại`}>
        <DataTable
          rows={rows}
          loading={reports.isLoading}
          error={reports.error}
          onRetry={() => reports.refetch()}
          pageSize={12}
          defaultSort={{ key: 'report_date', direction: 'desc' }}
          getRowKey={(row) => row.form_instance_id}
          empty="Chưa có daily report kế hoạch khớp bộ lọc."
          columns={[
            { key: 'report_date', header: 'Ngày', sortable: true, render: (row) => dateText(row.report_date) },
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
            { key: 'reported_by', header: 'Reporter', sortable: true, render: (row) => row.reported_by || row.submitted_by_name || '-' },
            {
              key: 'summary',
              header: 'Đã làm / Sẽ làm',
              align: 'center',
              sortValue: (row) => numberValue(row.completed_count) + numberValue(row.next_plan_count),
              render: (row) => (
                <span className="whitespace-nowrap text-xs font-semibold text-slate-700">
                  {numberValue(row.completed_count)} / {numberValue(row.next_plan_count)}
                </span>
              ),
            },
            { key: 'blocker_count', header: 'Blocker', align: 'center', sortable: true, render: (row) => numberValue(row.blocker_count) },
            { key: 'progress_percent', header: 'Tiến độ', align: 'right', sortable: true, render: (row) => row.progress_percent === null || row.progress_percent === undefined ? '-' : percent(row.progress_percent) },
            { key: 'status', header: 'Status', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            {
              key: 'actions',
              header: 'Action',
              render: (row) => (
                <RowActions onView={() => setDetailReport(row)}>
                  <button
                    className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:text-slate-300"
                    disabled={!row.work_item_id}
                    onClick={() => openWorkItem(row)}
                    title={row.work_item_id ? 'Mở work item/form' : 'Report chưa có work item'}
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

      <PlanningDailyReportDetailDialog report={detailReport} onClose={() => setDetailReport(null)} onOpenWorkItem={openWorkItem} />
      <WorkItemDetailDialog
        open={Boolean(selectedWorkItem)}
        item={selectedWorkItem}
        onClose={() => setSelectedWorkItem(null)}
        onChanged={() => {
          queryClient.invalidateQueries({ queryKey: ['planning-daily-reports'] });
          reports.refetch();
        }}
      />
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

function PlanningDailyReportDetailDialog({
  report,
  onClose,
  onOpenWorkItem,
}: {
  report: PlanningDailyReportItem | null;
  onClose: () => void;
  onOpenWorkItem: (report: PlanningDailyReportItem) => void;
}) {
  return (
    <Dialog
      open={Boolean(report)}
      title="Chi tiết daily report kế hoạch"
      description={report ? `${report.project_name} · ${dateText(report.report_date)} · ${report.reported_by || report.submitted_by_name || '-'}` : undefined}
      size="xl"
      onClose={onClose}
      footer={report ? (
        <div className="flex flex-wrap justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} type="button">Đóng</button>
          <button
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-200 disabled:text-slate-500"
            disabled={!report.work_item_id}
            onClick={() => onOpenWorkItem(report)}
            type="button"
          >
            <ExternalLink className="h-4 w-4" />
            Mở work item/form
          </button>
        </div>
      ) : null}
    >
      {report ? (
        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <DetailPill label="Status" value={<StatusBadge status={report.status} />} />
            <DetailPill label="Scope" value={planningScopeLabel(report.planning_scope)} />
            <DetailPill label="Progress" value={report.progress_percent === null || report.progress_percent === undefined ? '-' : percent(report.progress_percent)} />
            <DetailPill label="Updated" value={compactDateTime(report.updated_at)} />
          </div>
          <div className="grid gap-4 xl:grid-cols-3">
            <StructuredItems title="Đã làm" items={arrayValue(report.completed_items)} empty="Chưa có hạng mục đã làm." renderMeta={completedMeta} />
            <StructuredItems title="Sẽ làm" items={arrayValue(report.next_plan_items)} empty="Chưa có việc sẽ làm." renderMeta={nextPlanMeta} />
            <StructuredItems title="Blocker / Risk" items={arrayValue(report.blockers)} empty="Chưa có blocker." renderMeta={blockerMeta} />
          </div>
          <div className="grid gap-2 rounded-md border border-line bg-slate-50 p-3 text-xs text-slate-600 md:grid-cols-2">
            <p>Node: <span className="font-semibold text-ink">{report.run_node_name ?? '-'}</span></p>
            <p>Step: <span className="font-semibold text-ink">{report.run_step_name ?? '-'}</span></p>
            <p>Generated work items: <span className="font-semibold text-ink">{numberValue(report.generated_work_item_count)}</span></p>
            <p>Artifacts: <span className="font-semibold text-ink">{numberValue(report.signature_count)} chữ ký · {numberValue(report.pdf_export_count)} PDF · {numberValue(report.evidence_count)} evidence</span></p>
          </div>
        </div>
      ) : <EmptyState title="Chưa chọn daily report." />}
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

function StructuredItems({
  title,
  items,
  empty,
  renderMeta,
}: {
  title: string;
  items: Record<string, any>[];
  empty: string;
  renderMeta: (item: Record<string, any>) => string;
}) {
  return (
    <section className="min-w-0">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">{title}</h3>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{items.length}</span>
      </div>
      {items.length ? (
        <div className="space-y-2">
          {items.map((item, index) => (
            <article key={`${title}-${index}`} className="rounded-md border border-line bg-white px-3 py-2">
              <p className="break-words text-sm font-semibold text-ink">{itemTitle(item, title)}</p>
              <p className="mt-1 break-words text-xs text-slate-500">{renderMeta(item)}</p>
              {item.description || item.note ? <p className="mt-2 break-words text-xs leading-5 text-slate-600">{String(item.description ?? item.note)}</p> : null}
            </article>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-6 text-center text-sm text-slate-500">{empty}</div>
      )}
    </section>
  );
}

function planningDailyReportPath(filters: Filters) {
  const params = new URLSearchParams();
  if (filters.projectId) params.set('project_id', filters.projectId);
  if (filters.dateFrom) params.set('date_from', filters.dateFrom);
  if (filters.dateTo) params.set('date_to', filters.dateTo);
  if (filters.reportedBy.trim()) params.set('reported_by', filters.reportedBy.trim());
  if (filters.owner.trim()) params.set('owner', filters.owner.trim());
  if (filters.status && filters.status !== 'all') params.set('status', filters.status);
  if (filters.search.trim()) params.set('search', filters.search.trim());
  const query = params.toString();
  return query ? `/planning/daily-reports?${query}` : '/planning/daily-reports';
}

function dailyReportMetrics(rows: PlanningDailyReportItem[]) {
  return {
    total: rows.length,
    needsSubmit: rows.filter((row) => row.status !== 'submitted').length,
    submitted: rows.filter((row) => row.status === 'submitted').length,
    completedItems: rows.reduce((sum, row) => sum + numberValue(row.completed_count), 0),
    nextPlanItems: rows.reduce((sum, row) => sum + numberValue(row.next_plan_count), 0),
    blockers: rows.reduce((sum, row) => sum + numberValue(row.blocker_count), 0),
    generatedItems: rows.reduce((sum, row) => sum + numberValue(row.generated_work_item_count), 0),
  };
}

function reportToWorkItem(report: PlanningDailyReportItem): WorkItem | null {
  if (!report.work_item_id) return null;
  return {
    id: report.work_item_id,
    project_id: report.project_id,
    project_name: report.project_name,
    title: report.work_item_title ?? report.title,
    status: report.work_item_status ?? report.status,
    work_type: 'daily_report',
    source_type: 'pipeline_run_step',
    source_id: report.run_step_id,
    run_node_id: report.run_node_id,
    run_node_name: report.run_node_name,
    run_step_id: report.run_step_id,
    run_step_name: report.run_step_name,
    workflow_form_instance_id: report.form_instance_id,
    workflow_form_status: report.status,
    workflow_form_type: 'daily_report',
    workflow_form_revision: report.current_revision,
    workflow_form_submitted_at: report.submitted_at,
    workflow_form_template_code: report.template_code,
    workflow_form_template_name: report.template_name,
    workflow_form_version_no: report.version_no,
    evidence_count: numberValue(report.evidence_count),
  };
}

function arrayValue(value: unknown): Record<string, any>[] {
  return Array.isArray(value) ? value.filter((item): item is Record<string, any> => Boolean(item) && typeof item === 'object' && !Array.isArray(item)) : [];
}

function itemTitle(item: Record<string, any>, fallback: string) {
  const title = String(item.title ?? item.content ?? '').trim();
  return title || fallback;
}

function completedMeta(item: Record<string, any>) {
  return [
    resultLabels[String(item.result ?? '')] ?? item.result,
    item.owner_role ? `Owner: ${item.owner_role}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function nextPlanMeta(item: Record<string, any>) {
  return [
    priorityLabels[String(item.priority ?? '')] ?? item.priority,
    item.owner_role || item.owner_name ? `Owner: ${[item.owner_role, item.owner_name].filter(Boolean).join(' / ')}` : '',
    item.due_date ? `Due: ${dateText(item.due_date)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function blockerMeta(item: Record<string, any>) {
  return [
    item.impact ? `Impact: ${item.impact}` : '',
    severityLabels[String(item.severity ?? '')] ?? item.severity,
    item.owner_role ? `Owner: ${item.owner_role}` : '',
    item.target_date ? `Target: ${dateText(item.target_date)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function planningScopeLabel(value?: string) {
  const key = String(value ?? '').trim();
  return planningScopeLabels[key] ?? (key || '-');
}
