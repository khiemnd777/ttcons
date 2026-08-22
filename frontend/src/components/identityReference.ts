export const DELETED_ACCOUNT_SUFFIX = ' (tài khoản đã xoá)';

export function identityReferenceText(name: unknown, deleted?: boolean, fallback = '—') {
  const value = String(name ?? '').trim();
  if (!value) return fallback;
  return deleted ? `${value}${DELETED_ACCOUNT_SUFFIX}` : value;
}
