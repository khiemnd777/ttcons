import { useMemo, useState } from 'react';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCheck, CircleAlert, RefreshCw, Search, Settings2 } from 'lucide-react';
import clsx from 'clsx';
import { useNavigate } from 'react-router-dom';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  NotificationCatalogItem,
  NotificationPage,
  NotificationSummary,
  UserNotification,
} from '../../lib/types/domain';
import { NotificationItem } from './NotificationItem';
import { NotificationPreferencesDialog } from './NotificationPreferencesDialog';
import {
  applyAllNotificationsReadCache,
  applyNotificationArchiveCache,
  applyNotificationReadCache,
  notificationListState,
} from './notificationCache';
import { safeNotificationHref } from './notificationPresentation';

type NotificationState = 'all' | 'action_required' | 'unread' | 'archived';
type NotificationFilters = { state: NotificationState; category: string; projectID: string; search: string; fromDate: string; toDate: string };
type NotificationProject = { id: string; code: string; name: string };

const emptyFilters: NotificationFilters = { state: 'all', category: '', projectID: '', search: '', fromDate: '', toDate: '' };

export function NotificationsPage() {
  const { user } = useAuth();
  const actorID = user?.id ?? '';
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [draft, setDraft] = useState<NotificationFilters>(emptyFilters);
  const [applied, setApplied] = useState<NotificationFilters>(emptyFilters);
  const [preferencesOpen, setPreferencesOpen] = useState(false);

  const summary = useQuery({ queryKey: ['notification-summary', actorID], queryFn: () => get<NotificationSummary>('/me/notifications/summary'), enabled: Boolean(actorID), retry: false });
  const catalog = useQuery({ queryKey: ['notification-catalog'], queryFn: () => get<NotificationCatalogItem[]>('/me/notifications/catalog'), retry: false, staleTime: 5 * 60_000 });
  const projects = useQuery({ queryKey: ['notification-projects', actorID], queryFn: () => get<NotificationProject[]>('/me/notifications/projects'), enabled: Boolean(actorID), retry: false });
  const notifications = useInfiniteQuery({
    queryKey: ['notifications', actorID, 'page', applied],
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams({ state: applied.state, limit: '30' });
      if (applied.category) params.set('category', applied.category);
      if (applied.projectID) params.set('project_id', applied.projectID);
      if (applied.search) params.set('search', applied.search);
      if (applied.fromDate) params.set('from_date', applied.fromDate);
      if (applied.toDate) params.set('to_date', applied.toDate);
      if (pageParam) params.set('cursor', String(pageParam));
      return get<NotificationPage>(`/me/notifications?${params.toString()}`);
    },
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: Boolean(actorID),
    retry: false,
  });

  const markRead = useMutation({
    mutationFn: (notification: UserNotification) => put<{ id: string; read_at: string }>(`/me/notifications/${notification.id}/read`),
    onSuccess: (result, notification) => applyNotificationReadCache(queryClient, actorID, notification, result.read_at),
    onError: (error) => toast.error('Không cập nhật được trạng thái thông báo', error),
  });
  const markAll = useMutation({
    mutationFn: () => post<{ updated_count: number }>('/me/notifications/read-all'),
    onSuccess: (result) => {
      applyAllNotificationsReadCache(queryClient, actorID, new Date().toISOString(), result.updated_count);
      toast.success('Đã đánh dấu tất cả thông báo là đã đọc');
    },
    onError: (error) => toast.error('Không đánh dấu được tất cả thông báo', error),
  });
  const archive = useMutation({
    mutationFn: (notification: UserNotification) => put<{ id: string; archived_at: string; read_at: string }>(`/me/notifications/${notification.id}/archive`),
    onSuccess: (_result, notification) => applyNotificationArchiveCache(queryClient, actorID, notification),
    onError: (error) => toast.error('Không lưu trữ được thông báo', error),
  });

  const categories = useMemo(() => {
    const values = new Map<string, string>();
    for (const item of catalog.data ?? []) if (item.enabled) values.set(item.category_key, item.category_label);
    return Array.from(values, ([key, label]) => ({ key, label })).sort((left, right) => left.label.localeCompare(right.label, 'vi'));
  }, [catalog.data]);
  const rows = notifications.data?.pages.flatMap((page) => page.items) ?? [];
  const listState = notificationListState(notifications.isLoading, notifications.isError, Boolean(notifications.data));

  async function openNotification(notification: UserNotification) {
    if (!notification.read_at) await markRead.mutateAsync(notification).catch(() => undefined);
    navigate(safeNotificationHref(notification.action_href));
  }

  const changeState = (state: NotificationState) => {
    const next = { ...draft, state };
    setDraft(next);
    setApplied(next);
  };

  return (
    <div className="p-4 sm:p-6">
      <PageHeader
        title="Thông báo"
        description={`${summary.data?.unread_total ?? 0} chưa đọc · ${summary.data?.action_required ?? 0} cần xử lý`}
        actions={<><button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => setPreferencesOpen(true)}><Settings2 className="h-4 w-4" />Tùy chọn</button><button type="button" disabled={!summary.data?.unread_total || markAll.isPending} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={() => markAll.mutate()}><CheckCheck className="h-4 w-4" />Đọc tất cả</button></>}
      />

      <div className="grid gap-4 xl:grid-cols-[220px_minmax(0,1fr)]">
        <aside className="h-fit rounded-lg border border-line bg-white p-2 xl:sticky xl:top-20">
          {([
            ['action_required', 'Cần xử lý', summary.data?.action_required ?? 0],
            ['unread', 'Chưa đọc', summary.data?.unread_total ?? 0],
            ['all', 'Tất cả', null],
            ['archived', 'Đã lưu trữ', null],
          ] as const).map(([key, label, count]) => <button key={key} type="button" className={clsx('flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm font-semibold', applied.state === key ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-50')} onClick={() => changeState(key)}><span>{label}</span>{count !== null ? <span className="rounded-full bg-white px-2 py-0.5 text-xs">{count}</span> : null}</button>)}
          <div className="mt-2 border-t border-line pt-2">
            {categories.map((category) => <button key={category.key} type="button" className={clsx('flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm', applied.category === category.key ? 'bg-slate-100 font-semibold text-ink' : 'text-slate-600 hover:bg-slate-50')} onClick={() => { const next = { ...draft, category: applied.category === category.key ? '' : category.key }; setDraft(next); setApplied(next); }}><span>{category.label}</span><span className="text-xs text-slate-400">{summary.data?.by_category?.[category.key] ?? 0}</span></button>)}
          </div>
        </aside>

        <section className="min-w-0 space-y-4">
          <form className="rounded-lg border border-line bg-white p-4" onSubmit={(event) => { event.preventDefault(); setApplied({ ...draft, search: draft.search.trim() }); }}>
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(220px,1.3fr)_minmax(180px,1fr)_minmax(150px,.7fr)_minmax(150px,.7fr)]">
              <label className="block"><span className="text-xs font-semibold text-slate-600">Tìm kiếm</span><span className="relative mt-1 block"><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input value={draft.search} onChange={(event) => setDraft((current) => ({ ...current, search: event.target.value }))} className="h-10 w-full rounded-md border border-line pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" placeholder="Tiêu đề hoặc nội dung..." /></span></label>
              <label className="block"><span className="text-xs font-semibold text-slate-600">Công trình</span><select value={draft.projectID} onChange={(event) => setDraft((current) => ({ ...current, projectID: event.target.value }))} className="mt-1 h-10 w-full rounded-md border border-line bg-white px-3 text-sm"><option value="">Tất cả công trình</option>{projects.data?.map((project) => <option key={project.id} value={project.id}>{project.code} · {project.name}</option>)}</select></label>
              <label className="block"><span className="text-xs font-semibold text-slate-600">Từ ngày</span><input type="date" value={draft.fromDate} onChange={(event) => setDraft((current) => ({ ...current, fromDate: event.target.value }))} className="mt-1 h-10 w-full rounded-md border border-line px-3 text-sm" /></label>
              <label className="block"><span className="text-xs font-semibold text-slate-600">Đến ngày</span><input type="date" value={draft.toDate} onChange={(event) => setDraft((current) => ({ ...current, toDate: event.target.value }))} className="mt-1 h-10 w-full rounded-md border border-line px-3 text-sm" /></label>
            </div>
            <div className="mt-3 flex justify-end gap-2"><button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-50" onClick={() => { setDraft(emptyFilters); setApplied(emptyFilters); }}>Đặt lại</button><button type="submit" className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700">Áp dụng</button></div>
          </form>

          <div className="overflow-hidden rounded-lg border border-line bg-white">
            {listState === 'stale-error' ? <div className="flex flex-wrap items-center justify-between gap-2 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800" role="alert"><span>Chưa đồng bộ được dữ liệu mới nhất. Danh sách gần nhất vẫn được giữ.</span><button type="button" className="font-semibold underline" onClick={() => void notifications.refetch()}>Thử lại</button></div> : null}
            {listState === 'loading' ? <div className="grid min-h-56 place-items-center"><LoadingSpinner /></div> : listState === 'initial-error' ? (
              <div className="p-8 text-center"><CircleAlert className="mx-auto h-6 w-6 text-rose-600" /><p className="mt-2 text-sm text-rose-700">Không tải được danh sách thông báo.</p><button type="button" className="mt-3 rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={() => void notifications.refetch()}>Thử lại</button></div>
            ) : rows.length ? rows.map((notification) => <NotificationItem key={notification.id} notification={notification} busy={(markRead.isPending && markRead.variables?.id === notification.id) || (archive.isPending && archive.variables?.id === notification.id)} onOpen={() => void openNotification(notification)} onArchive={applied.state === 'archived' ? undefined : () => archive.mutate(notification)} />) : (
              <div className="p-5"><EmptyState title={applied.state === 'unread' ? 'Bạn đã đọc tất cả thông báo' : applied.state === 'action_required' ? 'Không có thông báo cần xử lý' : 'Không có thông báo phù hợp bộ lọc'} /></div>
            )}
            {listState !== 'initial-error' && notifications.hasNextPage ? <div className="border-t border-line p-4 text-center"><button type="button" disabled={notifications.isFetchingNextPage} className="inline-flex items-center gap-2 rounded-md border border-line px-4 py-2 text-sm font-semibold text-slate-700 disabled:opacity-50" onClick={() => void notifications.fetchNextPage()}>{notifications.isFetchingNextPage ? <RefreshCw className="h-4 w-4 animate-spin" /> : null}Tải thêm</button></div> : null}
          </div>
        </section>
      </div>

      <NotificationPreferencesDialog open={preferencesOpen} catalog={catalog} onClose={() => setPreferencesOpen(false)} />
    </div>
  );
}
