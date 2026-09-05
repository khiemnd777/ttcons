import type { InfiniteData } from '@tanstack/react-query';
import type { ActivityReactionSummary, ProjectActivityFeed, WorkItemActivity } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format.ts';

export function sameChatAuthor(previous: WorkItemActivity | undefined, current: WorkItemActivity) {
  if (!previous || previous.actor_id !== current.actor_id || !previous.actor_id) return false;
  const previousTime = new Date(previous.created_at ?? '').getTime();
  const currentTime = new Date(current.created_at ?? '').getTime();
  return Number.isFinite(previousTime) && Number.isFinite(currentTime) && currentTime - previousTime <= 5 * 60 * 1000;
}

export function actorInitials(name?: string) {
  const parts = (name ?? 'HT').trim().split(/\s+/).filter(Boolean);
  return parts.slice(-2).map((part) => part[0]?.toUpperCase()).join('') || 'HT';
}

export function chatBubbleTone(activityType: string, own: boolean) {
  if (activityType === 'blocker') return 'border-rose-200 bg-rose-50 text-rose-950';
  if (activityType === 'question') return 'border-amber-200 bg-amber-50 text-amber-950';
  if (activityType === 'decision') return 'border-violet-200 bg-violet-50 text-violet-950';
  if (activityType === 'reminder') return 'border-cyan-200 bg-cyan-50 text-cyan-950';
  return own ? 'border-blue-600 bg-blue-600 text-white' : 'border-line bg-white text-slate-700';
}


export function activityResolveLabel(item: WorkItemActivity) {
  if (item.activity_type === 'question') return 'Đã trả lời';
  if (item.activity_type === 'blocker') return 'Đã xử lý';
  if (item.activity_type === 'reminder') return 'Hoàn tất nhắc việc';
  return '';
}

export function activityResolved(item: WorkItemActivity) {
  const metadata = item.metadata_json ?? {};
  if (item.activity_type === 'question') return Boolean(metadata.answered_at || metadata.resolved_at || metadata.closed_at);
  if (item.activity_type === 'blocker') return Boolean(metadata.resolved_at || metadata.closed_at);
  if (item.activity_type === 'reminder') return Boolean(metadata.completed_at || metadata.done_at || metadata.closed_at);
  return false;
}

export function activityTone(activityType: string) {
  if (activityType === 'blocker') return 'bg-rose-50 text-rose-700';
  if (activityType === 'question') return 'bg-amber-50 text-amber-700';
  if (activityType === 'decision') return 'bg-violet-50 text-violet-700';
  if (activityType === 'reminder') return 'bg-cyan-50 text-cyan-700';
  if (['comment', 'status', 'file_note'].includes(activityType)) return 'bg-blue-50 text-blue-700';
  return 'bg-slate-100 text-slate-600';
}

export function groupActivitiesByDay(items: WorkItemActivity[]) {
  const groups: Array<{ key: string; label: string; items: WorkItemActivity[] }> = [];
  items.forEach((item) => {
    const date = item.created_at ? new Date(item.created_at) : null;
    const key = dateText(date, 'unknown');
    let group = groups.find((candidate) => candidate.key === key);
    if (!group) {
      group = { key, label: activityDayLabel(date), items: [] };
      groups.push(group);
    }
    group.items.push(item);
  });
  return groups;
}

export function activityDayLabel(date: Date | null) {
  if (!date || Number.isNaN(date.getTime())) return 'Không rõ ngày';
  const today = new Date();
  const yesterday = new Date(today.getTime() - 86_400_000);
  const key = dateText(date);
  if (key === dateText(today)) return 'Hôm nay';
  if (key === dateText(yesterday)) return 'Hôm qua';
  return key;
}

export function updateActivityReactionFeed(
  current: InfiniteData<ProjectActivityFeed> | undefined,
  variables: { activityId: string; reaction: ActivityReactionSummary['reaction']; active: boolean },
) {
  if (!current) return current;
  return {
    ...current,
    pages: current.pages.map((page) => ({
      ...page,
      items: page.items.map((item) => {
        if (item.id !== variables.activityId) return item;
        const reactions = [...(item.reactions ?? [])];
        const index = reactions.findIndex((entry) => entry.reaction === variables.reaction);
        if (index >= 0) {
          const existing = reactions[index];
          if (existing.reacted_by_me === variables.active) return item;
          const count = Math.max(0, Number(existing.count) + (variables.active ? 1 : -1));
          if (!count) reactions.splice(index, 1);
          else reactions[index] = { ...existing, count, reacted_by_me: variables.active };
        } else if (variables.active) {
          reactions.push({ reaction: variables.reaction, count: 1, reacted_by_me: true });
        }
        return { ...item, reactions };
      }),
    })),
  };
}

export function dedupeOptions<T>(items: T[], keyOf: (item: T) => string) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = keyOf(item);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
