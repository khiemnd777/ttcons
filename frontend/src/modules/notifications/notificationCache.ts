import type { InfiniteData, QueryClient, QueryKey } from '@tanstack/react-query';
import type { NotificationPage, NotificationSummary, UserNotification } from '../../lib/types/domain';

type NotificationListState = 'loading' | 'initial-error' | 'stale-error' | 'ready';
type NotificationQueryState = 'all' | 'unread' | 'action_required' | 'archived';

export const NOTIFICATION_REFRESH_DEBOUNCE_MS = 750;

export function notificationListState(isLoading: boolean, isError: boolean, hasData: boolean): NotificationListState {
  if (isLoading) return 'loading';
  if (isError && !hasData) return 'initial-error';
  if (isError) return 'stale-error';
  return 'ready';
}

function notificationQueryState(queryKey: QueryKey): NotificationQueryState | undefined {
  const directState = queryKey[2];
  if (directState === 'all' || directState === 'unread' || directState === 'action_required' || directState === 'archived') return directState;
  if (directState !== 'page' || !queryKey[3] || typeof queryKey[3] !== 'object') return undefined;
  const state = (queryKey[3] as { state?: unknown }).state;
  return state === 'all' || state === 'unread' || state === 'action_required' || state === 'archived' ? state : undefined;
}

function updateNotificationQueries(
  queryClient: QueryClient,
  actorID: string,
  update: (current: InfiniteData<NotificationPage>, queryKey: QueryKey) => InfiniteData<NotificationPage>,
) {
  const snapshots = queryClient.getQueriesData<InfiniteData<NotificationPage>>({ queryKey: ['notifications', actorID] });
  for (const [queryKey, current] of snapshots) {
    if (!current) continue;
    queryClient.setQueryData(queryKey, update(current, queryKey));
  }
}

function updateSummary(
  queryClient: QueryClient,
  actorID: string,
  update: (current: NotificationSummary) => NotificationSummary,
) {
  queryClient.setQueryData<NotificationSummary>(['notification-summary', actorID], (current) => current ? update(current) : current);
}

function decrementUnreadSummary(current: NotificationSummary, notification: UserNotification, removeActionRequired: boolean): NotificationSummary {
  const wasUnread = !notification.read_at;
  const wasActionRequired = notification.intent === 'action_required' && !notification.resolved_at;
  const byCategory = { ...current.by_category };
  if (wasUnread) byCategory[notification.category_key] = Math.max(0, (byCategory[notification.category_key] ?? 0) - 1);
  return {
    ...current,
    unread_total: wasUnread ? Math.max(0, current.unread_total - 1) : current.unread_total,
    action_required: removeActionRequired && wasActionRequired ? Math.max(0, current.action_required - 1) : current.action_required,
    by_category: byCategory,
    actor_revision: current.actor_revision + 1,
  };
}

function markNotificationQueriesStale(queryClient: QueryClient, actorID: string) {
  void queryClient.invalidateQueries({ queryKey: ['notifications', actorID], refetchType: 'none' });
  void queryClient.invalidateQueries({ queryKey: ['notification-projects', actorID], refetchType: 'none' });
}

export function applyNotificationReadCache(
  queryClient: QueryClient,
  actorID: string,
  notification: UserNotification,
  readAt: string,
) {
  let becameRead = false;
  updateNotificationQueries(queryClient, actorID, (current, queryKey) => {
    const state = notificationQueryState(queryKey);
    const pages = current.pages.map((page) => {
      const target = page.items.find((item) => item.id === notification.id);
      if (target && !target.read_at) becameRead = true;
      return {
        ...page,
        items: state === 'unread'
          ? page.items.filter((item) => item.id !== notification.id)
          : page.items.map((item) => item.id === notification.id ? { ...item, read_at: readAt } : item),
      };
    });
    return { ...current, pages };
  });
  if (becameRead) updateSummary(queryClient, actorID, (current) => decrementUnreadSummary(current, notification, false));
  markNotificationQueriesStale(queryClient, actorID);
}

export function applyNotificationArchiveCache(
  queryClient: QueryClient,
  actorID: string,
  notification: UserNotification,
) {
  let removed = false;
  updateNotificationQueries(queryClient, actorID, (current, queryKey) => {
    if (notificationQueryState(queryKey) === 'archived') return current;
    const pages = current.pages.map((page) => {
      if (page.items.some((item) => item.id === notification.id)) removed = true;
      return { ...page, items: page.items.filter((item) => item.id !== notification.id) };
    });
    return { ...current, pages };
  });
  if (removed) updateSummary(queryClient, actorID, (current) => decrementUnreadSummary(current, notification, true));
  markNotificationQueriesStale(queryClient, actorID);
}

export function applyAllNotificationsReadCache(queryClient: QueryClient, actorID: string, readAt: string, updatedCount: number) {
  updateNotificationQueries(queryClient, actorID, (current, queryKey) => {
    const state = notificationQueryState(queryKey);
    if (state === 'archived') return current;
    const pages = current.pages.map((page) => ({
      ...page,
      items: state === 'unread' ? [] : page.items.map((item) => ({ ...item, read_at: item.read_at ?? readAt })),
      ...(state === 'unread' ? { has_more: false, next_cursor: undefined } : {}),
    }));
    return { ...current, pages };
  });
  if (updatedCount > 0) {
    updateSummary(queryClient, actorID, (current) => ({
      ...current,
      unread_total: 0,
      by_category: Object.fromEntries(Object.keys(current.by_category).map((key) => [key, 0])),
      actor_revision: current.actor_revision + 1,
    }));
  }
  markNotificationQueriesStale(queryClient, actorID);
}

type NotificationTimer = ReturnType<typeof globalThis.setTimeout>;
type NotificationTimerAdapter = {
  set: (callback: () => void, delayMs: number) => NotificationTimer;
  clear: (timer: NotificationTimer) => void;
};

const defaultTimerAdapter: NotificationTimerAdapter = {
  set: (callback, delayMs) => globalThis.setTimeout(callback, delayMs),
  clear: (timer) => globalThis.clearTimeout(timer),
};

export function createNotificationInvalidationScheduler(
  invalidate: () => void,
  delayMs = NOTIFICATION_REFRESH_DEBOUNCE_MS,
  timers: NotificationTimerAdapter = defaultTimerAdapter,
) {
  let timer: NotificationTimer | undefined;
  return {
    schedule() {
      if (timer !== undefined) timers.clear(timer);
      timer = timers.set(() => {
        timer = undefined;
        invalidate();
      }, delayMs);
    },
    cancel() {
      if (timer !== undefined) timers.clear(timer);
      timer = undefined;
    },
  };
}
