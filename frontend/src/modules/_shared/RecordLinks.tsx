import type { ReactNode } from 'react';
import { ArrowUpRight } from 'lucide-react';
import { useAuth } from '../../lib/auth/AuthContext';
import { RecordSection } from '../../components/RecordDetailLayout';
import { canOpenRecord, recordKinds, recordPath, relatedRecordTargets, type RecordKind, type RecordTarget } from './recordNavigation';

export function RecordLink({ target, children, primary = false }: { target: RecordTarget | null; children?: ReactNode; primary?: boolean }) {
  const { hasPermission } = useAuth();
  if (!target || !canOpenRecord(target, hasPermission)) return primary ? null : <>{children ?? target?.label ?? '—'}</>;
  return <a href={recordPath(target)!} target="_blank" rel="noopener noreferrer"
    className={primary ? 'inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600' : 'inline-flex max-w-full items-baseline gap-1 font-semibold text-blue-700 underline decoration-blue-200 underline-offset-4 hover:decoration-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-600'}
    title="Mở trong tab mới">
    <span className="min-w-0 break-words">{children ?? target.label ?? recordKinds[target.kind].label}</span><ArrowUpRight aria-hidden="true" className="inline h-4 w-4 shrink-0 self-center" /><span className="sr-only"> (tab mới)</span>
  </a>;
}

export function RelatedRecords({ kind, record }: { kind: RecordKind; record: Record<string, any> }) {
  const { hasPermission } = useAuth();
  const targets = relatedRecordTargets(kind, record).filter(target => canOpenRecord(target, hasPermission));
  if (!targets.length) return null;
  return <RecordSection title="Hồ sơ liên quan"><div className="divide-y divide-slate-100">{targets.map(target => <div key={`${target.kind}:${target.id}`} className="flex flex-wrap items-baseline justify-between gap-2 py-2.5 text-sm">
    <span className="text-slate-500">{recordKinds[target.kind].label}</span><RecordLink target={target}>{target.label || `Mở ${recordKinds[target.kind].label.toLocaleLowerCase('vi')}`}</RecordLink>
  </div>)}</div></RecordSection>;
}
