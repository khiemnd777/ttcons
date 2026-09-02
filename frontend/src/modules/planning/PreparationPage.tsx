import { useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, ChevronDown, ChevronRight, FileCheck2, Search } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import type { ProjectOperationsResponse, WorkItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { WorkItemDetailWorkspace } from '../work-items/WorkItemDetailDialog';
import { DesignReleasePanel } from '../work-items/DesignReleasePanel';
import { executionTargetActionLabel } from '../work-items/workItemExecutionTarget';
import { safeReturnPath } from '../work-items/workItemNavigation';
import { buildWorkItemWorkflowNavigation } from '../work-items/workItemWorkflowNavigation';
import { groupPreparationItems, type PreparationStageGroup } from './preparationGrouping';
import {
  asArray,
  hasWorkflowFormSummary,
  workItemIsUpcoming,
  workflowFormBusinessLabel,
  workflowFormStatusLabel,
} from '../work-items/workItemPresentation';

type PreparationFilter = 'need_action' | 'upcoming' | 'done' | 'all';

const PREPARATION_TABLE_GRID = 'grid-cols-[8rem_7.5rem_minmax(18rem,1.8fr)_minmax(10rem,1fr)_5.5rem]';

export function PreparationPage() {
  const { id = '' } = useParams();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<PreparationFilter>('need_action');
  const taskId = searchParams.get('task') ?? '';
  const returnTo = safeReturnPath(searchParams.get('returnTo'));
  const operations = useQuery({
    queryKey: ['project-operations', id, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${id}/operations`),
    enabled: Boolean(id),
    retry: false,
  });
  const allPreparationItems = useMemo(
    () => (operations.data?.work_items ?? []).filter((item) => item.execution_target?.workspace === 'preparation'),
    [operations.data?.work_items],
  );
  const counts = useMemo(() => preparationCounts(allPreparationItems), [allPreparationItems]);
  const preparationItems = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase('vi');
    return allPreparationItems
      .filter((item) => preparationFilterMatches(item, filter))
      .filter((item) => !needle || [
        item.title,
        item.description,
        item.run_node_name,
        item.run_step_name,
        item.assignee_name,
        ...asArray<string>(item.role_ids),
        workflowFormBusinessLabel(item),
      ].some((value) => String(value ?? '').toLocaleLowerCase('vi').includes(needle)));
  }, [allPreparationItems, filter, search]);
  const selectedItem = allPreparationItems.find((item) => item.id === taskId);
  const workflowNavigation = useMemo(() => {
    const context = buildWorkItemWorkflowNavigation(operations.data?.work_items ?? [], taskId);
    return context ? {
      ...context,
      returnTo: returnTo || `/projects/${id}/preparation`,
      progressPath: `/projects/${id}/pipeline`,
    } : undefined;
  }, [id, operations.data?.work_items, returnTo, taskId]);
  const invalidTask = Boolean(taskId && !operations.isLoading && !operations.error && !selectedItem);

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'auto' });
  }, [taskId]);

  function selectTask(item: WorkItem) {
    navigate(withTask(location.pathname, location.search, item.id));
  }

  function closeTask() {
    if (returnTo) {
      navigate(returnTo);
      return;
    }
    const next = new URLSearchParams(location.search);
    next.delete('task');
    next.delete('returnTo');
    navigate(`${location.pathname}${next.size ? `?${next.toString()}` : ''}`);
  }

  if (taskId) {
    return (
      <div data-preparation-mode="detail">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
            onClick={closeTask}
          >
            <ArrowLeft className="h-4 w-4" />
            {returnTo ? 'Quay lại' : 'Danh sách hồ sơ'}
          </button>
          <Link className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50" to={`/projects/${id}/pipeline`}>
            Quy trình
          </Link>
        </div>

        {operations.isLoading ? <div className="py-12"><LoadingSpinner /></div> : null}
        {operations.error ? (
          <PreparationLoadError onRetry={() => void operations.refetch()} />
        ) : invalidTask ? (
          <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
            <p className="font-semibold">Công việc này không thuộc Hồ sơ chuẩn bị.</p>
            <p className="mt-1">Mở công việc từ Bàn làm việc hoặc Quy trình để App đưa đến đúng sổ nghiệp vụ.</p>
            <button className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 font-semibold" onClick={closeTask}>Đóng ngữ cảnh sai</button>
          </div>
        ) : selectedItem ? (
          <WorkItemDetailWorkspace
            workItemId={selectedItem.id}
            workflowNavigation={workflowNavigation}
            onClose={closeTask}
            onChanged={() => {
              void queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
              void queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
            }}
          />
        ) : null}
      </div>
    );
  }

  return (
    <div data-preparation-mode="queue">
      <PageHeader
        title="Hồ sơ chuẩn bị"
        description="Hồ sơ tiếp nhận, khảo sát và thiết kế cần chuẩn bị."
        actions={<Link className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50" to={`/projects/${id}/pipeline`}>Quy trình</Link>}
      />

      <div className="mt-4">
        <DesignReleasePanel projectId={id} workItems={allPreparationItems} />
      </div>

      <section className="mt-4 overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-label="Danh sách hồ sơ chuẩn bị">
        <div className="border-b border-line p-3 sm:p-4">
          <div className="flex min-w-0 items-center gap-2">
            <FileCheck2 className="h-4 w-4 shrink-0 text-blue-700" />
            <h2 className="text-sm font-semibold text-ink">Hồ sơ trong kế hoạch</h2>
            <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{counts.all}</span>
          </div>

          <div className="mt-3 flex gap-2 overflow-x-auto pb-1" role="group" aria-label="Lọc hồ sơ theo trạng thái">
            {preparationFilterOptions(counts).map((option) => (
              <button
                key={option.value}
                type="button"
                aria-pressed={filter === option.value}
                className={clsx(
                  'inline-flex shrink-0 items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold',
                  filter === option.value
                    ? 'border-blue-600 bg-blue-600 text-white'
                    : 'border-line bg-white text-slate-600 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700',
                )}
                onClick={() => setFilter(option.value)}
              >
                {option.label}
                <span className={clsx('rounded-full px-1.5 py-0.5 text-xs', filter === option.value ? 'bg-white/20 text-white' : 'bg-slate-100 text-slate-600')}>
                  {option.count}
                </span>
              </button>
            ))}
          </div>

          <label className="relative mt-3 block">
            <span className="sr-only">Tìm hồ sơ</span>
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
            <input
              className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Tìm tên hồ sơ, bước, người phụ trách..."
            />
          </label>
        </div>

        {operations.isLoading ? <PreparationQueueLoading /> : null}
        {operations.error ? <div className="p-4"><PreparationLoadError onRetry={() => void operations.refetch()} /></div> : null}
        {!operations.isLoading && !operations.error && !preparationItems.length ? (
          <div className="p-4">
            <EmptyState
              title={search ? 'Không có hồ sơ khớp tìm kiếm' : filter === 'need_action' ? 'Không có hồ sơ cần xử lý' : 'Không có hồ sơ trong nhóm này'}
              action={filter !== 'all' || search ? (
                <button
                  type="button"
                  className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50"
                  onClick={() => { setFilter('all'); setSearch(''); }}
                >
                  Xem tất cả {counts.all} hồ sơ
                </button>
              ) : undefined}
            />
          </div>
        ) : null}
        {!operations.isLoading && !operations.error && preparationItems.length ? (
          <PreparationQueue items={preparationItems} onOpen={selectTask} />
        ) : null}
      </section>
    </div>
  );
}

function PreparationQueue({ items, onOpen }: { items: WorkItem[]; onOpen: (item: WorkItem) => void }) {
  const groups = useMemo(() => groupPreparationItems(items), [items]);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => new Set());

  function toggleGroup(key: string) {
    setCollapsedGroups((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <>
      <div className="xl:hidden" data-preparation-mobile-list>
        {groups.map((group, index) => {
          const collapsed = collapsedGroups.has(group.key);
          const contentId = `preparation-mobile-stage-${index}`;
          return (
            <section key={group.key} data-preparation-stage-group data-preparation-stage-name={group.name}>
              <PreparationStageHeader
                group={group}
                collapsed={collapsed}
                controlsId={contentId}
                onToggle={() => toggleGroup(group.key)}
              />
              <div id={contentId} className={clsx('divide-y divide-line', collapsed && 'hidden')}>
                {group.items.map((item) => <PreparationMobileRow key={item.id} item={item} onOpen={() => onOpen(item)} />)}
              </div>
            </section>
          );
        })}
      </div>
      <div className="hidden xl:block" data-preparation-desktop-table>
        <div className={clsx('grid items-center gap-3 border-b border-line bg-slate-50 px-4 py-2.5 text-xs font-semibold text-slate-500', PREPARATION_TABLE_GRID)}>
          <span><span className="sr-only">Thao tác</span></span>
          <span>Trạng thái</span>
          <span>Hồ sơ / Công việc</span>
          <span>Phụ trách</span>
          <span>Hạn</span>
        </div>
        {groups.map((group, index) => {
          const collapsed = collapsedGroups.has(group.key);
          const contentId = `preparation-desktop-stage-${index}`;
          return (
            <section key={group.key} data-preparation-stage-group data-preparation-stage-name={group.name}>
              <PreparationStageHeader
                group={group}
                collapsed={collapsed}
                controlsId={contentId}
                onToggle={() => toggleGroup(group.key)}
              />
              <div id={contentId} className={clsx('divide-y divide-line', collapsed && 'hidden')}>
                {group.items.map((item) => <PreparationDesktopRow key={item.id} item={item} onOpen={() => onOpen(item)} />)}
              </div>
            </section>
          );
        })}
      </div>
    </>
  );
}

function PreparationStageHeader({
  group,
  collapsed,
  controlsId,
  onToggle,
}: {
  group: PreparationStageGroup;
  collapsed: boolean;
  controlsId: string;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      className="flex w-full items-center justify-between gap-3 border-b border-l-4 border-line border-l-blue-500 bg-slate-100 px-4 py-3 text-left hover:bg-slate-200/70"
      aria-expanded={!collapsed}
      aria-controls={controlsId}
      aria-label={`${collapsed ? 'Mở' : 'Thu gọn'} Giai đoạn ${group.name}`}
      onClick={onToggle}
    >
      <span className="flex min-w-0 items-center gap-2">
        {collapsed ? <ChevronRight className="h-4 w-4 shrink-0 text-blue-700" /> : <ChevronDown className="h-4 w-4 shrink-0 text-blue-700" />}
        <span className="min-w-0 truncate text-sm font-semibold text-ink">
          <span className="mr-1 text-xs uppercase tracking-wide text-slate-500">Giai đoạn</span>
          {group.name}
        </span>
      </span>
      <span className="shrink-0 rounded-full border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-600">
        {group.items.length} hồ sơ
      </span>
    </button>
  );
}

function PreparationDesktopRow({ item, onOpen }: { item: WorkItem; onOpen: () => void }) {
  return (
    <div className={clsx('grid items-center gap-3 px-4 py-3 hover:bg-slate-50', PREPARATION_TABLE_GRID)}>
      <div><PreparationOpenButton item={item} onOpen={onOpen} /></div>
      <div><StatusBadge status={item.status} /></div>
      <PreparationDocumentSummary item={item} />
      <p className="min-w-0 text-sm text-slate-600">{workItemOwner(item)}</p>
      <p className="whitespace-nowrap text-sm text-slate-500">{dateText(item.due_at)}</p>
    </div>
  );
}

function PreparationMobileRow({ item, onOpen }: { item: WorkItem; onOpen: () => void }) {
  return (
    <article className="p-4">
      <div><StatusBadge status={item.status} /></div>
      <div className="mt-3"><PreparationDocumentSummary item={item} /></div>
      <div className="mt-3 grid grid-cols-2 gap-2 text-xs text-slate-500">
        <p><span className="font-semibold text-slate-600">Phụ trách:</span> {workItemOwner(item)}</p>
        <p><span className="font-semibold text-slate-600">Hạn:</span> {dateText(item.due_at)}</p>
      </div>
      <div className="mt-3"><PreparationOpenButton item={item} onOpen={onOpen} fullWidth /></div>
    </article>
  );
}

function PreparationDocumentSummary({ item }: { item: WorkItem }) {
  const formLabel = hasWorkflowFormSummary(item) ? workflowFormBusinessLabel(item) : item.title;
  const stepNumber = Number(item.run_step_order_index ?? 0);
  const details = [
    stepNumber > 0 ? `Bước ${stepNumber}` : item.run_step_name,
    hasWorkflowFormSummary(item) ? `Hồ sơ ${item.workflow_form_required ? 'bắt buộc' : 'tùy chọn'}` : 'Minh chứng',
    hasWorkflowFormSummary(item) ? workflowFormStatusLabel(item.workflow_form_status) : null,
  ].filter(Boolean).join(' · ');
  return (
    <div className="min-w-0">
      <p className="font-semibold leading-5 text-ink">{formLabel}</p>
      {formLabel !== item.title ? <p className="mt-0.5 truncate text-xs font-medium text-slate-600">{item.title}</p> : null}
      <p className="mt-0.5 text-xs text-slate-500">{details}</p>
    </div>
  );
}

function PreparationOpenButton({ item, onOpen, fullWidth = false }: { item: WorkItem; onOpen: () => void; fullWidth?: boolean }) {
  const label = workItemIsUpcoming(item) ? 'Xem trước' : executionTargetActionLabel(item);
  return (
    <button
      type="button"
      aria-label={`${label} ${item.title}`}
      className={clsx(
        'inline-flex items-center justify-center gap-1.5 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50',
        fullWidth && 'w-full',
      )}
      onClick={onOpen}
    >
      {label}
      <ChevronRight className="h-4 w-4" />
    </button>
  );
}

function PreparationQueueLoading() {
  return (
    <div aria-label="Đang tải danh sách hồ sơ">
      <div className="flex animate-pulse items-center justify-between border-b border-l-4 border-line border-l-slate-200 bg-slate-100 px-4 py-3">
        <span className="h-5 w-48 rounded bg-slate-200" />
        <span className="h-5 w-16 rounded bg-white" />
      </div>
      <div className="space-y-px bg-line">
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="grid animate-pulse gap-3 bg-white px-4 py-4 sm:grid-cols-[7rem_minmax(0,1fr)_7rem]">
            <span className="h-5 rounded bg-slate-100" />
            <span className="h-5 rounded bg-slate-100" />
            <span className="h-5 rounded bg-slate-100" />
          </div>
        ))}
      </div>
    </div>
  );
}

function PreparationLoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">
      <p className="font-semibold">Không tải được danh sách hồ sơ.</p>
      <button type="button" className="mt-2 rounded-md border border-rose-200 bg-white px-3 py-2 font-semibold hover:bg-rose-100" onClick={onRetry}>Thử tải lại</button>
    </div>
  );
}

function preparationCounts(items: WorkItem[]) {
  return {
    needAction: items.filter((item) => preparationFilterMatches(item, 'need_action')).length,
    upcoming: items.filter((item) => preparationFilterMatches(item, 'upcoming')).length,
    done: items.filter((item) => preparationFilterMatches(item, 'done')).length,
    all: items.length,
  };
}

function preparationFilterOptions(counts: ReturnType<typeof preparationCounts>): { value: PreparationFilter; label: string; count: number }[] {
  return [
    { value: 'need_action', label: 'Cần xử lý', count: counts.needAction },
    { value: 'upcoming', label: 'Sắp tới', count: counts.upcoming },
    { value: 'done', label: 'Hoàn tất', count: counts.done },
    { value: 'all', label: 'Tất cả', count: counts.all },
  ];
}

function preparationFilterMatches(item: WorkItem, filter: PreparationFilter) {
  if (filter === 'all') return true;
  if (filter === 'upcoming') return workItemIsUpcoming(item);
  if (filter === 'done') return ['completed', 'submitted'].includes(item.status);
  return !workItemIsUpcoming(item) && !['completed', 'submitted', 'cancelled', 'skipped'].includes(item.status);
}

function workItemOwner(item: WorkItem) {
  if (!item.assignee_name) return asArray<string>(item.role_ids).join(', ') || '-';
  const label = identityReferenceText(item.assignee_name, item.assignee_account_deleted);
  return item.assignee_account_deleted ? `${label} · Cần phân công lại` : label;
}

function withTask(pathname: string, search: string, taskId: string) {
  const params = new URLSearchParams(search);
  params.set('task', taskId);
  return `${pathname}?${params.toString()}`;
}
