import { useEffect, useMemo, useRef, useState, type ReactNode, type RefObject } from 'react';
import {
  useInfiniteQuery,
  useIsFetching,
  useQuery,
  useQueryClient,
  type InfiniteData,
} from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import clsx from 'clsx';
import {
  AlertTriangle,
  CalendarDays,
  CheckCircle2,
  Clock3,
  Loader2,
  RefreshCw,
  Search,
  UserRound,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { EmptyState } from '../../components/EmptyState';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { IdentityReferenceLabel, identityReferenceText } from '../../components/IdentityReferenceLabel';
import { get, streamSSE, type StreamStatus } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  ProjectProgressItem,
  ProjectProgressStageKey,
  ProjectProgressStagePage,
  ProjectProgressSummaryResponse,
} from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import {
  mergeProjectProgressPages,
  projectProgressFilterQuery,
  projectProgressStageQuery,
} from './projectProgressBoard';

type ProgressStage = {
  key: ProjectProgressStageKey;
  label: string;
  headerClassName: string;
};

const progressStages: ProgressStage[] = [
  { key: 'not_started', label: 'Chưa khởi động', headerClassName: 'bg-slate-50 text-slate-700' },
  { key: 'survey', label: 'Khảo sát', headerClassName: 'bg-blue-50 text-blue-800' },
  { key: 'design', label: 'Thiết kế', headerClassName: 'bg-indigo-50 text-indigo-800' },
  { key: 'commercial', label: 'Dự toán & Hợp đồng', headerClassName: 'bg-violet-50 text-violet-800' },
  { key: 'planning', label: 'Kế hoạch', headerClassName: 'bg-cyan-50 text-cyan-800' },
  { key: 'construction', label: 'Thi công', headerClassName: 'bg-emerald-50 text-emerald-800' },
  { key: 'acceptance', label: 'Nghiệm thu', headerClassName: 'bg-amber-50 text-amber-800' },
  { key: 'handover_finance', label: 'Bàn giao & Tài chính', headerClassName: 'bg-orange-50 text-orange-800' },
  { key: 'completed', label: 'Hoàn tất', headerClassName: 'bg-teal-50 text-teal-800' },
  { key: 'other', label: 'Khác', headerClassName: 'bg-slate-100 text-slate-700' },
];

