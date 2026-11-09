/** Wordless is the default. Invalid/duplicate flags conservatively select the legacy UI. */
export function wordlessEnabled(search: string) {
  const values = new URLSearchParams(search).getAll('wordless');
  return values.length === 0 || (values.length === 1 && values[0] === '1');
}

export function wordlessScope(pathname: string) {
  return /^\/projects\/[^/]+\/(preparation|planning|pipeline)\/?$/.test(pathname)
    || /^\/(settings|statistics(?:\/(?:projects|procurement|warehouse|staff))?)\/?$/.test(pathname);
}

export function wordlessBannerSurface(pathname: string): 'discussion' | 'acceptance' | 'operations' | undefined {
  return /^\/projects\/[^/]+\/(discussion|acceptance|operations)\/?$/.exec(pathname)?.[1] as 'discussion' | 'acceptance' | 'operations' | undefined;
}

/** A standalone detail is eligible only when entered from the same project's scoped workspace. */
export function wordlessDetailScope(pathname: string, search: string) {
  const detail = /^\/projects\/([^/]+)\/work-items\/[^/]+\/?$/.exec(pathname);
  if (!detail) return false;
  const returns = new URLSearchParams(search).getAll('returnTo');
  if (returns.length !== 1 || !returns[0].startsWith('/') || returns[0].startsWith('//') || returns[0].includes('\\')) return false;
  const source = new URL(returns[0], 'http://consflow.local');
  return source.origin === 'http://consflow.local' && source.pathname.startsWith('/projects/') && wordlessScope(source.pathname) && source.pathname.split('/')[2] === detail[1];
}

/** Carry only the presentation flag; never copy task/filter context to another route. */
export function withWordless(target: string, sourceSearch: string) {
  if (!target || !target.startsWith('/') || target.startsWith('//') || target.includes('\\')) return target;
  const url = new URL(target, 'http://consflow.local');
  if (url.origin !== 'http://consflow.local' || !(wordlessScope(url.pathname) || wordlessBannerSurface(url.pathname) || wordlessDetailScope(url.pathname, url.search))) return target;
  url.searchParams.delete('wordless');
  if (!wordlessEnabled(sourceSearch)) url.searchParams.set('wordless', '0');
  else if (new URLSearchParams(sourceSearch).has('wordless')) url.searchParams.set('wordless', '1');
  return `${url.pathname}${url.search}${url.hash}`;
}
