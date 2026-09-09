import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { AlertCircle, ArrowUpRight, CheckCircle2, GitBranch, MessageSquareText, PlayCircle } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { get } from '../../lib/api/client';
import type { ProjectOperationsTimelineNode, WorkItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { discussionWorkflowActionLabel } from './discussionWorkDomain';

export type DiscussionWorkflowAction = {
  hasPipeline: boolean;
  pipelineStatus?: string;
  currentNode?: ProjectOperationsTimelineNode;
  primaryWork?: WorkItem;
  onOpenStartup: () => void;
};

export function DiscussionWorkflowActionCard({
  projectId,
  workflow,
  onOpenWork,
  onDiscuss,
}: {
  projectId: string;
  workflow: DiscussionWorkflowAction;
  onOpenWork: (item: WorkItem) => void;
  onDiscuss: (item: WorkItem) => void;
}) {
  const primaryWorkId = workflow.hasPipeline ? workflow.primaryWork?.id ?? '' : '';
  const detail = useQuery({
    queryKey: ['work-item', primaryWorkId],
    queryFn: () => get<WorkItem>(`/work-items/${primaryWorkId}`),
    enabled: Boolean(primaryWorkId),
    retry: false,
    refetchOnMount: 'always',
    refetchOnWindowFocus: false,
  });

  if (!workflow.hasPipeline) {
    return (
      <section className="rounded-lg border border-blue-200 bg-blue-50/70 p-3 shadow-panel" aria-label="Bước tiếp theo của quy trình">
        <ActionHeading label="Bước tiếp theo" />
        <div className="mt-3 flex items-center gap-2 text-xs font-semibold text-slate-600">
          <span className="rounded bg-white px-2 py-1 ring-1 ring-blue-100">Chưa khởi động</span>
        </div>
        <h3 className="mt-3 text-sm font-semibold leading-5 text-ink">Hoàn tất thiết lập công trình</h3>
        <p className="mt-1 text-xs leading-5 text-slate-600">Kiểm tra thông tin, nhân sự, PM và điều kiện bước đầu trước khi tạo công việc thật.</p>
        <p className="mt-2 text-xs font-medium text-blue-800">Thông tin → Nhân sự → Thiết lập</p>
        <button
          type="button"
          className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
          onClick={workflow.onOpenStartup}
        >
          <PlayCircle className="h-4 w-4" />
          Hoàn tất thiết lập
        </button>
      </section>
    );
  }

  if (!workflow.primaryWork) {
    const completed = ['completed', 'closed'].includes(workflow.pipelineStatus ?? '');
    return (
      <section className="rounded-lg border border-line bg-white p-3 shadow-panel" aria-label="Bước tiếp theo của quy trình">
        <ActionHeading label="Bước tiếp theo" />
        <div className="mt-3 flex items-start gap-2">
          <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-600" />
          <div>
            <h3 className="text-sm font-semibold text-ink">{completed ? 'Quy trình đã hoàn tất' : 'Hiện không có việc cần can thiệp'}</h3>
            <p className="mt-1 text-xs leading-5 text-slate-600">
              {completed ? 'Có thể xem lại toàn bộ giai đoạn và lịch sử xử lý.' : 'Theo dõi giai đoạn hiện tại hoặc xem các công việc sắp tới trong Quy trình.'}
            </p>
          </div>
        </div>
        <WorkflowLink projectId={projectId} label={completed ? 'Xem lịch sử quy trình' : 'Xem quy trình'} />
      </section>
    );
  }

  const currentWork = detail.isFetchedAfterMount && detail.data?.project_id === projectId ? detail.data : undefined;
  if (!detail.isFetchedAfterMount && !detail.isError) {
    return <WorkflowActionLoading stageName={workflow.currentNode?.name ?? workflow.primaryWork.run_node_name} />;
  }
  if (detail.isError || !currentWork) {
    return (
      <section className="rounded-lg border border-rose-200 bg-rose-50 p-3 shadow-panel" aria-label="Bước tiếp theo của quy trình">
        <ActionHeading label="Bước tiếp theo" />
        <div className="mt-3 flex items-start gap-2">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
          <div className="min-w-0">
            <p className="text-sm font-semibold text-rose-800">Không tải được bước tiếp theo.</p>
            <button type="button" className="mt-1 text-xs font-semibold text-rose-700 underline" onClick={() => void detail.refetch()}>Thử lại</button>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="rounded-lg border border-blue-200 bg-white p-3 shadow-panel" aria-label="Bước tiếp theo của quy trình">
      <ActionHeading label="Bước tiếp theo" />
      <p className="mt-2 truncate text-xs font-semibold text-blue-700" title={workflow.currentNode?.name ?? currentWork.run_node_name}>
        {workflow.currentNode?.name ?? currentWork.run_node_name ?? 'Giai đoạn hiện tại'}
      </p>
      <div className="mt-2"><StatusBadge status={currentWork.status} /></div>
      <h3 className="mt-3 break-words text-sm font-semibold leading-5 text-ink">{currentWork.title}</h3>
      <dl className="mt-2 space-y-1 text-xs leading-5 text-slate-600">
        <div className="flex gap-1"><dt className="shrink-0">Phụ trách:</dt><dd className="min-w-0 font-semibold text-slate-800">{workOwner(currentWork)}</dd></div>
        <div className="flex gap-1"><dt>Hạn:</dt><dd className="font-semibold text-slate-800">{dateText(workDueDate(currentWork), '—')}</dd></div>
      </dl>
      {currentWork.blocked_reason ? <p className="mt-2 rounded-md bg-rose-50 px-2.5 py-2 text-xs leading-5 text-rose-800">{currentWork.blocked_reason}</p> : null}
      <button
        type="button"
        className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
        onClick={() => onOpenWork(currentWork)}
      >
        {discussionWorkflowActionLabel(currentWork)}
        <ArrowUpRight className="h-4 w-4" />
      </button>
      <button
        type="button"
        className="mt-2 inline-flex w-full items-center justify-center gap-2 rounded-md px-3 py-2 text-xs font-semibold text-blue-700 hover:bg-blue-50"
        onClick={() => onDiscuss(currentWork)}
      >
        <MessageSquareText className="h-4 w-4" />
        Trao đổi về việc này
      </button>
    </section>
  );
}

function ActionHeading({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2">
      <GitBranch className="h-4 w-4 text-blue-600" />
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
    </div>
  );
}

function WorkflowLink({ projectId, label }: { projectId: string; label: string }) {
  return (
    <Link className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-xs font-semibold text-blue-700 hover:bg-blue-50" to={`/projects/${projectId}/pipeline`}>
      {label}
      <ArrowUpRight className="h-3.5 w-3.5" />
    </Link>
  );
}

function WorkflowActionLoading({ stageName }: { stageName?: string }) {
  return (
    <section className="rounded-lg border border-line bg-white p-3 shadow-panel" aria-label="Đang tải bước tiếp theo" aria-busy="true">
      <ActionHeading label="Bước tiếp theo" />
      {stageName ? <p className="mt-2 truncate text-xs font-semibold text-blue-700">{stageName}</p> : null}
      <div className="mt-3 space-y-2" role="status">
        <span className="sr-only">Đang tải bước tiếp theo…</span>
        <div className="h-5 w-20 animate-pulse rounded bg-slate-200" />
        <div className="h-4 w-4/5 animate-pulse rounded bg-slate-200" />
        <div className="h-9 w-full animate-pulse rounded bg-slate-200" />
      </div>
    </section>
  );
}

function workOwner(item: WorkItem) {
  if (item.assignee_name) return identityReferenceText(item.assignee_name, item.assignee_account_deleted);
  return item.owner_name || item.responsible_name || item.department_name || 'Chưa phân công';
}

function workDueDate(item: WorkItem) {
  return item.due_date ?? item.due_at ?? item.deadline ?? item.planned_end_date;
}
