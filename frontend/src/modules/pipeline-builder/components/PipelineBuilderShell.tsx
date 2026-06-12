import type { ReactNode } from 'react';
import clsx from 'clsx';
import { CheckCircle2, Copy, Pencil, Send, Trash2, Wand2 } from 'lucide-react';
import { StatusBadge } from '../../../components/StatusBadge';

type PipelineBuilderShellProps = {
  sidebar: ReactNode;
  children: ReactNode;
  inspector?: ReactNode;
  templateName?: string;
  templateStatus?: string;
  templateDescription?: string;
  selectedTemplateId: string;
  canDeleteSelection: boolean;
  deleteSelectionLabel: string;
  nodeCount: number;
  validationIssueCount: number;
  autoLayoutPending: boolean;
  deleteSelectionPending: boolean;
  onRenameTemplate: () => void;
  onCloneTemplate: () => void;
  onPublishTemplate: () => void;
  onAutoLayout: () => void;
  onValidatePipeline: () => void;
  onDeleteSelection: () => void;
};

export function PipelineBuilderShell({
  sidebar,
  children,
  inspector,
  templateName,
  templateStatus,
  templateDescription,
  selectedTemplateId,
  canDeleteSelection,
  deleteSelectionLabel,
  nodeCount,
  validationIssueCount,
  autoLayoutPending,
  deleteSelectionPending,
  onRenameTemplate,
  onCloneTemplate,
  onPublishTemplate,
  onAutoLayout,
  onValidatePipeline,
  onDeleteSelection,
}: PipelineBuilderShellProps) {
  return (
    <div className="flex h-screen overflow-hidden bg-surface">
      {sidebar}

      <main className="relative flex min-w-0 flex-1 flex-col overflow-hidden bg-white">
        <div className="border-b border-line px-4 py-3">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-base font-semibold text-ink">Workflow</h1>
                {templateName ? <span className="truncate text-sm font-medium text-slate-500">/ {templateName}</span> : null}
                {templateStatus ? <StatusBadge status={templateStatus} /> : null}
                <button
                  title="Đổi tên template"
                  disabled={!selectedTemplateId}
                  className="rounded-md border border-line p-1.5 text-slate-500 hover:bg-slate-100 hover:text-ink disabled:opacity-50"
                  onClick={onRenameTemplate}
                >
                  <Pencil className="h-4 w-4" />
                </button>
              </div>
              <p className="mt-1 line-clamp-1 text-sm text-slate-500">{templateDescription ?? 'Chọn template hoặc tạo template mới để bắt đầu.'}</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <button title="Clone template" disabled={!selectedTemplateId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={onCloneTemplate}>
                <Copy className="h-4 w-4" />
                Clone
              </button>
              <button title="Publish template" disabled={!selectedTemplateId} className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-50" onClick={onPublishTemplate}>
                <Send className="h-4 w-4" />
                Publish
              </button>
              <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={onAutoLayout} disabled={!nodeCount || autoLayoutPending}>
                <Wand2 className="h-4 w-4" />
                Tự sắp xếp
              </button>
              <button className={clsx('inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold', validationIssueCount ? 'border border-amber-200 bg-amber-50 text-amber-800 hover:bg-amber-100' : 'border border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100')} onClick={onValidatePipeline} disabled={!selectedTemplateId}>
                <CheckCircle2 className="h-4 w-4" />
                Kiểm tra
              </button>
              <button className="inline-flex items-center gap-2 rounded-md border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-50" onClick={onDeleteSelection} disabled={!canDeleteSelection || deleteSelectionPending}>
                <Trash2 className="h-4 w-4" />
                {deleteSelectionLabel}
              </button>
            </div>
          </div>
        </div>

        {children}
        {inspector}
      </main>
    </div>
  );
}
