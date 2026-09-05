import type { ProjectProgressItem, ProjectProgressStagePage } from '../../lib/types/domain';

export const PROJECT_PROGRESS_PAGE_SIZE = 25;

export type ProjectProgressServerFilter = {
  search: string;
  pmStaffId: string;
  status: string;
};

export function projectProgressFilterQuery(filter: ProjectProgressServerFilter) {
  const params = new URLSearchParams();
  const search = filter.search.trim();
  if (search) params.set('search', search);
  if (filter.pmStaffId) params.set('pm_staff_id', filter.pmStaffId);
  if (filter.status) params.set('status', filter.status);
  return params.toString();
}

export function projectProgressStageQuery(filterQuery: string, cursor = '') {
  const params = new URLSearchParams(filterQuery);
  params.set('limit', String(PROJECT_PROGRESS_PAGE_SIZE));
  if (cursor) params.set('cursor', cursor);
  return params.toString();
}

export function mergeProjectProgressPages(pages: ProjectProgressStagePage[] | undefined) {
  const unique = new Map<string, ProjectProgressItem>();
  for (const page of pages ?? []) {
    for (const item of page.items ?? []) unique.set(item.id, item);
  }
  return Array.from(unique.values());
}

export function projectProgressUpdateIDs(items: ProjectProgressItem[], virtualItems: readonly { index: number }[], nearViewport: boolean) {
  if (!nearViewport) return [];
  return [...new Set(virtualItems.flatMap(({ index }) => items[index] ? [items[index].id] : []))];
}
