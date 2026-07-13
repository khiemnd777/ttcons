import type { ReactNode } from 'react';
import clsx from 'clsx';
import { Archive, ArrowLeft, ChevronDown, ChevronRight, Plus, Trash2 } from 'lucide-react';
import { StatusBadge } from '../../../components/StatusBadge';
import type { PipelineTemplate } from '../../../lib/types/domain';

type TemplateSidebarProps = {
  templates: PipelineTemplate[];
  selectedTemplateId: string;
  expanded: boolean;
  children: ReactNode;
  showBackButton?: boolean;
  createTemplatePending: boolean;
  deleteTemplatePendingId?: string;
  onBack: () => void;
  onCreateTemplate: () => void;
  onDeleteTemplate: (template: PipelineTemplate) => void;
  onToggleExpanded: () => void;
  onSelectTemplate: (template: PipelineTemplate) => void;
};

export function TemplateSidebar({
  templates,
  selectedTemplateId,
  expanded,
  children,
  showBackButton = true,
  createTemplatePending,
  deleteTemplatePendingId,
  onBack,
  onCreateTemplate,
  onDeleteTemplate,
  onToggleExpanded,
  onSelectTemplate,
}: TemplateSidebarProps) {
  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-line bg-white">
      {showBackButton ? (
        <div className="border-b border-line px-4 py-4">
          <button
            type="button"
            title="Về trang Công trình"
            className="inline-flex w-full items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            onClick={onBack}
          >
            <ArrowLeft className="h-4 w-4" />
            Quay lại
          </button>
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-auto px-3 py-4">
        <section className="border-b border-line pb-4">
          <div className="mb-3 flex items-center gap-1">
            <div className="flex min-w-0 flex-1 items-center gap-2 px-2 py-1.5 text-sm font-semibold text-ink">
              <Archive className="h-4 w-4 text-slate-400" />
              Templates
            </div>
            <button
              title="Tạo template"
              disabled={createTemplatePending}
              className="rounded-md bg-blue-600 p-1.5 text-white hover:bg-blue-700 disabled:opacity-50"
              onClick={onCreateTemplate}
            >
              <Plus className="h-4 w-4" />
            </button>
            <button
              title={expanded ? 'Thu gọn templates' : 'Mở rộng templates'}
              className="rounded-md border border-line p-1.5 text-slate-600 hover:bg-slate-100"
              onClick={onToggleExpanded}
            >
              {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </button>
          </div>
          {expanded ? (
            <div className="space-y-2">
              {templates.map((template) => (
                <div
                  key={template.id}
                  className={clsx(
                    'group flex w-full gap-2 rounded-md border px-3 py-3 hover:border-blue-300 hover:bg-blue-50',
                    selectedTemplateId === template.id ? 'border-blue-300 bg-blue-50' : 'border-line',
                  )}
                >
                  <button className="min-w-0 flex-1 text-left" onClick={() => onSelectTemplate(template)}>
                    <p className="line-clamp-2 text-sm font-semibold text-ink">{template.name}</p>
                    <div className="mt-2 flex items-center justify-between gap-2">
                      <span className="text-xs text-slate-500">v{template.version} · {template.node_count ?? 0} node</span>
                      <StatusBadge status={template.status} />
                    </div>
                  </button>
                  <button
                    title="Xóa template"
                    disabled={deleteTemplatePendingId === template.id}
                    className="h-8 w-8 shrink-0 rounded-md p-2 text-slate-300 hover:bg-rose-50 hover:text-rose-600 disabled:opacity-50"
                    onClick={() => onDeleteTemplate(template)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </div>
          ) : null}
        </section>

        {children}
      </div>
    </aside>
  );
}
