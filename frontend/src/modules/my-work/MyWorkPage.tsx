import { useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertCircle,
  AtSign,
  Bell,
  BriefcaseBusiness,
  CheckCircle2,
  ChevronRight,
  Clock3,
  MessageSquare,
  RefreshCw,
  Search,
  ShieldAlert,
  SlidersHorizontal,
  X,
} from 'lucide-react';
import clsx from 'clsx';
import { get, post, streamSSE, type StreamStatus } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import type { ActivityAttentionItem, ApprovalRequest, WorkItem } from '../../lib/types/domain';
import { ApprovalPanel } from '../../components/ApprovalPanel';
import { dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import { ActivityAttachments } from '../work-items/ActivityAttachments';
import { ActivityReactionBar, ActivityReplyQuote } from '../work-items/ActivityConversationMeta';
import { WorkItemCard } from '../work-items/WorkItemCard';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import { executionTargetActionLabel, executionTargetPath, executionTargetWorkspaceLabel } from '../work-items/workItemExecutionTarget';
import {
  asArray,
  activityLabel,
  compactDateTime,
  departmentLabel,
  isWorkOverdue,
  numberValue,
  priorityLabel,
  scopeTypeLabel,
  workItemIsUpcoming,
  workItemWorkflowContext,
  workTypeLabel,
} from '../work-items/workItemPresentation';

type ModeKey = 'work' | 'attention';
type QueueKey = 'mine' | 'department' | 'pending_approval' | 'watching' | 'all';
type AttentionQueueKey = 'all' | 'mentions' | 'questions' | 'blockers' | 'reminders' | 'unread';
type SummaryBucket = 'all' | 'need_action' | 'in_progress' | 'overdue' | 'blocked' | 'waiting_decision' | 'upcoming' | 'done_today';

type FacetOption = string | {
  id?: string;
  value?: string;
  label?: string;
  code?: string;
  name?: string;
  project_id?: string;
  project_code?: string;
  project_name?: string;
};
type MyWorkFacets = {
  projects?: FacetOption[];
  statuses?: FacetOption[];
  work_types?: FacetOption[];
  priorities?: FacetOption[];
};
type MyWorkSummary = {
  total: number;
  need_action: number;
  in_progress: number;
  overdue: number;
  blocked: number;
  waiting_decision: number;
  waiting_approval: number;
  upcoming: number;
  done_today: number;
};
type MyWorkPageResponse = {
  items: WorkItem[];
  summary: MyWorkSummary;
  facets: MyWorkFacets;
  total: number;
  has_more: boolean;
  next_cursor?: string;
};

const queues: Array<{ key: QueueKey; label: string; description: string }> = [
  { key: 'mine', label: 'Của tôi', description: 'Được giao trực tiếp' },
  { key: 'department', label: 'Phòng ban', description: 'Theo phòng ban hoặc vai trò' },
  { key: 'pending_approval', label: 'Chờ tôi duyệt', description: 'Tôi có quyền quyết định' },
  { key: 'watching', label: 'Đang theo dõi', description: 'Việc tôi đang quan sát' },
  { key: 'all', label: 'Tất cả', description: 'Toàn bộ việc trong phạm vi' },
];

const attentionQueues: Array<{ key: AttentionQueueKey; label: string; icon: typeof MessageSquare }> = [
  { key: 'all', label: 'Tất cả', icon: MessageSquare },
  { key: 'mentions', label: 'Nhắc đến', icon: AtSign },
  { key: 'questions', label: 'Câu hỏi', icon: MessageSquare },
  { key: 'blockers', label: 'Điểm chặn', icon: AlertCircle },
  { key: 'reminders', label: 'Nhắc hạn', icon: Bell },
  { key: 'unread', label: 'Chưa đọc', icon: CheckCircle2 },
];

const statusOptions = [
  { value: 'ready', label: 'Sẵn sàng' },
  { value: 'in_progress', label: 'Đang xử lý' },
  { value: 'blocked', label: 'Bị chặn' },
  { value: 'waiting_review', label: 'Chờ rà soát' },
  { value: 'waiting_approval', label: 'Chờ duyệt' },
  { value: 'waiting_evidence', label: 'Chờ minh chứng' },
  { value: 'completed', label: 'Hoàn tất' },
];

const deadlineOptions = [
  { value: 'overdue', label: 'Quá hạn' },
  { value: 'due_7_days', label: 'Đến hạn trong 7 ngày' },
  { value: 'no_due', label: 'Chưa có hạn' },
];

const sortOptions = [
  { value: 'priority', label: 'Ưu tiên xử lý' },
  { value: 'due_asc', label: 'Hạn gần nhất' },
  { value: 'newest', label: 'Mới cập nhật' },
];

export function MyWorkPage() {
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [selectedWorkId, setSelectedWorkId] = useState<string | null>(null);
  const [searchDraft, setSearchDraft] = useState(searchParams.get('search') ?? '');
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('reconnecting');
  const [advancedFiltersOpen, setAdvancedFiltersOpen] = useState(false);

  const mode = validValue<ModeKey>(searchParams.get('mode'), ['work', 'attention'], 'work');
  const queue = validValue<QueueKey>(searchParams.get('queue'), ['mine', 'department', 'pending_approval', 'watching', 'all'], 'all');
  const attentionQueue = validValue<AttentionQueueKey>(searchParams.get('attention'), ['all', 'mentions', 'questions', 'blockers', 'reminders', 'unread'], 'all');
  const bucket = validValue<SummaryBucket>(searchParams.get('bucket'), ['all', 'need_action', 'in_progress', 'overdue', 'blocked', 'waiting_decision', 'upcoming', 'done_today'], 'need_action');
  const project = searchParams.get('project') ?? '';
  const status = searchParams.get('status') ?? '';
  const deadline = searchParams.get('deadline') ?? '';
  const workType = searchParams.get('type') ?? '';
  const priority = searchParams.get('priority') ?? '';
  const search = searchParams.get('search') ?? '';
  const sort = searchParams.get('sort') ?? 'priority';

  useEffect(() => setSearchDraft(search), [search]);

  const filterKey = useMemo(() => ({ queue, bucket, project, status, deadline, workType, priority, search, sort }), [bucket, deadline, priority, project, queue, search, sort, status, workType]);
  const workItems = useInfiniteQuery({
    queryKey: ['my-work-items', filterKey],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<MyWorkPageResponse>(buildWorkPath(filterKey, String(pageParam ?? ''))),
    getNextPageParam: (lastPage) => lastPage.has_more ? lastPage.next_cursor : undefined,
    retry: false,
  });
  const pendingNodeApprovals = useQuery({
    queryKey: ['approvals', 'pending', 'decidable', project],
    queryFn: () => {
      const params = new URLSearchParams({ decidable_only: 'true' });
      if (project) params.set('project_id', project);
      return get<ApprovalRequest[]>(`/approvals/pending?${params.toString()}`);
    },
    enabled: mode === 'work',
    retry: false,
  });

  const attentionPath = useMemo(() => {
    const params = new URLSearchParams({ limit: '120' });
    if (project) params.set('project_id', project);
    return `/me/activity-attention?${params.toString()}`;
  }, [project]);
  const activityAttention = useQuery({
    queryKey: ['activity-attention', project],
    queryFn: () => get<ActivityAttentionItem[]>(attentionPath),
    retry: false,
  });
  const markAttentionRead = useMutation({
    mutationFn: (activityId: string) => post(`/activities/${activityId}/read`),
    onSuccess: (_row, activityId) => {
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      queryClient.invalidateQueries({ queryKey: ['project-activity'] });
      const item = activityAttention.data?.find((row) => row.id === activityId);
      if (item?.project_id) queryClient.invalidateQueries({ queryKey: ['project-operations', item.project_id] });
    },
  });

  const firstPage = workItems.data?.pages[0];
  const rows = useMemo(() => dedupeWorkItems(workItems.data?.pages.flatMap((page) => page.items) ?? []), [workItems.data]);
  const baseSummary = firstPage?.summary ?? emptySummary;
  const nodeApprovalCount = pendingNodeApprovals.data?.filter((request) => request.target_type === 'pipeline_run_node').length ?? 0;
  const summary = useMemo(() => ({
    ...baseSummary,
    total: baseSummary.total + nodeApprovalCount,
    waiting_decision: baseSummary.waiting_decision + nodeApprovalCount,
  }), [baseSummary, nodeApprovalCount]);
  const facets = firstPage?.facets ?? {};
  const attentionRows = useMemo(() => asArray<ActivityAttentionItem>(activityAttention.data), [activityAttention.data]);
  const selectedItem = rows.find((item) => item.id === selectedWorkId) ?? rows[0] ?? null;

  useEffect(() => {
    if (!rows.length) setSelectedWorkId(null);
    else if (!selectedWorkId || !rows.some((item) => item.id === selectedWorkId)) setSelectedWorkId(rows[0].id);
  }, [rows, selectedWorkId]);

  useEffect(() => streamSSE('/me/activity-attention/stream', {
    onStatus: setStreamStatus,
    onEvent: (message) => {
      if (message.event === 'attention_changed') queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      if (message.event === 'work_queue_changed' || message.event === 'work_item_changed') {
        queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      }
    },
  }), [queryClient]);

  function updateParams(changes: Record<string, string | null>) {
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      Object.entries(changes).forEach(([key, value]) => {
        if (!value || (key === 'mode' && value === 'work') || (key === 'queue' && value === 'all') || (key === 'bucket' && value === 'need_action') || (key === 'sort' && value === 'priority')) next.delete(key);
        else next.set(key, value);
      });
      return next;
    });
  }

  function resetFilters() {
    setSearchDraft('');
    updateParams({ queue: 'all', bucket: 'need_action', project: null, status: null, deadline: null, type: null, priority: null, search: null, sort: 'priority' });
  }

  function selectBucket(nextBucket: SummaryBucket) {
    updateParams({
      mode: 'work',
      queue: nextBucket === 'waiting_decision' ? 'pending_approval' : 'all',
      bucket: bucket === nextBucket ? 'all' : nextBucket,
      status: null,
      deadline: null,
      type: null,
      priority: null,
      search: null,
    });
  }

  function refreshAll() {
    void workItems.refetch();
    void activityAttention.refetch();
    void pendingNodeApprovals.refetch();
  }

  function openWorkItem(item: WorkItem) {
    navigate(executionTargetPath(item, currentReturnTo(location)));
  }

  const hasFilters = Boolean(bucket !== 'need_action' || queue !== 'all' || project || status || deadline || workType || priority || search || sort !== 'priority');
  const total = firstPage?.total ?? summary.total ?? rows.length;

  return (
    <>
      <PageHeader
        title="Bàn làm việc"
        description={`${user?.full_name ?? 'Người dùng'}${user?.roles?.length ? ` · ${user.roles.join(', ')}` : ''} · Mở thẳng đúng sổ nghiệp vụ cần xử lý`}
        actions={(
          <div className="flex items-center gap-2">
            <LiveStatusBadge status={streamStatus} />
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={refreshAll}>
              <RefreshCw className={clsx('h-4 w-4', (workItems.isFetching || activityAttention.isFetching) && 'animate-spin')} />
              Làm mới
            </button>
          </div>
        )}
      />

      <SummaryStrip summary={summary} selected={bucket} onSelect={selectBucket} />

      <div className="mt-4 flex gap-1 border-b border-line">
        <ModeTab active={mode === 'work'} label="Công việc" count={summary.total} onClick={() => updateParams({ mode: 'work' })} />
        <ModeTab active={mode === 'attention'} label="Trao đổi cần phản hồi" count={attentionRows.length} onClick={() => updateParams({ mode: 'attention' })} />
      </div>

      {mode === 'work' ? (
        <section className="mt-4 rounded-lg border border-line bg-white p-4 shadow-panel">
          <FilterBar
            searchDraft={searchDraft}
            queue={queue}
            project={project}
            status={status}
            deadline={deadline}
            workType={workType}
            priority={priority}
            sort={sort}
            facets={facets}
            hasFilters={hasFilters}
            advancedOpen={advancedFiltersOpen}
            onSearchDraft={setSearchDraft}
            onSearch={() => updateParams({ search: searchDraft.trim() || null, bucket: 'all' })}
            onChange={(key, value) => updateParams({ [key]: value || null, bucket: 'all' })}
            onReset={resetFilters}
            onToggleAdvanced={() => setAdvancedFiltersOpen((open) => !open)}
          />

          {(queue === 'pending_approval' || bucket === 'waiting_decision') ? (
            <NodeApprovalLane
              rows={(pendingNodeApprovals.data ?? []).filter((request) => request.target_type === 'pipeline_run_node')}
              loading={pendingNodeApprovals.isLoading}
              error={pendingNodeApprovals.error}
              onRetry={() => void pendingNodeApprovals.refetch()}
            />
          ) : null}

          <div className="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
            <WorkList
              rows={rows}
              selectedId={selectedItem?.id}
              loading={workItems.isLoading}
              fetchingMore={workItems.isFetchingNextPage}
              error={workItems.error}
              total={total}
              hasMore={Boolean(workItems.hasNextPage)}
              filtered={hasFilters}
              onSelect={setSelectedWorkId}
              onOpen={openWorkItem}
              onLoadMore={() => void workItems.fetchNextPage()}
              onReset={resetFilters}
              onRetry={() => void workItems.refetch()}
            />
            <NextWorkPanel item={selectedItem} onOpen={openWorkItem} />
          </div>
        </section>
      ) : (
        <ActivityAttentionPanel
          rows={attentionRows}
          loading={activityAttention.isLoading}
          error={activityAttention.error}
          selectedQueue={attentionQueue}
          streamStatus={streamStatus}
          returnTo={currentReturnTo(location)}
          markingId={markAttentionRead.isPending ? markAttentionRead.variables : undefined}
          onSelectQueue={(value) => updateParams({ attention: value === 'all' ? null : value })}
          onMarkRead={(activityId) => markAttentionRead.mutate(activityId)}
        />
      )}

    </>
  );
}

