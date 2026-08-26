import { useEffect, useMemo, useState } from 'react';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCheck, CircleAlert, RefreshCw, Search, Settings2 } from 'lucide-react';
import clsx from 'clsx';
import { useNavigate } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  NotificationCatalogItem,
  NotificationPage,
  NotificationPreference,
  NotificationSummary,
  UserNotification,
} from '../../lib/types/domain';
import { NotificationItem } from './NotificationItem';
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

  const invalidateNotifications = () => {
    void queryClient.invalidateQueries({ queryKey: ['notification-summary', actorID] });
    void queryClient.invalidateQueries({ queryKey: ['notifications', actorID] });
  };
  const markRead = useMutation({ mutationFn: (id: string) => put(`/me/notifications/${id}/read`), onSuccess: invalidateNotifications, onError: (error) => toast.error('Không cập nhật được trạng thái thông báo', error) });
  const markAll = useMutation({ mutationFn: () => post('/me/notifications/read-all'), onSuccess: () => { invalidateNotifications(); toast.success('Đã đánh dấu tất cả thông báo là đã đọc'); }, onError: (error) => toast.error('Không đánh dấu được tất cả thông báo', error) });
  const archive = useMutation({ mutationFn: (id: string) => put(`/me/notifications/${id}/archive`), onSuccess: invalidateNotifications, onError: (error) => toast.error('Không lưu trữ được thông báo', error) });

  const categories = useMemo(() => {
    const values = new Map<string, string>();
    for (const item of catalog.data ?? []) if (item.enabled) values.set(item.category_key, item.category_label);
    return Array.from(values, ([key, label]) => ({ key, label })).sort((left, right) => left.label.localeCompare(right.label, 'vi'));
  }, [catalog.data]);
  const rows = notifications.data?.pages.flatMap((page) => page.items) ?? [];

  async function openNotification(notification: UserNotification) {
    if (!notification.read_at) await markRead.mutateAsync(notification.id).catch(() => undefined);
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
        description={`${summary.data?.unread_total ?? 0} chưa đọc · ${summary.data?.action_required ?? 0} cần xử lý. Thông báo chỉ dẫn đến màn hình nghiệp vụ; quyền và trạng thái nguồn luôn được kiểm tra lại.`}
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
            {notifications.isLoading ? <div className="grid min-h-56 place-items-center"><LoadingSpinner /></div> : notifications.isError ? (
              <div className="p-8 text-center"><CircleAlert className="mx-auto h-6 w-6 text-rose-600" /><p className="mt-2 text-sm text-rose-700">Không tải được danh sách thông báo.</p><button type="button" className="mt-3 rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={() => void notifications.refetch()}>Thử lại</button></div>
            ) : rows.length ? rows.map((notification) => <NotificationItem key={notification.id} notification={notification} busy={(markRead.isPending && markRead.variables === notification.id) || (archive.isPending && archive.variables === notification.id)} onOpen={() => void openNotification(notification)} onArchive={applied.state === 'archived' ? undefined : () => archive.mutate(notification.id)} />) : (
              <div className="p-5"><EmptyState title={applied.state === 'unread' ? 'Anh/chị đã đọc tất cả thông báo' : applied.state === 'action_required' ? 'Không có thông báo cần xử lý' : 'Không có thông báo phù hợp bộ lọc'} /></div>
            )}
            {notifications.hasNextPage ? <div className="border-t border-line p-4 text-center"><button type="button" disabled={notifications.isFetchingNextPage} className="inline-flex items-center gap-2 rounded-md border border-line px-4 py-2 text-sm font-semibold text-slate-700 disabled:opacity-50" onClick={() => void notifications.fetchNextPage()}>{notifications.isFetchingNextPage ? <RefreshCw className="h-4 w-4 animate-spin" /> : null}Tải thêm</button></div> : null}
          </div>
        </section>
      </div>

      <NotificationPreferencesDialog open={preferencesOpen} catalog={catalog.data ?? []} onClose={() => setPreferencesOpen(false)} />
    </div>
  );
}

function NotificationPreferencesDialog({ open, catalog, onClose }: { open: boolean; catalog: NotificationCatalogItem[]; onClose: () => void }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const preferences = useQuery({ queryKey: ['notification-preferences'], queryFn: () => get<NotificationPreference[]>('/me/notifications/preferences'), enabled: open, retry: false });
  const categories = useMemo(() => {
    const values = new Map<string, { key: string; label: string; required: boolean }>();
    for (const event of catalog) {
      if (!event.enabled) continue;
      const current = values.get(event.category_key);
      values.set(event.category_key, { key: event.category_key, label: event.category_label, required: Boolean(current?.required || event.required_delivery) });
    }
    return Array.from(values.values());
  }, [catalog]);
  const [values, setValues] = useState<Record<string, boolean>>({});
  useEffect(() => {
    if (!open || !preferences.data) return;
    const next: Record<string, boolean> = {};
    for (const category of categories) {
      const preference = preferences.data.find((item) => item.category_key === category.key && item.event_key === '*');
      next[category.key] = category.required || preference?.in_app_enabled !== false;
    }
    setValues(next);
  }, [categories, open, preferences.data]);
  const save = useMutation({
    mutationFn: async () => {
      for (const category of categories) {
        await put('/me/notifications/preferences', {
          category_key: category.key,
          event_key: '*',
          in_app_enabled: category.required || values[category.key] !== false,
          email_enabled: false,
          push_enabled: false,
          digest_mode: category.required || values[category.key] !== false ? 'immediate' : 'off',
          quiet_hours_json: {},
        });
      }
    },
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['notification-preferences'] }); toast.success('Đã lưu tùy chọn thông báo'); onClose(); },
    onError: (error) => toast.error('Không lưu được tùy chọn thông báo', error),
  });

  return (
    <Dialog open={open} title="Tùy chọn thông báo" description="Thông báo cần hành động hoặc bảo mật luôn được giữ trên App. Email và push sẽ được mở khi có delivery adapter chính thức." onClose={onClose} footer={<div className="flex justify-end gap-2"><button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={onClose}>Hủy</button><button type="button" disabled={save.isPending || preferences.isLoading} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => save.mutate()}>Lưu tùy chọn</button></div>}>
      {preferences.isLoading ? <LoadingSpinner /> : preferences.isError ? <p className="text-sm text-rose-700">Không tải được tùy chọn thông báo.</p> : categories.length ? <div className="space-y-3">{categories.map((category) => <label key={category.key} className="flex items-start justify-between gap-4 rounded-lg border border-line p-4"><span><strong className="block text-sm text-ink">{category.label}</strong><span className="mt-1 block text-xs text-slate-500">{category.required ? 'Có thông báo bắt buộc cần xử lý.' : 'Có thể tắt thông báo nhận biết trên App.'}</span></span><input type="checkbox" className="mt-1 h-4 w-4" checked={category.required || values[category.key] !== false} disabled={category.required} onChange={(event) => setValues((current) => ({ ...current, [category.key]: event.target.checked }))} /></label>)}</div> : <p className="text-sm text-slate-500">Chưa có nhóm thông báo đang hoạt động.</p>}
    </Dialog>
  );
}