export function ProjectProgressPage() {
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('reconnecting');
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [pmFilter, setPMFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [mobileStage, setMobileStage] = useState<ProjectProgressStageKey>('not_started');
  const [hasPendingUpdates, setHasPendingUpdates] = useState(false);
  const desktopBoardRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const timeoutID = window.setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => window.clearTimeout(timeoutID);
  }, [search]);

  const filterQuery = useMemo(() => projectProgressFilterQuery({
    search: debouncedSearch,
    pmStaffId: pmFilter,
    status: statusFilter,
  }), [debouncedSearch, pmFilter, statusFilter]);
  const actorQueryKey = user?.id ?? 'anonymous';
  const summaryQueryKey = useMemo(
    () => ['project-progress-board-summary', actorQueryKey, filterQuery] as const,
    [actorQueryKey, filterQuery],
  );
  const stageQueryRoot = useMemo(
    () => ['project-progress-board-stage', actorQueryKey, filterQuery] as const,
    [actorQueryKey, filterQuery],
  );
  const activeQueryContext = useRef<{ summaryKey: readonly unknown[]; stageRoot: readonly unknown[] }>({
    summaryKey: summaryQueryKey,
    stageRoot: stageQueryRoot,
  });
  activeQueryContext.current = { summaryKey: summaryQueryKey, stageRoot: stageQueryRoot };

  const summaryQuery = useQuery({
    queryKey: summaryQueryKey,
    queryFn: () => get<ProjectProgressSummaryResponse>(withQuery('/projects/progress-board/summary', filterQuery)),
    enabled: Boolean(user?.id),
    refetchInterval: streamStatus === 'connected' ? false : 60_000,
  });
  const stageFetchCount = useIsFetching({ queryKey: stageQueryRoot });

  useEffect(() => {
    setHasPendingUpdates(false);
  }, [actorQueryKey, filterQuery]);

  useEffect(() => {
    if (!user?.id) return undefined;
    let invalidateTimer: number | undefined;
    const stop = streamSSE('/projects/progress-board/stream', {
      onStatus: setStreamStatus,
      onEvent: (message) => {
        if (message.event !== 'project_progress_changed') return;
        if (invalidateTimer !== undefined) window.clearTimeout(invalidateTimer);
        invalidateTimer = window.setTimeout(() => {
          const context = activeQueryContext.current;
          const hasDeepPages = queryClient
            .getQueriesData<InfiniteData<ProjectProgressStagePage>>({ queryKey: context.stageRoot })
            .some(([, data]) => (data?.pages.length ?? 0) > 1);
          if (hasDeepPages) {
            setHasPendingUpdates(true);
            return;
          }
          void queryClient.invalidateQueries({ queryKey: context.summaryKey, exact: true });
          void queryClient.invalidateQueries({ queryKey: context.stageRoot, refetchType: 'active' });
        }, 500);
      },
    });
    return () => {
      stop();
      if (invalidateTimer !== undefined) window.clearTimeout(invalidateTimer);
    };
  }, [queryClient, user?.id]);

  async function refreshBoard() {
    setHasPendingUpdates(false);
    await Promise.all([
      queryClient.resetQueries({ queryKey: stageQueryRoot }),
      queryClient.invalidateQueries({ queryKey: summaryQueryKey, exact: true }),
    ]);
  }

  const stageCounts = summaryQuery.data?.stage_counts;
  const filteredTotal = Math.max(0, Number(summaryQuery.data?.filtered_total ?? 0));
  const selectedMobileStage = progressStages.find((stage) => stage.key === mobileStage) ?? progressStages[0];
  const hasFilters = Boolean(debouncedSearch || pmFilter || statusFilter);
  const isRefreshing = summaryQuery.isFetching || stageFetchCount > 0;

  return (
    <>
      <PageHeader
        title="Tiến trình công trình"
        description="Theo dõi giai đoạn hiện tại và điểm cần xử lý của tất cả công trình."
        actions={(
          <div className="flex items-center gap-2">
            <LiveStatus status={streamStatus} generatedAt={summaryQuery.data?.generated_at} />
            <button
              type="button"
              aria-label="Làm mới tiến trình"
              className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-line bg-white text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={isRefreshing}
              onClick={() => void refreshBoard()}
            >
              <RefreshCw className={clsx('h-4 w-4', isRefreshing && 'animate-spin')} />
            </button>
          </div>
        )}
      />

      <section className="mb-4 rounded-lg border border-line bg-white p-3 shadow-panel">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(320px,1fr)_220px_220px]">
          <label className="relative block">
            <span className="sr-only">Tìm công trình</span>
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              value={search}
              maxLength={120}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Tìm công trình, mã, chủ đầu tư"
              className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm focus-ring"
            />
          </label>
          <label className="block">
            <span className="sr-only">Lọc theo phụ trách</span>
            <select
              value={pmFilter}
              onChange={(event) => setPMFilter(event.target.value)}
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
            >
              <option value="">Tất cả phụ trách</option>
              {(summaryQuery.data?.pm_options ?? []).map((option) => (
                <option key={option.id} value={option.id}>{identityReferenceText(option.name, option.account_deleted)}</option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="sr-only">Lọc theo tình trạng công trình</span>
            <select
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
            >
              <option value="">Tất cả tình trạng</option>
              {(summaryQuery.data?.status_options ?? []).map((status) => (
                <option key={status} value={status}>{statusLabel(status)}</option>
              ))}
            </select>
          </label>
        </div>
      </section>

      {hasPendingUpdates ? (
        <div className="mb-4 flex flex-col items-start justify-between gap-2 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 sm:flex-row sm:items-center">
          <span>Có dữ liệu mới. Cập nhật board để giữ đúng thứ tự và vị trí phân trang.</span>
          <button type="button" className="shrink-0 font-semibold underline" onClick={() => void refreshBoard()}>
            Cập nhật
          </button>
        </div>
      ) : null}

      {summaryQuery.isError && summaryQuery.data ? (
        <div className="mb-4 flex items-center justify-between gap-3 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          <span>Chưa lấy được tổng hợp mới nhất. Board đang hiển thị dữ liệu gần nhất.</span>
          <button type="button" className="font-semibold underline" onClick={() => void summaryQuery.refetch()}>Thử lại</button>
        </div>
      ) : null}

      {summaryQuery.isPending ? <ProgressBoardSkeleton /> : null}

      {summaryQuery.isError && !summaryQuery.data ? (
        <EmptyState
          title="Không tải được tổng hợp tiến trình công trình"
          action={<button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => void summaryQuery.refetch()}>Thử lại</button>}
        />
      ) : null}

      {summaryQuery.data && filteredTotal === 0 ? (
        <EmptyState title={hasFilters ? 'Không có công trình phù hợp bộ lọc' : 'Chưa có công trình để theo dõi'} />
      ) : null}

      {summaryQuery.data && filteredTotal > 0 ? (
        <>
          <div className="mb-3 lg:hidden">
            <label className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Giai đoạn</label>
            <select
              value={mobileStage}
              onChange={(event) => setMobileStage(event.target.value as ProjectProgressStageKey)}
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
            >
              {progressStages.map((stage) => (
                <option key={stage.key} value={stage.key}>{stage.label} · {stageCounts?.[stage.key] ?? 0}</option>
              ))}
            </select>
          </div>
          <div className="lg:hidden">
            <ProgressColumn
              key={selectedMobileStage.key}
              stage={selectedMobileStage}
              total={stageCounts?.[selectedMobileStage.key] ?? 0}
              actorID={actorQueryKey}
              filterQuery={filterQuery}
              queryKeyRoot={stageQueryRoot}
              streamStatus={streamStatus}
              forceActive
            />
          </div>
          <div ref={desktopBoardRef} className="hidden overflow-x-auto pb-3 lg:block">
            <div className="grid auto-cols-[286px] grid-flow-col items-start gap-3">
              {progressStages.map((stage) => (
                <ProgressColumn
                  key={stage.key}
                  stage={stage}
                  total={stageCounts?.[stage.key] ?? 0}
                  actorID={actorQueryKey}
                  filterQuery={filterQuery}
                  queryKeyRoot={stageQueryRoot}
                  streamStatus={streamStatus}
                  horizontalRootRef={desktopBoardRef}
                />
              ))}
            </div>
          </div>
        </>
      ) : null}
    </>
  );
}

