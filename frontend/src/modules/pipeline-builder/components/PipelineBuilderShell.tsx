import type { ReactNode } from 'react';
import clsx from 'clsx';
import { CheckCircle2, Copy, Download, Pencil, Send, Trash2, Wand2 } from 'lucide-react';
import { StatusBadge } from '../../../components/StatusBadge';

type PipelineBuilderShellProps = {
  sidebar: ReactNode;
  children: ReactNode;
  inspector?: ReactNode;
  templateName?: string;
  templateStatus?: string;
  templateDescription?: string;
  selectedTemplateId: string;
  isReadOnly: boolean;
  isPublished: boolean;
  canEditLayout: boolean;
  canDeleteSelection: boolean;
  deleteSelectionLabel: string;
  nodeCount: number;
  validationIssueCount: number;
  autoLayoutPending: boolean;
  canExportPDF: boolean;
  exportPDFPending: boolean;
  deleteSelectionPending: boolean;
  onRenameTemplate: () => void;
  onCloneTemplate: () => void;
  onPublishTemplate: () => void;
  onAutoLayout: () => void;
  onValidatePipeline: () => void;
  onExportPDF: () => void;
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
  isReadOnly,
  isPublished,
  canEditLayout,
  canDeleteSelection,
  deleteSelectionLabel,
  nodeCount,
  validationIssueCount,
  autoLayoutPending,
  canExportPDF,
  exportPDFPending,
  deleteSelectionPending,
  onRenameTemplate,
  onCloneTemplate,
  onPublishTemplate,
  onAutoLayout,
  onValidatePipeline,
  onExportPDF,
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
                  disabled={!selectedTemplateId || isReadOnly}
                  className="rounded-md border border-line p-1.5 text-slate-500 hover:bg-slate-100 hover:text-ink disabled:opacity-50"
                  onClick={onRenameTemplate}
                >
                  <Pencil className="h-4 w-4" />
                </button>
              </div>
              <p className="mt-1 line-clamp-1 text-sm text-slate-500">{templateDescription ?? 'Chọn template hoặc tạo template mới để bắt đầu.'}</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <button title="Tạo revision nháp mới" disabled={!selectedTemplateId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={onCloneTemplate}>
                <Copy className="h-4 w-4" />
                Tạo revision
              </button>
              <button title="Phát hành revision" disabled={!selectedTemplateId || isReadOnly} className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-50" onClick={onPublishTemplate}>
                <Send className="h-4 w-4" />
                Phát hành
              </button>
              <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={onAutoLayout} disabled={!nodeCount || autoLayoutPending || !canEditLayout}>
                <Wand2 className="h-4 w-4" />
                Tự sắp xếp
              </button>
              <button className={clsx('inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold', validationIssueCount ? 'border border-amber-200 bg-amber-50 text-amber-800 hover:bg-amber-100' : 'border border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100')} onClick={onValidatePipeline} disabled={!selectedTemplateId}>
                <CheckCircle2 className="h-4 w-4" />
                Kiểm tra
              </button>
              <button
                title="Xuất mô tả node và checklist/step thành PDF"
                className="inline-flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-50"
                onClick={onExportPDF}
                disabled={!canExportPDF || exportPDFPending}
              >
                <Download className={clsx('h-4 w-4', exportPDFPending && 'animate-pulse')} />
                {exportPDFPending ? 'Đang xuất...' : 'Xuất PDF'}
              </button>
              <button className="inline-flex items-center gap-2 rounded-md border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-50" onClick={onDeleteSelection} disabled={!canDeleteSelection || deleteSelectionPending || isReadOnly}>
                <Trash2 className="h-4 w-4" />
                {deleteSelectionLabel}
              </button>
            </div>
          </div>
        </div>

        {isPublished && canEditLayout ? (
          <div className="border-b border-blue-200 bg-blue-50 px-4 py-2 text-sm text-blue-900">
            Revision đã phát hành khóa nội dung workflow. Vẫn có thể kéo node, <strong>Tự sắp xếp</strong> và đổi cạnh bám của connector trên chính node hiện tại.
          </div>
        ) : isPublished ? (
          <div className="border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900">
            Revision đã phát hành đang ở chế độ chỉ đọc. Chọn <strong>Tạo revision</strong> để chỉnh sửa mà không làm thay đổi các công trình lịch sử.
          </div>
        ) : isReadOnly ? (
          <div className="border-b border-slate-200 bg-slate-50 px-4 py-2 text-sm text-slate-700">
            Tài khoản hiện có quyền xem workflow. Cần quyền <strong>pipeline.edit</strong> để thay đổi nội dung hoặc bố cục.
          </div>
        ) : null}

        {children}
        {inspector}
      </main>
    </div>
  );
}
