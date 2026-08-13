import type { Project } from '../../lib/types/domain';

export type ProjectExplorerSortKey = 'project' | 'status' | 'owner';
export type ProjectExplorerSortDirection = 'asc' | 'desc';
export type ProjectExplorerSort = {
  key: ProjectExplorerSortKey;
  direction: ProjectExplorerSortDirection;
};

export const projectExplorerSortOptions: Array<{ value: string; label: string }> = [
  { value: 'project:asc', label: 'Cây công trình: tăng dần' },
  { value: 'project:desc', label: 'Cây công trình: giảm dần' },
  { value: 'status:asc', label: 'Trạng thái: tăng dần' },
  { value: 'status:desc', label: 'Trạng thái: giảm dần' },
  { value: 'owner:asc', label: 'Phụ trách: tăng dần' },
  { value: 'owner:desc', label: 'Phụ trách: giảm dần' },
];

const vietnameseCollator = new Intl.Collator('vi', {
  numeric: true,
  sensitivity: 'base',
});

export function nextProjectExplorerSort(
  key: ProjectExplorerSortKey,
  current: ProjectExplorerSort | null,
): ProjectExplorerSort {
  if (current?.key !== key) return { key, direction: 'asc' };
  return { key, direction: current.direction === 'asc' ? 'desc' : 'asc' };
}

export function projectExplorerSortFromValue(value: string): ProjectExplorerSort | null {
  const [key, direction, extra] = value.split(':');
  if (extra || !isProjectExplorerSortKey(key) || (direction !== 'asc' && direction !== 'desc')) return null;
  return { key, direction };
}

export function projectExplorerSortValue(sort: ProjectExplorerSort | null) {
  return sort ? `${sort.key}:${sort.direction}` : '';
}

export function sortProjectExplorerRows(
  rows: readonly Project[],
  sort: ProjectExplorerSort | null,
  statusDisplayLabel: (status: string) => string = (status) => status,
): Project[] {
  const result = [...rows];
  if (!sort) return result;

  return result.sort((left, right) => {
    const primary = compareProjectField(left, right, sort, statusDisplayLabel);
    if (primary !== 0) return primary;

    const code = compareLocalizedText(left.code, right.code, 'asc');
    if (code !== 0) return code;
    const name = compareLocalizedText(left.name, right.name, 'asc');
    if (name !== 0) return name;
    return compareLocalizedText(left.id, right.id, 'asc');
  });
}

function compareProjectField(
  left: Project,
  right: Project,
  sort: ProjectExplorerSort,
  statusDisplayLabel: (status: string) => string,
) {
  if (sort.key === 'project') {
    const code = compareLocalizedText(left.code, right.code, sort.direction);
    return code || compareLocalizedText(left.name, right.name, sort.direction);
  }
  if (sort.key === 'status') {
    const leftStatus = left.status ? statusDisplayLabel(left.status) : '';
    const rightStatus = right.status ? statusDisplayLabel(right.status) : '';
    return compareLocalizedText(leftStatus, rightStatus, sort.direction);
  }
  return compareLocalizedText(left.pm_name, right.pm_name, sort.direction);
}

function compareLocalizedText(
  left: unknown,
  right: unknown,
  direction: ProjectExplorerSortDirection,
) {
  const leftText = String(left ?? '').trim();
  const rightText = String(right ?? '').trim();
  if (!leftText && !rightText) return 0;
  if (!leftText) return 1;
  if (!rightText) return -1;
  const result = vietnameseCollator.compare(leftText, rightText);
  return direction === 'asc' ? result : -result;
}

function isProjectExplorerSortKey(value: string): value is ProjectExplorerSortKey {
  return value === 'project' || value === 'status' || value === 'owner';
}
