import type { WorkItem, WorkItemExecutionTarget } from '../../lib/types/domain';

const workspaceLabels: Record<string, string> = {
  preparation: 'Hồ sơ chuẩn bị',
  planning: 'Kế hoạch',
  boq: 'BOQ',
  contracts: 'Hợp đồng',
  procurement: 'Mua sắm',
  warehouse: 'Kho vật tư',
  operations: 'Thi công',
  acceptance: 'Nghiệm thu',
  handover: 'Bàn giao',
  project_finance: 'Bàn giao kế toán',
  work_item: 'Hồ sơ công việc',
};

export function executionTargetPath(item: WorkItem, returnTo?: string) {
  const target = item.execution_target;
  const route = safeExecutionRoute(item, target?.route);
  if (!route) return fallbackWorkItemPath(item, returnTo);
  return routeWithReturnTo(route, returnTo);
}

export function executionTargetActionLabel(item: WorkItem) {
  return cleanText(item.execution_target?.action_label) || (item.execution_target?.kind === 'approval' ? 'Mở phê duyệt' : 'Mở công việc');
}

export function executionTargetWorkspaceLabel(item: WorkItem) {
  const workspace = cleanText(item.execution_target?.workspace);
  return workspaceLabels[workspace] ?? 'Hồ sơ công việc';
}

export function executionTargetNeedsEmbeddedWorkItem(target?: WorkItemExecutionTarget) {
  return ['capture', 'evidence', 'approval', 'mixed', 'work_item'].includes(cleanText(target?.kind));
}

export function isDomainExecutionTarget(target?: WorkItemExecutionTarget) {
  return Boolean(target && target.workspace && target.workspace !== 'work_item');
}

export function workItemMatchesProject(item: WorkItem, projectId: string) {
  return Boolean(cleanText(projectId) && cleanText(item.project_id) === cleanText(projectId));
}

function routeWithReturnTo(route: string, returnTo?: string) {
  const safeReturnTo = safeReturnPath(returnTo);
  if (!safeReturnTo) return route;
  const [pathname, query = ''] = route.split('?', 2);
  const params = new URLSearchParams(query);
  params.set('returnTo', safeReturnTo);
  return `${pathname}?${params.toString()}`;
}

function fallbackWorkItemPath(item: WorkItem, returnTo?: string) {
  const base = item.project_id
    ? `/projects/${encodeURIComponent(item.project_id)}/work-items/${encodeURIComponent(item.id)}`
    : `/work-items/${encodeURIComponent(item.id)}`;
  return routeWithReturnTo(base, returnTo);
}

function safeReturnPath(value?: string | null) {
  const cleanValue = cleanText(value);
  if (!cleanValue || !cleanValue.startsWith('/') || cleanValue.startsWith('//')) return '';
  return cleanValue;
}

function safeInternalRoute(route?: string) {
  const value = cleanText(route);
  return value.startsWith('/') && !value.startsWith('//') ? value : '';
}

function safeExecutionRoute(item: WorkItem, route?: string) {
  const value = safeInternalRoute(route);
  if (!value) return '';
  const projectId = cleanText(item.project_id);
  if (!projectId) return value;
  const pathname = value.split('?', 1)[0];
  const expectedProjectPath = `/projects/${encodeURIComponent(projectId)}`;
  return pathname === expectedProjectPath || pathname.startsWith(`${expectedProjectPath}/`) ? value : '';
}

function cleanText(value: unknown) {
  const text = String(value ?? '').trim();
  return text === '<nil>' ? '' : text;
}
