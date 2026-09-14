import { useMemo, useState } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Ban, CheckCircle2, ChevronDown, ClipboardList, GitBranch, Play, Search, Send, Settings2 } from 'lucide-react';
import { ApiError, get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { EnumChip } from '../../components/EnumChip';
import { Dialog } from '../../components/Dialog';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import type { ProjectOperationsResponse, ProjectOperationsTimelineNode, WorkItem } from '../../lib/types/domain';
import { enumLabel, statusLabel } from '../../lib/utils/status';
import { ProjectWorkflowWorkspace } from '../projects/ProjectDetailPage';
import { currentWorkspaceReturnTo, projectPipelinePath, projectTaskDetailNavigationState } from '../work-items/workItemNavigation';
import { executionTargetNeedsEmbeddedWorkItem, executionTargetPath } from '../work-items/workItemExecutionTarget';
import {
  asArray,
  hasWorkflowFormSummary,
  numberValue,
  workflowFormBusinessLabel,
  workflowFormChipText,
  workflowFormNextAction,
} from '../work-items/workItemPresentation';
import { ConstructionHandoffPanel } from './ConstructionHandoffPanel';
import { ApprovalPanel } from '../../components/ApprovalPanel';
import type { ApprovalRequest } from '../../lib/types/domain';

export function PlanningPage({ mode = 'planning' }: { mode?: 'planning' | 'pipeline' }) {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const toast = useToast();
  const selectedNodeId = new URLSearchParams(location.search).get('stage') ?? '';
  const [blockReason, setBlockReason] = useState('');
  const [blockNodeId, setBlockNodeId] = useState('');
  const [recreateOpen, setRecreateOpen] = useState(false);
  const [workSearch, setWorkSearch] = useState('');
  const [queueFilter, setQueueFilter] = useState<PlanningQueueFilter>('actionable');
  const [workTypeFilter, setWorkTypeFilter] = useState('all');
  const [workStatusFilter, setWorkStatusFilter] = useState('all');
  const [workRequirementFilter, setWorkRequirementFilter] = useState('all');
  const run = useQuery({
    queryKey: ['pipeline-run', id],
    queryFn: () => get<Record<string, any>>(`/projects/${id}/pipeline-run`),
    enabled: Boolean(id),
    retry: false,
  });
  const operations = useQuery({
    queryKey: ['project-operations', id, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${id}/operations`),
    enabled: Boolean(id) && Boolean(run.data),
    retry: false,
  });
  const approvals = useQuery({
    queryKey: ['approvals', 'pending', id],
    queryFn: () => get<ApprovalRequest[]>(`/approvals/pending?project_id=${encodeURIComponent(id)}`),
    enabled: Boolean(id) && Boolean(run.data),
    retry: false,
  });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['pipeline-run', id] });
    queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
    queryClient.invalidateQueries({ queryKey: ['work-item'] });
    queryClient.invalidateQueries({ queryKey: ['work-item-form'] });
    queryClient.invalidateQueries({ queryKey: ['approvals'] });
  };
  const start = useMutation({
    mutationFn: () => post(`/projects/${id}/start-pipeline`, {}),
    onSuccess: () => {
      invalidate();
      setRecreateOpen(false);
      toast.success('Đã khởi động quy trình đang chạy');
    },
    onError: (error) => toast.error('Không khởi động được quy trình đang chạy', error),
  });
  const requestApproval = useMutation({
    mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/request-approval`, { reason: 'Yêu cầu rà soát/phê duyệt để chuyển node' }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã gửi yêu cầu duyệt');
    },
    onError: (error) => toast.error('Không gửi được yêu cầu duyệt', error),
  });
  const advance = useMutation({
    mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/advance`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã chuyển node');
    },
    onError: (error) => toast.error('Không chuyển được node', error),
  });
  const block = useMutation({
    mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/block`, { reason: blockReason || 'Bị chặn bởi điều kiện vận hành' }),
    onSuccess: () => {
      invalidate();
      setBlockNodeId('');
      setBlockReason('');
      toast.success('Đã chặn node');
    },
    onError: (error) => toast.error('Không chặn được node', error),
  });

  const nodes = run.data?.nodes ?? [];
  const currentNode = nodes.find((node: Record<string, any>) => node.status === 'in_progress') ?? nodes.find((node: Record<string, any>) => node.status === 'blocked') ?? nodes[0];
  const planningWorkRows = useMemo(() => buildPlanningWorkRows(nodes, run.data), [nodes, run.data]);
  const planningWorkMetrics = useMemo(() => buildPlanningWorkMetrics(planningWorkRows), [planningWorkRows]);
  const planningProgress = useMemo(() => buildPlanningProgress(nodes), [nodes]);
  const nextPlanningWork = useMemo(() => selectNextPlanningWork(planningWorkRows), [planningWorkRows]);
  const filteredPlanningWorkRows = useMemo(() => (
    planningWorkRows.filter((row) => planningWorkRowMatchesFilters(row, {
      search: workSearch,
      queue: queueFilter,
      type: workTypeFilter,
      status: workStatusFilter,
      requirement: workRequirementFilter,
    }))
  ), [planningWorkRows, queueFilter, workRequirementFilter, workSearch, workStatusFilter, workTypeFilter]);

  function openStepWorkItem(step: Record<string, any>) {
    const workItem = stepToWorkItem(step, run.data);
    if (!workItem) {
      toast.warn({ title: 'Bước chưa có đầu việc', description: 'Quy trình đang chạy chưa tạo đầu việc cho bước này.' });
      return;
    }
    const enriched = operations.data?.work_items?.find((item) => item.id === workItem.id) ?? workItem;
    const targetPath = executionTargetPath(enriched, currentWorkspaceReturnTo(location));
    if (executionTargetNeedsEmbeddedWorkItem(enriched.execution_target)) {
      navigate(targetPath, { state: projectTaskDetailNavigationState(enriched.id, location.state) });
      return;
    }
    navigate(targetPath);
  }

  if (run.isLoading) return mode === 'planning' ? <PlanningWorkspaceLoading /> : <LoadingSpinner />;

  const runNotFound = run.error instanceof ApiError && run.error.status === 404;
  const terminalRun = ['completed', 'cancelled'].includes(String(run.data?.status ?? ''));

  return (
    <>
      <PageHeader
        density="compact"
        title={mode === 'pipeline' ? 'Quy trình' : 'Kế hoạch công việc'}
        description={run.data ? `${run.data.project_name} · Mẫu: ${run.data.template_name}` : 'Chưa có quy trình đang chạy cho công trình này.'}
        actions={
          mode === 'pipeline' ? (
            run.data ? (
              <PipelineManagementMenu onRecreate={() => setRecreateOpen(true)} />
            ) : null
          ) : (
            <>
              <ProjectBackLink projectId={id} />
              {run.data ? (
                <button
                  className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
                  onClick={() => navigate(`/projects/${id}/pipeline`)}
                >
                  <GitBranch className="h-4 w-4" />
                  Xem quy trình
                </button>
              ) : null}
              {terminalRun ? (
                <button
                  className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={start.isPending}
                  onClick={() => start.mutate()}
                >
                  <Play className="h-4 w-4" />
                  Bắt đầu lượt quy trình mới
                </button>
              ) : null}
            </>
          )
        }
      />
      {run.isError && !runNotFound ? (
        <EmptyState
          title="Không tải được kế hoạch công trình."
          action={
            <button
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
              onClick={() => run.refetch()}
            >
              Thử lại
            </button>
          }
        />
      ) : !run.data ? (
        <EmptyState
          title="Chưa có quy trình đang chạy. Khởi động quy trình để tạo bản vận hành từ mẫu."
          action={
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={start.isPending}
              onClick={() => start.mutate()}
            >
              <Play className="h-4 w-4" />
              Khởi động quy trình
            </button>
          }
        />
      ) : mode === 'planning' ? (
        <div className="space-y-4">
          <ConstructionHandoffPanel projectId={id} currentNode={currentNode} />
          <PlanningActionWorkspace
            rows={planningWorkRows}
            filteredRows={filteredPlanningWorkRows}
            metrics={planningWorkMetrics}
            progress={planningProgress}
            currentNode={currentNode}
            nextWork={nextPlanningWork}
            search={workSearch}
            queueFilter={queueFilter}
            typeFilter={workTypeFilter}
            statusFilter={workStatusFilter}
            requirementFilter={workRequirementFilter}
            onSearchChange={setWorkSearch}
            onQueueFilterChange={setQueueFilter}
            onTypeFilterChange={setWorkTypeFilter}
            onStatusFilterChange={setWorkStatusFilter}
            onRequirementFilterChange={setWorkRequirementFilter}
            onOpenRow={(row) => openStepWorkItem(row.step)}
            onOpenPipeline={() => navigate(`/projects/${id}/pipeline`)}
          />
        </div>
      ) : operations.isLoading ? (
        <PipelineWorkspaceLoading />
      ) : operations.error ? (
        <PipelineWorkspaceError onRetry={() => operations.refetch()} />
      ) : (
        <ProjectWorkflowWorkspace
          projectId={id}
          nodes={asArray<ProjectOperationsTimelineNode>(operations.data?.workflow_timeline)}
          workItems={asArray<WorkItem>(operations.data?.work_items)}
          selectedNodeId={selectedNodeId}
          hasPipeline={Boolean(run.data)}
          startPending={start.isPending}
          onSelectNode={(nodeId) => navigate(projectPipelinePath(id, nodeId, location), { state: location.state })}
          onStartPipeline={() => start.mutate()}
          onOpenWork={(item) => navigate(executionTargetPath(item, projectPipelinePath(id, item.run_node_id, location)))}
          renderStageContext={(timelineNode) => {
            const runNode = nodes.find((node: Record<string, any>) => String(node.id) === timelineNode.id);
            const approval = approvals.data?.find((request) => request.target_type === 'pipeline_run_node' && request.target_id === timelineNode.id);
            return (
              <div className="space-y-3">
                {approvals.isLoading && String((runNode ?? timelineNode).status) === 'waiting_approval' ? <div className="rounded-md border border-line bg-slate-50 px-3 py-3 text-sm text-slate-600">Đang tải yêu cầu phê duyệt…</div> : null}
                {approvals.isError && String((runNode ?? timelineNode).status) === 'waiting_approval' ? <div className="flex items-center justify-between gap-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-3 text-sm text-rose-700"><span>Không tải được quyền và hành động phê duyệt.</span><button type="button" className="font-semibold" onClick={() => approvals.refetch()}>Thử lại</button></div> : null}
                <ApprovalPanel request={approval} onChanged={invalidate} />
                <PipelineStageContext
                  node={runNode ?? timelineNode}
                  approvalPending={Boolean(approval)}
                  actionPending={requestApproval.isPending || advance.isPending || block.isPending}
                  onRequestApproval={() => requestApproval.mutate(timelineNode.id)}
                  onAdvance={() => advance.mutate(timelineNode.id)}
                  onBlock={() => setBlockNodeId(timelineNode.id)}
                />
              </div>
            );
          }}
        />
      )}

      <Dialog
        open={recreateOpen}
        title="Tạo lại quy trình công trình?"
        description="Hành động này gọi lại quy trình khởi động hiện có. Các điều kiện bảo vệ runtime ở backend vẫn được giữ nguyên."
        onClose={() => setRecreateOpen(false)}
        size="sm"
        footer={(
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              onClick={() => setRecreateOpen(false)}
            >
              Hủy
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:opacity-60"
              disabled={start.isPending}
              onClick={() => start.mutate()}
            >
              <Play className="h-4 w-4" />
              Xác nhận tạo lại
            </button>
          </div>
        )}
      >
        <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-3 text-sm text-amber-900">
          Chỉ thực hiện khi anh muốn bắt đầu lại quy trình vận hành của công trình này.
        </div>
      </Dialog>

      <Dialog
        open={Boolean(blockNodeId)}
        title="Chặn giai đoạn"
        description="Ghi rõ nguyên nhân để người liên quan có thể xử lý và truy vết."
        onClose={() => {
          setBlockNodeId('');
          setBlockReason('');
        }}
        size="sm"
        footer={(
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              onClick={() => {
                setBlockNodeId('');
                setBlockReason('');
              }}
            >
              Hủy
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:opacity-60"
              disabled={!blockReason.trim() || block.isPending}
              onClick={() => blockNodeId && block.mutate(blockNodeId)}
            >
              <Ban className="h-4 w-4" />
              Xác nhận chặn
            </button>
          </div>
        )}
      >
        <label className="block">
          <span className="text-sm font-semibold text-slate-700">Lý do chặn <span className="text-rose-600">*</span></span>
          <textarea
            className="mt-2 min-h-24 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100"
            value={blockReason}
            onChange={(event) => setBlockReason(event.target.value)}
            placeholder="Ví dụ: Chưa đủ hồ sơ khảo sát để yêu cầu phê duyệt"
          />
        </label>
      </Dialog>
    </>
  );
}

