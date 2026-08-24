import clsx from 'clsx';
import { ArrowUpRight } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import { IdentityReferenceLabel } from '../../components/IdentityReferenceLabel';
import { dateText } from '../../lib/utils/format';
import type { WorkItem } from '../../lib/types/domain';
import { asArray, departmentLabel, hasWorkflowFormSummary, isWorkOverdue, numberValue, workItemIsUpcoming, workItemWorkflowContext, workTypeLabel, workflowFormStatusLabel } from './workItemPresentation';
import { executionTargetActionLabel } from './workItemExecutionTarget';

export function WorkItemCard({
  item,
  onOpen,
  onSelect,
  selected = false,
  compact = false,
  showWorkflowPrefix = false,
  displayStatus,
}: {
  item: WorkItem;
  onOpen: () => void;
  onSelect?: () => void;
  selected?: boolean;
  compact?: boolean;
  showWorkflowPrefix?: boolean;
  displayStatus?: string;
}) {
  const overdue = isWorkOverdue(item);
  const title = workItemTitle(item, showWorkflowPrefix);
  const context = workItemWorkflowContext(item);
  const upcoming = workItemIsUpcoming(item);
  const ownerRoles = uniqueLabels(asArray<string>(item.role_ids));
  const openLabel = upcoming ? 'Xem trước công việc' : executionTargetActionLabel(item);
  return (
    <article className={clsx(
      'group relative flex border-b border-line transition-colors last:border-b-0',
      selected ? 'bg-blue-50/70' : overdue ? 'bg-rose-50/40' : upcoming ? 'bg-amber-50/30' : 'bg-white hover:bg-slate-50',
    )}>
      {selected ? <span className="absolute inset-y-2 left-0 w-1 rounded-r bg-blue-600" aria-hidden="true" /> : null}
      <button
        type="button"
        className={clsx('min-w-0 flex-1 px-4 py-3 text-left outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500', compact && 'py-2.5')}
        aria-pressed={selected}
        onClick={onSelect ?? onOpen}
      >
        <span className="flex min-w-0 flex-col gap-1.5">
          <span className="flex min-w-0 flex-wrap items-center gap-2">
            <StatusBadge status={displayStatus ?? (overdue ? 'overdue' : item.status)} />
            <span className={clsx('min-w-0 flex-1 truncate font-semibold text-ink', compact ? 'text-sm' : 'text-[15px]')} title={title}>{title}</span>
            {upcoming ? <span className="text-xs font-semibold text-amber-700">Sắp tới · chỉ xem</span> : null}
          </span>
          <span className="truncate text-xs font-medium text-slate-600" title={context.path}>{context.path}</span>
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-slate-500">
            <span className="inline-flex items-center gap-1">Phụ trách: {item.assignee_name ? <IdentityReferenceLabel name={item.assignee_name} deleted={item.assignee_account_deleted} liveOwnership /> : (ownerRoles.join(', ') || departmentLabel(item.department) || '—')}</span>
            <span aria-hidden="true">·</span>
            <span>{item.due_at ? `Hạn ${dateText(item.due_at)}` : 'Chưa có hạn'}</span>
            <span aria-hidden="true">·</span>
            <span>{numberValue(item.evidence_count)} minh chứng</span>
            {hasWorkflowFormSummary(item) ? <><span aria-hidden="true">·</span><span>Hồ sơ {item.workflow_form_required ? 'bắt buộc' : 'tùy chọn'} · {workflowFormStatusLabel(item.workflow_form_status)}</span></> : null}
          </span>
          {item.blocked_reason ? <span className="line-clamp-1 text-xs font-medium text-rose-700" title={item.blocked_reason}>Điểm chặn: {item.blocked_reason}</span> : null}
        </span>
      </button>
      <button
        type="button"
        className="m-2 ml-0 inline-flex w-10 shrink-0 items-center justify-center self-stretch rounded-md text-blue-700 outline-none hover:bg-blue-100 focus-visible:ring-2 focus-visible:ring-blue-500"
        aria-label={openLabel}
        title={openLabel}
        onClick={onOpen}
      >
        <ArrowUpRight className="h-4 w-4" />
      </button>
    </article>
  );
}

function uniqueLabels(values: string[]) {
  return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)));
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