function ProgressColumn({
  stage,
  total,
  actorID,
  filterQuery,
  queryKeyRoot,
  streamStatus,
  horizontalRootRef,
  forceActive = false,
}: {
  stage: ProgressStage;
  total: number;
  actorID: string;
  filterQuery: string;
  queryKeyRoot: readonly unknown[];
  streamStatus: StreamStatus;
  horizontalRootRef?: RefObject<HTMLDivElement>;
  forceActive?: boolean;
}) {
  const columnRef = useRef<HTMLElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const active = useNearHorizontalViewport(columnRef, horizontalRootRef, forceActive);
  const query = useInfiniteQuery({
    queryKey: [...queryKeyRoot, stage.key],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<ProjectProgressStagePage>(
      `/projects/progress-board/stages/${stage.key}?${projectProgressStageQuery(filterQuery, String(pageParam ?? ''))}`,
    ),
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: active && total > 0 && actorID !== 'anonymous',
    refetchInterval: (state) => {
      if (streamStatus === 'connected') return false;
      const data = state.state.data as InfiniteData<ProjectProgressStagePage> | undefined;
      return (data?.pages.length ?? 0) <= 1 ? 60_000 : false;
    },
  });
  const items = useMemo(() => mergeProjectProgressPages(query.data?.pages), [query.data?.pages]);
  const authoritativeTotal = Math.max(0, Number(query.data?.pages[0]?.total ?? total));
  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => listRef.current,
    estimateSize: () => 238,
    getItemKey: (index) => items[index]?.id ?? index,
    overscan: 3,
  });

  return (
    <section ref={columnRef} className="overflow-hidden rounded-lg border border-line bg-slate-50/70">
      <header className={clsx('flex items-center justify-between gap-2 border-b border-line px-3 py-3', stage.headerClassName)}>
        <h2 className="min-w-0 text-sm font-semibold">{stage.label}</h2>
        <span className="inline-flex items-center gap-1.5">
          {query.isFetching && !query.isFetchingNextPage ? <RefreshCw className="h-3 w-3 animate-spin" aria-label="Đang cập nhật" /> : null}
          <span className="inline-flex min-w-6 items-center justify-center rounded-md bg-white/90 px-1.5 py-0.5 text-xs font-semibold text-slate-700 shadow-sm ring-1 ring-inset ring-slate-200">
            {authoritativeTotal}
          </span>
        </span>
      </header>

      {total === 0 ? <ProgressColumnEmpty /> : null}
      {total > 0 && !active ? <ProgressColumnDeferred /> : null}
      {total > 0 && active && query.isPending ? <ProgressColumnSkeleton /> : null}
      {total > 0 && active && query.isError && !query.data ? (
        <div className="p-3 text-center text-xs text-rose-700">
          <p>Không tải được cột này.</p>
          <button type="button" className="mt-2 font-semibold underline" onClick={() => void query.refetch()}>Thử lại</button>
        </div>
      ) : null}

      {query.data ? (
        <>
          {query.isError ? (
            <div className="mx-2 mt-2 rounded-md border border-rose-200 bg-rose-50 px-2 py-1.5 text-xs text-rose-700">
              Dữ liệu cũ đang được giữ lại. <button type="button" className="font-semibold underline" onClick={() => void query.refetch()}>Thử lại</button>
            </div>
          ) : null}
          {items.length > 0 ? (
            <div
              ref={listRef}
              aria-busy={query.isFetching}
              className="max-h-[calc(100vh-350px)] min-h-[160px] overflow-y-auto p-2"
            >
              <div className="relative w-full" style={{ height: `${virtualizer.getTotalSize()}px` }}>
                {virtualizer.getVirtualItems().map((virtualItem) => {
                  const item = items[virtualItem.index];
                  return (
                    <div
                      key={item.id}
                      ref={virtualizer.measureElement}
                      data-index={virtualItem.index}
                      className="absolute left-0 top-0 w-full pb-2"
                      style={{ transform: `translateY(${virtualItem.start}px)` }}
                    >
                      <ProgressCard item={item} />
                    </div>
                  );
                })}
              </div>
            </div>
          ) : <ProgressColumnEmpty />}

          <div className="border-t border-line bg-white/70 p-2 text-center">
            {query.hasNextPage ? (
              <button
                type="button"
                disabled={query.isFetchingNextPage}
                className="inline-flex items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => void query.fetchNextPage()}
              >
                {query.isFetchingNextPage ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
                Tải thêm 25
              </button>
            ) : (
              <span className="text-[11px] text-slate-500">Đã tải toàn bộ {authoritativeTotal} công trình</span>
            )}
          </div>
        </>
      ) : null}
    </section>
  );
}

