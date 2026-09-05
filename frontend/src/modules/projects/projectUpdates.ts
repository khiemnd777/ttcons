import type { ProjectUpdateKind, WorkItemActivity } from '../../lib/types/domain';

export const projectUpdateGroups: { kind: ProjectUpdateKind; label: string; count: 'notification_unread' | 'discussion_unread' | 'system_unread' }[] = [
  { kind: 'notification', label: 'Thông báo', count: 'notification_unread' },
  { kind: 'discussion', label: 'Trao đổi', count: 'discussion_unread' },
  { kind: 'system', label: 'Hoạt động', count: 'system_unread' },
];

export function projectUpdateIDs(ids: string[]) {
  return [...new Set(ids)].sort().join(',');
}

export function projectUpdateCount(value: number) {
  return value > 99 ? '99+' : String(Math.max(0, value));
}

// A read-through cursor covers older entries too. Do not advance it while older
// unread pages or a hidden activity-type subset have not been loaded.
export function projectActivityReadBoundary(items: WorkItemActivity[], hasMore: boolean, filtered: boolean) {
  if (hasMore || filtered) return undefined;
  return items.find((item) => !item.withdrawn_at)?.id;
}
