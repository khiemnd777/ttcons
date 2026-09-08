import type { Project } from '../../lib/types/domain';

export type ProjectExplorerSortKey = 'project' | 'status' | 'owner' | 'created_at' | 'updated_at';
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
  { value: 'created_at:asc', label: 'Ngày tạo: cũ nhất trước' },
  { value: 'created_at:desc', label: 'Ngày tạo: mới nhất trước' },
  { value: 'updated_at:asc', label: 'Ngày cập nhật: cũ nhất trước' },
  { value: 'updated_at:desc', label: 'Ngày cập nhật: mới nhất trước' },
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
  if (sort.key === 'owner') {
    return compareLocalizedText(left.pm_name, right.pm_name, sort.direction);
  }
  return compareDateValues(
    sort.key === 'created_at' ? left.created_at : left.updated_at,
    sort.key === 'created_at' ? right.created_at : right.updated_at,
    sort.direction,
  );
}

function compareDateValues(
  left: unknown,
  right: unknown,
  direction: ProjectExplorerSortDirection,
) {
  const leftTimestamp = timestamp(left);
  const rightTimestamp = timestamp(right);
  if (leftTimestamp === null && rightTimestamp === null) return 0;
  if (leftTimestamp === null) return 1;
  if (rightTimestamp === null) return -1;
  const result = leftTimestamp - rightTimestamp;
  return direction === 'asc' ? result : -result;
}

function timestamp(value: unknown) {
  if (typeof value !== 'string' || !value.trim()) return null;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : null;
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
  return value === 'project' || value === 'status' || value === 'owner' || value === 'created_at' || value === 'updated_at';
}
