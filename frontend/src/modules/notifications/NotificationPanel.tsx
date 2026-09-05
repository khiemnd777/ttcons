import { useEffect, useRef, useState } from 'react';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Bell, CheckCheck, CircleAlert, RefreshCw, X } from 'lucide-react';
import clsx from 'clsx';
import { useNavigate } from 'react-router-dom';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { NotificationPage, NotificationSummary, UserNotification } from '../../lib/types/domain';
import { useToast } from '../../components/ToastProvider';
import { NotificationItem } from './NotificationItem';
import { compactNotificationCount, safeNotificationHref } from './notificationPresentation';

type NotificationState = 'all' | 'unread' | 'action_required';

export function NotificationPanel({ summary }: { summary?: NotificationSummary }) {
  const { user } = useAuth();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [state, setState] = useState<NotificationState>('all');
  const rootRef = useRef<HTMLDivElement>(null);
  const dialogRef = useRef<HTMLElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const actorID = user?.id ?? '';
  const unreadTotal = summary?.unread_total ?? 0;

  const notifications = useInfiniteQuery({
    queryKey: ['notifications', actorID, state],
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams({ state, limit: '20' });
      if (pageParam) params.set('cursor', String(pageParam));
      return get<NotificationPage>(`/me/notifications?${params.toString()}`);
    },
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: open && Boolean(actorID),
    retry: false,
  });

  const invalidateNotifications = () => {
    void queryClient.invalidateQueries({ queryKey: ['notification-summary', actorID] });
    void queryClient.invalidateQueries({ queryKey: ['notifications', actorID] });
  };

  const markRead = useMutation({
    mutationFn: (notificationID: string) => put(`/me/notifications/${notificationID}/read`),
    onSuccess: invalidateNotifications,
    onError: (error) => toast.error('Không cập nhật được trạng thái thông báo', error),
  });
  const markAllRead = useMutation({
    mutationFn: () => post('/me/notifications/read-all'),
    onSuccess: () => {
      invalidateNotifications();
      toast.success('Đã đánh dấu tất cả thông báo là đã đọc');
    },
    onError: (error) => toast.error('Không đánh dấu được tất cả thông báo', error),
  });
  const archive = useMutation({
    mutationFn: (notificationID: string) => put(`/me/notifications/${notificationID}/archive`),
    onSuccess: invalidateNotifications,
    onError: (error) => toast.error('Không lưu trữ được thông báo', error),
  });

  const closePanel = (restoreFocus = false) => {
    setOpen(false);
    if (restoreFocus) window.requestAnimationFrame(() => triggerRef.current?.focus());
  };

  useEffect(() => {
    if (!open) return undefined;
    window.requestAnimationFrame(() => closeRef.current?.focus());
    const closeOutside = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) closePanel();
    };
    const handleKeyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        closePanel(true);
        return;
      }
      if (event.key !== 'Tab') return;
      const focusable = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>('button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])') ?? []);
      if (!focusable.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('mousedown', closeOutside);
    window.addEventListener('keydown', handleKeyboard);
    return () => {
      document.removeEventListener('mousedown', closeOutside);
      window.removeEventListener('keydown', handleKeyboard);
    };
  }, [open]);

  const rows = notifications.data?.pages.flatMap((page) => page.items) ?? [];

  async function openNotification(notification: UserNotification) {
    if (!notification.read_at) await markRead.mutateAsync(notification.id).catch(() => undefined);
    closePanel();
    navigate(safeNotificationHref(notification.action_href));
  }

  return (
    <div ref={rootRef} className="relative">
      <button ref={triggerRef} type="button" aria-label={unreadTotal ? `Thông báo, ${unreadTotal} chưa đọc` : 'Thông báo'} aria-haspopup="dialog" aria-expanded={open} className="relative grid h-10 w-10 place-items-center rounded-md border border-line text-slate-600 hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500" onClick={() => setOpen((current) => !current)}>
        <Bell className="h-5 w-5" />
        {unreadTotal > 0 ? <span className="absolute -right-1.5 -top-1.5 min-w-5 rounded-full bg-rose-600 px-1 text-center text-[10px] font-bold leading-5 text-white" aria-hidden="true">{compactNotificationCount(unreadTotal)}</span> : null}
      </button>

      {open ? (
        <section ref={dialogRef} role="dialog" aria-modal="true" aria-label="Danh sách thông báo" className="fixed inset-x-4 top-16 z-50 overflow-hidden rounded-lg border border-line bg-white shadow-2xl sm:absolute sm:inset-x-auto sm:right-0 sm:top-12 sm:w-[28rem]">
          <header className="flex items-start justify-between gap-3 border-b border-line px-4 py-3">
            <div><h2 className="font-semibold text-ink">Thông báo</h2><p className="text-xs text-slate-500">{unreadTotal ? `${unreadTotal} chưa đọc · ${summary?.action_required ?? 0} cần xử lý` : 'Không có mục chưa đọc'}</p></div>
            <div className="flex items-center gap-1">
              <button type="button" className="rounded-md px-2 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" disabled={!unreadTotal || markAllRead.isPending} onClick={() => markAllRead.mutate()}><CheckCheck className="mr-1 inline h-4 w-4" />Đọc tất cả</button>
              <button ref={closeRef} type="button" aria-label="Đóng thông báo" className="grid h-8 w-8 place-items-center rounded-md text-slate-500 hover:bg-slate-100" onClick={() => closePanel(true)}><X className="h-4 w-4" /></button>
            </div>
          </header>
          <div className="flex gap-1 overflow-x-auto border-b border-line px-3 py-2" role="tablist" aria-label="Lọc thông báo">
            {([['all', 'Tất cả'], ['action_required', `Cần xử lý ${summary?.action_required ?? 0}`], ['unread', `Chưa đọc ${unreadTotal}`]] as const).map(([key, label]) => <button key={key} type="button" role="tab" aria-selected={state === key} className={clsx('shrink-0 rounded-md px-3 py-1.5 text-xs font-semibold', state === key ? 'bg-blue-50 text-blue-700' : 'text-slate-500 hover:bg-slate-100')} onClick={() => setState(key)}>{label}</button>)}
          </div>
          <div className="max-h-[min(65vh,34rem)] overflow-y-auto">
            {notifications.isLoading ? <NotificationLoading /> : notifications.isError ? (
              <div className="p-6 text-center text-sm text-rose-700"><CircleAlert className="mx-auto mb-2 h-5 w-5" />Không tải được thông báo.<button type="button" className="mt-3 block w-full font-semibold text-blue-700" onClick={() => void notifications.refetch()}>Thử lại</button></div>
            ) : rows.length ? rows.map((notification) => (
              <NotificationItem key={notification.id} compact notification={notification} busy={(markRead.isPending && markRead.variables === notification.id) || (archive.isPending && archive.variables === notification.id)} onOpen={() => void openNotification(notification)} onArchive={() => archive.mutate(notification.id)} />
            )) : (
              <div className="p-8 text-center text-sm text-slate-500"><Bell className="mx-auto mb-2 h-6 w-6 text-slate-300" />{state === 'unread' ? 'Bạn đã đọc tất cả thông báo.' : state === 'action_required' ? 'Không có thông báo cần xử lý.' : 'Chưa có thông báo nào.'}</div>
            )}
            {notifications.hasNextPage ? <div className="border-t border-line p-3 text-center"><button type="button" className="inline-flex items-center gap-2 text-sm font-semibold text-blue-700 disabled:opacity-50" disabled={notifications.isFetchingNextPage} onClick={() => void notifications.fetchNextPage()}>{notifications.isFetchingNextPage ? <RefreshCw className="h-4 w-4 animate-spin" /> : null}Tải thêm</button></div> : null}
          </div>
          <footer className="border-t border-line bg-slate-50 p-3 text-center"><button type="button" className="text-sm font-semibold text-blue-700 hover:text-blue-800" onClick={() => { closePanel(); navigate('/notifications'); }}>Xem tất cả thông báo →</button></footer>
        </section>
      ) : null}
    </div>
  );
}

function NotificationLoading() {
  return <div className="space-y-3 p-4" aria-label="Đang tải thông báo">{[0, 1, 2].map((item) => <div key={item} className="flex animate-pulse gap-3"><span className="h-9 w-9 rounded-full bg-slate-100" /><span className="flex-1 space-y-2"><span className="block h-3 w-2/3 rounded bg-slate-100" /><span className="block h-3 w-full rounded bg-slate-100" /></span></div>)}</div>;
}
