import { useEffect, useMemo, useState } from 'react';
import { ArrowUpRight, ChevronLeft, ChevronRight, Search, SlidersHorizontal, X } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import type { WorkItem } from '../../lib/types/domain';
import {
  asArray,
  departmentLabel,
  hasWorkflowFormSummary,
  isWorkOverdue,
  numberValue,
  orderWorkItemsByWorkflow,
  workflowFormChipText,
} from '../work-items/workItemPresentation';
import { executionTargetActionLabel } from '../work-items/workItemExecutionTarget';

type WorkQueueFilter = 'actionable' | 'waiting' | 'blocked' | 'future' | 'completed' | 'skipped' | 'all';
type WorkSort = 'priority' | 'workflow' | 'due';

const PAGE_SIZE = 20;

const queueOptions: Array<{
  id: WorkQueueFilter;
  label: string;
  activeClassName: string;
  dotClassName: string;
}> = [
  { id: 'actionable', label: 'Cần xử lý', activeClassName: 'border-blue-200 bg-blue-50 text-blue-800', dotClassName: 'bg-blue-600' },
  { id: 'waiting', label: 'Chờ duyệt', activeClassName: 'border-amber-200 bg-amber-50 text-amber-800', dotClassName: 'bg-amber-500' },
  { id: 'blocked', label: 'Bị chặn', activeClassName: 'border-rose-200 bg-rose-50 text-rose-800', dotClassName: 'bg-rose-500' },
  { id: 'future', label: 'Chưa đến lượt', activeClassName: 'border-slate-300 bg-slate-100 text-slate-800', dotClassName: 'bg-slate-400' },
  { id: 'completed', label: 'Hoàn tất', activeClassName: 'border-emerald-200 bg-emerald-50 text-emerald-800', dotClassName: 'bg-emerald-500' },
  { id: 'skipped', label: 'Đã bỏ qua', activeClassName: 'border-slate-300 bg-slate-100 text-slate-700', dotClassName: 'bg-slate-300' },
  { id: 'all', label: 'Tất cả', activeClassName: 'border-blue-200 bg-blue-50 text-blue-800', dotClassName: 'bg-slate-500' },
];