function PipelineManagementMenu({ onRecreate }: { onRecreate: () => void }) {
  return (
    <details className="group relative">
      <summary className="inline-flex cursor-pointer list-none items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:border-blue-300 hover:bg-blue-50 [&::-webkit-details-marker]:hidden">
        <Settings2 className="h-4 w-4" />
        Quản lý quy trình
        <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
      </summary>
      <div className="absolute right-0 z-20 mt-2 w-64 rounded-md border border-line bg-white p-1.5 shadow-xl">
        <button
          type="button"
          className="flex w-full items-start gap-2 rounded px-3 py-2 text-left text-sm text-rose-700 hover:bg-rose-50"
          onClick={onRecreate}
        >
          <Play className="mt-0.5 h-4 w-4 shrink-0" />
          <span>
            <span className="block font-semibold">Tạo lại quy trình…</span>
            <span className="mt-0.5 block text-xs text-slate-500">Yêu cầu xác nhận trước khi thực hiện</span>
          </span>
        </button>
      </div>
    </details>
  );
}

function PipelineStageContext({
  node,
  approvalPending,
  actionPending,
  onRequestApproval,
  onAdvance,
  onBlock,
}: {
  node: Record<string, any>;
  approvalPending: boolean;
  actionPending: boolean;
  onRequestApproval: () => void;
  onAdvance: () => void;
  onBlock: () => void;
}) {
  const roleLabels = asArray<string>(node.responsible_role_ids).filter(Boolean).join(', ') || '-';

  return (
    <div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 px-3 py-3 lg:flex-row lg:items-center lg:justify-between">
      <dl className="flex min-w-0 flex-wrap gap-x-5 gap-y-2 text-xs">
        <div>
          <dt className="font-semibold text-slate-500">Vai trò</dt>
          <dd className="mt-0.5 font-medium text-slate-700">{roleLabels}</dd>
        </div>
        <div>
          <dt className="font-semibold text-slate-500">Phê duyệt</dt>
          <dd className="mt-0.5 font-medium text-slate-700">{node.approval_rule ?? '-'}</dd>
        </div>
        <div>
          <dt className="font-semibold text-slate-500">Nhánh khi không đạt</dt>
          <dd className="mt-0.5"><EnumChip group="fail_path" value={node.fail_path} /></dd>
        </div>
      </dl>

      <details className="group relative shrink-0">
        <summary className="inline-flex w-full cursor-pointer list-none items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:border-blue-300 hover:bg-blue-50 lg:w-auto [&::-webkit-details-marker]:hidden">
          Thao tác giai đoạn
          <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
        </summary>
        <div className="right-0 z-20 mt-2 w-full rounded-md border border-line bg-white p-1.5 shadow-xl lg:absolute lg:w-64">
          <button
            type="button"
            className="flex w-full items-center gap-2 rounded px-3 py-2 text-left text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50"
            disabled={actionPending || approvalPending}
            onClick={onRequestApproval}
          >
            <Send className="h-4 w-4" />
            {approvalPending ? 'Đang chờ phê duyệt' : 'Yêu cầu phê duyệt'}
          </button>
          <button
            type="button"
            className="flex w-full items-center gap-2 rounded px-3 py-2 text-left text-sm font-semibold text-emerald-700 hover:bg-emerald-50 disabled:opacity-50"
            disabled={actionPending}
            onClick={onAdvance}
          >
            <CheckCircle2 className="h-4 w-4" />
            Chuyển giai đoạn tiếp theo
          </button>
          <button
            type="button"
            className="flex w-full items-center gap-2 rounded px-3 py-2 text-left text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-50"
            disabled={actionPending}
            onClick={onBlock}
          >
            <Ban className="h-4 w-4" />
            Chặn giai đoạn…
          </button>
        </div>
      </details>
    </div>
  );
}

