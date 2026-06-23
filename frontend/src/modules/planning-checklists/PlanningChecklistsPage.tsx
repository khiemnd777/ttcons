import { useMemo, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ClipboardCheck, ExternalLink, FileCheck2, RotateCcw, Search } from 'lucide-react';
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
import type { PlanningChecklistItem, WorkItem } from '../../lib/types/domain';
import { WorkItemDetailDialog } from '../work-items/WorkItemDetailDialog';
import { compactDateTime, numberValue } from '../work-items/workItemPresentation';

type Filters = {
  projectId: string;
  dateFrom: string;
  dateTo: string;
  checkedBy: string;
  assignee: string;
  itemStatus: string;
  evidence: string;
  status: string;
  search: string;
};

const statusOptions = [
  { value: 'all', label: 'Tất cả form status' },
  { value: 'draft', label: 'Nháp' },
  { value: 'submitted', label: 'Đã submit' },
  { value: 'reopened', label: 'Mở lại' },
];

const itemStatusOptions = [
  { value: 'all', label: 'Tất cả item status' },
  { value: 'done', label: 'Done' },
  { value: 'not_applicable', label: 'N/A' },
  { value: 'needs_follow_up', label: 'Needs follow-up' },
  { value: 'blocked', label: 'Blocked' },
];

