import type { WorkItem } from '../../lib/types/domain';

const hiddenStatuses = new Set(['cancelled', 'deleted', 'skipped']);
const suggestedStatuses = new Set(['in_progress', 'ready']);

export type AcceptanceWorkItemGroups = {
  suggested: WorkItem[];
  others: WorkItem[];
};

export function normalizeAcceptanceWorkItemSearch(value: unknown) {
  return String(value ?? '')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/đ/g, 'd')
    .replace(/Đ/g, 'D')
    .toLocaleLowerCase('vi')
    .trim();
}

export function acceptanceWorkItemMatchesSearch(item: WorkItem, query: string) {
  const needle = normalizeAcceptanceWorkItemSearch(query);
  if (!needle) return true;
  const haystack = normalizeAcceptanceWorkItemSearch([
    item.title,
    item.name,
    item.run_node_name,
    item.run_step_name,
    item.assignee_name,
    item.department,
    item.status,
  ].filter(Boolean).join(' '));
  return haystack.includes(needle);
}

export function groupAcceptanceWorkItems(items: WorkItem[], query = '', selectedId = ''): AcceptanceWorkItemGroups {
  const visible = items
    .filter((item) => item.id && (!hiddenStatuses.has(item.status) || item.id === selectedId))
    .filter((item) => acceptanceWorkItemMatchesSearch(item, query))
    .sort(compareAcceptanceWorkItems);

  return visible.reduce<AcceptanceWorkItemGroups>((groups, item) => {
    if (suggestedStatuses.has(item.status) || item.id === selectedId) groups.suggested.push(item);
    else groups.others.push(item);
    return groups;
  }, { suggested: [], others: [] });
}

function compareAcceptanceWorkItems(left: WorkItem, right: WorkItem) {
  const statusDifference = workItemStatusRank(left.status) - workItemStatusRank(right.status);
  if (statusDifference) return statusDifference;
  const nodeDifference = numericOrder(left.run_node_order_index) - numericOrder(right.run_node_order_index);
  if (nodeDifference) return nodeDifference;
  const stepDifference = numericOrder(left.run_step_order_index) - numericOrder(right.run_step_order_index);
  if (stepDifference) return stepDifference;
  return String(left.title || left.name).localeCompare(String(right.title || right.name), 'vi');
}

function workItemStatusRank(status: string) {
  if (status === 'in_progress') return 0;
  if (status === 'ready') return 1;
  if (status === 'blocked') return 2;
  if (status === 'waiting_review' || status === 'waiting_approval') return 3;
  if (status === 'pending') return 4;
  if (status === 'completed' || status === 'done') return 5;
  return 6;
}

function numericOrder(value: unknown) {
  const number = Number(value);
  return Number.isFinite(number) ? number : Number.MAX_SAFE_INTEGER;
}