function PipelineWorkspaceLoading() {
  return (
    <div className="space-y-4" aria-label="Đang tải quy trình công trình">
      <div className="h-36 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
      <div className="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
        <div className="h-96 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        <div className="h-96 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
      </div>
    </div>
  );
}

function PipelineWorkspaceError({ onRetry }: { onRetry: () => void }) {
  return (
    <section className="rounded-lg border border-rose-200 bg-white p-5 shadow-panel">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="font-semibold text-rose-700">Không tải được dữ liệu quy trình</h2>
          <p className="mt-1 text-sm text-slate-600">Vui lòng thử lại hoặc kiểm tra quyền truy cập công trình.</p>
        </div>
        <button
          type="button"
          className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
          onClick={onRetry}
        >
          Thử lại
        </button>
      </div>
    </section>
  );
}

type PlanningQueueFilter = 'actionable' | 'waiting' | 'future' | 'completed' | 'all';

type PlanningWorkRow = {
  id: string;
  step: Record<string, any>;
  node: Record<string, any>;
  workItem: WorkItem | null;
  title: string;
  formTitle?: string;
  formStatus: string;
  workStatus: string;
  workType: string;
  workTypeLabel: string;
  queue: PlanningQueueFilter;
  required: boolean;
  hasForm: boolean;
  missingInstance: boolean;
  blocked: boolean;
  evidenceCount: number;
  ownerLabel: string;
  nextAction: string;
  searchText: string;
  workflowOrder: number;
};

