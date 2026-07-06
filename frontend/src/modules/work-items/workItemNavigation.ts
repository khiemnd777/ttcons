type LocationLike = {
  pathname: string;
  search?: string;
  hash?: string;
};

export function currentReturnTo(location: LocationLike) {
  return `${location.pathname}${location.search ?? ''}${location.hash ?? ''}`;
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
