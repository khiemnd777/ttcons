import type { WorkItem } from '../../lib/types/domain';

export type PreparationStageGroup = {
  key: string;
  name: string;
  items: WorkItem[];
};

export function groupPreparationItems(items: WorkItem[]): PreparationStageGroup[] {
  const groups: PreparationStageGroup[] = [];
  const groupsByKey = new Map<string, PreparationStageGroup>();

  items.forEach((item) => {
    const name = item.run_node_name?.trim() || 'Ngoài quy trình';
    const key = item.run_node_id || `${Number(item.run_node_order_index ?? 0)}:${name}`;
    const existing = groupsByKey.get(key);
    if (existing) {
      existing.items.push(item);
      return;
    }

    const group = { key, name, items: [item] };
    groupsByKey.set(key, group);
    groups.push(group);
  });

  return groups;
}