type PlanningWorkMetrics = {
  total: number;
  actionable: number;
  waiting: number;
  future: number;
  completed: number;
  blocked: number;
  formsTotal: number;
  submittedForms: number;
  missingForms: number;
};

type PlanningProgress = {
  total: number;
  completed: number;
  skipped: number;
  percent: number;
};

type PlanningWorkFilters = {
  search: string;
  queue: PlanningQueueFilter;
  type: string;
  status: string;
  requirement: string;
};

const planningWorkRequirementOptions = [
  ['all', 'Mọi yêu cầu'],
  ['required', 'Chỉ bắt buộc'],
  ['optional', 'Chỉ tùy chọn'],
  ['none', 'Không có biểu mẫu'],
];

function PlanningActionWorkspace({
  rows,
  filteredRows,
  metrics,
  progress,
  currentNode,
  nextWork,
  search,
  queueFilter,
  typeFilter,
  statusFilter,
  requirementFilter,
  onSearchChange,
  onQueueFilterChange,
  onTypeFilterChange,
  onStatusFilterChange,
  onRequirementFilterChange,
  onOpenRow,
  onOpenPipeline,
}: {
  rows: PlanningWorkRow[];
  filteredRows: PlanningWorkRow[];
  metrics: PlanningWorkMetrics;
  progress: PlanningProgress;
  currentNode?: Record<string, any>;
  nextWork?: PlanningWorkRow;
  search: string;
  queueFilter: PlanningQueueFilter;
  typeFilter: string;
  statusFilter: string;
  requirementFilter: string;
  onSearchChange: (value: string) => void;
  onQueueFilterChange: (value: PlanningQueueFilter) => void;
  onTypeFilterChange: (value: string) => void;
  onStatusFilterChange: (value: string) => void;
  onRequirementFilterChange: (value: string) => void;
  onOpenRow: (row: PlanningWorkRow) => void;
  onOpenPipeline: () => void;
}) {
  const queueTabs: Array<{ id: PlanningQueueFilter; label: string; count: number }> = [
    { id: 'actionable', label: 'Cần xử lý', count: metrics.actionable },
    { id: 'waiting', label: 'Chờ duyệt', count: metrics.waiting },
    { id: 'future', label: 'Sắp tới', count: metrics.future },
    { id: 'completed', label: 'Hoàn tất', count: metrics.completed },
    { id: 'all', label: 'Tất cả', count: metrics.total },
  ];
  const workTypeOptions = [...new Set(rows.map((row) => row.workType).filter(Boolean))]
    .map((value) => [value, enumLabel('step_type', value)] as const)
    .sort((left, right) => left[1].localeCompare(right[1], 'vi'));
  const workStatusOptions = [...new Set(rows.map((row) => row.workStatus).filter(Boolean))]
    .map((value) => [value, statusLabel(value)] as const)
    .sort((left, right) => left[1].localeCompare(right[1], 'vi'));
  const nodeProgress = buildPlanningProgress(currentNode ? [currentNode] : []);
  const hasExtraFilters = Boolean(search || typeFilter !== 'all' || statusFilter !== 'all' || requirementFilter !== 'all');

  function resetFilters() {
    onSearchChange('');
    onTypeFilterChange('all');
    onStatusFilterChange('all');
    onRequirementFilterChange('all');
  }

  return (
    <section className="space-y-4">
      <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Giai đoạn hiện tại</p>
              <StatusBadge status={currentNode?.status ?? 'pending'} />
            </div>
            <h2 className="mt-2 text-lg font-bold text-ink">{currentNode?.name ?? 'Chưa có giai đoạn đang chạy'}</h2>
            <p className="mt-1 line-clamp-2 text-sm text-slate-600">{currentNode?.description ?? 'Mở tiến độ để kiểm tra trạng thái vận hành và phụ thuộc.'}</p>
          </div>
          <div className="w-full shrink-0 lg:w-80">
            <div className="mb-2 flex items-center justify-between gap-3 text-xs font-semibold">
              <span className="text-slate-600">Tiến độ giai đoạn</span>
              <span className="text-blue-700">{nodeProgress.percent}% · {nodeProgress.completed}/{nodeProgress.total} bước</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-slate-100">
              <div className="h-full rounded-full bg-blue-600" style={{ width: `${nodeProgress.percent}%` }} />
            </div>
            <p className="mt-1.5 text-right text-[11px] text-slate-500">
              Toàn quy trình: {progress.completed}/{progress.total} bước{progress.skipped ? ` · ${progress.skipped} bước đã bỏ qua` : ''}
            </p>
          </div>
        </div>

        <div className="mt-4 grid gap-px overflow-hidden rounded-md border border-line bg-line sm:grid-cols-2 xl:grid-cols-4">
          <PlanningMetric label="Cần xử lý" value={metrics.actionable} tone="text-blue-700" />
          <PlanningMetric label="Chờ duyệt" value={metrics.waiting} tone="text-amber-700" />
          <PlanningMetric label="Bị chặn" value={metrics.blocked} tone="text-rose-700" />
          <PlanningMetric label="Hồ sơ đã chốt" value={`${metrics.submittedForms}/${metrics.formsTotal}`} tone="text-emerald-700" />
        </div>
      </section>

      {nextWork ? (
        <section className={`rounded-lg border p-4 shadow-panel ${nextWork.blocked ? 'border-rose-200 bg-rose-50/50' : nextWork.queue === 'waiting' ? 'border-amber-200 bg-amber-50/40' : 'border-blue-200 bg-blue-50/40'}`}>
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="min-w-0">
              <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Việc tiếp theo</p>
              <div className="mt-2 flex flex-wrap items-center gap-1.5">
                <StatusBadge status={nextWork.workStatus} />
                <EnumChip group="step_type" value={nextWork.workType} />
                {nextWork.hasForm ? (
                  <span className="rounded-md bg-white px-2 py-1 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-line">
                    {nextWork.required ? 'Hồ sơ bắt buộc' : 'Hồ sơ tùy chọn'}
                  </span>
                ) : null}
              </div>
              <h2 className="mt-2 text-base font-bold text-ink">{nextWork.title}</h2>
              <p className="mt-1 text-sm text-slate-600">{nextWork.node.name ?? '-'} · {nextWork.ownerLabel} · {nextWork.evidenceCount} minh chứng</p>
              <p className="mt-2 text-sm font-medium text-slate-700">Tiếp theo: {nextWork.nextAction}</p>
            </div>
            <button
              className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700"
              onClick={() => onOpenRow(nextWork)}
            >
              {planningWorkActionLabel(nextWork)}
              <ArrowRight className="h-4 w-4" />
            </button>
          </div>
        </section>
      ) : (
        <section className="rounded-lg border border-emerald-200 bg-emerald-50/50 p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="font-semibold text-emerald-800">Không có việc cần xử lý ngay</p>
              <p className="mt-1 text-sm text-emerald-700">Kiểm tra việc sắp tới hoặc mở quy trình để xem trạng thái giai đoạn.</p>
            </div>
            <button className="rounded-md border border-emerald-200 bg-white px-3 py-2 text-sm font-semibold text-emerald-800 hover:bg-emerald-50" onClick={() => onQueueFilterChange('future')}>
              Xem việc sắp tới
            </button>
          </div>
        </section>
      )}

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_300px]">
        <section className="min-w-0 rounded-lg border border-line bg-white p-4 shadow-panel">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
              <ClipboardList className="h-5 w-5 shrink-0 text-blue-700" />
              <div>
                <h2 className="font-semibold text-ink">Hàng đợi công việc</h2>
                <p className="mt-1 text-xs text-slate-500">{filteredRows.length}/{rows.length} đầu việc trong quy trình đang chạy</p>
              </div>
            </div>
          </div>

          <div className="mb-4 flex gap-2 overflow-x-auto pb-1" aria-label="Lọc hàng đợi công việc">
            {queueTabs.map((tab) => (
              <button
                key={tab.id}
                aria-pressed={queueFilter === tab.id}
                className={`shrink-0 rounded-full border px-3 py-1.5 text-xs font-semibold ${queueFilter === tab.id ? 'border-blue-200 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-600 hover:bg-slate-50'}`}
                onClick={() => onQueueFilterChange(tab.id)}
              >
                {tab.label} <span className="ml-1">{tab.count}</span>
              </button>
            ))}
          </div>

          <div className="mb-4 grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_170px_170px_180px]">
            <label className="relative block">
              <span className="sr-only">Tìm công việc kế hoạch</span>
              <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
              <input
                className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="Tìm công việc, node, bước"
                value={search}
                onChange={(event) => onSearchChange(event.target.value)}
              />
            </label>
            <select aria-label="Lọc loại công việc" className="rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500" value={typeFilter} onChange={(event) => onTypeFilterChange(event.target.value)}>
              <option value="all">Tất cả loại</option>
              {workTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
            <select aria-label="Lọc trạng thái công việc" className="rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500" value={statusFilter} onChange={(event) => onStatusFilterChange(event.target.value)}>
              <option value="all">Tất cả trạng thái</option>
              {workStatusOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
            <select aria-label="Lọc yêu cầu biểu mẫu" className="rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500" value={requirementFilter} onChange={(event) => onRequirementFilterChange(event.target.value)}>
              {planningWorkRequirementOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </div>

          {!rows.length ? (
            <EmptyState title="Kế hoạch tiến độ này chưa có đầu việc." />
          ) : filteredRows.length ? (
            <PlanningWorkList rows={filteredRows} onOpen={onOpenRow} />
          ) : (
            <EmptyState
              title={queueFilter === 'actionable' && !hasExtraFilters ? 'Giai đoạn hiện tại chưa có việc cần xử lý.' : 'Không có công việc khớp bộ lọc hiện tại.'}
              action={
                queueFilter === 'actionable' && !hasExtraFilters ? (
                  <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => onQueueFilterChange('future')}>Xem việc sắp tới</button>
                ) : (
                  <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={resetFilters}>Xóa bộ lọc</button>
                )
              }
            />
          )}
        </section>

        <aside className="h-fit rounded-lg border border-line bg-white p-4 shadow-panel xl:sticky xl:top-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Giai đoạn hiện tại</p>
              <h2 className="mt-2 font-semibold text-ink">{currentNode?.name ?? 'Chưa có node'}</h2>
            </div>
            <StatusBadge status={currentNode?.status ?? 'pending'} />
          </div>
          <div className="mt-4">
            <div className="mb-2 flex items-center justify-between text-xs font-semibold">
              <span className="text-slate-600">Bước trong giai đoạn</span>
              <span className="text-blue-700">{nodeProgress.completed}/{nodeProgress.total}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-slate-100">
              <div className="h-full rounded-full bg-blue-600" style={{ width: `${nodeProgress.percent}%` }} />
            </div>
          </div>
          <dl className="mt-4 space-y-3 text-sm">
            <div>
              <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Vai trò</dt>
              <dd className="mt-1 text-slate-700">{arrayText(currentNode?.responsible_role_ids)}</dd>
            </div>
            <div>
              <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Phê duyệt</dt>
              <dd className="mt-1 text-slate-700">{String(currentNode?.approval_rule ?? '-')}</dd>
            </div>
            {metrics.missingForms ? (
              <div className="rounded-md border border-rose-100 bg-rose-50 px-3 py-2 text-rose-700">
                <dt className="text-xs font-semibold">Thiếu biểu mẫu</dt>
                <dd className="mt-1">{metrics.missingForms} bước bắt buộc chưa có instance.</dd>
              </div>
            ) : null}
          </dl>
          <button className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50" onClick={onOpenPipeline}>
            <GitBranch className="h-4 w-4" />
            Xem toàn bộ quy trình
          </button>
        </aside>
      </div>
    </section>
  );
}

function PlanningMetric({ label, value, tone }: { label: string; value: string | number; tone: string }) {
  return (
    <div className="bg-white px-4 py-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className={`mt-1 text-xl font-black ${tone}`}>{value}</p>
    </div>
  );
}

function PlanningWorkList({ rows, onOpen }: { rows: PlanningWorkRow[]; onOpen: (row: PlanningWorkRow) => void }) {
  return (
    <>
      <div className="consflow-table-viewport hidden lg:block">
        <table className="w-full min-w-[840px] border-collapse text-left text-sm">
          <thead className="bg-slate-50">
            <tr className="border-b border-line text-xs font-semibold uppercase tracking-wide text-slate-500">
              <th className="px-3 py-2"><span className="sr-only">Thao tác</span></th>
              <th className="px-3 py-2">Công việc</th>
              <th className="px-3 py-2">Giai đoạn</th>
              <th className="px-3 py-2">Loại</th>
              <th className="px-3 py-2">Trạng thái</th>
              <th className="px-3 py-2">Yêu cầu</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id} className="border-b border-line last:border-0 hover:bg-slate-50/70">
                <td className="px-3 py-3 align-top"><PlanningWorkButton row={row} onOpen={() => onOpen(row)} /></td>
                <td className="max-w-sm px-3 py-3 align-top">
                  <p className="font-semibold text-ink">{row.title}</p>
                  <p className="mt-1 line-clamp-1 text-xs text-slate-500">{row.formTitle ? `Hồ sơ: ${row.formTitle}` : row.nextAction}</p>
                </td>
                <td className="px-3 py-3 align-top">
                  <p className="font-medium text-slate-700">{row.node.name ?? '-'}</p>
                  <p className="mt-1 max-w-48 truncate text-xs text-slate-500">{row.ownerLabel}</p>
                </td>
                <td className="px-3 py-3 align-top"><EnumChip group="step_type" value={row.workType} /></td>
                <td className="px-3 py-3 align-top"><PlanningWorkStatus row={row} /></td>
                <td className="px-3 py-3 align-top">
                  <p className="text-xs font-semibold text-slate-700">{planningWorkRequirementLabel(row)}</p>
                  <p className="mt-1 text-xs text-slate-500">{row.evidenceCount} minh chứng</p>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="space-y-3 lg:hidden">
        {rows.map((row) => (
          <article key={row.id} className={`rounded-md border p-3 ${row.blocked || row.missingInstance ? 'border-rose-200 bg-rose-50/30' : 'border-line bg-white'}`}>
            <div className="flex flex-wrap items-center gap-1.5">
              <PlanningWorkStatus row={row} />
              <EnumChip group="step_type" value={row.workType} />
            </div>
            <h3 className="mt-2 text-sm font-semibold text-ink">{row.title}</h3>
            <p className="mt-1 text-xs text-slate-500">{row.node.name ?? '-'} · {row.ownerLabel}</p>
            <div className="mt-3 grid grid-cols-2 gap-2 text-xs text-slate-600">
              <span>{planningWorkRequirementLabel(row)}</span>
              <span>{row.evidenceCount} minh chứng</span>
            </div>
            <div className="mt-3"><PlanningWorkButton row={row} onOpen={() => onOpen(row)} full /></div>
          </article>
        ))}
      </div>
    </>
  );
}

