import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import clsx from 'clsx';

export type WorkspaceTabItem<T extends string> = {
  id: T;
  label: string;
  icon: LucideIcon;
  count?: number | string;
};

export function WorkspaceTabs<T extends string>({
  tabs,
  activeTab,
  onChange,
  ariaLabel,
  actions,
  className,
}: {
  tabs: WorkspaceTabItem<T>[];
  activeTab: T;
  onChange: (tab: T) => void;
  ariaLabel: string;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <div className={clsx('flex min-w-0 items-end gap-2 border-b border-line', className)} data-workspace-tabs>
      <div className="min-w-0 flex-1 overflow-x-auto">
        <div className="flex min-w-max gap-1" role="tablist" aria-label={ariaLabel}>
          {tabs.map((tab) => {
            const active = tab.id === activeTab;
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                type="button"
                role="tab"
                aria-selected={active}
                className={clsx(
                  'inline-flex items-center gap-2 whitespace-nowrap border-b-2 px-3 py-2 text-sm font-semibold transition focus-ring',
                  active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-600 hover:border-slate-300 hover:text-ink',
                )}
                onClick={() => onChange(tab.id)}
              >
                <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                {tab.label}
                {typeof tab.count === 'number' || typeof tab.count === 'string' ? (
                  <span
                    className={clsx(
                      'rounded px-1.5 py-0.5 text-xs',
                      active ? 'bg-blue-50 text-blue-700' : 'bg-slate-100 text-slate-500',
                    )}
                  >
                    {tab.count}
                  </span>
                ) : null}
              </button>
            );
          })}
        </div>
      </div>
      {actions ? <div className="flex shrink-0 items-center pb-1">{actions}</div> : null}
    </div>
  );
}
