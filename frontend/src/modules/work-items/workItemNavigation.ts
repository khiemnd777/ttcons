type LocationLike = {
  pathname: string;
  search?: string;
  hash?: string;
};

const projectTaskDetailIntentKey = 'projectTaskDetailIntent';
const constructionHandoffIntentKey = 'constructionHandoffIntent';

export function currentReturnTo(location: LocationLike) {
  return `${location.pathname}${location.search ?? ''}${location.hash ?? ''}`;
}

export function currentWorkspaceReturnTo(location: LocationLike) {
  const params = new URLSearchParams(location.search ?? '');
  params.delete('task');
  params.delete('returnTo');
  const query = params.toString();
  return `${location.pathname}${query ? `?${query}` : ''}${location.hash ?? ''}`;
}

export function projectTaskDetailNavigationState(workItemId: string, currentState?: unknown) {
  const nextState = { ...navigationStateRecord(currentState) };
  const cleanWorkItemId = String(workItemId ?? '').trim();
  if (cleanWorkItemId) nextState[projectTaskDetailIntentKey] = cleanWorkItemId;
  return nextState;
}

export function projectTaskDetailIntentMatches(state: unknown, workItemId: string) {
  const cleanWorkItemId = String(workItemId ?? '').trim();
  return Boolean(cleanWorkItemId && navigationStateRecord(state)[projectTaskDetailIntentKey] === cleanWorkItemId);
}

export function consumeProjectTaskDetailNavigationState(state: unknown) {
  const nextState = { ...navigationStateRecord(state) };
  delete nextState[projectTaskDetailIntentKey];
  return Object.keys(nextState).length ? nextState : null;
}

export function constructionHandoffNavigationState(projectId: string, currentState?: unknown) {
  const nextState = { ...navigationStateRecord(currentState) };
  const cleanProjectId = String(projectId ?? '').trim();
  if (cleanProjectId) nextState[constructionHandoffIntentKey] = cleanProjectId;
  return nextState;
}

export function constructionHandoffIntentMatches(state: unknown, projectId: string) {
  const cleanProjectId = String(projectId ?? '').trim();
  return Boolean(cleanProjectId && navigationStateRecord(state)[constructionHandoffIntentKey] === cleanProjectId);
}

export function consumeConstructionHandoffNavigationState(state: unknown) {
  const nextState = { ...navigationStateRecord(state) };
  delete nextState[constructionHandoffIntentKey];
  return Object.keys(nextState).length ? nextState : null;
}

export function workItemDetailPath(workItemId: string, returnTo?: string, projectId?: string) {
  const params = new URLSearchParams();
  const safeReturnTo = safeReturnPath(returnTo);
  if (safeReturnTo) params.set('returnTo', safeReturnTo);
  const query = params.toString();
  const basePath = projectId
    ? `/projects/${encodeURIComponent(projectId)}/work-items/${encodeURIComponent(workItemId)}`
    : `/work-items/${encodeURIComponent(workItemId)}`;
  return `${basePath}${query ? `?${query}` : ''}`;
}

export function safeReturnPath(value?: string | null) {
  const cleanValue = String(value ?? '').trim();
  if (!cleanValue || !cleanValue.startsWith('/') || cleanValue.startsWith('//')) return '';
  return cleanValue;
}

function navigationStateRecord(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  return value as Record<string, unknown>;
}