function PlanningWorkStatus({ row }: { row: PlanningWorkRow }) {
  return (
    <div className="flex flex-wrap gap-1.5">
      <StatusBadge status={row.workStatus} />
      {row.missingInstance ? (
        <span className="rounded-md bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700 ring-1 ring-inset ring-rose-200">Thiếu biểu mẫu</span>
      ) : row.hasForm && row.formStatus ? <StatusBadge status={row.formStatus} /> : null}
    </div>
  );
}

function PlanningWorkButton({ row, onOpen, full = false }: { row: PlanningWorkRow; onOpen: () => void; full?: boolean }) {
  const disabled = !row.workItem;

  return (
    <button
      className={`inline-flex items-center justify-center gap-1.5 rounded-md border border-line bg-white px-3 py-1.5 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400 ${full ? 'w-full py-2 text-sm' : ''}`}
      disabled={disabled}
      onClick={onOpen}
      title={disabled ? 'Đầu việc không còn khả dụng trong hàng đợi vận hành' : planningWorkActionLabel(row)}
    >
      {planningWorkActionLabel(row)}
      <ArrowRight className="h-3.5 w-3.5" />
    </button>
  );
}

function buildPlanningWorkRows(nodes: Record<string, any>[], run?: Record<string, any>): PlanningWorkRow[] {
  const rows: PlanningWorkRow[] = [];
  let workflowOrder = 0;
  nodes.forEach((node) => {
    (node.steps ?? []).forEach((step: Record<string, any>) => {
      const hasSummary = hasWorkflowFormSummary(step);
      const required = Boolean(step.workflow_form_required);
      const hasInstance = Boolean(step.workflow_form_instance_id);
      const workItem = stepToWorkItem(step, run);
      const hasForm = hasSummary || required || hasInstance;
      const formStatus = hasInstance ? String(step.workflow_form_status ?? 'draft') : required ? 'missing' : '';
      const missingInstance = required && !hasInstance;
      const workStatus = String(workItem?.status ?? step.status ?? 'pending');
      const blocked = step.status === 'blocked' || workStatus === 'blocked';
      const workType = String(step.step_type ?? workItem?.work_type ?? 'task');
      const workTypeLabel = enumLabel('step_type', workType);
      const formTitle = hasForm ? (hasSummary ? workflowFormChipText(step) : workflowFormBusinessLabel(step)) : undefined;
      const ownerLabel = arrayText(workItem?.role_ids ?? step.required_role_ids);
      const nextAction = planningWorkNextAction({ step, workStatus, hasForm, formStatus, missingInstance });
      const queue = planningQueueForStatus(workStatus);
      const title = String(workItem?.title ?? step.name ?? 'Đầu việc chưa đặt tên');
      const searchText = [
        title,
        formTitle,
        workType,
        workTypeLabel,
        workStatus,
        statusLabel(workStatus),
        node.name,
        node.description,
        step.name,
        step.description,
        ownerLabel,
      ].filter(Boolean).join(' ').toLowerCase();

      rows.push({
        id: String(step.id ?? `${node.id}:${rows.length}`),
        step,
        node,
        workItem,
        title,
        formTitle,
        formStatus,
        workStatus,
        workType,
        workTypeLabel,
        queue,
        required,
        hasForm,
        missingInstance,
        blocked,
        evidenceCount: numberValue(step.evidence_count),
        ownerLabel,
        nextAction,
        searchText,
        workflowOrder,
      });
      workflowOrder += 1;
    });
  });
  return rows;
}

