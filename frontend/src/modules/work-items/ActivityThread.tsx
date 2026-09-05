import { useDeferredValue, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient, type InfiniteData } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import {
  AlertCircle,
  ArrowDown,
  AtSign,
  BellRing,
  CheckCircle2,
  ChevronDown,
  CircleHelp,
  Filter,
  ListChecks,
  Loader2,
  MessageSquare,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Reply,
  Search,
  ShieldAlert,
  Trash2,
  Unplug,
} from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { Dialog } from '../../components/Dialog';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { ApiError, get, post, put, streamSSE, type StreamStatus } from '../../lib/api/client';
import { useToast } from '../../components/ToastProvider';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  ActivityPayload,
  ActivityReactionSummary,
  ProjectActivityFeed,
  ProjectActivityOptions,
  ProjectActivitySummary,
  WorkItemActivity,
} from '../../lib/types/domain';
import { activityLabel, compactDateTime, scopeTypeLabel, workflowFormActivityComment } from './workItemPresentation';
import { ActivityAttachments } from './ActivityAttachments';
import { ActivityTopicCard, ActivityTopicProvider } from './ActivityTopic';
import { activityVisibilityLabel } from './activityTopicDomain';
import { ActivityReactionBar, ActivityReplyQuote } from './ActivityConversationMeta';
import {
  ActivityComposer,
  type ActivityAttachmentOption,
  type ActivityMentionOption,
  type ActivityTargetOption,
} from './ActivityComposer';
import {
  activityResolveLabel,
  activityResolved,
  activityTone,
  actorInitials,
  chatBubbleTone,
  dedupeOptions,
  groupActivitiesByDay,
  sameChatAuthor,
  updateActivityReactionFeed,
} from './activityThreadDomain';

type ActivityKind = '' | 'discussion' | 'decision' | 'system';
type AttentionFilter = '' | 'unread' | 'mentions' | 'questions' | 'blockers' | 'reminders';

const emptySummary: ProjectActivitySummary = {
  total: 0,
  discussion: 0,
  system: 0,
  decisions: 0,
  unread: 0,
  discussion_unread: 0,
  mentioned: 0,
  questions: 0,
  blockers: 0,
  reminders: 0,
  attention: 0,
};

const kindTabs: Array<{ value: ActivityKind; label: string; countKey: keyof ProjectActivitySummary }> = [
  { value: '', label: 'Tất cả', countKey: 'total' },
  { value: 'discussion', label: 'Thảo luận', countKey: 'discussion' },
  { value: 'decision', label: 'Quyết định', countKey: 'decisions' },
  { value: 'system', label: 'Nhật ký hệ thống', countKey: 'system' },
];

const attentionItems: Array<{
  value: AttentionFilter;
  label: string;
  countKey: keyof ProjectActivitySummary;
  icon: typeof MessageSquare;
}> = [
  { value: 'unread', label: 'Chưa đọc', countKey: 'unread', icon: MessageSquare },
  { value: 'mentions', label: 'Nhắc đến tôi', countKey: 'mentioned', icon: AtSign },
  { value: 'questions', label: 'Câu hỏi mở', countKey: 'questions', icon: CircleHelp },
  { value: 'blockers', label: 'Điểm chặn', countKey: 'blockers', icon: ShieldAlert },
  { value: 'reminders', label: 'Sắp đến hạn', countKey: 'reminders', icon: BellRing },
];

const activityTypeFilters = [
  '',
  'comment',
  'status',
  'blocker',
  'question',
  'decision',
  'reminder',
  'file_note',
];

const visibilityLabels: Record<string, string> = {
  internal: 'Nội bộ',
  project: 'Thành viên công trình',
  client: 'Chủ đầu tư',
};

export function ActivityThread(props: Parameters<typeof ActivityThreadContent>[0]) {
  return <ActivityTopicProvider projectId={props.projectId}><ActivityThreadContent {...props} /></ActivityTopicProvider>;
}

