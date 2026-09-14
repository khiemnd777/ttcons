import assert from 'node:assert/strict';
import test from 'node:test';
import { QueryClient } from '@tanstack/react-query';
import type { InfiniteData } from '@tanstack/react-query';
import type { NotificationPage, NotificationSummary, UserNotification } from '../src/lib/types/domain.ts';
import {
  applyAllNotificationsReadCache,
  applyNotificationArchiveCache,
  applyNotificationReadCache,
  createNotificationInvalidationScheduler,
  notificationListState,
} from '../src/modules/notifications/notificationCache.ts';

const actorID = 'actor-a';

function notification(id: string, overrides: Partial<UserNotification> = {}): UserNotification {
  return {
    id,
    event_key: 'work_item.review_requested',
    notification_type: 'work_item.review_requested',
    category_key: 'workflow',
    category_label: 'Quy trình',
    intent: 'action_required',
    severity: 'warning',
    icon_key: 'approval',
    tone_key: 'warning',
    source_kind: 'work_item',
    source_id: 'source-a',
    source_event_id: 'event-a',
    project_id: 'project-a',
    project_code: 'P-001',
    project_name: 'Công trình A',
    title_snapshot: 'Yêu cầu xử lý',
    message_snapshot: 'Nội dung',
    action_key: 'open_work_item',
    action_label: 'Mở công việc',
    action_href: '/my-work?task=source-a',
    read_at: null,
    resolved_at: null,
    archived_at: null,
    created_at: '2026-09-10T05:00:00Z',
    ...overrides,
  };
}

function page(items: UserNotification[]): InfiniteData<NotificationPage> {
  return { pages: [{ items, has_more: false }], pageParams: [''] };
}

function summary(): NotificationSummary {
  return { unread_total: 2, action_required: 1, by_category: { workflow: 2 }, actor_revision: 7 };
}

test('notification list keeps cached data visible when only a background refresh fails', () => {
  assert.equal(notificationListState(true, false, false), 'loading');
  assert.equal(notificationListState(false, true, false), 'initial-error');
  assert.equal(notificationListState(false, true, true), 'stale-error');
  assert.equal(notificationListState(false, false, true), 'ready');
});

test('archive removes the row from every actor list cache without a refetch and updates summary once', () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const target = notification('notice-a');
  const remaining = notification('notice-b', { intent: 'awareness' });
  const allKey = ['notifications', actorID, 'all'];
  const unreadKey = ['notifications', actorID, 'page', { state: 'unread', category: '', projectID: '', search: '', fromDate: '', toDate: '' }];
  const archivedKey = ['notifications', actorID, 'page', { state: 'archived', category: '', projectID: '', search: '', fromDate: '', toDate: '' }];
  client.setQueryData(allKey, page([target, remaining]));
  client.setQueryData(unreadKey, page([target, remaining]));
  client.setQueryData(archivedKey, page([]));
  client.setQueryData(['notification-summary', actorID], summary());

  applyNotificationArchiveCache(client, actorID, target);

  assert.deepEqual(client.getQueryData<InfiniteData<NotificationPage>>(allKey)?.pages[0].items.map((item) => item.id), ['notice-b']);
  assert.deepEqual(client.getQueryData<InfiniteData<NotificationPage>>(unreadKey)?.pages[0].items.map((item) => item.id), ['notice-b']);
  assert.deepEqual(client.getQueryData<InfiniteData<NotificationPage>>(archivedKey)?.pages[0].items, []);
  assert.deepEqual(client.getQueryData<NotificationSummary>(['notification-summary', actorID]), {
    unread_total: 1,
    action_required: 0,
    by_category: { workflow: 1 },
    actor_revision: 8,
  });
  assert.equal(client.getQueryState(allKey)?.isInvalidated, true);
  client.clear();
});

test('read transitions remove only unread-list rows and preserve unresolved action count', () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const target = notification('notice-a');
  const allKey = ['notifications', actorID, 'all'];
  const unreadKey = ['notifications', actorID, 'unread'];
  const actionKey = ['notifications', actorID, 'action_required'];
  for (const key of [allKey, unreadKey, actionKey]) client.setQueryData(key, page([target]));
  client.setQueryData(['notification-summary', actorID], summary());

  applyNotificationReadCache(client, actorID, target, '2026-09-10T05:01:00Z');

  assert.equal(client.getQueryData<InfiniteData<NotificationPage>>(unreadKey)?.pages[0].items.length, 0);
  assert.equal(client.getQueryData<InfiniteData<NotificationPage>>(allKey)?.pages[0].items[0].read_at, '2026-09-10T05:01:00Z');
  assert.equal(client.getQueryData<InfiniteData<NotificationPage>>(actionKey)?.pages[0].items[0].read_at, '2026-09-10T05:01:00Z');
  assert.deepEqual(client.getQueryData<NotificationSummary>(['notification-summary', actorID]), {
    unread_total: 1,
    action_required: 1,
    by_category: { workflow: 1 },
    actor_revision: 8,
  });
  client.clear();
});

test('read-all empties unread caches while keeping action-required rows actionable', () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const target = notification('notice-a');
  const unreadKey = ['notifications', actorID, 'unread'];
  const actionKey = ['notifications', actorID, 'action_required'];
  client.setQueryData(unreadKey, page([target]));
  client.setQueryData(actionKey, page([target]));
  client.setQueryData(['notification-summary', actorID], summary());

  applyAllNotificationsReadCache(client, actorID, '2026-09-10T05:02:00Z', 2);

  assert.deepEqual(client.getQueryData<InfiniteData<NotificationPage>>(unreadKey)?.pages[0], { items: [], has_more: false, next_cursor: undefined });
  assert.equal(client.getQueryData<InfiniteData<NotificationPage>>(actionKey)?.pages[0].items[0].read_at, '2026-09-10T05:02:00Z');
  assert.deepEqual(client.getQueryData<NotificationSummary>(['notification-summary', actorID]), {
    unread_total: 0,
    action_required: 1,
    by_category: { workflow: 0 },
    actor_revision: 8,
  });
  client.clear();
});

test('burst notification events coalesce into one trailing invalidation', () => {
  let invalidations = 0;
  let nextTimer = 0;
  const pending = new Map<number, () => void>();
  const scheduler = createNotificationInvalidationScheduler(
    () => { invalidations += 1; },
    750,
    {
      set: ((callback: () => void) => {
        nextTimer += 1;
        pending.set(nextTimer, callback);
        return nextTimer;
      }) as any,
      clear: ((timer: number) => { pending.delete(timer); }) as any,
    },
  );

  for (let index = 0; index < 14; index += 1) scheduler.schedule();
  assert.equal(pending.size, 1);
  const trailing = pending.values().next().value;
  pending.clear();
  trailing?.();
  assert.equal(invalidations, 1);
  assert.equal(pending.size, 0);
  scheduler.cancel();
});