function buildPlanningWorkMetrics(rows: PlanningWorkRow[]): PlanningWorkMetrics {
  return {
    total: rows.length,
    actionable: rows.filter((row) => row.queue === 'actionable').length,
    waiting: rows.filter((row) => row.queue === 'waiting').length,
    future: rows.filter((row) => row.queue === 'future').length,
    completed: rows.filter((row) => row.queue === 'completed').length,
    blocked: rows.filter((row) => row.blocked).length,
    formsTotal: rows.filter((row) => row.hasForm).length,
    submittedForms: rows.filter((row) => row.formStatus === 'submitted').length,
    missingForms: rows.filter((row) => row.missingInstance).length,
  };
}

function planningWorkRowMatchesFilters(row: PlanningWorkRow, filters: PlanningWorkFilters) {
  const search = filters.search.trim().toLowerCase();
  if (search && !row.searchText.includes(search)) return false;
  if (filters.queue !== 'all' && row.queue !== filters.queue) return false;
  if (filters.type !== 'all' && row.workType !== filters.type) return false;
  if (filters.status !== 'all' && row.workStatus !== filters.status) return false;
  if (filters.requirement === 'required' && !row.required) return false;
  if (filters.requirement === 'optional' && (!row.hasForm || row.required)) return false;
  if (filters.requirement === 'none' && row.hasForm) return false;
  return true;
}

