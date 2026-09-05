import type { Project, ProjectUpdateKind, ProjectUpdatesPage, WorkItemActivity } from '../../lib/types/domain';

export type ProjectUpdateIdentity = Pick<Project, 'id' | 'code' | 'name'>;

export const projectUpdateGroups: { kind: ProjectUpdateKind; label: string; count: 'notification_unread' | 'discussion_unread' | 'system_unread' }[] = [
  { kind: 'notification', label: 'Thông báo', count: 'notification_unread' },
  { kind: 'discussion', label: 'Trao đổi', count: 'discussion_unread' },
  { kind: 'system', label: 'Hoạt động', count: 'system_unread' },
];

export function projectUpdateIDs(ids: string[]) {
  return [...new Set(ids)].sort().join(',');
}

// Both Explorer pages and virtual board windows use the same bounded endpoint.
// Keep a larger caller from exceeding its 100-project request contract.
export async function loadProjectUpdates(ids: string[], loadBatch: (projectIDs: string) => Promise<ProjectUpdatesPage>): Promise<ProjectUpdatesPage> {
  const uniqueIDs = [...new Set(ids)].sort();
  const items: ProjectUpdatesPage['items'] = [];
  for (let start = 0; start < uniqueIDs.length; start += 100) {
    const page = await loadBatch(uniqueIDs.slice(start, start + 100).join(','));
    items.push(...page.items);
  }
  return { items };
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