function NodeApprovalLane({ rows, loading, error, onRetry }: { rows: ApprovalRequest[]; loading: boolean; error: unknown; onRetry: () => void }) {
  return (
    <section className="mt-4 rounded-md border border-amber-200 bg-amber-50/50 p-3" aria-labelledby="node-approval-lane-title">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <h2 id="node-approval-lane-title" className="text-sm font-semibold text-ink">Phê duyệt giai đoạn quy trình</h2>
          <p className="mt-1 text-xs text-slate-600">Quyết định ở cấp node; không tạo đầu việc giả trong danh sách công việc.</p>
        </div>
        <span className="rounded-md bg-white px-2 py-1 text-xs font-semibold text-amber-800">{rows.length} yêu cầu</span>
      </div>
      {loading ? <LoadingSpinner /> : error ? <SourceError title="Không tải được yêu cầu phê duyệt giai đoạn" onRetry={onRetry} /> : rows.length ? <div className="space-y-2">{rows.map((request) => <ApprovalPanel key={request.id} request={request} />)}</div> : <p className="rounded-md border border-dashed border-amber-200 bg-white p-4 text-sm text-slate-600">Không có giai đoạn nào đang chờ anh phê duyệt.</p>}
    </section>
  );
}

function buildWorkPath(filters: {
  queue: QueueKey;
  bucket: SummaryBucket;
  project: string;
  status: string;
  deadline: string;
  workType: string;
  priority: string;
  search: string;
  sort: string;
}, cursor: string) {
  const params = new URLSearchParams({ queue: filters.queue, sort: filters.sort, limit: '30' });
  params.set('bucket', filters.bucket);
  if (filters.project) params.set('project_id', filters.project);
  if (filters.status) params.set('status', filters.status);
  if (filters.deadline) params.set('deadline', filters.deadline);
  if (filters.workType) params.set('work_type', filters.workType);
  if (filters.priority) params.set('priority', filters.priority);
  if (filters.search) params.set('search', filters.search);
  if (cursor) params.set('cursor', cursor);
  return `/me/work-items?${params.toString()}`;
}

