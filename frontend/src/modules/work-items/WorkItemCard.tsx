import clsx from 'clsx';
import { StatusBadge } from '../../components/StatusBadge';
import { dateText } from '../../lib/utils/format';
import type { WorkItem } from '../../lib/types/domain';
import { asArray, departmentLabel, hasWorkflowFormSummary, isWorkOverdue, numberValue, workItemIsUpcoming, workItemWorkflowContext, workTypeLabel, workflowFormChipText, workflowFormStatusLabel } from './workItemPresentation';

export function WorkItemCard({
  item,
  onOpen,
  compact = false,
  showWorkflowPrefix = false,
  displayStatus,
}: {
  item: WorkItem;
  onOpen: () => void;
  compact?: boolean;
  showWorkflowPrefix?: boolean;
  displayStatus?: string;
}) {
  const overdue = isWorkOverdue(item);
  const title = workItemTitle(item, showWorkflowPrefix);
  const context = workItemWorkflowContext(item);
  const upcoming = workItemIsUpcoming(item);
  return (
    <article className={clsx('rounded-md border border-line bg-white p-3', overdue && 'border-red-200 bg-red-50/50')}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={displayStatus ?? (overdue ? 'overdue' : item.status)} />
            {upcoming ? <span className="rounded-md bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-700">Sắp tới · chỉ xem</span> : null}
            {item.project_code ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{item.project_code}</span> : null}
            {item.department ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{departmentLabel(item.department)}</span> : null}
            {hasWorkflowFormSummary(item) ? (
              <span className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">
                {workflowFormChipText(item)} · {workflowFormStatusLabel(item.workflow_form_status)}
              </span>
            ) : null}
          </div>
          <p className={clsx('mt-2 font-semibold text-ink', compact ? 'text-sm' : 'text-base')}>{title}</p>
          <p className="mt-1 line-clamp-2 text-xs font-medium text-slate-600">{context.path}</p>
          {item.description && !compact ? <p className="mt-1 line-clamp-2 text-sm text-slate-600">{item.description}</p> : null}
          <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-slate-500">
            <span>Phụ trách: {item.assignee_name ?? (asArray<string>(item.role_ids).join(', ') || '-')}</span>
            <span>Hạn: {dateText(item.due_at)}</span>
            <span>Minh chứng: {numberValue(item.evidence_count)}</span>
            {hasWorkflowFormSummary(item) ? <span>Hồ sơ: {item.workflow_form_required ? 'Bắt buộc' : 'Tùy chọn'}</span> : null}
          </div>
        </div>
        <button
          className="shrink-0 rounded-md border border-line px-2.5 py-1.5 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
          onClick={onOpen}
        >
          {upcoming ? 'Xem' : 'Mở xử lý'}
        </button>
      </div>
      {item.blocked_reason ? <p className="mt-3 rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{item.blocked_reason}</p> : null}
    </article>
  );
}

function workItemTitle(item: WorkItem, showWorkflowPrefix: boolean) {
  if (!showWorkflowPrefix) return item.title;

  const prefix = [item.run_node_name, item.run_step_name, item.work_type ? workTypeLabel(item.work_type) : undefined]
    .find((value) => String(value ?? '').trim());
  const cleanPrefix = String(prefix ?? '').trim();
  const cleanTitle = item.title.trim();
  const normalizedPrefix = cleanPrefix.toLowerCase();
  const normalizedTitle = cleanTitle.toLowerCase();

  if (!cleanPrefix || normalizedTitle === normalizedPrefix || normalizedTitle.startsWith(`${normalizedPrefix} /`)) {
    return item.title;
  }

  return `${cleanPrefix} / ${item.title}`;
}