function planningQueueForStatus(workStatus: string): PlanningQueueFilter {
  if (workStatus === 'waiting_approval' || workStatus === 'waiting_review') return 'waiting';
  if (workStatus === 'pending') return 'future';
  if (['completed', 'submitted', 'done', 'cancelled', 'skipped'].includes(workStatus)) return 'completed';
  return 'actionable';
}

function planningWorkNextAction({
  step,
  workStatus,
  hasForm,
  formStatus,
  missingInstance,
}: {
  step: Record<string, any>;
  workStatus: string;
  hasForm: boolean;
  formStatus: string;
  missingInstance: boolean;
}) {
  if (missingInstance) return 'Kiểm tra việc gắn biểu mẫu bắt buộc';
  if (workStatus === 'blocked') return 'Xử lý vướng mắc trong đầu việc';
  if (workStatus === 'waiting_approval' || workStatus === 'waiting_review') return 'Mở yêu cầu duyệt hoặc rà soát';
  if (workStatus === 'pending') return 'Chưa tới lượt xử lý trong quy trình';
  if (['completed', 'done'].includes(workStatus)) return 'Xem kết quả đã hoàn tất';
  if (hasForm) return workflowFormNextAction({ ...step, workflow_form_status: formStatus });
  if (step.photo_required || step.step_type === 'photo_upload') return 'Bổ sung ảnh minh chứng';
  if (step.document_required || step.step_type === 'document_upload') return 'Bổ sung tài liệu yêu cầu';
  return 'Mở đầu việc để tiếp tục';
}