function dedupeWorkItems(rows: WorkItem[]) {
  return Array.from(new Map(rows.map((item) => [item.id, item])).values());
}

function validValue<T extends string>(value: string | null, values: readonly T[], fallback: T): T {
  return value && values.includes(value as T) ? value as T : fallback;
}

const emptySummary: MyWorkSummary = { total: 0, need_action: 0, in_progress: 0, overdue: 0, blocked: 0, waiting_decision: 0, waiting_approval: 0, upcoming: 0, done_today: 0 };

function SummaryStrip({ summary, selected, onSelect }: { summary: MyWorkSummary; selected: SummaryBucket; onSelect: (bucket: SummaryBucket) => void }) {
  const cards: Array<{ bucket: SummaryBucket; label: string; value: number; icon: typeof BriefcaseBusiness; activeClass: string }> = [
    { bucket: 'need_action', label: 'Làm ngay', value: summary.need_action, icon: BriefcaseBusiness, activeClass: 'border-blue-300 bg-blue-50 ring-blue-200' },
    { bucket: 'in_progress', label: 'Đang làm', value: summary.in_progress, icon: Clock3, activeClass: 'border-emerald-300 bg-emerald-50 ring-emerald-200' },
    { bucket: 'waiting_decision', label: 'Chờ tôi duyệt', value: summary.waiting_decision, icon: ShieldAlert, activeClass: 'border-amber-300 bg-amber-50 ring-amber-200' },
    { bucket: 'blocked', label: 'Bị chặn', value: summary.blocked, icon: AlertCircle, activeClass: 'border-rose-300 bg-rose-50 ring-rose-200' },
    { bucket: 'upcoming', label: 'Sắp tới · chỉ xem', value: summary.upcoming, icon: Clock3, activeClass: 'border-slate-300 bg-slate-50 ring-slate-200' },
  ];
  return (
    <section className="flex gap-2 overflow-x-auto pb-1" aria-label="Tóm tắt công việc">
      {cards.map((card) => {
        const Icon = card.icon;
        const active = selected === card.bucket;
        return (
          <button
            key={card.bucket}
            className={clsx(
              'flex min-w-[10.5rem] flex-1 items-center justify-between gap-3 rounded-lg border bg-white px-3 py-3 text-left shadow-panel transition',
              active ? `ring-1 ${card.activeClass}` : 'border-line hover:border-blue-200 hover:bg-slate-50',
            )}
            onClick={() => onSelect(card.bucket)}
          >
            <span><span className="block text-xs font-semibold text-slate-500">{card.label}</span><span className="mt-1 block text-xl font-semibold text-ink">{card.value}</span></span>
            <Icon className={clsx('h-4 w-4', active ? 'text-blue-700' : 'text-slate-400')} />
          </button>
        );
      })}
    </section>
  );
}

