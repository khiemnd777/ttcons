import { useState } from 'react';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import { IdentityReferenceLabel } from '../../components/IdentityReferenceLabel';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { NotificationPage, ProjectActivityFeed, ProjectUpdateKind, ProjectUpdateSummary, UserNotification, WorkItemActivity } from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';
import { NotificationItem } from '../notifications/NotificationItem';
import { safeNotificationHref } from '../notifications/notificationPresentation';
import { ActivityAttachments } from '../work-items/ActivityAttachments';
import { ActivityReplyQuote } from '../work-items/ActivityConversationMeta';
import { activityLabel, compactDateTime, workflowFormActivityComment } from '../work-items/workItemPresentation';
import { projectActivityReadBoundary, projectUpdateCount, projectUpdateGroups, type ProjectUpdateIdentity } from './projectUpdates';

export function ProjectUpdatesDialog({ project, initialKind, summary, onClose }: {
  project: ProjectUpdateIdentity;
  initialKind: ProjectUpdateKind;
  summary?: ProjectUpdateSummary;
  onClose: () => void;
}) {
  const { user } = useAuth();
  const actorID = user?.id ?? '';
  const [kind, setKind] = useState(initialKind);
  const [unreadOnly, setUnreadOnly] = useState(Boolean(summary && summary[projectUpdateGroups.find((group) => group.kind === initialKind)!.count] > 0));
  const [transitionsOnly, setTransitionsOnly] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const navigate = useNavigate();
  const notifications = useInfiniteQuery({
    queryKey: ['project-update-feed', actorID, project.id, 'notification', unreadOnly],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<NotificationPage>(`/me/notifications?${new URLSearchParams({ project_id: project.id, state: unreadOnly ? 'unread' : 'all', limit: '30', cursor: pageParam })}`),
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: Boolean(actorID && kind === 'notification'),
    retry: false,
    refetchInterval: 60_000,
  });
  const activity = useInfiniteQuery({
    queryKey: ['project-update-feed', actorID, project.id, kind, unreadOnly, transitionsOnly],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<ProjectActivityFeed>(`/projects/${project.id}/activity?${new URLSearchParams({ kind, attention: unreadOnly ? `${kind}_unread` : '', transitions_only: String(transitionsOnly), limit: '30', cursor: pageParam })}`),
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: Boolean(actorID && kind !== 'notification'),
    retry: false,
    refetchInterval: 60_000,
  });
  const feed = kind === 'notification' ? notifications : activity;
  // A failed re-authorization must not leave a previously visible feed on screen.
  const messages = activity.isError ? [] : activity.data?.pages.flatMap((page) => page.items) ?? [];
  const notices = notifications.isError ? [] : notifications.data?.pages.flatMap((page) => page.items) ?? [];
  const readBoundary = projectActivityReadBoundary(messages, Boolean(activity.hasNextPage), transitionsOnly);
  const hasUnread = kind === 'notification' ? notices.some((item) => !item.read_at)
    : messages.some((item) => kind === 'discussion' ? item.is_discussion_unread : item.is_system_unread);
  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['project-updates', actorID] }),
      queryClient.invalidateQueries({ queryKey: ['project-update-feed', actorID, project.id] }),
      queryClient.invalidateQueries({ queryKey: ['notification-summary', actorID] }),
      queryClient.invalidateQueries({ queryKey: ['notifications', actorID] }),
      queryClient.invalidateQueries({ queryKey: ['project-discussions'] }),
      queryClient.invalidateQueries({ queryKey: ['utility-project-activity', project.id] }),
      queryClient.invalidateQueries({ queryKey: ['project-activity', project.id] }),
    ]);
  };
  const markRead = useMutation({
    mutationFn: async (selection: { kind: ProjectUpdateKind; activityID?: string; notificationIDs: string[] }) => {
      if (selection.kind === 'notification') {
        // Exact displayed IDs; never the global read-all endpoint.
        for (const id of selection.notificationIDs) await put(`/me/notifications/${id}/read`);
      } else if (selection.activityID) {
        const path = selection.kind === 'system' ? 'system-read-through' : 'read-through';
        await post(`/projects/${project.id}/activity/${path}`, { activity_id: selection.activityID });
      }
    },
    onSuccess: async () => { await invalidate(); toast.success('Đã cập nhật trạng thái đã đọc'); },
    onError: (error) => { void invalidate(); toast.error('Không cập nhật được trạng thái đã đọc', error); },
  });
  const openNotification = async (notification: UserNotification) => {
    if (!notification.read_at) {
      try { await markRead.mutateAsync({ kind: 'notification', notificationIDs: [notification.id] }); }
      catch { return; }
    }
    onClose();
    navigate(safeNotificationHref(notification.action_href));
  };
  const rows = kind === 'notification' ? notices : messages;
  const busy = feed.isFetching || markRead.isPending;
  const canRead = hasUnread && !busy && !feed.isError && (kind === 'notification' || Boolean(readBoundary));
  return (
    <Dialog open title={`Cập nhật · ${project.code} — ${project.name}`} size="lg" manageFocus onClose={onClose}
      footer={<div className="flex flex-wrap items-center justify-between gap-3">
        <Link className="text-sm font-semibold text-blue-700 focus-ring" to={`/projects/${project.id}?tab=activity`} onClick={onClose}>Mở trao đổi đầy đủ</Link>
        <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50 focus-ring" disabled={!canRead} onClick={() => markRead.mutate({ kind, activityID: readBoundary, notificationIDs: notices.filter((item) => !item.read_at).map((item) => item.id) })}>{markRead.isPending ? 'Đang đánh dấu…' : 'Đánh dấu phần đang xem đã đọc'}</button>
      </div>}>
      <div className="flex flex-wrap gap-2" role="group" aria-label="Nhóm cập nhật">
        {projectUpdateGroups.map((group) => <button key={group.kind} type="button" aria-pressed={kind === group.kind} disabled={markRead.isPending} className={`rounded-md border px-3 py-2 text-sm font-semibold focus-ring ${kind === group.kind ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-line text-slate-600'}`} onClick={() => { setKind(group.kind); setTransitionsOnly(false); }}>{group.label} {summary ? projectUpdateCount(summary[group.count]) : '…'}</button>)}
      </div>
      <div className="my-4 flex flex-wrap items-center justify-between gap-3 text-sm">
        <label className="flex items-center gap-2"><input type="checkbox" checked={unreadOnly} disabled={markRead.isPending} onChange={(event) => setUnreadOnly(event.target.checked)} />Chưa xem</label>
        {kind === 'system' ? <label className="flex items-center gap-2">Loại hoạt động<select className="rounded-md border border-line bg-white px-2 py-1.5" value={transitionsOnly ? 'transitions' : 'all'} disabled={markRead.isPending} onChange={(event) => setTransitionsOnly(event.target.value === 'transitions')}><option value="all">Tất cả</option><option value="transitions">Chuyển trạng thái</option></select></label> : null}
      </div>
      {feed.isLoading ? <p className="p-6 text-center text-sm text-slate-500" role="status">Đang tải cập nhật…</p> : null}
      {feed.isError ? <div className="mb-3 rounded-md bg-amber-50 p-3 text-sm text-amber-800" role="alert">Chưa tải được cập nhật. <button type="button" className="font-semibold underline" onClick={() => void feed.refetch()}>Thử lại</button></div> : null}
      {!feed.isLoading && !feed.isError && !rows.length ? <div className="py-8 text-center text-sm text-slate-500"><p>{unreadOnly ? 'Không có cập nhật mới' : 'Chưa có nội dung trong nhóm này'}</p>{unreadOnly ? <button type="button" className="mt-3 font-semibold text-blue-700" onClick={() => setUnreadOnly(false)}>Xem lịch sử</button> : null}</div> : null}
      <div className="overflow-hidden rounded-md border border-line">
        {kind === 'notification' ? notices.map((item) => <NotificationItem key={item.id} notification={item} busy={markRead.isPending} compact onOpen={() => void openNotification(item)} />)
          : messages.map((item) => <ProjectUpdateActivity key={item.id} item={item} projectID={project.id} unread={Boolean(kind === 'discussion' ? item.is_discussion_unread : item.is_system_unread)} onOpen={onClose} />)}
      </div>
      {feed.hasNextPage ? <button type="button" disabled={busy} className="mt-4 w-full rounded-md border border-line py-2 text-sm font-semibold text-blue-700 disabled:opacity-50" onClick={() => void feed.fetchNextPage()}>{feed.isFetchingNextPage ? 'Đang tải…' : 'Tải thêm'}</button> : null}
      {kind !== 'notification' && hasUnread && !readBoundary ? <p className="mt-3 text-xs text-slate-500">{transitionsOnly ? 'Chọn “Tất cả” để đánh dấu nhóm đã đọc.' : 'Tải hết nội dung của nhóm đang xem trước khi đánh dấu đã đọc.'}</p> : null}
    </Dialog>
  );
}