function useNearHorizontalViewport(
  columnRef: RefObject<HTMLElement>,
  rootRef: RefObject<HTMLDivElement> | undefined,
  forceActive: boolean,
) {
  const [active, setActive] = useState(forceActive);
  useEffect(() => {
    if (forceActive) {
      setActive(true);
      return undefined;
    }
    const column = columnRef.current;
    const root = rootRef?.current;
    if (!column || !root || typeof IntersectionObserver === 'undefined') {
      setActive(true);
      return undefined;
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        setActive(true);
        observer.disconnect();
      }
    }, {
      root,
      rootMargin: '0px 600px 0px 600px',
      threshold: 0.01,
    });
    observer.observe(column);
    return () => observer.disconnect();
  }, [columnRef, forceActive, rootRef]);
  return active;
}

function ProgressColumnDeferred() {
  return (
    <div className="m-2 flex min-h-40 items-center justify-center rounded-md border border-dashed border-slate-300 bg-white/70 px-3 text-center text-xs text-slate-500">
      Dữ liệu sẽ tải khi cột xuất hiện gần vùng xem.
    </div>
  );
}

function ProgressColumnEmpty() {
  return (
    <div className="m-2 rounded-md border border-dashed border-slate-300 bg-white/70 px-3 py-6 text-center text-xs text-slate-500">
      Không có công trình
    </div>
  );
}

