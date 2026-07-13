import { useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, AtSign, Bell, BriefcaseBusiness, CheckCircle2, Clock3, MessageSquare, RefreshCw, ShieldAlert, SlidersHorizontal } from 'lucide-react';
import clsx from 'clsx';
import { ApiError, get, post, streamSSE, type StreamStatus } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import type { ActivityAttentionItem, WorkItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { ActivityAttachments } from '../work-items/ActivityAttachments';
import { ActivityReactionBar, ActivityReplyQuote } from '../work-items/ActivityConversationMeta';
import { WorkItemCard } from '../work-items/WorkItemCard';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import {
  asArray,
  activityLabel,
  compactDateTime,
  departmentLabel,
  isWorkOverdue,
  numberValue,
  priorityLabel,
  prioritizeWorkItems,
  scopeTypeLabel,
  workTypeLabel,
} from '../work-items/workItemPresentation';

type QueueKey = 'mine' | 'department' | 'pending_approval' | 'watching' | 'all';
type AttentionQueueKey = 'all' | 'mentions' | 'questions' | 'blockers' | 'reminders' | 'unread';

const queues: Array<{ key: QueueKey; label: string; description: string }> = [
  { key: 'mine', label: 'Mine', description: 'Công việc gắn trực tiếp với tôi' },
  { key: 'department', label: 'Department', description: 'Queue theo phòng ban / role scope' },
  { key: 'pending_approval', label: 'Pending approval', description: 'Việc đang chờ review hoặc duyệt' },
  { key: 'watching', label: 'Watching', description: 'Việc đang theo dõi' },
  { key: 'all', label: 'All visible', description: 'Tất cả work item trong scope của tôi' },
];

const statusOptions = [
  { value: 'all', label: 'Tất cả trạng thái' },
  { value: 'ready', label: 'Sẵn sàng' },
  { value: 'in_progress', label: 'Đang xử lý' },
  { value: 'blocked', label: 'Bị chặn' },
  { value: 'waiting_review', label: 'Chờ review' },
  { value: 'waiting_approval', label: 'Chờ duyệt' },
  { value: 'waiting_evidence', label: 'Chờ evidence' },
  { value: 'completed', label: 'Hoàn tất' },
];

const attentionQueues: Array<{ key: AttentionQueueKey; label: string; icon: typeof MessageSquare }> = [
  { key: 'all', label: 'All', icon: MessageSquare },
  { key: 'mentions', label: 'Mentions', icon: AtSign },
  { key: 'questions', label: 'Questions', icon: MessageSquare },
  { key: 'blockers', label: 'Blockers', icon: AlertCircle },
  { key: 'reminders', label: 'Reminders', icon: Bell },
  { key: 'unread', label: 'Unread', icon: CheckCircle2 },
];

export function MyWorkPage() {
  const { user } = useAuth();
  const [selectedQueue, setSelectedQueue] = useState<QueueKey>('mine');
  const [selectedProject, setSelectedProject] = useState('all');
  const [selectedStatus, setSelectedStatus] = useState('all');
  const [selectedDeadline, setSelectedDeadline] = useState('all');
  const [selectedWorkType, setSelectedWorkType] = useState('all');
  const [selectedPriority, setSelectedPriority] = useState('all');
  const [selectedAttentionQueue, setSelectedAttentionQueue] = useState<AttentionQueueKey>('all');
  const [attentionStreamStatus, setAttentionStreamStatus] = useState<StreamStatus>('reconnecting');
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const workItems = useQuery({
    queryKey: ['my-work-items'],
    queryFn: () => get<WorkItem[]>('/me/work-items'),
    retry: false,
  });
  const attentionPath = useMemo(() => {
    const params = new URLSearchParams();
    params.set('limit', '120');
    if (selectedProject !== 'all') params.set('project_id', selectedProject);
    return `/me/activity-attention?${params.toString()}`;
  }, [selectedProject]);
  const activityAttention = useQuery({
    queryKey: ['activity-attention', selectedProject],
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

  const rows = useMemo(() => prioritizeWorkItems(asArray<WorkItem>(workItems.data)), [workItems.data]);
  const projectOptions = useMemo(() => {
    const index = new Map<string, string>();
    rows.forEach((item) => {
      if (item.project_id) index.set(item.project_id, item.project_code ? `${item.project_code} · ${item.project_name ?? 'Project'}` : item.project_name ?? item.project_id);
    });
    return Array.from(index.entries()).map(([value, label]) => ({ value, label }));
  }, [rows]);
  const workTypeOptions = useMemo(() => uniqueOptionValues(rows.map((item) => item.work_type)), [rows]);
  const priorityOptions = useMemo(() => uniqueOptionValues(rows.map((item) => item.priority)), [rows]);

  const queueCounts = useMemo(() => {
    return queues.reduce<Record<QueueKey, number>>((acc, queue) => {
      acc[queue.key] = rows.filter((item) => inQueue(item, queue.key)).length;
      return acc;
    }, { mine: 0, department: 0, pending_approval: 0, watching: 0, all: 0 });
  }, [rows]);

  const filteredRows = useMemo(() => {
    return rows.filter((item) => {
      if (!inQueue(item, selectedQueue)) return false;
      if (selectedProject !== 'all' && item.project_id !== selectedProject) return false;
      if (selectedStatus !== 'all' && item.status !== selectedStatus) return false;
      if (selectedDeadline !== 'all' && !inDeadlineBucket(item, selectedDeadline)) return false;
      if (selectedWorkType !== 'all' && item.work_type !== selectedWorkType) return false;
      if (selectedPriority !== 'all' && item.priority !== selectedPriority) return false;
      return true;
    });
  }, [rows, selectedDeadline, selectedPriority, selectedProject, selectedQueue, selectedStatus, selectedWorkType]);

  const selectedContext = filteredRows[0] ?? null;
  const summary = useMemo(() => buildSummary(rows), [rows]);
  const attentionRows = useMemo(() => asArray<ActivityAttentionItem>(activityAttention.data), [activityAttention.data]);

  useEffect(() => {
    return streamSSE('/me/activity-attention/stream', {
      onStatus: setAttentionStreamStatus,
      onEvent: (message) => {
        if (message.event === 'attention_changed') {
          queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
        }
      },
    });
  }, [queryClient]);

  if (workItems.isLoading) return <MyWorkSkeleton />;
  if (workItems.error) {
    return (
      <MyWorkError
        error={workItems.error}
        onRetry={() => {
          void workItems.refetch();
        }}
      />
    );
  }

  return (
    <>
      <PageHeader
        title="Việc của tôi"
        description={`${user?.full_name ?? 'Current user'} · ${user?.roles?.join(', ') || 'Operational inbox'} · Cross-project work queue`}
        actions={
          <button
            className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            onClick={() => {
              void workItems.refetch();
            }}
          >
            <RefreshCw className="h-4 w-4" />
            Refresh
          </button>
        }
      />

      <SummaryStrip summary={summary} />

      <ActivityAttentionPanel
        rows={attentionRows}
        loading={activityAttention.isLoading}
        error={activityAttention.error}
        selectedQueue={selectedAttentionQueue}
        streamStatus={attentionStreamStatus}
        markingId={markAttentionRead.isPending ? markAttentionRead.variables : undefined}
        onSelectQueue={setSelectedAttentionQueue}
        onRefresh={() => {
          void activityAttention.refetch();
        }}
        onMarkRead={(activityId) => markAttentionRead.mutate(activityId)}
      />

      <section className="mt-5 rounded-lg border border-line bg-white p-4 shadow-panel">
        <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex items-center gap-2">
            <SlidersHorizontal className="h-5 w-5 text-blue-700" />
            <h2 className="font-semibold text-ink">Queue filters</h2>
          </div>
          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
            <select
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={selectedProject}
              onChange={(event) => setSelectedProject(event.target.value)}
            >
              <option value="all">Tất cả công trình</option>
              {projectOptions.map((project) => <option key={project.value} value={project.value}>{project.label}</option>)}
            </select>
            <select
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={selectedStatus}
              onChange={(event) => setSelectedStatus(event.target.value)}
            >
              {statusOptions.map((status) => <option key={status.value} value={status.value}>{status.label}</option>)}
            </select>
            <select
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={selectedDeadline}
              onChange={(event) => setSelectedDeadline(event.target.value)}
            >
              <option value="all">Tất cả hạn việc</option>
              <option value="overdue">Quá hạn</option>
              <option value="due_7_days">Đến hạn 7 ngày</option>
              <option value="no_due">Chưa có hạn</option>
            </select>
            <select
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={selectedWorkType}
              onChange={(event) => setSelectedWorkType(event.target.value)}
            >
              <option value="all">Tất cả loại việc</option>
              {workTypeOptions.map((workType) => <option key={workType} value={workType}>{workTypeLabel(workType)}</option>)}
            </select>
            <select
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={selectedPriority}
              onChange={(event) => setSelectedPriority(event.target.value)}
            >
              <option value="all">Tất cả ưu tiên</option>
              {priorityOptions.map((priority) => <option key={priority} value={priority}>{priorityLabel(priority)}</option>)}
            </select>
          </div>
        </div>

        <div className="grid gap-4 xl:grid-cols-[240px_minmax(0,1fr)_340px]">
          <QueueTabs selectedQueue={selectedQueue} counts={queueCounts} onSelect={setSelectedQueue} />
          <WorkQueueList
            rows={filteredRows}
            onOpen={(item) => navigate(workItemDetailPath(item.id, currentReturnTo(location), item.project_id))}
          />
          <ContextPanel item={selectedContext} />
        </div>
      </section>
    </>
  );
}

function inQueue(item: WorkItem, queue: QueueKey) {
  if (queue === 'all') return true;
  const scopes = asArray<string>(item.queue_scopes);
  if (scopes.includes(queue)) return true;
  if (queue === 'pending_approval') return item.status === 'waiting_approval' || item.status === 'waiting_review';
  return false;
}

function attentionQueueLabel(queue: string) {
  return attentionQueues.find((item) => item.key === queue)?.label ?? queue;
}

function uniqueOptionValues(values: Array<string | undefined>) {
  return Array.from(new Set(values.filter((value): value is string => Boolean(value)))).sort((a, b) => a.localeCompare(b));
}

function inDeadlineBucket(item: WorkItem, bucket: string) {
  if (bucket === 'overdue') return isWorkOverdue(item);
  if (bucket === 'no_due') return !item.due_at;
  if (bucket === 'due_7_days') {
    if (!item.due_at || ['completed', 'cancelled'].includes(item.status)) return false;
    const due = new Date(item.due_at);
    if (Number.isNaN(due.getTime())) return false;
    const now = Date.now();
    const sevenDays = 7 * 24 * 60 * 60 * 1000;
    return due.getTime() >= now && due.getTime() <= now + sevenDays;
  }
  return true;
}

function buildSummary(rows: WorkItem[]) {
  const today = new Date().toISOString().slice(0, 10);
  return {
    needAction: rows.filter((item) => !['completed', 'cancelled'].includes(item.status)).length,
    overdue: rows.filter(isWorkOverdue).length,
    blocked: rows.filter((item) => item.status === 'blocked').length,
    waitingApproval: rows.filter((item) => item.status === 'waiting_approval' || item.status === 'waiting_review').length,
    doneToday: rows.filter((item) => item.status === 'completed' && String(item.completed_at ?? '').slice(0, 10) === today).length,
  };
}

function SummaryStrip({ summary }: { summary: ReturnType<typeof buildSummary> }) {
  const cards = [
    { label: 'Need action', value: summary.needAction, tone: 'border-blue-200 bg-blue-50', icon: <BriefcaseBusiness className="h-4 w-4" /> },
    { label: 'Overdue', value: summary.overdue, tone: summary.overdue > 0 ? 'border-red-200 bg-red-50' : 'border-line bg-white', icon: <Clock3 className="h-4 w-4" /> },
    { label: 'Blocked', value: summary.blocked, tone: summary.blocked > 0 ? 'border-rose-200 bg-rose-50' : 'border-line bg-white', icon: <AlertCircle className="h-4 w-4" /> },
    { label: 'Approval', value: summary.waitingApproval, tone: summary.waitingApproval > 0 ? 'border-amber-200 bg-amber-50' : 'border-line bg-white', icon: <ShieldAlert className="h-4 w-4" /> },
    { label: 'Done today', value: summary.doneToday, tone: 'border-emerald-200 bg-emerald-50', icon: <CheckCircle2 className="h-4 w-4" /> },
  ];
  return (
    <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      {cards.map((card) => (
        <div key={card.label} className={clsx('rounded-lg border p-4 shadow-panel', card.tone)}>
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{card.label}</p>
              <p className="mt-2 text-2xl font-semibold text-ink">{card.value}</p>
            </div>
            <div className="rounded-md bg-white/75 p-2 text-slate-700">{card.icon}</div>
          </div>
        </div>
      ))}
    </section>
  );
}

function ActivityAttentionPanel({
  rows,
  loading,
  error,
  selectedQueue,
  streamStatus,
  markingId,
  onSelectQueue,
  onRefresh,
  onMarkRead,
}: {
  rows: ActivityAttentionItem[];
  loading: boolean;
  error: unknown;
  selectedQueue: AttentionQueueKey;
  streamStatus: StreamStatus;
  markingId?: string;
  onSelectQueue: (queue: AttentionQueueKey) => void;
  onRefresh: () => void;
  onMarkRead: (activityId: string) => void;
}) {
  const counts = useMemo(() => {
    return attentionQueues.reduce<Record<AttentionQueueKey, number>>((acc, queue) => {
      acc[queue.key] = rows.filter((item) => inAttentionQueue(item, queue.key)).length;
      return acc;
    }, { all: 0, mentions: 0, questions: 0, blockers: 0, reminders: 0, unread: 0 });
  }, [rows]);
  const visibleRows = useMemo(() => rows.filter((item) => inAttentionQueue(item, selectedQueue)), [rows, selectedQueue]);

  return (
    <section className="mt-5 rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="font-semibold text-ink">Communication attention</h2>
          <p className="mt-1 text-sm text-slate-500">Mentions, open questions, blockers, and due reminders</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <LiveStatusBadge status={streamStatus} />
          <button
            className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            onClick={onRefresh}
          >
            <RefreshCw className="h-4 w-4" />
            Refresh
          </button>
        </div>
      </div>

      <div className="mb-3 flex gap-2 overflow-x-auto">
        {attentionQueues.map((queue) => {
          const Icon = queue.icon;
          const active = selectedQueue === queue.key;
          return (
            <button
              key={queue.key}
              className={clsx(
                'inline-flex min-w-fit items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold',
                active ? 'border-blue-200 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700 hover:bg-slate-50',
              )}
              onClick={() => onSelectQueue(queue.key)}
            >
              <Icon className="h-4 w-4" />
              {queue.label}
              <span className="rounded-md bg-white px-2 py-0.5 text-xs text-slate-600">{counts[queue.key]}</span>
            </button>
          );
        })}
      </div>

      {loading ? <LoadingSpinner /> : null}
      {error ? (
        <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">
          Không tải được communication attention.
        </div>
      ) : null}
      {!loading && !error && !visibleRows.length ? <EmptyState title="Không có communication attention trong queue này" /> : null}

      {visibleRows.length ? (
        <div className="grid gap-2 xl:grid-cols-2">
          {visibleRows.slice(0, 8).map((item) => (
            <div key={item.id} className={clsx('rounded-md border p-3', item.is_unread ? 'border-blue-200 bg-blue-50' : 'border-line bg-slate-50')}>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-1">
                    {item.is_unread ? <span className="rounded-md bg-blue-600 px-2 py-0.5 text-xs font-semibold text-white">Unread</span> : null}
                    {item.mentioned_me ? <span className="rounded-md bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-700">@you</span> : null}
                    {asArray<string>(item.queue_types).slice(0, 3).map((queue) => (
                      <span key={queue} className="rounded-md bg-white px-2 py-0.5 text-xs font-semibold text-slate-600">{attentionQueueLabel(queue)}</span>
                    ))}
                  </div>
                  <p className="mt-2 text-sm font-semibold text-ink">{activityLabel(item.activity_type)}</p>
                  <p className="mt-1 line-clamp-1 text-xs text-slate-500">
                    {item.actor_name ?? 'System'} · {item.target_label ?? item.work_item_title ?? (item.scope_type ? scopeTypeLabel(item.scope_type) : 'Công trình')} · {compactDateTime(item.created_at)}
                  </p>
                </div>
                {item.is_unread ? (
                  <button
                    className="inline-flex shrink-0 items-center gap-1 rounded-md border border-blue-200 bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60"
                    disabled={markingId === item.id}
                    onClick={() => onMarkRead(item.id)}
                  >
                    <CheckCircle2 className="h-3 w-3" />
                    Read
                  </button>
                ) : null}
              </div>
              <ActivityReplyQuote replyTo={item.reply_to} />
              {item.comment ? <p className="mt-2 line-clamp-2 text-sm leading-6 text-slate-700">{item.comment}</p> : null}
              <ActivityAttachments attachments={item.attachments} compact />
              {item.reactions?.length ? <div className="mt-2"><ActivityReactionBar reactions={item.reactions} /></div> : null}
              {item.project_id ? (
                <Link to={`/projects/${item.project_id}`} className="mt-2 inline-flex text-xs font-semibold text-blue-700 hover:text-blue-800">
                  Open project
                </Link>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}

function LiveStatusBadge({ status }: { status: StreamStatus }) {
  const label = status === 'connected' ? 'Live connected' : status === 'reconnecting' ? 'Reconnecting' : 'Offline';
  const tone = status === 'connected' ? 'bg-emerald-50 text-emerald-700' : status === 'reconnecting' ? 'bg-amber-50 text-amber-700' : 'bg-slate-100 text-slate-600';
  const dot = status === 'connected' ? 'bg-emerald-500' : status === 'reconnecting' ? 'bg-amber-500' : 'bg-slate-400';
  return (
    <span className={clsx('inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-xs font-semibold', tone)}>
      <span className={clsx('h-2 w-2 rounded-full', dot)} />
      {label}
    </span>
  );
}

function inAttentionQueue(item: ActivityAttentionItem, queue: AttentionQueueKey) {
  if (queue === 'all') return true;
  if (queue === 'unread') return Boolean(item.is_unread);
  return asArray<string>(item.queue_types).includes(queue);
}

function QueueTabs({
  selectedQueue,
  counts,
  onSelect,
}: {
  selectedQueue: QueueKey;
  counts: Record<QueueKey, number>;
  onSelect: (queue: QueueKey) => void;
}) {
  return (
    <nav className="flex gap-2 overflow-x-auto xl:block xl:space-y-2 xl:overflow-visible">
      {queues.map((queue) => {
        const active = selectedQueue === queue.key;
        return (
          <button
            key={queue.key}
            className={clsx(
              'min-w-44 rounded-md border px-3 py-2 text-left xl:w-full',
              active ? 'border-blue-200 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700 hover:bg-slate-50',
            )}
            onClick={() => onSelect(queue.key)}
          >
            <span className="flex items-center justify-between gap-3">
              <span className="text-sm font-semibold">{queue.label}</span>
              <span className="rounded-md bg-white px-2 py-0.5 text-xs font-semibold text-slate-600">{counts[queue.key]}</span>
            </span>
            <span className="mt-1 block text-xs leading-5 text-slate-500">{queue.description}</span>
          </button>
        );
      })}
    </nav>
  );
}

function WorkQueueList({
  rows,
  onOpen,
}: {
  rows: WorkItem[];
  onOpen: (item: WorkItem) => void;
}) {
  if (!rows.length) return <EmptyState title="Không có work item trong queue này" />;
  return (
    <div className="max-h-[720px] space-y-3 overflow-y-auto pr-1">
      {rows.map((item) => (
        <div key={item.id}>
          <WorkItemCard item={item} onOpen={() => onOpen(item)} />
        </div>
      ))}
    </div>
  );
}

function ContextPanel({ item }: { item: WorkItem | null }) {
  if (!item) {
    return (
      <aside className="rounded-lg border border-dashed border-slate-300 bg-slate-50 p-4">
        <p className="text-sm font-semibold text-ink">Context / Activity</p>
        <p className="mt-2 text-sm leading-6 text-slate-600">Chọn một work item để xem project context, evidence và activity.</p>
      </aside>
    );
  }

  return (
    <aside className="rounded-lg border border-line bg-slate-50 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Project context</p>
          <p className="mt-1 truncate text-sm font-semibold text-ink">{item.project_code ? `${item.project_code} · ` : ''}{item.project_name ?? 'Project'}</p>
        </div>
        <StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />
      </div>
      <div className="mt-4 space-y-3 text-sm">
        <ContextRow label="Node" value={item.run_node_name ?? '-'} />
        <ContextRow label="Step" value={item.run_step_name ?? '-'} />
        <ContextRow label="Department" value={departmentLabel(item.department)} />
        <ContextRow label="Due" value={dateText(item.due_at)} />
        <ContextRow label="Evidence" value={numberValue(item.evidence_count)} />
      </div>
      {item.blocked_reason ? <p className="mt-4 rounded-md bg-rose-50 p-3 text-sm leading-6 text-rose-700">{item.blocked_reason}</p> : null}
      {item.project_id ? (
        <Link
          to={`/projects/${item.project_id}`}
          className="mt-4 inline-flex w-full justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
        >
          Open project dashboard
        </Link>
      ) : null}
    </aside>
  );
}

function ContextRow({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-line pb-2 last:border-0 last:pb-0">
      <span className="text-slate-500">{label}</span>
      <span className="min-w-0 truncate font-semibold text-ink">{value}</span>
    </div>
  );
}

function MyWorkSkeleton() {
  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-line bg-white p-5 shadow-panel">
        <LoadingSpinner />
      </div>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        {Array.from({ length: 5 }).map((_, index) => (
          <div key={index} className="h-28 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        ))}
      </div>
      <div className="h-[520px] animate-pulse rounded-lg border border-line bg-white shadow-panel" />
    </div>
  );
}

function MyWorkError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const apiError = error instanceof ApiError ? error : null;
  const forbidden = apiError?.status === 403;
  const title = forbidden ? 'Bạn không có quyền xem inbox công việc' : 'Không tải được việc của tôi';
  const description = error instanceof Error ? error.message : 'Vui lòng thử lại hoặc kiểm tra quyền truy cập.';

  return (
    <>
      <PageHeader title="Việc của tôi" description="Cross-project operational inbox" />
      <section className="rounded-lg border border-line bg-white p-6 shadow-panel">
        <div className="flex items-start gap-3">
          {forbidden ? <ShieldAlert className="mt-0.5 h-6 w-6 text-amber-600" /> : <AlertCircle className="mt-0.5 h-6 w-6 text-rose-600" />}
          <div className="min-w-0 flex-1">
            <h2 className="text-base font-semibold text-ink">{title}</h2>
            <p className="mt-1 text-sm leading-6 text-slate-600">{description}</p>
            {!forbidden ? (
              <button
                className="mt-4 inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
                onClick={onRetry}
              >
                <RefreshCw className="h-4 w-4" />
                Retry
              </button>
            ) : null}
          </div>
        </div>
      </section>
    </>
  );
}
