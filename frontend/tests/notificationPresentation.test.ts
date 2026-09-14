import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('notification platform uses server-owned presentation, safe routes, and one actor stream', async () => {
  const panel = await source('../src/modules/notifications/NotificationPanel.tsx');
  const indicators = await source('../src/modules/notifications/useShellIndicators.ts');
  const presentation = await source('../src/modules/notifications/notificationPresentation.tsx');
  const realtime = await source('../src/modules/notifications/ActorRealtimeProvider.tsx');
  const page = await source('../src/modules/notifications/NotificationsPage.tsx');
  const router = await source('../src/app/router.tsx');
  const myWork = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(panel, /get<NotificationPage>\(`\/me\/notifications\?/);
  assert.match(panel, /`\/me\/notifications\/\$\{notification\.id\}\/read`/);
  assert.match(panel, /'\/me\/notifications\/read-all'/);
  assert.match(panel, /`\/me\/notifications\/\$\{notification\.id\}\/archive`/);
  assert.match(panel, /applyNotificationReadCache/);
  assert.match(panel, /applyNotificationArchiveCache/);
  assert.match(panel, /listState === 'stale-error'/);
  assert.match(panel, /Danh sách gần nhất vẫn được giữ/);
  assert.doesNotMatch(panel, /work_report_(submitted|resubmitted|needs_changes|acknowledged)/);
  assert.doesNotMatch(panel, /action_key ===/);
  assert.match(panel, /safeNotificationHref\(notification\.action_href\)/);
  assert.match(panel, /role="dialog" aria-modal="true" aria-label="Danh sách thông báo"/);
  assert.match(panel, /notifications\.hasNextPage/);
  assert.match(presentation, /iconCatalog/);
  assert.match(presentation, /toneCatalog/);
  assert.match(presentation, /parsed\.origin !== window\.location\.origin/);

  assert.match(indicators, /get<NotificationSummary>\('\/me\/notifications\/summary'\)/);
  assert.match(indicators, /get<WorkReportSummary>\('\/work-reports\/summary'\)/);
  assert.match(indicators, /get<MyWorkNavigationSummary>\('\/me\/work-items\/navigation-summary'\)/);
  assert.doesNotMatch(indicators, /streamSSE/);
  assert.match(realtime, /streamSSE\('\/me\/activity-attention\/stream'/);
  assert.match(realtime, /status !== 'connected'/);
  assert.match(realtime, /message\.event === 'notification_changed'/);
  assert.match(realtime, /createNotificationInvalidationScheduler/);
  assert.match(realtime, /notificationInvalidations\.schedule\(\)/);
  assert.match(realtime, /previous\?\.actor_revision === current\.actor_revision/);
  assert.doesNotMatch(realtime, /message\.event === 'notification_changed'[\s\S]{0,240}invalidateQueries\(\{ queryKey: \['notifications'/);
  assert.match(realtime, /message\.event === 'work_report_changed'[\s\S]*?\['my-work-navigation-summary', actorID\]/);
  assert.doesNotMatch(myWork, /streamSSE\('\/me\/activity-attention\/stream'/);

  assert.match(page, /\/me\/notifications\/catalog/);
  assert.match(page, /\/me\/notifications\/projects/);
  assert.match(page, /\/me\/notifications\/preferences/);
  assert.match(page, /\['action_required', 'Cần xử lý'/);
  assert.match(page, /listState === 'stale-error'/);
  assert.match(page, /Danh sách gần nhất vẫn được giữ/);
  assert.match(router, /path: '\/notifications'/);
  assert.match(myWork, /searchParams\.get\('task'\)/);
});
