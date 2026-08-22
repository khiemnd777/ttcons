import clsx from 'clsx';
import { DELETED_ACCOUNT_SUFFIX } from './identityReference';

export { DELETED_ACCOUNT_SUFFIX, identityReferenceText } from './identityReference';

export function IdentityReferenceLabel({
  name,
  deleted = false,
  liveOwnership = false,
  fallback = '—',
  className,
}: {
  name?: unknown;
  deleted?: boolean;
  liveOwnership?: boolean;
  fallback?: string;
  className?: string;
}) {
  const value = String(name ?? '').trim();
  if (!value) return <span className={clsx('text-slate-400', className)}>{fallback}</span>;
  return (
    <span className={clsx('inline-flex flex-wrap items-center gap-x-1 gap-y-0.5', className)}>
      <span>{value}{deleted ? <span className="font-normal text-slate-500">{DELETED_ACCOUNT_SUFFIX}</span> : null}</span>
      {deleted && liveOwnership ? <span className="rounded bg-amber-100 px-1.5 py-0.5 text-[11px] font-semibold text-amber-800">Cần phân công lại</span> : null}
    </span>
  );
}