function ProjectUpdateActivity({ item, projectID, unread, onOpen }: { item: WorkItemActivity; projectID: string; unread: boolean; onOpen: () => void }) {
  const href = item.work_item_id ? `/projects/${projectID}/work-items/${item.work_item_id}` : `/projects/${projectID}?tab=activity`;
  return <article className={`border-b border-line p-4 last:border-0 ${unread ? 'bg-blue-50/60' : ''}`}>
    <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
      {unread ? <span className="h-2 w-2 rounded-full bg-blue-600" aria-label="Chưa xem" /> : null}
      <IdentityReferenceLabel name={item.actor_name || 'Hệ thống'} deleted={item.actor_account_deleted} />
      <span>· {compactDateTime(item.created_at)}</span>
    </div>
    <ActivityReplyQuote replyTo={item.reply_to} />
    <p className="text-sm font-semibold text-ink">{item.work_item_title || item.target_label || activityLabel(item.activity_type)}</p>
    {item.from_status && item.to_status && item.from_status !== item.to_status ? <p className="mt-1 text-sm text-blue-700">{statusLabel(item.from_status)} → {statusLabel(item.to_status)}</p> : null}
    <p className="mt-1 whitespace-pre-wrap break-words text-sm text-slate-600">{item.withdrawn_at ? 'Tin nhắn đã được thu hồi' : workflowFormActivityComment(item) || activityLabel(item.activity_type)}</p>
    {!item.withdrawn_at ? <ActivityAttachments attachments={item.attachments} compact /> : null}
    <Link className="mt-2 inline-block text-xs font-semibold text-blue-700 focus-ring" to={href} onClick={onOpen}>{item.work_item_id ? 'Mở công việc' : 'Mở trao đổi'}</Link>
  </article>;
}
