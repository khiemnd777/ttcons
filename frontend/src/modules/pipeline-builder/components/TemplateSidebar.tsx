import type { ReactNode } from 'react';
import clsx from 'clsx';
import { Archive, ArrowLeft, ChevronDown, ChevronRight } from 'lucide-react';
import { StatusBadge } from '../../../components/StatusBadge';
import type { PipelineTemplate } from '../../../lib/types/domain';

type TemplateSidebarProps = {
  templates: PipelineTemplate[];
  selectedTemplateId: string;
  expanded: boolean;
  children: ReactNode;
  onBack: () => void;
  onToggleExpanded: () => void;
  onSelectTemplate: (template: PipelineTemplate) => void;
};

export function TemplateSidebar({
  templates,
  selectedTemplateId,
  expanded,
  children,
  onBack,
  onToggleExpanded,
  onSelectTemplate,
}: TemplateSidebarProps) {
  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-line bg-white">
      <div className="border-b border-line px-4 py-4">
        <button
          className="inline-flex w-full items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          onClick={onBack}
        >
          <ArrowLeft className="h-4 w-4" />
          Back
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-auto px-3 py-4">
        <section className="border-b border-line pb-4">
          <button
            className="mb-3 flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left hover:bg-slate-100"
            onClick={onToggleExpanded}
          >
            <span className="inline-flex items-center gap-2 text-sm font-semibold text-ink">
              <Archive className="h-4 w-4 text-slate-400" />
              Templates
            </span>
            {expanded ? <ChevronDown className="h-4 w-4 text-slate-400" /> : <ChevronRight className="h-4 w-4 text-slate-400" />}
          </button>
          {expanded ? (
            <div className="space-y-2">
              {templates.map((template) => (
                <button
                  key={template.id}
                  className={clsx(
                    'w-full rounded-md border px-3 py-3 text-left hover:border-blue-300 hover:bg-blue-50',
                    selectedTemplateId === template.id ? 'border-blue-300 bg-blue-50' : 'border-line',
                  )}
                  onClick={() => onSelectTemplate(template)}
                >
                  <p className="line-clamp-2 text-sm font-semibold text-ink">{template.name}</p>
                  <div className="mt-2 flex items-center justify-between gap-2">
                    <span className="text-xs text-slate-500">v{template.version} · {template.node_count ?? 0} node</span>
                    <StatusBadge status={template.status} />
                  </div>
                </button>
              ))}
            </div>
          ) : null}
        </section>

        {children}
      </div>
    </aside>
  );
}
