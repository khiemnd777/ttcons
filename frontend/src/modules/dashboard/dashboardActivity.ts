import type { DashboardActivity } from './dashboardTypes';
import { dateText, dateTimeText } from '../../lib/utils/format.ts';

export const dashboardActivityRanges = [
  { value: 'today', label: 'Hôm nay' },
  { value: '7d', label: '7 ngày gần đây' },
  { value: '30d', label: '30 ngày gần đây' },
  { value: '90d', label: '90 ngày gần đây' },
  { value: 'all', label: 'Toàn bộ thời gian' },
] as const;

export const dashboardActivityTypes = ['created', 'started', 'completed', 'blocked', 'blocker_resolved', 'status_changed', 'review_requested', 'approval_requested', 'approved', 'rejected', 'changes_requested', 'evidence_added', 'evidence_accepted', 'evidence_rejected', 'system_event'];

export function dashboardActivityQuery(range: string, search: string, activityType: string, cursor = '') {
  const params = new URLSearchParams({ range, limit: '20' });
  if (search.trim()) params.set('search', search.trim());
  if (activityType) params.set('activity_type', activityType);
  if (cursor) params.set('cursor', cursor);
  return params.toString();
}

export function dashboardActivityDate(value: string) {
  return dateTimeText(value, 'Không rõ thời gian');
}

export function dashboardActivityDay(value: string) {
  return dateText(value, 'Không rõ ngày');
}

export function dashboardActivityGroups(items: DashboardActivity[]) {
  const groups: Array<{ label: string; items: DashboardActivity[] }> = [];
  const seen = new Set<string>();
  for (const item of items) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    const label = dashboardActivityDay(item.created_at);
    const last = groups[groups.length - 1];
    if (last?.label === label) last.items.push(item);
    else groups.push({ label, items: [item] });
  }
  return groups;
}

export function dashboardActivityHref(item: DashboardActivity) {
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  if (!uuid.test(item.project_id)) return '';
  return uuid.test(item.work_item_id) ? `/projects/${item.project_id}/work-items/${item.work_item_id}` : `/projects/${item.project_id}/discussion`;
}
