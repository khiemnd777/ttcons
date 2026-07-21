import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import {
  AlertTriangle,
  CalendarDays,
  CheckCircle2,
  Clock3,
  RefreshCw,
  Search,
  UserRound,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { EmptyState } from '../../components/EmptyState';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { get, streamSSE, type StreamStatus } from '../../lib/api/client';
import type {
  ProjectProgressBoardResponse,
  ProjectProgressItem,
  ProjectProgressStageKey,
} from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';

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
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('reconnecting');
  const [search, setSearch] = useState('');
  const [pmFilter, setPMFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [mobileStage, setMobileStage] = useState<ProjectProgressStageKey>('not_started');

  const progressQuery = useQuery({
    queryKey: ['project-progress-board'],
    queryFn: () => get<ProjectProgressBoardResponse>('/projects/progress-board'),
    refetchInterval: streamStatus === 'connected' ? false : 60_000,
  });

  useEffect(() => {
    let invalidateTimer: number | undefined;
    const stop = streamSSE('/projects/progress-board/stream', {
      onStatus: setStreamStatus,
      onEvent: (message) => {
        if (message.event !== 'project_progress_changed') return;
        if (invalidateTimer !== undefined) window.clearTimeout(invalidateTimer);
        invalidateTimer = window.setTimeout(() => {
          void queryClient.invalidateQueries({ queryKey: ['project-progress-board'] });
        }, 500);
      },
    });
    return () => {
      stop();
      if (invalidateTimer !== undefined) window.clearTimeout(invalidateTimer);
    };
  }, [queryClient]);

  const items = progressQuery.data?.items ?? [];
  const pmOptions = useMemo(
    () => Array.from(new Set(items.map((item) => item.pm_name).filter(Boolean) as string[])).sort((a, b) => a.localeCompare(b, 'vi')),
    [items],
  );
  const statusOptions = useMemo(
    () => Array.from(new Set(items.map((item) => item.project_status).filter(Boolean))).sort(),
    [items],
  );
  const filteredItems = useMemo(() => {
    const query = search.trim().toLocaleLowerCase('vi');
    return items.filter((item) => {
      if (pmFilter && item.pm_name !== pmFilter) return false;
      if (statusFilter && item.project_status !== statusFilter) return false;
      if (!query) return true;
      return [item.code, item.name, item.client_name, item.pm_name, item.current_node_name]
        .filter(Boolean)
        .join(' ')
        .toLocaleLowerCase('vi')
        .includes(query);
    });
  }, [items, pmFilter, search, statusFilter]);
  const groupedItems = useMemo(() => {
    const groups = {} as Record<ProjectProgressStageKey, ProjectProgressItem[]>;
    progressStages.forEach((stage) => {
      groups[stage.key] = [];
    });
    filteredItems.forEach((item) => {
      const stageKey = progressStages.some((stage) => stage.key === item.stage_key) ? item.stage_key : 'other';
      groups[stageKey].push(item);
    });
    return groups;
  }, [filteredItems]);
  const selectedMobileStage = progressStages.find((stage) => stage.key === mobileStage) ?? progressStages[0];

  return (
    <>
      <PageHeader
        title="Tiến trình công trình"
        description="Theo dõi giai đoạn hiện tại và điểm cần xử lý của tất cả công trình."
        actions={(
          <div className="flex items-center gap-2">
            <LiveStatus status={streamStatus} generatedAt={progressQuery.data?.generated_at} />
            <button
              type="button"
              aria-label="Làm mới tiến trình"
              className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-line bg-white text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={progressQuery.isFetching}
              onClick={() => void progressQuery.refetch()}
            >
              <RefreshCw className={clsx('h-4 w-4', progressQuery.isFetching && 'animate-spin')} />
            </button>
          </div>
        )}
      />

      <section className="mb-4 rounded-lg border border-line bg-white p-3 shadow-panel">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(320px,1fr)_220px_220px]">
          <label className="relative block">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Tìm công trình, mã, chủ đầu tư"
              className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm focus-ring"
            />
          </label>
          <select
            value={pmFilter}
            onChange={(event) => setPMFilter(event.target.value)}
            className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
          >
            <option value="">Tất cả phụ trách</option>
            {pmOptions.map((pmName) => <option key={pmName} value={pmName}>{pmName}</option>)}
          </select>
          <select
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value)}
            className="rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
          >
            <option value="">Tất cả tình trạng</option>
            {statusOptions.map((status) => <option key={status} value={status}>{statusLabel(status)}</option>)}
          </select>
        </div>
      </section>

      {progressQuery.isError && progressQuery.data ? (
        <div className="mb-4 flex items-center justify-between gap-3 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          <span>Chưa lấy được bản cập nhật mới nhất. Board đang hiển thị dữ liệu gần nhất.</span>
          <button type="button" className="font-semibold underline" onClick={() => void progressQuery.refetch()}>Thử lại</button>
        </div>
      ) : null}

      {progressQuery.isLoading ? <ProgressBoardSkeleton /> : null}

      {progressQuery.isError && !progressQuery.data ? (
        <EmptyState
          title="Không tải được tiến trình công trình"
          action={<button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => void progressQuery.refetch()}>Thử lại</button>}
        />
      ) : null}

      {progressQuery.data && filteredItems.length === 0 ? (
        <EmptyState title={items.length === 0 ? 'Chưa có công trình để theo dõi' : 'Không có công trình phù hợp bộ lọc'} />
      ) : null}

      {progressQuery.data && filteredItems.length > 0 ? (
        <>
          <div className="mb-3 lg:hidden">
            <label className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Giai đoạn</label>
            <select
              value={mobileStage}
              onChange={(event) => setMobileStage(event.target.value as ProjectProgressStageKey)}
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring"
            >
              {progressStages.map((stage) => (
                <option key={stage.key} value={stage.key}>{stage.label} · {groupedItems[stage.key].length}</option>
              ))}
            </select>
          </div>
          <div className="lg:hidden">
            <ProgressColumn stage={selectedMobileStage} items={groupedItems[selectedMobileStage.key]} />
          </div>
          <div className="hidden overflow-x-auto pb-3 lg:block">
            <div className="grid auto-cols-[286px] grid-flow-col items-start gap-3">
              {progressStages.map((stage) => (
                <ProgressColumn key={stage.key} stage={stage} items={groupedItems[stage.key]} scrollable />
              ))}
            </div>
          </div>
        </>
      ) : null}
    </>
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

