import { AlertTriangle, ArrowRight, CheckCircle2, ChevronDown, Circle, Shield } from 'lucide-react';
import clsx from 'clsx';
import type { RequirementRow, RequirementState } from './workItemDetailDomain';
import type { WorkItemRequirementNavigationTarget } from './workItemRequirements';

export function WorkItemRequirementPanel({
  rows,
  onNavigate,
  openWhenBlocked = false,
}: {
  rows: RequirementRow[];
  onNavigate: (target: WorkItemRequirementNavigationTarget) => void;
  openWhenBlocked?: boolean;
}) {
  if (!rows.length) return null;
  const incompleteCount = rows.filter((row) => row.state === 'blocked' || row.state === 'warning').length;

  return (
    <details
      className={clsx('group rounded-md border bg-white shadow-panel', incompleteCount ? 'border-amber-200' : 'border-line')}
      open={openWhenBlocked && incompleteCount > 0}
    >
      <summary className={clsx('flex cursor-pointer list-none items-center justify-between gap-3 rounded-md px-3 py-3', incompleteCount ? 'bg-amber-50 hover:bg-amber-100' : 'bg-slate-50 hover:bg-slate-100')}>
        <div className="flex min-w-0 items-center gap-2">
          {incompleteCount ? <AlertTriangle className="h-4 w-4 shrink-0 text-amber-700" /> : <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />}
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Điều kiện bắt buộc</p>
            <h3 className="mt-0.5 text-sm font-semibold text-ink">
              {incompleteCount ? `Còn ${incompleteCount} điều kiện cần xử lý` : 'Không có điều kiện đang chặn'}
            </h3>
          </div>
        </div>
        <span className="inline-flex shrink-0 items-center gap-2 text-xs font-semibold text-slate-600">
          Xem chi tiết
          <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
        </span>
      </summary>
      <div className="divide-y divide-line border-t border-line bg-white">
        {rows.map((row) => {
          const navigationTarget = row.target;
          const action = navigationTarget && row.actionLabel ? (
            <span className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-blue-700">
              {row.actionLabel}
              <ArrowRight className="h-3.5 w-3.5" />
            </span>
          ) : null;
          const content = (
            <>
              <RequirementIcon state={row.state} kind={row.kind} />
              <div className="min-w-0 flex-1 text-left">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-semibold text-ink">{row.label}</p>
                  {row.kind === 'permission' ? (
                    <span className="rounded-md bg-blue-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-blue-700">
                      Thông tin truy cập
                    </span>
                  ) : null}
                </div>
                <p className="mt-0.5 text-xs leading-5 text-slate-600">{row.description}</p>
                <p className="mt-1 text-xs leading-5 text-slate-700">
                  <span className="font-semibold">{row.kind === 'permission' ? 'Khi không thể thao tác:' : 'Cách xử lý:'}</span> {row.guidance}
                </p>
                {action ? <span className="mt-2 block sm:hidden">{action}</span> : null}
              </div>
              {action ? <span className="hidden self-center sm:block">{action}</span> : null}
            </>
          );

          return navigationTarget ? (
            <button
              key={row.id}
              type="button"
              className="flex w-full items-start gap-3 px-3 py-2.5 transition hover:bg-blue-50 focus-visible:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500"
              onClick={() => onNavigate(navigationTarget)}
              aria-label={`${row.label}: ${row.actionLabel ?? 'Mở nội dung liên quan'}`}
            >
              {content}
            </button>
          ) : (
            <div key={row.id} className="flex items-start gap-3 px-3 py-2.5">
              {content}
            </div>
          );
        })}
      </div>
    </details>
  );
}

function RequirementIcon({ state, kind }: { state: RequirementState; kind?: RequirementRow['kind'] }) {
  if (kind === 'permission') return <Shield className="mt-0.5 h-4 w-4 shrink-0 text-blue-600" />;
  if (state === 'done') return <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />;
  if (state === 'blocked') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />;
  if (state === 'warning') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />;
  return <Circle className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />;
}
