import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, CheckCircle2, RefreshCw } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { get, post, streamSSE, type StreamStatus } from '../../lib/api/client';
import { useToast } from '../../components/ToastProvider';
import type { ActivityPayload, WorkItemActivity } from '../../lib/types/domain';
import { activityLabel, compactDateTime, scopeTypeLabel, visibilityLabel } from './workItemPresentation';
import { ActivityAttachments } from './ActivityAttachments';
import { ActivityComposer, type ActivityAttachmentOption, type ActivityTargetOption } from './ActivityComposer';

const filterScopes = [
  { value: '', label: 'All scopes' },
  { value: 'project', label: scopeTypeLabel('project') },
  { value: 'work_item', label: scopeTypeLabel('work_item') },
  { value: 'boq_item', label: scopeTypeLabel('boq_item') },
  { value: 'procurement_request', label: scopeTypeLabel('procurement_request') },
  { value: 'stock_movement', label: scopeTypeLabel('stock_movement') },
  { value: 'acceptance', label: scopeTypeLabel('acceptance') },
  { value: 'rework', label: scopeTypeLabel('rework') },
  { value: 'handover', label: scopeTypeLabel('handover') },
  { value: 'payable', label: scopeTypeLabel('payable') },
  { value: 'receivable', label: scopeTypeLabel('receivable') },
  { value: 'payment', label: scopeTypeLabel('payment') },
];

export function ActivityThread({
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
  const queryClient = useQueryClient();
  const [scopeFilter, setScopeFilter] = useState('');
  const [scopeIdFilter, setScopeIdFilter] = useState('');
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('reconnecting');
  const onChangedRef = useRef(onChanged);

  const queryPath = useMemo(() => {
    const params = new URLSearchParams();
    if (scopeFilter) params.set('scope_type', scopeFilter);
    if (scopeIdFilter.trim()) params.set('scope_id', scopeIdFilter.trim());
    params.set('limit', '80');
    return `/projects/${projectId}/activity?${params.toString()}`;
  }, [projectId, scopeFilter, scopeIdFilter]);

  const activity = useQuery({
    queryKey: ['project-activity', projectId, scopeFilter, scopeIdFilter],
    queryFn: () => get<WorkItemActivity[]>(queryPath),
    enabled: Boolean(projectId),
    retry: false,
  });

  const addActivity = useMutation({
    mutationFn: (payload: ActivityPayload) => post<WorkItemActivity>(`/projects/${projectId}/activity`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-activity', projectId] });
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      onChanged?.();
      toast.success('Đã gửi activity');
    },
    onError: (error) => toast.error('Không gửi được activity', error),
  });

  const markRead = useMutation({
    mutationFn: (activityId: string) => post(`/activities/${activityId}/read`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-activity', projectId] });
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
    },
    onError: (error) => toast.error('Không cập nhật được read state', error),
  });

  const rows = activity.data ?? [];

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

  return (
    <div className="space-y-2">
      <ActivityComposer
        mode="project"
        allowTarget
        targetOptions={targetOptions}
        attachmentOptions={attachmentOptions}
        submitting={addActivity.isPending}
        onSubmit={(payload) => addActivity.mutateAsync(payload).then(() => undefined)}
      />

      <div className="grid gap-2 sm:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)_auto_auto]">
        <select
          className="rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
          value={scopeFilter}
          onChange={(event) => {
            setScopeFilter(event.target.value);
            setScopeIdFilter('');
          }}
        >
          {filterScopes.map((scope) => <option key={scope.value || 'all'} value={scope.value}>{scope.label}</option>)}
        </select>
        <input
          className={clsx('rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500', !scopeFilter && 'opacity-60')}
          disabled={!scopeFilter}
          value={scopeIdFilter}
          onChange={(event) => setScopeIdFilter(event.target.value)}
          placeholder={scopeFilter ? 'Filter target ID' : 'Target filter disabled'}
        />
        <button
          className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          onClick={() => {
            void activity.refetch();
          }}
        >
          <RefreshCw className="h-4 w-4" />
        </button>
        <LiveStatusBadge status={streamStatus} />
      </div>

      {activity.isLoading ? <LoadingSpinner /> : null}
      {activity.error ? (
        <div className="rounded-md border border-rose-200 bg-rose-50 p-3">
          <div className="flex items-start gap-2">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
            <p className="text-sm text-rose-700">Không tải được activity thread.</p>
          </div>
        </div>
      ) : null}
      {!activity.isLoading && !activity.error && !rows.length ? <EmptyState title="Chưa có activity trong scope này" /> : null}

      {rows.length ? (
        <div className="space-y-2">
          {rows.map((item) => (
            <div key={item.id} className={clsx('rounded-md border px-3 py-2', item.is_unread ? 'border-blue-200 bg-blue-50' : 'border-line')}>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex min-w-0 flex-wrap items-center gap-1">
                    {item.is_unread ? <span className="rounded-md bg-blue-600 px-2 py-0.5 text-xs font-semibold text-white">Unread</span> : null}
                    {item.mentioned_me ? <span className="rounded-md bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-700">@you</span> : null}
                    <p className="text-sm font-semibold text-ink">{activityLabel(item.activity_type)}</p>
                  </div>
                  <p className="mt-1 line-clamp-1 text-xs text-slate-500">
                    {item.actor_name ?? 'System'} · {item.target_label ?? item.work_item_title ?? (item.scope_type ? scopeTypeLabel(item.scope_type) : 'Công trình')} · {compactDateTime(item.created_at)}
                  </p>
                </div>
                <div className="flex shrink-0 flex-wrap justify-end gap-1">
                  {item.scope_type ? <span className="rounded-md bg-blue-50 px-2 py-0.5 text-xs font-semibold text-blue-700">{scopeTypeLabel(item.scope_type)}</span> : null}
                  {item.visibility ? <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{visibilityLabel(item.visibility)}</span> : null}
                  {item.is_unread ? (
                    <button
                      className="inline-flex items-center gap-1 rounded-md border border-blue-200 bg-white px-2 py-0.5 text-xs font-semibold text-blue-700 hover:bg-blue-50"
                      disabled={markRead.isPending}
                      onClick={() => markRead.mutate(item.id)}
                    >
                      <CheckCircle2 className="h-3 w-3" />
                      Read
                    </button>
                  ) : null}
                </div>
              </div>
              {item.comment ? <p className="mt-2 line-clamp-3 text-sm text-slate-700">{item.comment}</p> : null}
              <ActivityAttachments attachments={item.attachments} />
            </div>
          ))}
        </div>
      ) : null}
    </div>
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