function selectNextPlanningWork(rows: PlanningWorkRow[]) {
  return [...rows]
    .filter((row) => row.workItem && (row.queue === 'actionable' || row.queue === 'waiting'))
    .sort((left, right) => {
      const priorityDiff = planningWorkPriority(left) - planningWorkPriority(right);
      if (priorityDiff !== 0) return priorityDiff;
      return left.workflowOrder - right.workflowOrder;
    })[0];
}

function planningWorkPriority(row: PlanningWorkRow) {
  if (row.blocked) return 0;
  if (row.queue === 'waiting') return 1;
  return 2;
}

function planningWorkActionLabel(row: PlanningWorkRow) {
  if (row.blocked) return 'Xử lý vướng mắc';
  if (row.queue === 'waiting') return 'Mở yêu cầu duyệt';
  if (row.queue === 'future') return 'Xem việc';
  if (row.queue === 'completed') return 'Xem kết quả';
  if (row.step.photo_required || row.workType === 'photo_upload') return 'Tải minh chứng';
  if (row.hasForm) return row.formStatus === 'submitted' ? 'Xem hồ sơ' : 'Tiếp tục hồ sơ';
  return 'Mở việc';
}

function planningWorkRequirementLabel(row: PlanningWorkRow) {
  if (!row.hasForm) return 'Không có biểu mẫu';
  return row.required ? 'Hồ sơ bắt buộc' : 'Hồ sơ tùy chọn';
}

function buildPlanningProgress(nodes: Record<string, any>[]): PlanningProgress {
  const steps = nodes.flatMap((node) => Array.isArray(node.steps) ? node.steps : []);
  const skipped = steps.filter((step: Record<string, any>) => ['cancelled', 'skipped'].includes(String(step.status ?? ''))).length;
  const executable = steps.filter((step: Record<string, any>) => !['cancelled', 'skipped'].includes(String(step.status ?? '')));
  const completedSteps = executable.filter((step: Record<string, any>) => ['completed', 'submitted'].includes(String(step.status ?? ''))).length;
  return {
    total: executable.length,
    completed: completedSteps,
    skipped,
    percent: executable.length ? Math.round((completedSteps / executable.length) * 100) : 0,
  };
}

function arrayText(value: unknown) {
  if (Array.isArray(value)) {
    const text = value.map((item) => String(item).trim()).filter(Boolean).join(', ');
    return text || '-';
  }
  const text = String(value ?? '').trim();
  return text || '-';
}

function PlanningWorkspaceLoading() {
  return (
    <div className="space-y-4 animate-pulse">
      <div className="h-24 rounded-lg border border-line bg-white" />
      <div className="h-36 rounded-lg border border-blue-100 bg-blue-50/40" />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_300px]">
        <div className="space-y-3 rounded-lg border border-line bg-white p-4">
          <div className="h-9 rounded bg-slate-100" />
          <div className="h-12 rounded bg-slate-100" />
          <div className="h-12 rounded bg-slate-100" />
          <div className="h-12 rounded bg-slate-100" />
        </div>
        <div className="h-64 rounded-lg border border-line bg-white" />
      </div>
    </div>
  );
}

function stepToWorkItem(step: Record<string, any>, run?: Record<string, any>): WorkItem | null {
  const workItemId = String(step.work_item_id ?? '').trim();
  if (!workItemId) return null;
  const node = (run?.nodes ?? []).find((candidate: Record<string, any>) =>
    (candidate.steps ?? []).some((candidateStep: Record<string, any>) => candidateStep.id === step.id),
  );
  return {
    id: workItemId,
    project_id: run?.project_id,
    project_name: run?.project_name,
    title: step.work_item_title ?? step.name,
    description: step.description,
    status: step.work_item_status ?? step.status,
    work_type: step.step_type,
    source_type: 'pipeline_run_step',
    source_id: step.id,
    role_ids: Array.isArray(step.work_item_role_ids) ? step.work_item_role_ids : step.required_role_ids,
    evidence_count: Number(step.evidence_count ?? 0),
    workflow_form_instance_id: step.workflow_form_instance_id,
    workflow_form_status: step.workflow_form_status,
    workflow_form_type: step.workflow_form_type,
    workflow_form_revision: step.workflow_form_revision,
    workflow_form_submitted_at: step.workflow_form_submitted_at,
    workflow_form_template_id: step.workflow_form_template_id,
    workflow_form_version_id: step.workflow_form_version_id,
    workflow_form_version_no: step.workflow_form_version_no,
    workflow_form_version_status: step.workflow_form_version_status,
    workflow_form_template_code: step.workflow_form_template_code,
    workflow_form_template_name: step.workflow_form_template_name,
    workflow_form_required: step.workflow_form_required,
    evidence_requirement_json: step.evidence_requirement_json,
    pipeline_run_id: run?.id,
    run_node_id: step.run_node_id,
    run_node_name: node?.name,
    run_step_id: step.id,
    run_step_name: step.name,
  };
}
