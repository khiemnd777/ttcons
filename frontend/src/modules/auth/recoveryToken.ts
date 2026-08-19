export function recoveryTokenFromFragment() {
  if (typeof window === 'undefined') return '';
  const params = new URLSearchParams(window.location.hash.replace(/^#/, ''));
  return params.get('token')?.trim() ?? '';
}

export function clearRecoveryTokenFragment() {
  if (typeof window === 'undefined' || !window.location.hash) return;
  window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}`);
}