function ProgressColumnSkeleton() {
  return (
    <div className="space-y-2 p-2">
      {Array.from({ length: 3 }, (_, index) => (
        <div key={index} className="animate-pulse rounded-md border border-line bg-white p-3">
          <div className="h-3 w-20 rounded bg-slate-200" />
          <div className="mt-2 h-4 w-4/5 rounded bg-slate-200" />
          <div className="mt-4 h-2 rounded bg-slate-100" />
        </div>
      ))}
    </div>
  );
}

function LiveStatus({ status, generatedAt }: { status: StreamStatus; generatedAt?: string }) {
  const label = status === 'connected'
    ? 'Cập nhật trực tiếp'
    : status === 'reconnecting'
      ? 'Đang kết nối lại'
      : 'Mất kết nối · cập nhật định kỳ';
  return (
    <span
      title={generatedAt ? `Dữ liệu lúc ${new Date(generatedAt).toLocaleString('vi-VN')}` : undefined}
      className={clsx(
        'inline-flex items-center gap-2 rounded-md px-2.5 py-2 text-xs font-semibold',
        status === 'connected' && 'bg-emerald-50 text-emerald-700',
        status === 'reconnecting' && 'bg-amber-50 text-amber-700',
        status === 'offline' && 'bg-slate-100 text-slate-600',
      )}
    >
      <span className={clsx('h-2 w-2 rounded-full', status === 'connected' ? 'bg-emerald-500' : status === 'reconnecting' ? 'bg-amber-500' : 'bg-slate-400')} />
      <span className="hidden sm:inline">{label}</span>
    </span>
  );
}