function ActivityThreadContent({
  projectId,
  targetOptions,
  attachmentOptions = [],
  onChanged,
}: {
  projectId: string;
  targetOptions: ActivityTargetOption[];
  attachmentOptions?: ActivityAttachmentOption[];
  onChanged?: () => void;
}) {
  const toast = useToast();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [kind, setKind] = useState<ActivityKind>('');
  const [attention, setAttention] = useState<AttentionFilter>('');
  const [scopeFilter, setScopeFilter] = useState('');
  const [scopeIdFilter, setScopeIdFilter] = useState('');
  const [activityTypeFilter, setActivityTypeFilter] = useState('');
  const [visibilityFilter, setVisibilityFilter] = useState('');
  const [actorFilter, setActorFilter] = useState('');
  const [search, setSearch] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  const [replyTo, setReplyTo] = useState<WorkItemActivity | null>(null);
  const [editItem, setEditItem] = useState<WorkItemActivity | null>(null);
  const [withdrawItem, setWithdrawItem] = useState<WorkItemActivity | null>(null);
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('reconnecting');
  const deferredSearch = useDeferredValue(search.trim());
  const onChangedRef = useRef(onChanged);

  const options = useQuery({
    queryKey: ['project-activity-options', projectId],
    queryFn: () => get<ProjectActivityOptions>(`/projects/${projectId}/activity/options`),
    enabled: Boolean(projectId),
    retry: false,
  });

  const queryKey = [
    'project-activity',
    projectId,
    kind,
    attention,
    scopeFilter,
    scopeIdFilter,
    activityTypeFilter,
    visibilityFilter,
    actorFilter,
    deferredSearch,
  ];

  const activity = useInfiniteQuery({
    queryKey,
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams();
      if (kind) params.set('kind', kind);
      if (attention) params.set('attention', attention);
      if (scopeFilter) params.set('scope_type', scopeFilter);
      if (scopeIdFilter) params.set('scope_id', scopeIdFilter);
      if (activityTypeFilter) params.set('activity_type', activityTypeFilter);
      if (visibilityFilter) params.set('visibility', visibilityFilter);
      if (actorFilter) params.set('actor_id', actorFilter);
      if (deferredSearch) params.set('search', deferredSearch);
      if (pageParam) params.set('cursor', String(pageParam));
      params.set('limit', '30');
      return get<ProjectActivityFeed>(`/projects/${projectId}/activity?${params.toString()}`);
    },
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: Boolean(projectId),
    retry: false,
  });

  const invalidateActivity = () => {
    queryClient.invalidateQueries({ queryKey: ['project-activity', projectId] });
    queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
    queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
    onChanged?.();
  };

  const addActivity = useMutation({
    mutationFn: (payload: ActivityPayload) => post<WorkItemActivity>(`/projects/${projectId}/activity`, payload),
    onSuccess: () => {
      invalidateActivity();
      setReplyTo(null);
      toast.success('Đã gửi trao đổi');
    },
    onError: (error) => toast.error('Không gửi được trao đổi', error),
  });

  const markRead = useMutation({
    mutationFn: (activityId: string) => post(`/activities/${activityId}/read`),
    onSuccess: invalidateActivity,
    onError: (error) => toast.error('Không cập nhật được trạng thái đã đọc', error),
  });

  const markAllRead = useMutation({
    mutationFn: () => post(`/projects/${projectId}/activity/read-all`),
    onSuccess: () => {
      invalidateActivity();
      toast.success('Đã đánh dấu tất cả là đã đọc');
    },
    onError: (error) => toast.error('Không đánh dấu được tất cả hoạt động', error),
  });

  const resolveActivity = useMutation({
    mutationFn: (activityId: string) => post(`/activities/${activityId}/resolve`, {}),
    onSuccess: () => {
      invalidateActivity();
      toast.success('Đã cập nhật hoạt động');
    },
    onError: (error) => toast.error('Không xử lý được hoạt động', error),
  });

  const setReaction = useMutation({
    mutationFn: ({ activityId, reaction, active }: { activityId: string; reaction: ActivityReactionSummary['reaction']; active: boolean }) =>
      post(`/activities/${activityId}/reactions`, { reaction, active }),
    onMutate: async (variables) => {
      const key = ['project-activity', projectId];
      await queryClient.cancelQueries({ queryKey: key });
      const snapshots = queryClient.getQueriesData<InfiniteData<ProjectActivityFeed>>({ queryKey: key });
      queryClient.setQueriesData<InfiniteData<ProjectActivityFeed>>({ queryKey: key }, (current) => updateActivityReactionFeed(current, variables));
      return { snapshots };
    },
    onError: (error, _variables, context) => {
      context?.snapshots.forEach(([key, value]) => queryClient.setQueryData(key, value));
      toast.error('Không cập nhật được phản ứng', error);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['project-activity', projectId] }),
  });

  const updateMessage = useMutation({
    mutationFn: ({ activityId, comment }: { activityId: string; comment: string }) =>
      put<WorkItemActivity>(`/activities/${activityId}/message`, { comment }),
    onSuccess: () => {
      invalidateActivity();
      setEditItem(null);
      toast.success('Đã cập nhật tin nhắn');
    },
    onError: (error) => {
      if (error instanceof ApiError && error.code === 'VALIDATION_ERROR') {
        toast.warn({ title: 'Không thể lưu chỉnh sửa', description: error.message });
        return;
      }
      toast.error('Không chỉnh sửa được tin nhắn', error);
    },
  });

  const withdrawMessage = useMutation({
    mutationFn: (activityId: string) => post<WorkItemActivity>(`/activities/${activityId}/withdraw`),
    onSuccess: (_data, activityId) => {
      invalidateActivity();
      setWithdrawItem(null);
      setEditItem((current) => current?.id === activityId ? null : current);
      setReplyTo((current) => current?.id === activityId ? null : current);
      toast.success('Đã thu hồi tin nhắn');
    },
    onError: (error) => toast.error('Không thu hồi được tin nhắn', error),
  });

  const rows = useMemo(() => activity.data?.pages.flatMap((page) => page.items) ?? [], [activity.data]);
  const summary = activity.data?.pages[0]?.summary ?? emptySummary;

  const serverTargets = options.data?.targets ?? [];
  const mergedTargets = useMemo(
    () => dedupeOptions([...targetOptions, ...serverTargets.map((item) => ({
      scope_type: item.scope_type,
      scope_id: item.scope_id,
      label: item.label,
      work_item_id: item.work_item_id,
    }))], (item) => `${item.scope_type}:${item.scope_id}`),
    [serverTargets, targetOptions],
  );
  const mergedAttachments = useMemo(
    () => dedupeOptions([...attachmentOptions, ...(options.data?.attachments ?? [])], (item) => `${item.type}:${item.id}`),
    [attachmentOptions, options.data?.attachments],
  );
  const mentionOptions = useMemo<ActivityMentionOption[]>(() => [
    ...(options.data?.members ?? []).map((item) => ({
      type: 'staff' as const,
      id: item.id,
      label: item.label,
      description: [item.position, item.department].filter(Boolean).join(' · '),
    })),
    ...(options.data?.roles ?? []).map((item) => ({ type: 'role' as const, id: item.id, label: item.label })),
  ], [options.data]);
  const scopeOptions = useMemo(() => {
    const scopes = new Set(mergedTargets.map((item) => item.scope_type));
    return ['project', ...Array.from(scopes).filter((scope) => scope !== 'project')];
  }, [mergedTargets]);
  const filteredTargets = mergedTargets.filter((item) => item.scope_type === scopeFilter);

  useEffect(() => {
    onChangedRef.current = onChanged;
  }, [onChanged]);

  useEffect(() => {
    if (!projectId) return undefined;
    return streamSSE(`/projects/${projectId}/activity/stream`, {
      onStatus: setStreamStatus,
      onEvent: (message) => {
        if (message.event === 'activity_changed' || message.event === 'activity_read') {
          queryClient.invalidateQueries({ queryKey: ['project-activity', projectId] });
          queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
          queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
          onChangedRef.current?.();
        }
      },
    });
  }, [projectId, queryClient]);

  function selectAttention(value: AttentionFilter) {
    setAttention((current) => current === value ? '' : value);
  }

  function clearFilters() {
    setAttention('');
    setScopeFilter('');
    setScopeIdFilter('');
    setActivityTypeFilter('');
    setVisibilityFilter('');
    setActorFilter('');
    setSearch('');
  }

  const hasFilters = Boolean(attention || scopeFilter || activityTypeFilter || visibilityFilter || actorFilter || deferredSearch);

  return (
    <div className="space-y-3">
      {kind !== 'discussion' ? (
        <ActivityComposer
          mode="project"
          allowTarget
          targetOptions={mergedTargets}
          attachmentOptions={mergedAttachments}
          mentionOptions={mentionOptions}
          submitting={addActivity.isPending}
          onSubmit={(payload) => addActivity.mutateAsync(payload).then(() => undefined)}
        />
      ) : null}

      <div className="flex gap-2 overflow-x-auto pb-1 xl:hidden">
        {attentionItems.map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.value}
              type="button"
              className={clsx(
                'inline-flex min-w-fit items-center gap-1.5 rounded-md border px-3 py-2 text-xs font-semibold',
                attention === item.value ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-line bg-white text-slate-600',
              )}
              onClick={() => selectAttention(item.value)}
            >
              <Icon className="h-3.5 w-3.5" />
              {item.label}
              <span className="rounded bg-white px-1.5 py-0.5 text-[11px] text-slate-600">{summary[item.countKey] ?? 0}</span>
            </button>
          );
        })}
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_280px]">
        <section className="min-w-0 rounded-lg border border-line bg-white shadow-panel">
          <div className="border-b border-line p-3">
            <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
              <div className="flex gap-1 overflow-x-auto">
                {kindTabs.map((tab) => (
                  <button
                    key={tab.value || 'all'}
                    type="button"
                    className={clsx(
                      'inline-flex min-w-fit items-center gap-1.5 rounded-md px-3 py-2 text-sm font-semibold',
                      kind === tab.value ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-50',
                    )}
                    onClick={() => setKind(tab.value)}
                  >
                    {tab.label}
                    <span className="text-xs text-slate-400">{summary[tab.countKey] ?? 0}</span>
                  </button>
                ))}
              </div>
              <div className="flex items-center gap-2">
                <label className="relative min-w-0 flex-1 lg:w-64">
                  <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
                  <input
                    className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm outline-none focus:border-blue-300 focus:ring-2 focus:ring-blue-100"
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder="Tìm trong trao đổi..."
                  />
                </label>
                <button
                  type="button"
                  className={clsx(
                    'inline-flex items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold',
                    showFilters || hasFilters ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-line text-slate-600 hover:bg-slate-50',
                  )}
                  onClick={() => setShowFilters((value) => !value)}
                >
                  <Filter className="h-4 w-4" />
                  <span className="hidden sm:inline">Bộ lọc</span>
                  <ChevronDown className={clsx('h-3.5 w-3.5 transition', showFilters && 'rotate-180')} />
                </button>
              </div>
            </div>

            {showFilters ? (
              <div className="mt-3 grid gap-2 rounded-md border border-line bg-slate-50 p-3 sm:grid-cols-2 xl:grid-cols-3">
                <FilterSelect label="Loại" value={activityTypeFilter} onChange={setActivityTypeFilter}>
                  <option value="">Tất cả loại</option>
                  {activityTypeFilters.filter(Boolean).map((value) => <option key={value} value={value}>{activityLabel(value)}</option>)}
                </FilterSelect>
                <FilterSelect label="Gắn vào" value={scopeFilter} onChange={(value) => { setScopeFilter(value); setScopeIdFilter(''); }}>
                  <option value="">Tất cả đối tượng</option>
                  {scopeOptions.map((scope) => <option key={scope} value={scope}>{scopeTypeLabel(scope)}</option>)}
                </FilterSelect>
                <FilterSelect label="Đối tượng" value={scopeIdFilter} onChange={setScopeIdFilter} disabled={!scopeFilter || scopeFilter === 'project'}>
                  <option value="">{scopeFilter && scopeFilter !== 'project' ? 'Tất cả trong loại này' : 'Chọn loại đối tượng trước'}</option>
                  {filteredTargets.map((item) => <option key={`${item.scope_type}:${item.scope_id}`} value={item.scope_id}>{item.label}</option>)}
                </FilterSelect>
                <FilterSelect label="Người gửi" value={actorFilter} onChange={setActorFilter}>
                  <option value="">Tất cả người gửi</option>
                  {(options.data?.members ?? []).filter((item) => item.user_id).map((item) => <option key={item.user_id} value={item.user_id}>{item.label}</option>)}
                </FilterSelect>
                <FilterSelect label="Ai có thể xem" value={visibilityFilter} onChange={setVisibilityFilter}>
                  <option value="">Tất cả phạm vi xem</option>
                  {Object.entries(visibilityLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </FilterSelect>
                <div className="flex items-end">
                  <button type="button" className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100 disabled:opacity-50" disabled={!hasFilters} onClick={clearFilters}>
                    Xóa bộ lọc
                  </button>
                </div>
              </div>
            ) : null}
          </div>

          {kind === 'discussion' ? (
            <div className="flex min-h-[520px] flex-col bg-slate-50/70">
              {activity.isLoading ? <div className="p-3"><ActivitySkeleton /></div> : null}
              {activity.error ? (
                <div className="p-3">
                  <ActivityError onRetry={() => void activity.refetch()} />
                </div>
              ) : null}
              {!activity.isLoading && !activity.error && !rows.length ? (
                <div className="flex min-h-[380px] items-center justify-center p-4">
                  <EmptyState
                    title={hasFilters ? 'Không có thảo luận phù hợp' : 'Chưa có thảo luận do người dùng tạo'}
                    action={hasFilters ? (
                      <button type="button" className="text-sm font-semibold text-blue-700" onClick={clearFilters}>Xóa bộ lọc</button>
                    ) : summary.system > 0 ? (
                      <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setKind('system')}>
                        Mở Nhật ký hệ thống ({summary.system})
                      </button>
                    ) : undefined}
                  />
                </div>
              ) : null}
              {rows.length ? (
                <ChatDiscussion
                  key={[attention, scopeFilter, scopeIdFilter, activityTypeFilter, visibilityFilter, actorFilter, deferredSearch].join(':')}
                  rows={rows}
                  currentUserId={user?.id}
                  projectId={projectId}
                  hasOlder={Boolean(activity.hasNextPage)}
                  loadingOlder={activity.isFetchingNextPage}
                  onLoadOlder={() => activity.fetchNextPage().then(() => undefined)}
                  markingId={markRead.isPending ? markRead.variables : undefined}
                  resolvingId={resolveActivity.isPending ? resolveActivity.variables : undefined}
                  onMarkRead={(id) => markRead.mutate(id)}
                  onResolve={(id) => resolveActivity.mutate(id)}
                  onReply={(item) => { setEditItem(null); setReplyTo(item); }}
                  onEdit={(item) => { setReplyTo(null); setEditItem(item); }}
                  onWithdraw={setWithdrawItem}
                  mutatingMessageId={updateMessage.isPending ? updateMessage.variables.activityId : withdrawMessage.isPending ? withdrawMessage.variables : undefined}
                  reactingKey={setReaction.isPending ? `${setReaction.variables.activityId}:${setReaction.variables.reaction}` : undefined}
                  onReaction={(activityId, reaction, active) => setReaction.mutate({ activityId, reaction, active })}
                />
              ) : null}
              <div className="sticky bottom-0 z-10 mt-auto border-t border-line bg-white/95 p-3 backdrop-blur">
                <ActivityComposer
                  mode="project"
                  variant="chat"
                  replyTo={replyTo}
                  editItem={editItem}
                  onCancelReply={() => setReplyTo(null)}
                  onCancelEdit={() => setEditItem(null)}
                  onSubmitEdit={(comment) => editItem ? updateMessage.mutateAsync({ activityId: editItem.id, comment }).then(() => undefined) : Promise.resolve()}
                  allowTarget
                  targetOptions={mergedTargets}
                  attachmentOptions={mergedAttachments}
                  mentionOptions={mentionOptions}
                  mentionOptionsLoading={options.isLoading}
                  mentionOptionsError={options.isError}
                  onRetryMentionOptions={() => void options.refetch()}
                  submitting={addActivity.isPending || updateMessage.isPending}
                  onSubmit={(payload) => addActivity.mutateAsync(payload).then(() => undefined)}
                />
              </div>
            </div>
          ) : (
            <div className="p-3">
              {activity.isLoading ? <ActivitySkeleton /> : null}
              {activity.error ? <ActivityError onRetry={() => void activity.refetch()} /> : null}
              {!activity.isLoading && !activity.error && !rows.length ? (
                <EmptyState
                  title={hasFilters || kind ? 'Không có hoạt động phù hợp' : 'Chưa có trao đổi'}
                  action={hasFilters ? <button type="button" className="text-sm font-semibold text-blue-700" onClick={clearFilters}>Xóa bộ lọc</button> : undefined}
                />
              ) : null}

              {rows.length ? (
                <div className="space-y-4">
                  {groupActivitiesByDay(rows).map((group) => (
                    <div key={group.key}>
                      <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">{group.label}</p>
                      <div className="space-y-2">
                        {group.items.map((item) => (
                          <ActivityCard
                            key={item.id}
                            item={item}
                            projectId={projectId}
                            marking={markRead.isPending && markRead.variables === item.id}
                            resolving={resolveActivity.isPending && resolveActivity.variables === item.id}
                            onMarkRead={() => markRead.mutate(item.id)}
                            onResolve={() => resolveActivity.mutate(item.id)}
                          />
                        ))}
                      </div>
                    </div>
                  ))}
                  {activity.hasNextPage ? (
                    <div className="flex justify-center pt-1">
                      <button
                        type="button"
                        className="inline-flex items-center gap-2 rounded-md border border-line px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-50 disabled:opacity-50"
                        disabled={activity.isFetchingNextPage}
                        onClick={() => void activity.fetchNextPage()}
                      >
                        {activity.isFetchingNextPage ? <Loader2 className="h-4 w-4 animate-spin" /> : <ListChecks className="h-4 w-4" />}
                        Tải thêm hoạt động
                      </button>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>
          )}
        </section>

        <aside className="hidden space-y-3 xl:block">
          <section className="rounded-lg border border-line bg-white p-3 shadow-panel">
            <div className="flex items-center justify-between gap-2">
              <div>
                <p className="text-sm font-semibold text-ink">Cần chú ý</p>
                <p className="mt-0.5 text-xs text-slate-500">{summary.attention} hoạt động đang chờ</p>
              </div>
              <LiveStatusBadge status={streamStatus} compact />
            </div>
            <div className="mt-3 space-y-1">
              {attentionItems.map((item) => {
                const Icon = item.icon;
                return (
                  <button
                    key={item.value}
                    type="button"
                    className={clsx(
                      'flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm',
                      attention === item.value ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-50',
                    )}
                    onClick={() => selectAttention(item.value)}
                  >
                    <Icon className="h-4 w-4" />
                    <span className="flex-1">{item.label}</span>
                    <span className="font-semibold">{summary[item.countKey] ?? 0}</span>
                  </button>
                );
              })}
            </div>
            <button
              type="button"
              className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50 disabled:opacity-50"
              disabled={!summary.unread || markAllRead.isPending}
              onClick={() => markAllRead.mutate()}
            >
              {markAllRead.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <CheckCircle2 className="h-3.5 w-3.5" />}
              Đánh dấu tất cả đã đọc
            </button>
          </section>
          <section className="rounded-lg border border-line bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Kết nối</p>
            <div className="mt-2 flex items-center justify-between gap-2">
              <LiveStatusBadge status={streamStatus} />
              <button type="button" className="rounded-md p-2 text-slate-500 hover:bg-white" onClick={() => void activity.refetch()} aria-label="Làm mới trao đổi">
                <RefreshCw className="h-4 w-4" />
              </button>
            </div>
          </section>
        </aside>
      </div>
      <Dialog
        open={Boolean(withdrawItem)}
        title="Thu hồi tin nhắn?"
        description="Nội dung sẽ bị ẩn nhưng các câu trả lời vẫn được giữ. Thao tác này được ghi vào lịch sử kiểm toán."
        size="sm"
        onClose={() => { if (!withdrawMessage.isPending) setWithdrawItem(null); }}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100 disabled:opacity-50" disabled={withdrawMessage.isPending} onClick={() => setWithdrawItem(null)}>Hủy</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:opacity-50" disabled={!withdrawItem || withdrawMessage.isPending} onClick={() => withdrawItem && withdrawMessage.mutate(withdrawItem.id)}>
              {withdrawMessage.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
              Thu hồi
            </button>
          </div>
        )}
      >
        <p className="text-sm leading-6 text-slate-600">Tin nhắn đã thu hồi không thể khôi phục hoặc tiếp tục nhận phản hồi và reaction.</p>
      </Dialog>
    </div>
  );
}

function ActivityError({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="rounded-md border border-rose-200 bg-rose-50 p-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-start gap-2">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
          <p className="text-sm text-rose-700">Không tải được luồng trao đổi.</p>
        </div>
        <button type="button" className="text-sm font-semibold text-rose-700" onClick={onRetry}>Thử lại</button>
      </div>
    </div>
  );
}

function ChatDiscussion({
  rows,
  currentUserId,
  projectId,
  hasOlder,
  loadingOlder,
  onLoadOlder,
  markingId,
  resolvingId,
  onMarkRead,
  onResolve,
  onReply,
  onEdit,
  onWithdraw,
  mutatingMessageId,
  reactingKey,
  onReaction,
}: {
  rows: WorkItemActivity[];
  currentUserId?: string;
  projectId: string;
  hasOlder: boolean;
  loadingOlder: boolean;
  onLoadOlder: () => Promise<void>;
  markingId?: string;
  resolvingId?: string;
  onMarkRead: (id: string) => void;
  onResolve: (id: string) => void;
  onReply: (item: WorkItemActivity) => void;
  onEdit: (item: WorkItemActivity) => void;
  onWithdraw: (item: WorkItemActivity) => void;
  mutatingMessageId?: string;
  reactingKey?: string;
  onReaction: (activityId: string, reaction: ActivityReactionSummary['reaction'], active: boolean) => void;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const nearBottomRef = useRef(true);
  const initializedRef = useRef(false);
  const newestIdRef = useRef<string>();
  const [showLatest, setShowLatest] = useState(false);
  const chronologicalRows = useMemo(() => [...rows].reverse(), [rows]);

  function scrollToLatest(behavior: ScrollBehavior = 'smooth') {
    const element = scrollRef.current;
    if (!element) return;
    element.scrollTo({ top: element.scrollHeight, behavior });
    nearBottomRef.current = true;
    setShowLatest(false);
  }

  useEffect(() => {
    const newestId = rows[0]?.id;
    const hasNewMessage = Boolean(initializedRef.current && newestId && newestId !== newestIdRef.current);
    newestIdRef.current = newestId;
    const frame = window.requestAnimationFrame(() => {
      if (!initializedRef.current) {
        initializedRef.current = true;
        scrollToLatest('auto');
        return;
      }
      if (hasNewMessage) {
        if (nearBottomRef.current) scrollToLatest();
        else setShowLatest(true);
      }
    });
    return () => window.cancelAnimationFrame(frame);
  }, [rows.length, rows[0]?.id]);

  async function loadOlder() {
    const element = scrollRef.current;
    const previousHeight = element?.scrollHeight ?? 0;
    const previousTop = element?.scrollTop ?? 0;
    await onLoadOlder();
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        if (!element) return;
        element.scrollTop = previousTop + element.scrollHeight - previousHeight;
      });
    });
  }

  return (
    <div className="relative flex-1">
      <div
        ref={scrollRef}
        className="h-[min(64vh,680px)] min-h-[420px] overflow-y-auto px-3 py-4 sm:px-5"
        onScroll={(event) => {
          const element = event.currentTarget;
          nearBottomRef.current = element.scrollHeight - element.scrollTop - element.clientHeight < 96;
          if (nearBottomRef.current) setShowLatest(false);
        }}
      >
        {hasOlder ? (
          <div className="mb-4 flex justify-center">
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-full border border-line bg-white px-3 py-1.5 text-xs font-semibold text-slate-600 shadow-sm hover:bg-slate-50 disabled:opacity-50"
              disabled={loadingOlder}
              onClick={() => void loadOlder()}
            >
              {loadingOlder ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ListChecks className="h-3.5 w-3.5" />}
              Tải tin nhắn cũ hơn
            </button>
          </div>
        ) : (
          <p className="mb-4 text-center text-xs text-slate-400">Đã đến đầu cuộc thảo luận</p>
        )}

        <div className="space-y-5">
          {groupActivitiesByDay(chronologicalRows).map((group) => (
            <div key={group.key}>
              <div className="mb-4 flex items-center gap-3">
                <span className="h-px flex-1 bg-slate-200" />
                <span className="rounded-full bg-slate-200/80 px-2.5 py-1 text-[11px] font-semibold text-slate-500">{group.label}</span>
                <span className="h-px flex-1 bg-slate-200" />
              </div>
              <div className="space-y-1.5">
                {group.items.map((item, index) => (
                  <ChatMessage
                    key={item.id}
                    item={item}
                    projectId={projectId}
                    own={Boolean(currentUserId && item.actor_id === currentUserId)}
                    compact={sameChatAuthor(group.items[index - 1], item)}
                    marking={markingId === item.id}
                    resolving={resolvingId === item.id}
                    onMarkRead={() => onMarkRead(item.id)}
                    onResolve={() => onResolve(item.id)}
                    onReply={() => onReply(item)}
                    onEdit={() => onEdit(item)}
                    onWithdraw={() => onWithdraw(item)}
                    mutating={mutatingMessageId === item.id}
                    pendingReaction={reactingKey?.startsWith(`${item.id}:`) ? reactingKey.split(':')[1] as ActivityReactionSummary['reaction'] : undefined}
                    onReaction={(reaction, active) => onReaction(item.id, reaction, active)}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
      {showLatest ? (
        <button
          type="button"
          className="absolute bottom-3 left-1/2 inline-flex -translate-x-1/2 items-center gap-1.5 rounded-full bg-blue-600 px-3 py-2 text-xs font-semibold text-white shadow-lg hover:bg-blue-700"
          onClick={() => scrollToLatest()}
        >
          <ArrowDown className="h-3.5 w-3.5" />
          Có tin nhắn mới
        </button>
      ) : null}
    </div>
  );
}

function ChatMessage({
  item,
  projectId,
  own,
  compact,
  marking,
  resolving,
  onMarkRead,
  onResolve,
  onReply,
  onEdit,
  onWithdraw,
  mutating,
  pendingReaction,
  onReaction,
}: {
  item: WorkItemActivity;
  projectId: string;
  own: boolean;
  compact: boolean;
  marking: boolean;
  resolving: boolean;
  onMarkRead: () => void;
  onResolve: () => void;
  onReply: () => void;
  onEdit: () => void;
  onWithdraw: () => void;
  mutating: boolean;
  pendingReaction?: ActivityReactionSummary['reaction'];
  onReaction: (reaction: ActivityReactionSummary['reaction'], active: boolean) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!menuOpen) return undefined;
    const closeOnOutside = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMenuOpen(false);
    };
    document.addEventListener('mousedown', closeOnOutside);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', closeOnOutside);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [menuOpen]);

  const system = !['comment', 'status', 'blocker', 'question', 'decision', 'reminder', 'file_note'].includes(item.activity_type);
  const comment = workflowFormActivityComment(item);
  if (system) {
    return (
      <div className="flex justify-center py-1">
        <span className="max-w-[90%] rounded-full bg-slate-200/80 px-3 py-1 text-center text-[11px] text-slate-500">
          {identityReferenceText(item.actor_name, item.actor_account_deleted, 'Hệ thống')} · {activityLabel(item.activity_type)}{comment ? ` · ${comment}` : ''} · {compactDateTime(item.created_at)}
        </span>
      </div>
    );
  }

  const resolveLabel = activityResolveLabel(item);
  const resolved = activityResolved(item);
  const withdrawn = Boolean(item.withdrawn_at);
  const ownBlueBubble = own && !['blocker', 'question', 'decision', 'reminder'].includes(item.activity_type) && !withdrawn;
  const metadata = item.metadata_json ?? {};
  return (
    <article className={clsx('flex items-end gap-2', own ? 'justify-end' : 'justify-start', compact ? 'pt-0' : 'pt-2')}>
      {!own ? (
        <div className={clsx('flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-200 text-[11px] font-bold text-slate-600', compact && 'invisible')}>
          {actorInitials(item.actor_name)}
        </div>
      ) : null}
      <div className={clsx('max-w-[82%] sm:max-w-[72%]', own && 'text-right')}>
        {!compact ? (
          <div className={clsx('mb-1 flex flex-wrap items-center gap-1.5 text-[11px] text-slate-500', own && 'justify-end')}>
            <span className="font-semibold text-slate-700">{own ? 'Bạn' : identityReferenceText(item.actor_name, item.actor_account_deleted, 'Hệ thống')}</span>
            <span>{compactDateTime(item.created_at)}</span>
          </div>
        ) : null}
        <div className={clsx(
          'border px-3 py-2.5 text-left shadow-sm',
          own ? 'rounded-2xl rounded-br-md' : 'rounded-2xl rounded-bl-md',
          withdrawn ? 'border-slate-200 bg-slate-100 text-slate-500' : chatBubbleTone(item.activity_type, own),
          item.is_unread && 'ring-2 ring-blue-200',
        )}>
          {!withdrawn ? <ActivityReplyQuote replyTo={item.reply_to} /> : null}
          <div className="flex flex-wrap items-center gap-1.5">
            {item.activity_type !== 'comment' ? <span className={clsx('rounded px-2 py-0.5 text-[11px] font-semibold', activityTone(item.activity_type))}>{activityLabel(item.activity_type)}</span> : null}
            {item.mentioned_me ? <span className="rounded bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-700">@tôi</span> : null}
            {resolved ? <span className="rounded bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-700">Đã xử lý</span> : null}
            {own && !withdrawn && (item.can_edit || item.can_withdraw) ? (
              <div ref={menuRef} className="relative ml-auto">
                <button type="button" className="flex h-7 w-7 items-center justify-center rounded-full bg-white/80 text-slate-600 hover:bg-white" aria-label="Tùy chọn tin nhắn" aria-expanded={menuOpen} onClick={() => setMenuOpen((current) => !current)}>
                  {mutating ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <MoreHorizontal className="h-4 w-4" />}
                </button>
                {menuOpen ? <div className="absolute right-0 top-8 z-20 w-36 overflow-hidden rounded-md border border-line bg-white py-1 text-left shadow-lg">
                  {item.can_edit ? <button type="button" className="flex w-full items-center gap-2 px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50" disabled={mutating} onClick={() => { setMenuOpen(false); onEdit(); }}><Pencil className="h-3.5 w-3.5" />Chỉnh sửa</button> : null}
                  {item.can_withdraw ? <button type="button" className="flex w-full items-center gap-2 px-3 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-50" disabled={mutating} onClick={() => { setMenuOpen(false); onWithdraw(); }}><Trash2 className="h-3.5 w-3.5" />Thu hồi</button> : null}
                </div> : null}
              </div>
            ) : null}
          </div>
          {!withdrawn ? <ActivityTopicCard source={item} /> : null}
          {withdrawn ? <p className="text-sm italic leading-6">Tin nhắn đã được thu hồi</p> : comment ? <p className={clsx('whitespace-pre-wrap break-words text-sm leading-6', item.activity_type === 'comment' ? '' : 'mt-1.5')}>{comment}</p> : null}
          {!withdrawn && metadata.due_at ? <p className="mt-2 text-xs font-semibold">Hạn: {compactDateTime(metadata.due_at)}</p> : null}
          {!withdrawn ? <ActivityAttachments attachments={item.attachments} /> : null}
          {activityVisibilityLabel(item.visibility) || compact || (item.edited_at && !withdrawn) ? <div className={clsx('mt-2 flex flex-wrap items-center gap-2 text-[11px]', ownBlueBubble ? 'text-blue-100' : 'text-slate-500')}>
            {activityVisibilityLabel(item.visibility) ? <span>{activityVisibilityLabel(item.visibility)}</span> : null}
            {compact ? <span className={ownBlueBubble ? 'text-blue-100' : 'text-slate-400'}>{compactDateTime(item.created_at)}</span> : null}
            {item.edited_at && !withdrawn ? <span className={ownBlueBubble ? 'text-blue-100' : 'text-slate-400'}>đã chỉnh sửa</span> : null}
          </div> : null}
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {!withdrawn ? <button type="button" className="inline-flex items-center gap-1 rounded-md bg-white/80 px-2 py-1 text-xs font-semibold text-slate-600 hover:bg-white" onClick={onReply}>
              <Reply className="h-3 w-3" />
              Trả lời{item.reply_count ? ` · ${item.reply_count}` : ''}
            </button> : item.reply_count ? <span className="text-xs font-semibold text-slate-500">{item.reply_count} câu trả lời</span> : null}
            {!withdrawn && item.is_unread ? (
              <button type="button" className="rounded-md bg-white/80 px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-white disabled:opacity-50" disabled={marking} onClick={onMarkRead}>
                {marking ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : 'Đánh dấu đã đọc'}
              </button>
            ) : null}
            {!withdrawn && resolveLabel && !resolved ? (
              <button type="button" className="inline-flex items-center gap-1 rounded-md bg-white/80 px-2 py-1 text-xs font-semibold text-emerald-700 hover:bg-white disabled:opacity-50" disabled={resolving} onClick={onResolve}>
                {resolving ? <Loader2 className="h-3 w-3 animate-spin" /> : <CheckCircle2 className="h-3 w-3" />}
                {resolveLabel}
              </button>
            ) : null}
            {item.work_item_id ? <Link className={clsx('text-xs font-semibold hover:underline', ownBlueBubble ? 'text-white' : 'text-blue-700')} to={`/projects/${projectId}/work-items/${item.work_item_id}`}>Mở đầu việc ↗</Link> : null}
          </div>
          {!withdrawn ? <div className="mt-2">
            <ActivityReactionBar reactions={item.reactions} pendingReaction={pendingReaction} onToggle={onReaction} />
          </div> : null}
        </div>
      </div>
    </article>
  );
}

function FilterSelect({
  label,
  value,
  onChange,
  disabled,
  children,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  children: ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</span>
      <select
        className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700 disabled:bg-slate-100 disabled:text-slate-400"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      >
        {children}
      </select>
    </label>
  );
}

function ActivityCard({
  item,
  projectId,
  marking,
  resolving,
  onMarkRead,
  onResolve,
}: {
  item: WorkItemActivity;
  projectId: string;
  marking: boolean;
  resolving: boolean;
  onMarkRead: () => void;
  onResolve: () => void;
}) {
  const system = !['comment', 'status', 'blocker', 'question', 'decision', 'reminder', 'file_note'].includes(item.activity_type);
  const resolveLabel = activityResolveLabel(item);
  const resolved = activityResolved(item);
  const withdrawn = Boolean(item.withdrawn_at);
  const comment = workflowFormActivityComment(item);
  return (
    <article className={clsx(
      'rounded-md border px-3 py-3',
      item.is_unread ? 'border-blue-200 bg-blue-50/70' : system ? 'border-line bg-slate-50' : 'border-line bg-white',
    )}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            {item.is_unread ? <span className="rounded bg-blue-600 px-2 py-0.5 text-[11px] font-semibold text-white">Chưa đọc</span> : null}
            {item.mentioned_me ? <span className="rounded bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-700">@tôi</span> : null}
            <span className={clsx('rounded px-2 py-0.5 text-xs font-semibold', activityTone(item.activity_type))}>{activityLabel(item.activity_type)}</span>
            {resolved ? <span className="rounded bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700">Đã xử lý</span> : null}
          </div>
          <p className="mt-1.5 truncate text-xs text-slate-500">
            <span className="font-semibold text-slate-700">{identityReferenceText(item.actor_name, item.actor_account_deleted, 'Hệ thống')}</span>
            {' · '}{compactDateTime(item.created_at)}
            {item.edited_at && !withdrawn ? ' · đã chỉnh sửa' : ''}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {activityVisibilityLabel(item.visibility) ? <span className="rounded bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-500">{activityVisibilityLabel(item.visibility)}</span> : null}
          {item.is_unread ? (
            <button type="button" className="rounded-md border border-blue-200 bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" disabled={marking} onClick={onMarkRead}>
              {marking ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : 'Đã đọc'}
            </button>
          ) : null}
        </div>
      </div>
      {!withdrawn ? <ActivityTopicCard source={item} /> : null}
      {!withdrawn ? <ActivityReplyQuote replyTo={item.reply_to} /> : null}
      {withdrawn ? <p className="mt-2 text-sm italic text-slate-500">Tin nhắn đã được thu hồi</p> : comment ? <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{comment}</p> : null}
      {!withdrawn ? <ActivityAttachments attachments={item.attachments} /> : null}
      {!withdrawn && item.reactions?.length ? <div className="mt-2"><ActivityReactionBar reactions={item.reactions} /></div> : null}
      <div className="mt-2 flex flex-wrap items-center gap-2">
        {resolveLabel && !resolved ? (
          <button type="button" className="inline-flex items-center gap-1 rounded-md border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 hover:bg-emerald-100 disabled:opacity-50" disabled={resolving} onClick={onResolve}>
            {resolving ? <Loader2 className="h-3 w-3 animate-spin" /> : <CheckCircle2 className="h-3 w-3" />}
            {resolveLabel}
          </button>
        ) : null}
        {item.work_item_id ? (
          <Link className="text-xs font-semibold text-blue-700 hover:underline" to={`/projects/${projectId}/work-items/${item.work_item_id}`}>Mở đầu việc ↗</Link>
        ) : null}
      </div>
    </article>
  );
}

function ActivitySkeleton() {
  return (
    <div className="space-y-2" aria-label="Đang tải trao đổi">
      {[0, 1, 2].map((item) => (
        <div key={item} className="animate-pulse rounded-md border border-line p-3">
          <div className="h-3 w-1/3 rounded bg-slate-200" />
          <div className="mt-3 h-3 w-full rounded bg-slate-100" />
          <div className="mt-2 h-3 w-2/3 rounded bg-slate-100" />
        </div>
      ))}
    </div>
  );
}

function LiveStatusBadge({ status, compact = false }: { status: StreamStatus; compact?: boolean }) {
  const label = status === 'connected' ? 'Trực tuyến' : status === 'reconnecting' ? 'Đang nối lại' : 'Mất kết nối';
  const tone = status === 'connected' ? 'bg-emerald-50 text-emerald-700' : status === 'reconnecting' ? 'bg-amber-50 text-amber-700' : 'bg-slate-100 text-slate-600';
  const dot = status === 'connected' ? 'bg-emerald-500' : status === 'reconnecting' ? 'bg-amber-500' : 'bg-slate-400';
  return (
    <span className={clsx('inline-flex items-center justify-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-semibold', tone)} title={label}>
      {status === 'offline' ? <Unplug className="h-3 w-3" /> : <span className={clsx('h-2 w-2 rounded-full', dot)} />}
      {!compact ? label : null}
    </span>
  );
}