const evidenceOptions = [
  { value: 'all', label: 'Tất cả evidence' },
  { value: 'linked', label: 'Có evidence' },
  { value: 'missing', label: 'Thiếu evidence' },
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

export function PlanningChecklistsPage() {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<Filters>({
    projectId: '',
    dateFrom: '',
    dateTo: '',
    checkedBy: '',
    assignee: '',
    itemStatus: 'all',
    evidence: 'all',
    status: 'all',
    search: '',
  });
  const [detailChecklist, setDetailChecklist] = useState<PlanningChecklistItem | null>(null);
  const [selectedWorkItem, setSelectedWorkItem] = useState<WorkItem | null>(null);

  const queryPath = useMemo(() => planningChecklistPath(filters), [filters]);
  const checklists = useQuery({
    queryKey: ['planning-checklists', filters],
    queryFn: () => get<PlanningChecklistItem[]>(queryPath),
  });
  const rows = checklists.data ?? [];
  const metrics = useMemo(() => checklistMetrics(rows), [rows]);

  function updateFilter(key: keyof Filters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }));
  }

  function resetFilters() {
    setFilters({
      projectId: '',
      dateFrom: '',
      dateTo: '',
      checkedBy: '',
      assignee: '',
      itemStatus: 'all',
      evidence: 'all',
      status: 'all',
      search: '',
    });
  }

  function openWorkItem(checklist: PlanningChecklistItem) {
    const item = checklistToWorkItem(checklist);
    if (item) setSelectedWorkItem(item);
  }

  return (
    <>
      <PageHeader
        title="Checklist kế hoạch"
        description="Theo dõi checklist planning từ workflow form runtime, assignee, evidence và trạng thái item."
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
        <div className="grid gap-3 xl:grid-cols-[220px_150px_150px_minmax(0,1fr)_minmax(0,1fr)]">
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
            Người kiểm tra
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Tên hoặc submitter"
              value={filters.checkedBy}
              onChange={(event) => updateFilter('checkedBy', event.target.value)}
            />
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Assignee
            <input
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              placeholder="Role hoặc tên assignee"
              value={filters.assignee}
              onChange={(event) => updateFilter('assignee', event.target.value)}
            />
          </label>
        </div>
        <div className="mt-3 grid gap-3 md:grid-cols-3">
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Item status
            <select
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              value={filters.itemStatus}
              onChange={(event) => updateFilter('itemStatus', event.target.value)}
            >
              {itemStatusOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Evidence
            <select
              className="rounded-md border border-line px-3 py-2 text-sm normal-case tracking-normal text-ink outline-none focus:ring-2 focus:ring-blue-100"
              value={filters.evidence}
              onChange={(event) => updateFilter('evidence', event.target.value)}
            >
              {evidenceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="grid gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
            Form status
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
          <span className="sr-only">Tìm checklist kế hoạch</span>
          <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
          <input
            className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
            placeholder="Tìm công trình, node, step, hạng mục, assignee, comment"
            value={filters.search}
            onChange={(event) => updateFilter('search', event.target.value)}
          />
        </label>
      </section>

      <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard icon={<ClipboardCheck className="h-5 w-5" />} label="Checklists" value={metrics.total} sub={`${metrics.needsSubmit} cần submit`} />
        <MetricCard icon={<FileCheck2 className="h-5 w-5" />} label="Items" value={metrics.items} sub="checklist rows" />
        <MetricCard icon={<CheckCircle2 className="h-5 w-5" />} label="Done / N/A" value={metrics.done} sub={`${metrics.submitted} đã submit`} />
        <MetricCard icon={<ClipboardCheck className="h-5 w-5" />} label="Needs action" value={metrics.needsAction} sub={`${metrics.evidenceMissing} thiếu evidence`} />
      </div>

      <ListSection title="Planning checklists" description={`${rows.length} checklist khớp bộ lọc hiện tại`}>
        <DataTable
          rows={rows}
          loading={checklists.isLoading}
          error={checklists.error}
          onRetry={() => checklists.refetch()}
          pageSize={12}
          defaultSort={{ key: 'check_date', direction: 'desc' }}
          getRowKey={(row) => row.form_instance_id}
          empty="Chưa có checklist kế hoạch khớp bộ lọc."
          columns={[
            { key: 'check_date', header: 'Ngày', sortable: true, render: (row) => dateText(row.check_date) },
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
            { key: 'planning_scope', header: 'Scope', sortable: true, render: (row) => planningScopeLabel(row.planning_scope) },
            { key: 'checked_by', header: 'Checked by', sortable: true, render: (row) => row.checked_by || row.submitted_by_name || '-' },
            {
              key: 'items',
              header: 'Done / Total',
              align: 'center',
              sortValue: (row) => numberValue(row.done_count) + numberValue(row.item_count),
              render: (row) => (
                <span className="whitespace-nowrap text-xs font-semibold text-slate-700">
                  {numberValue(row.done_count)} / {numberValue(row.item_count)}
                </span>
              ),
            },
            { key: 'needs_action_count', header: 'Needs', align: 'center', sortable: true, render: (row) => numberValue(row.needs_action_count) },
            {
              key: 'evidence',
              header: 'Evidence',
              align: 'center',
              sortValue: (row) => numberValue(row.evidence_linked_count),
              render: (row) => (
                <span className="whitespace-nowrap text-xs font-semibold text-slate-700">
                  {numberValue(row.evidence_linked_count)} linked
                </span>
              ),
            },
            { key: 'status', header: 'Status', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            {
              key: 'actions',
              header: 'Action',
              render: (row) => (
                <RowActions onView={() => setDetailChecklist(row)}>
                  <button
                    className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:text-slate-300"
                    disabled={!row.work_item_id}
                    onClick={() => openWorkItem(row)}
                    title={row.work_item_id ? 'Mở work item/form' : 'Checklist chưa có work item'}
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

      <PlanningChecklistDetailDialog checklist={detailChecklist} onClose={() => setDetailChecklist(null)} onOpenWorkItem={openWorkItem} />
      <WorkItemDetailDialog
        open={Boolean(selectedWorkItem)}
        item={selectedWorkItem}
        onClose={() => setSelectedWorkItem(null)}
        onChanged={() => {
          queryClient.invalidateQueries({ queryKey: ['planning-checklists'] });
          checklists.refetch();
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

function PlanningChecklistDetailDialog({
  checklist,
  onClose,
  onOpenWorkItem,
}: {
  checklist: PlanningChecklistItem | null;
  onClose: () => void;
  onOpenWorkItem: (checklist: PlanningChecklistItem) => void;
}) {
  return (
    <Dialog
      open={Boolean(checklist)}
      title="Chi tiết checklist kế hoạch"
      description={checklist ? `${checklist.project_name} · ${dateText(checklist.check_date)} · ${planningScopeLabel(checklist.planning_scope)}` : undefined}
      size="xl"
      onClose={onClose}
      footer={checklist ? (
        <div className="flex flex-wrap justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} type="button">Đóng</button>
          <button
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-200 disabled:text-slate-500"
            disabled={!checklist.work_item_id}
            onClick={() => onOpenWorkItem(checklist)}
            type="button"
          >
            <ExternalLink className="h-4 w-4" />
            Mở work item/form
          </button>
        </div>
      ) : null}
    >
      {checklist ? (
        <div className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <DetailPill label="Status" value={<StatusBadge status={checklist.status} />} />
            <DetailPill label="Check date" value={dateText(checklist.check_date)} />
            <DetailPill label="Checked by" value={checklist.checked_by || checklist.submitted_by_name || '-'} />
            <DetailPill label="Updated" value={compactDateTime(checklist.updated_at)} />
          </div>
          <section className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-sm font-semibold text-ink">{checklistTitle(checklist)}</p>
            <div className="mt-2 grid gap-2 text-xs text-slate-600 md:grid-cols-2">
              <p>Scope: <span className="font-semibold text-ink">{planningScopeLabel(checklist.planning_scope)}</span></p>
              <p>Submitted by: <span className="font-semibold text-ink">{checklist.submitted_by_name || '-'}</span></p>
              <p>Node: <span className="font-semibold text-ink">{checklist.run_node_name ?? '-'}</span></p>
              <p>Step: <span className="font-semibold text-ink">{checklist.run_step_name ?? '-'}</span></p>
            </div>
          </section>
          <ChecklistItems checklist={checklist} />
          {checklist.overall_comment || checklist.items_done_text || checklist.exceptions_text ? (
            <section className="grid gap-3 xl:grid-cols-3">
              <LegacyText title="Items done" text={checklist.items_done_text} empty="Không có legacy items done." />
              <LegacyText title="Exceptions" text={checklist.exceptions_text} empty="Không có ngoại lệ." />
              <LegacyText title="Ghi chú tổng quan" text={checklist.overall_comment} empty="Không có ghi chú tổng quan." />
            </section>
          ) : null}
          <div className="grid gap-2 rounded-md border border-line bg-slate-50 p-3 text-xs text-slate-600 md:grid-cols-2">
            <p>Summary: <span className="font-semibold text-ink">{numberValue(checklist.done_count)} Done/N/A · {numberValue(checklist.needs_action_count)} needs action · {numberValue(checklist.evidence_linked_count)} evidence linked</span></p>
            <p>Artifacts: <span className="font-semibold text-ink">{numberValue(checklist.signature_count)} chữ ký · {numberValue(checklist.pdf_export_count)} PDF · {numberValue(checklist.evidence_count)} evidence</span></p>
          </div>
        </div>
      ) : <EmptyState title="Chưa chọn checklist kế hoạch." />}
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

function ChecklistItems({ checklist }: { checklist: PlanningChecklistItem }) {
  const items = arrayValue(checklist.checklist_items);
  return (
    <section>
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">Checklist items</h3>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{numberValue(checklist.item_count)}</span>
      </div>
      {items.length ? (
        <div className="grid gap-2 xl:grid-cols-2">
          {items.map((item, index) => (
            <article key={`checklist-item-${index}`} className="rounded-md border border-line bg-white px-3 py-2">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <p className="break-words text-sm font-semibold text-ink">#{index + 1} {itemTitle(item)}</p>
                <StatusBadge status={String(item.status || 'draft')} />
              </div>
              <p className="mt-1 break-words text-xs text-slate-500">{itemMeta(item)}</p>
              {item.comment ? <p className="mt-2 whitespace-pre-wrap break-words text-xs leading-5 text-slate-600">{String(item.comment)}</p> : null}
            </article>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-6 text-center text-sm text-slate-500">Chưa có structured checklist item.</div>
      )}
    </section>
  );
}

function LegacyText({ title, text, empty }: { title: string; text?: string; empty: string }) {
  return (
    <section className="rounded-md border border-line bg-white p-3">
      <h3 className="text-sm font-semibold text-ink">{title}</h3>
      {text ? <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{text}</p> : <p className="mt-2 text-sm text-slate-500">{empty}</p>}
    </section>
  );
}

function planningChecklistPath(filters: Filters) {
  const params = new URLSearchParams();
  if (filters.projectId) params.set('project_id', filters.projectId);
  if (filters.dateFrom) params.set('date_from', filters.dateFrom);
  if (filters.dateTo) params.set('date_to', filters.dateTo);
  if (filters.checkedBy.trim()) params.set('checked_by', filters.checkedBy.trim());
  if (filters.assignee.trim()) params.set('assignee', filters.assignee.trim());
  if (filters.itemStatus && filters.itemStatus !== 'all') params.set('item_status', filters.itemStatus);
  if (filters.evidence && filters.evidence !== 'all') params.set('evidence', filters.evidence);
  if (filters.status && filters.status !== 'all') params.set('status', filters.status);
  if (filters.search.trim()) params.set('search', filters.search.trim());
  const query = params.toString();
  return query ? `/planning/checklists?${query}` : '/planning/checklists';
}

function checklistMetrics(rows: PlanningChecklistItem[]) {
  return {
    total: rows.length,
    needsSubmit: rows.filter((row) => row.status !== 'submitted').length,
    submitted: rows.filter((row) => row.status === 'submitted').length,
    items: rows.reduce((sum, row) => sum + numberValue(row.item_count), 0),
    done: rows.reduce((sum, row) => sum + numberValue(row.done_count), 0),
    needsAction: rows.reduce((sum, row) => sum + numberValue(row.needs_action_count), 0),
    evidenceMissing: rows.reduce((sum, row) => sum + numberValue(row.evidence_missing_count), 0),
  };
}

function checklistToWorkItem(checklist: PlanningChecklistItem): WorkItem | null {
  if (!checklist.work_item_id) return null;
  return {
    id: checklist.work_item_id,
    project_id: checklist.project_id,
    project_name: checklist.project_name,
    title: checklist.work_item_title ?? checklistTitle(checklist),
    status: checklist.work_item_status ?? checklist.status,
    work_type: 'checklist',
    source_type: 'pipeline_run_step',
    source_id: checklist.run_step_id,
    run_node_id: checklist.run_node_id,
    run_node_name: checklist.run_node_name,
    run_step_id: checklist.run_step_id,
    run_step_name: checklist.run_step_name,
    workflow_form_instance_id: checklist.form_instance_id,
    workflow_form_status: checklist.status,
    workflow_form_type: 'checklist',
    workflow_form_revision: checklist.current_revision,
    workflow_form_submitted_at: checklist.submitted_at,
    workflow_form_template_code: checklist.template_code,
    workflow_form_template_name: checklist.template_name,
    workflow_form_version_no: checklist.version_no,
    evidence_count: numberValue(checklist.evidence_count),
  };
}

function arrayValue(value: unknown): Record<string, any>[] {
  return Array.isArray(value) ? value.filter((item): item is Record<string, any> => Boolean(item) && typeof item === 'object' && !Array.isArray(item)) : [];
}

function checklistTitle(checklist: PlanningChecklistItem) {
  const title = String(checklist.title ?? checklist.work_item_title ?? '').trim();
  return title || 'Checklist kế hoạch';
}

function itemTitle(item: Record<string, any>) {
  const title = String(item.title ?? item.content ?? '').trim();
  return title || 'Checklist item';
}

function itemMeta(item: Record<string, any>) {
  return [
    item.assignee_role || item.assignee_name ? `Assignee: ${[item.assignee_role, item.assignee_name].filter(Boolean).join(' / ')}` : '',
    item.due_date ? `Due: ${dateText(item.due_date)}` : '',
    item.evidence_id ? 'Evidence linked' : 'Evidence missing',
  ].filter(Boolean).join(' · ') || '-';
}

function planningScopeLabel(value?: string) {
  const key = String(value ?? '').trim();
  return planningScopeLabels[key] ?? (key || '-');
}