function ModeTab({ active, label, count, onClick }: { active: boolean; label: string; count: number; onClick: () => void }) {
  return (
    <button className={clsx('border-b-2 px-4 py-3 text-sm font-semibold', active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-slate-800')} onClick={onClick}>
      {label} <span className="ml-1 rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-600">{count}</span>
    </button>
  );
}

function FilterBar({ searchDraft, queue, project, status, deadline, workType, priority, sort, facets, hasFilters, advancedOpen, onSearchDraft, onSearch, onChange, onReset, onToggleAdvanced }: {
  searchDraft: string; queue: QueueKey; project: string; status: string; deadline: string; workType: string; priority: string; sort: string; facets: MyWorkFacets; hasFilters: boolean; advancedOpen: boolean;
  onSearchDraft: (value: string) => void; onSearch: () => void; onChange: (key: string, value: string) => void; onReset: () => void; onToggleAdvanced: () => void;
}) {
  const projects = normalizeFacetOptions(facets.projects, 'Công trình');
  const statuses = normalizeFacetOptions(facets.statuses, 'Trạng thái');
  const workTypes = normalizeFacetOptions(facets.work_types, 'Loại việc');
  const priorities = normalizeFacetOptions(facets.priorities, 'Ưu tiên');
  return (
    <div className="space-y-3 border-b border-line pb-4">
      <div className="flex items-center justify-between gap-3"><div className="flex items-center gap-2"><SlidersHorizontal className="h-4 w-4 text-blue-700" /><h2 className="text-sm font-semibold text-ink">Tìm công việc</h2></div>{hasFilters ? <button className="inline-flex items-center gap-1 text-xs font-semibold text-slate-600 hover:text-blue-700" onClick={onReset}><X className="h-3.5 w-3.5" />Về mặc định</button> : null}</div>
      <div className="grid gap-2 md:grid-cols-[minmax(14rem,1fr)_minmax(12rem,18rem)_auto]">
        <form className="flex min-w-0" onSubmit={(event) => { event.preventDefault(); onSearch(); }}>
          <div className="relative min-w-0 flex-1"><Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" /><input className="w-full rounded-l-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:border-blue-400" value={searchDraft} onChange={(event) => onSearchDraft(event.target.value)} placeholder="Tên việc, công trình..." /></div>
          <button className="rounded-r-md border border-l-0 border-line px-3 text-sm font-semibold text-blue-700 hover:bg-blue-50" type="submit">Tìm</button>
        </form>
        <FilterSelect value={project} label="Tất cả công trình" options={projects} onChange={(value) => onChange('project', value)} />
        <button className={clsx('inline-flex items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold', advancedOpen ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-line text-slate-700 hover:bg-slate-50')} onClick={onToggleAdvanced}><SlidersHorizontal className="h-4 w-4" />Bộ lọc thêm</button>
      </div>
      {advancedOpen ? <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-6">
        <FilterSelect value={queue} label="Tất cả phạm vi" options={queues.map((item) => ({ value: item.key, label: item.label }))} onChange={(value) => onChange('queue', value)} />
        <FilterSelect value={status} label="Tất cả trạng thái" options={statuses.length ? statuses.map((item) => ({ ...item, label: statusLabel(item.value) })) : statusOptions} onChange={(value) => onChange('status', value)} />
        <FilterSelect value={deadline} label="Tất cả hạn việc" options={deadlineOptions} onChange={(value) => onChange('deadline', value)} />
        <FilterSelect value={workType} label="Tất cả loại việc" options={workTypes.map((item) => ({ ...item, label: workTypeLabel(item.value) }))} onChange={(value) => onChange('type', value)} />
        <FilterSelect value={priority} label="Tất cả ưu tiên" options={priorities.map((item) => ({ ...item, label: priorityLabel(item.value) }))} onChange={(value) => onChange('priority', value)} />
        <FilterSelect value={sort} label="Sắp xếp" options={sortOptions} onChange={(value) => onChange('sort', value)} />
      </div> : null}
    </div>
  );
}

function normalizeFacetOptions(values: FacetOption[] | undefined, fallback: string) {
  return asArray<FacetOption>(values).map((item) => {
    if (typeof item === 'string') return { value: item, label: item };
    const value = item.value ?? item.id ?? item.project_id ?? '';
    return { value, label: item.label ?? ([item.code ?? item.project_code, item.name ?? item.project_name].filter(Boolean).join(' · ') || fallback) };
  }).filter((item) => item.value);
}

function FilterSelect({ value, label, options, onChange }: { value: string; label: string; options: Array<{ value: string; label: string }>; onChange: (value: string) => void }) {
  return <select className="min-w-0 rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700" value={value} onChange={(event) => onChange(event.target.value)}><option value="">{label}</option>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>;
}

function WorkList({ rows, selectedId, loading, fetchingMore, error, total, hasMore, filtered, onSelect, onOpen, onLoadMore, onReset, onRetry }: {
  rows: WorkItem[]; selectedId?: string; loading: boolean; fetchingMore: boolean; error: unknown; total: number; hasMore: boolean; filtered: boolean;
  onSelect: (id: string) => void; onOpen: (item: WorkItem) => void; onLoadMore: () => void; onReset: () => void; onRetry: () => void;
}) {
  if (loading) return <div className="rounded-md border border-line p-8"><LoadingSpinner /></div>;
  if (error && !rows.length) return <SourceError title="Không tải được danh sách công việc" onRetry={onRetry} />;
  if (!rows.length) return <EmptyState title={filtered ? 'Không có công việc khớp bộ lọc' : 'Không có việc cần xử lý trong phạm vi này'} action={filtered ? <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700" onClick={onReset}>Xóa bộ lọc</button> : undefined} />;
  return (
    <div className="min-w-0">
      <div className="mb-2 flex items-center justify-between gap-3"><p className="text-sm font-semibold text-ink">{total} việc</p>{error ? <span className="text-xs font-semibold text-rose-600">Chưa cập nhật được dữ liệu mới</span> : null}</div>
      <div className="max-h-[720px] space-y-2 overflow-y-auto pr-1">
        {rows.map((item) => (
          <div key={item.id} role="button" tabIndex={0} className={clsx('rounded-lg outline-none ring-offset-1', selectedId === item.id && 'ring-2 ring-blue-400')} onClick={() => onSelect(item.id)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') onSelect(item.id); }}>
            <WorkItemCard item={item} onOpen={() => onOpen(item)} />
          </div>
        ))}
      </div>
      {hasMore ? <button className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" disabled={fetchingMore} onClick={onLoadMore}>{fetchingMore ? <RefreshCw className="h-4 w-4 animate-spin" /> : null}Tải thêm · Đã hiển thị {rows.length}/{total}</button> : <p className="mt-3 text-center text-xs text-slate-500">Đã hiển thị {rows.length}/{total}</p>}
    </div>
  );
}

function NextWorkPanel({ item, onOpen }: { item: WorkItem | null; onOpen: (item: WorkItem) => void }) {
  if (!item) return <aside className="order-first rounded-lg border border-dashed border-slate-300 bg-slate-50 p-4 xl:order-last"><p className="text-sm font-semibold text-ink">Nơi làm việc</p><p className="mt-2 text-sm text-slate-600">Chọn một công việc để mở đúng sổ nghiệp vụ.</p></aside>;
  const context = workItemWorkflowContext(item);
  const upcoming = workItemIsUpcoming(item);
  return (
    <aside className="order-first rounded-lg border border-line bg-slate-50 p-4 xl:order-last xl:sticky xl:top-4 xl:self-start">
      <div className="flex flex-wrap items-center gap-2"><StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />{upcoming ? <span className="rounded-md bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-700">Sắp tới · chỉ xem</span> : null}{item.priority ? <span className="rounded-md bg-white px-2 py-1 text-xs font-semibold text-slate-600">{priorityLabel(item.priority)}</span> : null}</div>
      <p className="mt-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Nơi làm việc</p>
      <h3 className="mt-1 text-base font-semibold text-ink">{item.title}</h3>
      <p className="mt-2 text-xs font-medium leading-5 text-slate-600">{context.path}</p>
      <p className="mt-2 rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-800">Thực hiện tại: {executionTargetWorkspaceLabel(item)}</p>
      {upcoming ? <p className="mt-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">Bước này chưa đến lượt. Có thể xem trước hồ sơ nhưng chưa thể bắt đầu hoặc cập nhật.</p> : item.blocked_reason ? <p className="mt-3 rounded-md bg-rose-50 p-3 text-sm text-rose-700">{item.blocked_reason}</p> : <p className="mt-3 text-sm leading-6 text-slate-600">{nextActionCopy(item)}</p>}
      <div className="mt-4 space-y-2 text-sm"><ContextRow label="Dự án" value={context.project} /><ContextRow label="Node" value={context.node} /><ContextRow label="Bước" value={context.step} /><ContextRow label="Phụ trách" value={item.assignee_name ? identityReferenceText(item.assignee_name, item.assignee_account_deleted) : departmentLabel(item.department)} /><ContextRow label="Hạn" value={dateText(item.due_at)} /><ContextRow label="Minh chứng" value={numberValue(item.evidence_count)} /></div>
      <button className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => onOpen(item)}>{upcoming ? 'Xem trước công việc' : executionTargetActionLabel(item)} <ChevronRight className="h-4 w-4" /></button>
      {item.project_id ? <Link className="mt-2 inline-flex w-full justify-center rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to={`/projects/${item.project_id}`}>Mở công trình</Link> : null}
    </aside>
  );
}

function nextActionCopy(item: WorkItem) {
  if (workItemIsUpcoming(item)) return 'Chờ quy trình chuyển đến node này.';
  if (item.status === 'waiting_approval' || item.status === 'waiting_review') return (item.capabilities?.can_decide ?? item.can_decide) ? 'Rà soát hồ sơ và đưa ra quyết định.' : 'Theo dõi phản hồi từ người có thẩm quyền.';
  if (item.status === 'waiting_evidence') return 'Bổ sung đủ minh chứng trước khi hoàn tất.';
  if (item.status === 'completed') return 'Công việc đã hoàn tất. Có thể xem lại hồ sơ và lịch sử.';
  return 'Mở không gian xử lý để thực hiện bước tiếp theo.';
}

function ActivityAttentionPanel({ rows, loading, error, selectedQueue, streamStatus, returnTo, markingId, onSelectQueue, onMarkRead }: {
  rows: ActivityAttentionItem[]; loading: boolean; error: unknown; selectedQueue: AttentionQueueKey; streamStatus: StreamStatus; returnTo: string; markingId?: string;
  onSelectQueue: (queue: AttentionQueueKey) => void; onMarkRead: (activityId: string) => void;
}) {
  const counts = useMemo(() => attentionQueues.reduce<Record<AttentionQueueKey, number>>((acc, item) => { acc[item.key] = rows.filter((row) => inAttentionQueue(row, item.key)).length; return acc; }, { all: 0, mentions: 0, questions: 0, blockers: 0, reminders: 0, unread: 0 }), [rows]);
  const visibleRows = rows.filter((item) => inAttentionQueue(item, selectedQueue));
  return (
    <section className="mt-4 rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-col gap-3 border-b border-line pb-4 md:flex-row md:items-center md:justify-between"><div><h2 className="font-semibold text-ink">Trao đổi cần phản hồi</h2><p className="mt-1 text-sm text-slate-500">Nhắc đến, câu hỏi, điểm chặn và nhắc hạn</p></div><LiveStatusBadge status={streamStatus} /></div>
      <div className="my-3 flex gap-2 overflow-x-auto">{attentionQueues.map((item) => { const Icon = item.icon; return <button key={item.key} className={clsx('inline-flex min-w-fit items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold', selectedQueue === item.key ? 'border-blue-200 bg-blue-50 text-blue-800' : 'border-line text-slate-700 hover:bg-slate-50')} onClick={() => onSelectQueue(item.key)}><Icon className="h-4 w-4" />{item.label}<span className="rounded-md bg-white px-2 py-0.5 text-xs text-slate-600">{counts[item.key]}</span></button>; })}</div>
      {loading ? <LoadingSpinner /> : null}
      {error ? <SourceError title="Không tải được trao đổi cần phản hồi" /> : null}
      {!loading && !error && !visibleRows.length ? <EmptyState title="Không có trao đổi cần phản hồi trong nhóm này" /> : null}
      {visibleRows.length ? <div className="grid gap-2 xl:grid-cols-2">{visibleRows.map((item) => <AttentionCard key={item.id} item={item} returnTo={returnTo} marking={markingId === item.id} onMarkRead={onMarkRead} />)}</div> : null}
    </section>
  );
}

function AttentionCard({ item, returnTo, marking, onMarkRead }: { item: ActivityAttentionItem; returnTo: string; marking: boolean; onMarkRead: (id: string) => void }) {
  const target = item.work_item_id ? workItemDetailPath(item.work_item_id, returnTo, item.project_id) : item.project_id ? `/projects/${item.project_id}` : '';
  return (
    <article className={clsx('rounded-md border p-3', item.is_unread ? 'border-blue-200 bg-blue-50' : 'border-line bg-slate-50')}>
      <div className="flex items-start justify-between gap-3"><div className="min-w-0"><div className="flex flex-wrap gap-1">{item.is_unread ? <span className="rounded-md bg-blue-600 px-2 py-0.5 text-xs font-semibold text-white">Chưa đọc</span> : null}{item.mentioned_me ? <span className="rounded-md bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-700">@tôi</span> : null}{asArray<string>(item.queue_types).slice(0, 3).map((queue) => <span key={queue} className="rounded-md bg-white px-2 py-0.5 text-xs font-semibold text-slate-600">{attentionQueueLabel(queue)}</span>)}</div><p className="mt-2 text-sm font-semibold text-ink">{activityLabel(item.activity_type)}</p><p className="mt-1 text-xs text-slate-500">{identityReferenceText(item.actor_name, item.actor_account_deleted, 'Hệ thống')} · {item.target_label ?? item.work_item_title ?? (item.scope_type ? scopeTypeLabel(item.scope_type) : 'Công trình')} · {compactDateTime(item.created_at)}</p></div>{item.is_unread ? <button className="shrink-0 rounded-md border border-blue-200 bg-white px-2 py-1 text-xs font-semibold text-blue-700 disabled:opacity-60" disabled={marking} onClick={() => onMarkRead(item.id)}>Đã đọc</button> : null}</div>
      <ActivityReplyQuote replyTo={item.reply_to} />
      {item.comment ? <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-700">{item.comment}</p> : null}
      <ActivityAttachments attachments={item.attachments} compact />
      {item.reactions?.length ? <div className="mt-2"><ActivityReactionBar reactions={item.reactions} /></div> : null}
      {target ? <Link to={target} className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-blue-700 hover:text-blue-800">{item.work_item_id ? 'Mở đúng công việc' : 'Mở công trình'} <ChevronRight className="h-3.5 w-3.5" /></Link> : null}
    </article>
  );
}

function attentionQueueLabel(queue: string) { return attentionQueues.find((item) => item.key === queue)?.label ?? queue; }
function inAttentionQueue(item: ActivityAttentionItem, queue: AttentionQueueKey) { if (queue === 'all') return true; if (queue === 'unread') return Boolean(item.is_unread); return asArray<string>(item.queue_types).includes(queue); }

function LiveStatusBadge({ status }: { status: StreamStatus }) {
  const label = status === 'connected' ? 'Đang trực tuyến' : status === 'reconnecting' ? 'Đang kết nối lại' : 'Mất kết nối · Làm mới thủ công';
  return <span className={clsx('inline-flex items-center gap-2 rounded-md px-2.5 py-1.5 text-xs font-semibold', status === 'connected' ? 'bg-emerald-50 text-emerald-700' : status === 'reconnecting' ? 'bg-amber-50 text-amber-700' : 'bg-slate-100 text-slate-600')}><span className={clsx('h-2 w-2 rounded-full', status === 'connected' ? 'bg-emerald-500' : status === 'reconnecting' ? 'bg-amber-500' : 'bg-slate-400')} />{label}</span>;
}

function ContextRow({ label, value }: { label: string; value: string | number }) { return <div className="flex items-center justify-between gap-3 border-b border-line pb-2 last:border-0 last:pb-0"><span className="text-slate-500">{label}</span><span className="min-w-0 truncate text-right font-semibold text-ink">{value}</span></div>; }

function SourceError({ title, onRetry }: { title: string; onRetry?: () => void }) { return <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700"><p className="font-semibold">{title}</p>{onRetry ? <button className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 font-semibold" onClick={onRetry}>Thử lại</button> : null}</div>; }