function ProgressCard({ item }: { item: ProjectProgressItem }) {
  const progress = Math.min(100, Math.max(0, Number(item.progress_percent ?? 0)));
  const openIssues = nonNegativeNumber(item.open_issues);
  const pendingApprovals = nonNegativeNumber(item.pending_approvals);
  const blockedItems = nonNegativeNumber(item.blocked_items);
  const overdueItems = nonNegativeNumber(item.overdue_items);
  const blocked = item.project_status === 'blocked' || item.current_node_status === 'blocked' || blockedItems > 0;

  return (
    <Link
      to={`/projects/${item.id}`}
      className="group block rounded-md border border-line bg-white p-3 shadow-sm transition hover:border-blue-300 hover:shadow-panel focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
      aria-label={`Mở công trình ${item.code} · ${item.name}`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-[11px] font-semibold uppercase tracking-wide text-slate-500">{item.code}</p>
          <h3 className="mt-1 line-clamp-2 text-sm font-semibold leading-5 text-ink group-hover:text-blue-700">{item.name}</h3>
        </div>
        <StatusBadge status={item.project_status} className="shrink-0 px-1.5 py-0.5 text-[10px]" />
      </div>

      {item.client_name ? <p className="mt-1.5 truncate text-xs text-slate-500">{item.client_name}</p> : null}
      <div className="mt-3 flex items-center gap-2 text-xs text-slate-600">
        <UserRound className="h-3.5 w-3.5 shrink-0" />
        <IdentityReferenceLabel name={item.pm_name} deleted={item.pm_account_deleted} liveOwnership fallback="Chưa phân công" className="truncate" />
      </div>
      <div className="mt-2 rounded-md bg-slate-50 px-2 py-1.5 text-xs text-slate-600">
        <span className="font-medium text-slate-700">{item.current_node_name || 'Chưa khởi động quy trình'}</span>
      </div>

      <div className="mt-3 flex items-center gap-2">
        <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100">
          <div className={clsx('h-full rounded-full', progress >= 100 ? 'bg-teal-500' : blocked ? 'bg-rose-500' : 'bg-blue-600')} style={{ width: `${progress}%` }} />
        </div>
        <span className="text-xs font-semibold text-slate-600">{Math.round(progress)}%</span>
      </div>

      <div className="mt-3 flex items-center gap-1.5 text-xs text-slate-500">
        <CalendarDays className="h-3.5 w-3.5" />
        <span>{item.planned_end_date ? dateText(item.planned_end_date) : 'Chưa có hạn dự kiến'}</span>
      </div>

      {(blocked || openIssues > 0 || pendingApprovals > 0 || overdueItems > 0) ? (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {blocked ? <AttentionBadge tone="rose" icon={<AlertTriangle className="h-3 w-3" />} label={blockedItems > 0 ? `${blockedItems} bị chặn` : 'Bị chặn'} /> : null}
          {openIssues > 0 ? <AttentionBadge tone="rose" icon={<AlertTriangle className="h-3 w-3" />} label={`${openIssues} vướng mắc`} /> : null}
          {pendingApprovals > 0 ? <AttentionBadge tone="amber" icon={<Clock3 className="h-3 w-3" />} label={`${pendingApprovals} chờ duyệt`} /> : null}
          {overdueItems > 0 ? <AttentionBadge tone="amber" icon={<Clock3 className="h-3 w-3" />} label={`${overdueItems} quá hạn`} /> : null}
        </div>
      ) : progress >= 100 ? (
        <div className="mt-3"><AttentionBadge tone="emerald" icon={<CheckCircle2 className="h-3 w-3" />} label="Đã hoàn tất" /></div>
      ) : null}
    </Link>
  );
}

function AttentionBadge({ tone, icon, label }: { tone: 'rose' | 'amber' | 'emerald'; icon: ReactNode; label: string }) {
  return (
    <span className={clsx(
      'inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] font-semibold ring-1 ring-inset',
      tone === 'rose' && 'bg-rose-50 text-rose-700 ring-rose-200',
      tone === 'amber' && 'bg-amber-50 text-amber-700 ring-amber-200',
      tone === 'emerald' && 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    )}>
      {icon}
      {label}
    </span>
  );
}

function ProgressBoardSkeleton() {
  return (
    <div className="grid gap-3 lg:grid-cols-4">
      {Array.from({ length: 4 }, (_, columnIndex) => (
        <div key={columnIndex} className="animate-pulse rounded-lg border border-line bg-slate-50 p-2">
          <div className="mb-3 h-10 rounded-md bg-slate-200" />
          {Array.from({ length: 3 }, (_, cardIndex) => (
            <div key={cardIndex} className="mb-2 rounded-md border border-line bg-white p-3">
              <div className="h-3 w-20 rounded bg-slate-200" />
              <div className="mt-2 h-4 w-4/5 rounded bg-slate-200" />
              <div className="mt-4 h-2 rounded bg-slate-100" />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

function withQuery(path: string, query: string) {
  return query ? `${path}?${query}` : path;
}

function nonNegativeNumber(value: unknown) {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? Math.max(0, number) : 0;
}