function ProgressColumn({ stage, items, scrollable = false }: { stage: ProgressStage; items: ProjectProgressItem[]; scrollable?: boolean }) {
  return (
    <section className="overflow-hidden rounded-lg border border-line bg-slate-50/70">
      <header className={clsx('flex items-center justify-between gap-2 border-b border-line px-3 py-3', stage.headerClassName)}>
        <h2 className="min-w-0 text-sm font-semibold">{stage.label}</h2>
        <span className="inline-flex min-w-6 items-center justify-center rounded-md bg-white/90 px-1.5 py-0.5 text-xs font-semibold text-slate-700 shadow-sm ring-1 ring-inset ring-slate-200">
          {items.length}
        </span>
      </header>
      <div className={clsx('space-y-2 p-2', scrollable && 'max-h-[calc(100vh-330px)] overflow-y-auto')}>
        {items.length ? items.map((item) => <ProgressCard key={item.id} item={item} />) : (
          <div className="rounded-md border border-dashed border-slate-300 bg-white/70 px-3 py-6 text-center text-xs text-slate-500">
            Không có công trình
          </div>
        )}
      </div>
    </section>
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
        <span className="truncate">{item.pm_name || 'Chưa phân công'}</span>
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

function nonNegativeNumber(value: unknown) {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? Math.max(0, number) : 0;
}