export function ProjectWorkTab({ rows, onOpenWork }: { rows: WorkItem[]; onOpenWork: (item: WorkItem) => void }) {
  const [queue, setQueue] = useState<WorkQueueFilter>('actionable');
  const [query, setQuery] = useState('');
  const [stage, setStage] = useState('');
  const [owner, setOwner] = useState('');
  const [status, setStatus] = useState('');
  const [sort, setSort] = useState<WorkSort>('priority');
  const [page, setPage] = useState(1);

  const counts = useMemo(() => workQueueCounts(rows), [rows]);
  const stageOptions = useMemo(() => buildStageOptions(rows), [rows]);
  const ownerOptions = useMemo(() => buildOwnerOptions(rows), [rows]);
  const statusOptions = useMemo(() => buildStatusOptions(rows), [rows]);
  const filteredRows = useMemo(() => {
    const normalizedQuery = normalizeText(query);
    const filtered = rows.filter((item) => {
      if (!matchesQueue(item, queue)) return false;
      if (stage && workStageValue(item) !== stage) return false;
      if (owner && !workOwnerOptions(item).some((option) => option.value === owner)) return false;
      if (status && item.status !== status) return false;
      if (normalizedQuery && !workSearchText(item).includes(normalizedQuery)) return false;
      return true;
    });
    return sortWorkItems(filtered, sort);
  }, [owner, query, queue, rows, sort, stage, status]);

  const totalPages = Math.max(1, Math.ceil(filteredRows.length / PAGE_SIZE));
  const currentPage = Math.min(page, totalPages);
  const pageStart = (currentPage - 1) * PAGE_SIZE;
  const pageRows = filteredRows.slice(pageStart, pageStart + PAGE_SIZE);
  const rangeStart = filteredRows.length ? pageStart + 1 : 0;
  const rangeEnd = filteredRows.length ? Math.min(pageStart + pageRows.length, filteredRows.length) : 0;
  const hasFilters = Boolean(queue !== 'actionable' || query || stage || owner || status || sort !== 'priority');

  useEffect(() => {
    setPage(1);
  }, [owner, query, queue, sort, stage, status]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  function clearFilters() {
    setQueue('all');
    setQuery('');
    setStage('');
    setOwner('');
    setStatus('');
    setSort('priority');
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-ink">Công việc dự án</h2>
          <p className="mt-1 text-sm text-slate-600">Ưu tiên công việc cần hành động và vẫn giữ đầy đủ lịch sử workflow.</p>
        </div>
        <span className="text-sm font-semibold tabular-nums text-slate-600">{rows.length} đầu việc</span>
      </div>

      <div className="flex gap-2 overflow-x-auto pb-1" aria-label="Lọc nhanh theo trạng thái xử lý">
        {queueOptions.map((option) => {
          const selected = queue === option.id;
          return (
            <button
              key={option.id}
              type="button"
              aria-pressed={selected}
              className={clsx(
                'inline-flex shrink-0 items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold transition-colors',
                selected ? option.activeClassName : 'border-line bg-white text-slate-600 hover:border-slate-300 hover:bg-slate-50',
              )}
              onClick={() => setQueue(option.id)}
            >
              <span className={clsx('h-2 w-2 rounded-full', option.dotClassName)} />
              <span>{option.label}</span>
              <span className={clsx('tabular-nums', selected ? 'text-current' : 'text-slate-500')}>{counts[option.id]}</span>
            </button>
          );
        })}
      </div>

      <div className="rounded-lg border border-line bg-white shadow-panel">
        <div className="border-b border-line p-3">
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-[minmax(260px,1.5fr)_minmax(160px,0.8fr)_minmax(180px,0.9fr)_minmax(150px,0.7fr)_minmax(170px,0.75fr)_auto]">
            <label className="relative block">
              <span className="sr-only">Tìm công việc</span>
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                type="search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Tìm tên công việc..."
                className="h-10 w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm outline-none focus:border-blue-300 focus:ring-2 focus:ring-blue-100"
              />
            </label>
            <WorkFilterSelect label="Giai đoạn" value={stage} onChange={setStage} options={stageOptions} />
            <WorkFilterSelect label="Phụ trách" value={owner} onChange={setOwner} options={ownerOptions} />
            <WorkFilterSelect label="Trạng thái" value={status} onChange={setStatus} options={statusOptions} />
            <label>
              <span className="sr-only">Sắp xếp</span>
              <select
                value={sort}
                onChange={(event) => setSort(event.target.value as WorkSort)}
                className="h-10 w-full rounded-md border border-line bg-white px-3 text-sm text-slate-700 outline-none focus:border-blue-300 focus:ring-2 focus:ring-blue-100"
              >
                <option value="priority">Ưu tiên xử lý</option>
                <option value="workflow">Thứ tự workflow</option>
                <option value="due">Hạn gần nhất</option>
              </select>
            </label>
            <button
              type="button"
              disabled={!hasFilters}
              onClick={() => {
                setQueue('actionable');
                setQuery('');
                setStage('');
                setOwner('');
                setStatus('');
                setSort('priority');
              }}
              className="inline-flex h-10 items-center justify-center gap-1.5 rounded-md border border-line px-3 text-sm font-semibold text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
            >
              <X className="h-4 w-4" />
              Xóa lọc
            </button>
          </div>
          <div className="mt-3 flex items-center gap-2 text-xs font-medium text-slate-500">
            <SlidersHorizontal className="h-3.5 w-3.5" />
            Đang hiển thị {filteredRows.length}/{rows.length} đầu việc
          </div>
        </div>

        {!rows.length ? (
          <div className="p-4"><EmptyState title="Workflow chưa tạo công việc" /></div>
        ) : !filteredRows.length ? (
          <div className="p-4">
            <EmptyState
              title="Không có công việc phù hợp"
              action={(
                <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={clearFilters}>
                  Xem tất cả công việc
                </button>
              )}
            />
          </div>
        ) : (
          <>
            <div className="hidden overflow-x-auto lg:block">
              <table className="min-w-full divide-y divide-line">
                <thead className="bg-slate-50">
                  <tr>
                    <WorkTableHeader label="Trạng thái" />
                    <WorkTableHeader label="Công việc" />
                    <WorkTableHeader label="Giai đoạn" />
                    <WorkTableHeader label="Phụ trách" />
                    <WorkTableHeader label="Hạn" />
                    <WorkTableHeader label="Yêu cầu" />
                    <WorkTableHeader label="" align="right" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-line">
                  {pageRows.map((item) => (
                    <ProjectWorkRow key={item.id} item={item} onOpen={() => onOpenWork(item)} />
                  ))}
                </tbody>
              </table>
            </div>

            <div className="space-y-3 p-3 lg:hidden">
              {pageRows.map((item) => (
                <ProjectWorkMobileCard key={item.id} item={item} onOpen={() => onOpenWork(item)} />
              ))}
            </div>
          </>
        )}

        {filteredRows.length > PAGE_SIZE ? (
          <div className="flex flex-col gap-3 border-t border-line bg-slate-50 px-4 py-3 text-xs font-medium text-slate-600 sm:flex-row sm:items-center sm:justify-between">
            <span className="tabular-nums">{rangeStart}-{rangeEnd} / {filteredRows.length} đầu việc</span>
            <div className="flex items-center justify-end gap-2">
              <span>Trang {currentPage} / {totalPages}</span>
              <button
                type="button"
                className="rounded-md border border-line bg-white p-1.5 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                disabled={currentPage <= 1}
                title="Trang trước"
                onClick={() => setPage((value) => Math.max(1, value - 1))}
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <button
                type="button"
                className="rounded-md border border-line bg-white p-1.5 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                disabled={currentPage >= totalPages}
                title="Trang sau"
                onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function WorkFilterSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
}) {
  return (
    <label>
      <span className="sr-only">{label}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-10 w-full rounded-md border border-line bg-white px-3 text-sm text-slate-700 outline-none focus:border-blue-300 focus:ring-2 focus:ring-blue-100"
      >
        <option value="">{label}: Tất cả</option>
        {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
    </label>
  );
}

function WorkTableHeader({ label, align = 'left' }: { label: string; align?: 'left' | 'right' }) {
  return <th className={clsx('px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500', align === 'right' && 'text-right')}>{label}</th>;
}

function ProjectWorkRow({ item, onOpen }: { item: WorkItem; onOpen: () => void }) {
  const owner = workOwnerPresentation(item);
  const stage = item.run_node_name ?? 'Ngoài workflow';
  const step = item.run_step_name && item.run_step_name !== item.title ? item.run_step_name : undefined;
  return (
    <tr className={clsx('hover:bg-slate-50', item.status === 'cancelled' && 'bg-slate-50/70')}>
      <td className="whitespace-nowrap px-4 py-3 align-top"><WorkStatus item={item} /></td>
      <td className="min-w-[300px] px-4 py-3 align-top">
        <p className="font-semibold text-ink">{item.title}</p>
        {step ? <p className="mt-1 line-clamp-1 text-xs text-slate-500">{step}</p> : null}
        {item.blocked_reason ? <p className="mt-2 line-clamp-2 text-xs font-medium text-rose-700">{item.blocked_reason}</p> : null}
      </td>
      <td className="min-w-[150px] px-4 py-3 align-top">
        <p className="text-sm font-medium text-slate-700">{stage}</p>
        {item.run_node_order_index ? <p className="mt-1 text-xs text-slate-500">Giai đoạn {item.run_node_order_index}</p> : null}
      </td>
      <td className="min-w-[170px] px-4 py-3 align-top">
        <p className="text-sm font-medium text-slate-700">{owner.primary}</p>
        {owner.secondary ? <p className="mt-1 text-xs text-slate-500">{owner.secondary}</p> : null}
      </td>
      <td className="whitespace-nowrap px-4 py-3 align-top text-sm text-slate-600">{dateText(workDueDate(item))}</td>
      <td className="min-w-[180px] px-4 py-3 align-top">
        <p className={clsx('text-xs font-semibold', item.workflow_form_required ? 'text-emerald-700' : 'text-slate-600')}>
          {workFormRequirement(item)}
        </p>
        <p className="mt-1 text-xs text-slate-500">{numberValue(item.evidence_count)} minh chứng</p>
      </td>
      <td className="px-4 py-3 text-right align-top">
        <OpenWorkButton item={item} onOpen={onOpen} />
      </td>
    </tr>
  );
}

function ProjectWorkMobileCard({ item, onOpen }: { item: WorkItem; onOpen: () => void }) {
  const owner = workOwnerPresentation(item);
  return (
    <article className={clsx('rounded-md border border-line bg-white p-3', isWorkOverdue(item) && 'border-rose-200 bg-rose-50/40')}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <WorkStatus item={item} />
          <p className="mt-2 font-semibold text-ink">{item.title}</p>
          <p className="mt-1 text-xs text-slate-500">{item.run_node_name ?? 'Ngoài workflow'}</p>
        </div>
        <OpenWorkButton item={item} onOpen={onOpen} />
      </div>
      <div className="mt-3 grid gap-1.5 text-xs text-slate-600 sm:grid-cols-2">
        <span>Phụ trách: {owner.primary}</span>
        <span>Hạn: {dateText(workDueDate(item))}</span>
        <span>{workFormRequirement(item)}</span>
        <span>{numberValue(item.evidence_count)} minh chứng</span>
      </div>
      {item.blocked_reason ? <p className="mt-3 rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{item.blocked_reason}</p> : null}
    </article>
  );
}

function WorkStatus({ item }: { item: WorkItem }) {
  const displayStatus = item.status === 'cancelled' ? 'skipped' : isWorkOverdue(item) ? 'overdue' : item.status;
  return <StatusBadge status={displayStatus} />;
}

function OpenWorkButton({ item, onOpen }: { item: WorkItem; onOpen: () => void }) {
  return (
    <button
      type="button"
      className="inline-flex shrink-0 items-center justify-center gap-1 rounded-md border border-line bg-white px-2.5 py-1.5 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
      onClick={onOpen}
      aria-label={`Mở ${item.title}`}
    >
      {executionTargetActionLabel(item)}
      <ArrowUpRight className="h-3.5 w-3.5" />
    </button>
  );
}

function workQueueCounts(rows: WorkItem[]): Record<WorkQueueFilter, number> {
  return rows.reduce<Record<WorkQueueFilter, number>>((counts, item) => {
    counts.all += 1;
    counts[queueForWorkItem(item)] += 1;
    return counts;
  }, { actionable: 0, waiting: 0, blocked: 0, future: 0, completed: 0, skipped: 0, all: 0 });
}

function queueForWorkItem(item: WorkItem): Exclude<WorkQueueFilter, 'all'> {
  if (item.status === 'cancelled') return 'skipped';
  if (item.status === 'completed') return 'completed';
  if (item.status === 'blocked') return 'blocked';
  if (item.status === 'waiting_approval' || item.status === 'waiting_review') return 'waiting';
  if (item.status === 'pending') return 'future';
  return 'actionable';
}

function matchesQueue(item: WorkItem, queue: WorkQueueFilter) {
  return queue === 'all' || queueForWorkItem(item) === queue;
}

function buildStageOptions(rows: WorkItem[]) {
  const options = new Map<string, string>();
  rows.forEach((item) => {
    const value = workStageValue(item);
    if (!options.has(value)) {
      options.set(value, item.run_node_name
        ? `${item.run_node_order_index ? `${item.run_node_order_index}. ` : ''}${item.run_node_name}`
        : 'Ngoài workflow');
    }
  });
  return [...options.entries()].map(([value, label]) => ({ value, label }));
}

function workStageValue(item: WorkItem) {
  return item.run_node_id ?? item.run_node_name ?? '__outside_workflow__';
}

function buildOwnerOptions(rows: WorkItem[]) {
  const options = new Map<string, string>();
  rows.forEach((item) => {
    workOwnerOptions(item).forEach((option) => options.set(option.value, option.label));
  });
  return [...options.entries()]
    .map(([value, label]) => ({ value, label }))
    .sort((left, right) => left.label.localeCompare(right.label, 'vi'));
}

function workOwnerOptions(item: WorkItem): Array<{ value: string; label: string }> {
  const options: Array<{ value: string; label: string }> = [];
  if (item.assignee_name) {
    options.push({ value: `assignee:${item.assignee_staff_id ?? item.assignee_name}`, label: item.assignee_name });
  }
  asArray<string>(item.role_ids).filter(Boolean).forEach((role) => {
    options.push({ value: `role:${role}`, label: `Vai trò: ${role}` });
  });
  if (!options.length && (item.department_name || item.department)) {
    const department = item.department_name ?? departmentLabel(item.department);
    options.push({ value: `department:${department}`, label: `Bộ phận: ${department}` });
  }
  if (!options.length) options.push({ value: 'unassigned', label: 'Chưa phân công' });
  return options;
}

function workOwnerPresentation(item: WorkItem) {
  const roles = asArray<string>(item.role_ids).filter(Boolean);
  if (item.assignee_name) {
    return { primary: item.assignee_name, secondary: roles.length ? `Vai trò: ${roles.join(', ')}` : undefined };
  }
  if (roles.length) return { primary: roles.join(', '), secondary: 'Vai trò yêu cầu' };
  if (item.department_name || item.department) {
    return { primary: item.department_name ?? departmentLabel(item.department), secondary: 'Bộ phận phụ trách' };
  }
  return { primary: 'Chưa phân công', secondary: undefined };
}

function buildStatusOptions(rows: WorkItem[]) {
  return [...new Set(rows.map((item) => item.status).filter(Boolean))]
    .map((value) => ({ value, label: statusLabel(value === 'cancelled' ? 'skipped' : value) }))
    .sort((left, right) => left.label.localeCompare(right.label, 'vi'));
}

function sortWorkItems(rows: WorkItem[], sort: WorkSort) {
  if (sort === 'workflow') return orderWorkItemsByWorkflow(rows);
  return [...rows].sort((left, right) => {
    if (sort === 'priority') {
      const priorityDiff = workPriorityRank(left) - workPriorityRank(right);
      if (priorityDiff !== 0) return priorityDiff;
    }
    const dueDiff = dateSortValue(workDueDate(left)) - dateSortValue(workDueDate(right));
    if (dueDiff !== 0) return dueDiff;
    return compareWorkflowPosition(left, right);
  });
}

function workPriorityRank(item: WorkItem) {
  if (item.status === 'blocked') return 0;
  if (item.status === 'waiting_approval' || item.status === 'waiting_review') return 1;
  if (isWorkOverdue(item)) return 2;
  if (queueForWorkItem(item) === 'actionable') return 3;
  if (item.status === 'pending') return 4;
  if (item.status === 'completed') return 5;
  return 6;
}

function compareWorkflowPosition(left: WorkItem, right: WorkItem) {
  const nodeDiff = numberSortValue(left.run_node_order_index) - numberSortValue(right.run_node_order_index);
  if (nodeDiff !== 0) return nodeDiff;
  const stepDiff = numberSortValue(left.run_step_order_index) - numberSortValue(right.run_step_order_index);
  if (stepDiff !== 0) return stepDiff;
  return String(left.id).localeCompare(String(right.id));
}

function numberSortValue(value: unknown) {
  const number = Number(value);
  return Number.isFinite(number) ? number : Number.MAX_SAFE_INTEGER;
}

function dateSortValue(value: unknown) {
  if (!value) return Number.MAX_SAFE_INTEGER;
  const time = new Date(String(value)).getTime();
  return Number.isNaN(time) ? Number.MAX_SAFE_INTEGER : time;
}

function workDueDate(item: WorkItem) {
  return item.due_at ?? item.due_date ?? item.deadline ?? item.planned_end_date;
}

function workFormRequirement(item: WorkItem) {
  if (item.workflow_form_required) return 'Hồ sơ bắt buộc';
  if (hasWorkflowFormSummary(item)) return 'Hồ sơ tùy chọn';
  return 'Không yêu cầu hồ sơ';
}

function workSearchText(item: WorkItem) {
  const owner = workOwnerPresentation(item);
  return normalizeText([
    item.title,
    item.description,
    item.run_node_name,
    item.run_step_name,
    owner.primary,
    owner.secondary,
    item.department,
    hasWorkflowFormSummary(item) ? workflowFormChipText(item) : '',
  ].filter(Boolean).join(' '));
}

function normalizeText(value: unknown) {
  return String(value ?? '')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLocaleLowerCase('vi-VN')
    .trim();
}
