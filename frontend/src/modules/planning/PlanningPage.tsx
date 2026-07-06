import { useMemo, useState } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Ban, CheckCircle2, ClipboardList, FileCheck2, FileText, Play, Search, Send } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { Timeline } from '../../components/Timeline';
import { useToast } from '../../components/ToastProvider';
import { EnumChip } from '../../components/EnumChip';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import type { WorkItem } from '../../lib/types/domain';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import {
  hasWorkflowFormSummary,
  numberValue,
  workflowFormBusinessLabel,
  workflowFormChipText,
  workflowFormNextAction,
  workflowFormTypeLabel,
} from '../work-items/workItemPresentation';

export function PlanningPage({ mode = 'planning' }: { mode?: 'planning' | 'pipeline' }) {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [selectedNodeId, setSelectedNodeId] = useState('');
  const [blockReason, setBlockReason] = useState('');
  const [formSearch, setFormSearch] = useState('');
  const [formTypeFilter, setFormTypeFilter] = useState('all');
  const [formStatusFilter, setFormStatusFilter] = useState('all');
  const [formRequirementFilter, setFormRequirementFilter] = useState('all');
  const run = useQuery({
    queryKey: ['pipeline-run', id],
    queryFn: () => get<Record<string, any>>(`/projects/${id}/pipeline-run`),
    enabled: Boolean(id),
    retry: false,
  });
  const audit = useQuery({
    queryKey: ['project-audit', id],
    queryFn: () => get<Record<string, any>[]>(`/projects/${id}/audit-logs`),
    enabled: Boolean(id),
  });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['pipeline-run', id] });
    queryClient.invalidateQueries({ queryKey: ['project-audit', id] });
    queryClient.invalidateQueries({ queryKey: ['work-item'] });
    queryClient.invalidateQueries({ queryKey: ['work-item-form'] });
  };
  const start = useMutation({
    mutationFn: () => post(`/projects/${id}/start-pipeline`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã khởi động quy trình đang chạy');
    },
    onError: (error) => toast.error('Không khởi động được quy trình đang chạy', error),
  });
  const submitStep = useMutation({
    mutationFn: (stepId: string) => post(`/pipeline-run-steps/${stepId}/submit`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã gửi bước');
    },
    onError: (error) => toast.error('Không gửi được bước', error),
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
      toast.success('Đã chặn node');
    },
    onError: (error) => toast.error('Không chặn được node', error),
  });

  const nodes = run.data?.nodes ?? [];
  const selectedNode = nodes.find((node: Record<string, any>) => node.id === selectedNodeId) ?? nodes.find((node: Record<string, any>) => node.status === 'in_progress') ?? nodes[0];
  const completed = nodes.filter((node: Record<string, any>) => node.status === 'completed').length;
  const percent = nodes.length ? Math.round((completed / nodes.length) * 100) : 0;
  const planningFormRows = useMemo(() => buildPlanningFormRows(nodes, run.data), [nodes, run.data]);
  const planningFormMetrics = useMemo(() => buildPlanningFormMetrics(planningFormRows), [planningFormRows]);
  const filteredPlanningFormRows = useMemo(() => (
    planningFormRows.filter((row) => planningFormRowMatchesFilters(row, {
      search: formSearch,
      type: formTypeFilter,
      status: formStatusFilter,
      requirement: formRequirementFilter,
    }))
  ), [formRequirementFilter, formSearch, formStatusFilter, formTypeFilter, planningFormRows]);

  function openStepWorkItem(step: Record<string, any>) {
    const workItem = stepToWorkItem(step, run.data);
    if (!workItem) {
      toast.warn({ title: 'Bước chưa có đầu việc', description: 'Quy trình đang chạy chưa tạo đầu việc cho bước này.' });
      return;
    }
    navigate(workItemDetailPath(workItem.id, currentReturnTo(location), workItem.project_id ?? id));
  }

  if (run.isLoading) return <LoadingSpinner />;

  return (
    <>
      <PageHeader
        title={mode === 'pipeline' ? 'Quy trình công trình' : 'Không gian kế hoạch'}
        description={run.data ? `${run.data.project_name} · Mẫu: ${run.data.template_name}` : 'Chưa có quy trình đang chạy cho công trình này.'}
        actions={
          <>
            <ProjectBackLink projectId={id} />
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={start.isPending}
              onClick={() => start.mutate()}
            >
              <Play className="h-4 w-4" />
              Khởi động / tạo lại quy trình
            </button>
          </>
        }
      />
      {!run.data ? (
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
        <div className="space-y-5">
          <PlanningFormWorkspace
            rows={planningFormRows}
            filteredRows={filteredPlanningFormRows}
            metrics={planningFormMetrics}
            search={formSearch}
            typeFilter={formTypeFilter}
            statusFilter={formStatusFilter}
            requirementFilter={formRequirementFilter}
            onSearchChange={setFormSearch}
            onTypeFilterChange={setFormTypeFilter}
            onStatusFilterChange={setFormStatusFilter}
            onRequirementFilterChange={setFormRequirementFilter}
            onOpenRow={(row) => openStepWorkItem(row.step)}
          />
          <PipelineRuntimeSection
            showHeading
            nodes={nodes}
            selectedNode={selectedNode}
            completed={completed}
            percent={percent}
            auditEvents={audit.data?.slice(0, 8) ?? []}
            blockReason={blockReason}
            onSelectNode={setSelectedNodeId}
            onOpenStepWorkItem={openStepWorkItem}
            onSubmitStep={(stepId) => submitStep.mutate(stepId)}
            onAdvance={(nodeId) => advance.mutate(nodeId)}
            onRequestApproval={(nodeId) => requestApproval.mutate(nodeId)}
            onBlock={(nodeId) => block.mutate(nodeId)}
            onBlockReasonChange={setBlockReason}
          />
        </div>
      ) : (
        <PipelineRuntimeSection
          nodes={nodes}
          selectedNode={selectedNode}
          completed={completed}
          percent={percent}
          auditEvents={audit.data?.slice(0, 8) ?? []}
          blockReason={blockReason}
          onSelectNode={setSelectedNodeId}
          onOpenStepWorkItem={openStepWorkItem}
          onSubmitStep={(stepId) => submitStep.mutate(stepId)}
          onAdvance={(nodeId) => advance.mutate(nodeId)}
          onRequestApproval={(nodeId) => requestApproval.mutate(nodeId)}
          onBlock={(nodeId) => block.mutate(nodeId)}
          onBlockReasonChange={setBlockReason}
        />
      )}
    </>
  );
}

type PlanningFormRow = {
  id: string;
  step: Record<string, any>;
  node: Record<string, any>;
  workItem: WorkItem | null;
  formTitle: string;
  formType: string;
  formTypeLabel: string;
  formStatus: string;
  required: boolean;
  hasInstance: boolean;
  missingInstance: boolean;
  blocked: boolean;
  submitted: boolean;
  needsSubmit: boolean;
  evidenceCount: number;
  nextAction: string;
  searchText: string;
};

type PlanningFormMetrics = {
  total: number;
  required: number;
  needsSubmit: number;
  submitted: number;
  blocked: number;
  missing: number;
};

type PlanningFormFilters = {
  search: string;
  type: string;
  status: string;
  requirement: string;
};

const planningFormTypeOptions = [
  ['all', 'Tất cả loại'],
  ['meeting_minutes', 'Biên bản họp'],
  ['daily_report', 'Nhật ký'],
  ['checklist', 'Checklist'],
  ['generic_submission', 'Biểu mẫu gửi thông tin'],
];

const planningFormStatusOptions = [
  ['all', 'Tất cả trạng thái'],
  ['needs_submit', 'Cần gửi'],
  ['draft', 'Nháp'],
  ['submitted', 'Đã gửi'],
  ['blocked', 'Bị chặn'],
  ['missing', 'Thiếu biểu mẫu'],
];

const planningFormRequirementOptions = [
  ['all', 'Bắt buộc + tùy chọn'],
  ['required', 'Chỉ bắt buộc'],
  ['optional', 'Chỉ tùy chọn'],
];

function PlanningFormWorkspace({
  rows,
  filteredRows,
  metrics,
  search,
  typeFilter,
  statusFilter,
  requirementFilter,
  onSearchChange,
  onTypeFilterChange,
  onStatusFilterChange,
  onRequirementFilterChange,
  onOpenRow,
}: {
  rows: PlanningFormRow[];
  filteredRows: PlanningFormRow[];
  metrics: PlanningFormMetrics;
  search: string;
  typeFilter: string;
  statusFilter: string;
  requirementFilter: string;
  onSearchChange: (value: string) => void;
  onTypeFilterChange: (value: string) => void;
  onStatusFilterChange: (value: string) => void;
  onRequirementFilterChange: (value: string) => void;
  onOpenRow: (row: PlanningFormRow) => void;
}) {
  const metricCards = [
    { label: 'Biểu mẫu', value: metrics.total, sub: `${metrics.required} bắt buộc`, tone: 'text-blue-700', bg: 'bg-blue-50' },
    { label: 'Cần gửi', value: metrics.needsSubmit, sub: 'Nháp / mở lại', tone: 'text-amber-800', bg: 'bg-amber-50' },
    { label: 'Đã gửi', value: metrics.submitted, sub: 'Chờ ký / PDF / bước', tone: 'text-emerald-700', bg: 'bg-emerald-50' },
    { label: 'Bị chặn / thiếu', value: metrics.blocked + metrics.missing, sub: `${metrics.blocked} bị chặn · ${metrics.missing} thiếu`, tone: 'text-rose-700', bg: 'bg-rose-50' },
  ];

  return (
    <section className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {metricCards.map((card) => (
          <div key={card.label} className="rounded-lg border border-line bg-white p-4 shadow-panel">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{card.label}</p>
                <p className={`mt-2 text-2xl font-black ${card.tone}`}>{card.value}</p>
              </div>
              <div className={`grid h-10 w-10 shrink-0 place-items-center rounded-md ${card.bg} ${card.tone}`}>
                {card.label === 'Bị chặn / thiếu' ? <AlertTriangle className="h-5 w-5" /> : <FileText className="h-5 w-5" />}
              </div>
            </div>
            <p className="mt-2 text-xs text-slate-500">{card.sub}</p>
          </div>
        ))}
      </div>

      <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <ClipboardList className="h-5 w-5 shrink-0 text-blue-700" />
            <div>
              <h2 className="font-semibold text-ink">Biểu mẫu kế hoạch</h2>
              <p className="mt-1 text-xs text-slate-500">{filteredRows.length}/{rows.length} biểu mẫu trong quy trình đang chạy</p>
            </div>
          </div>
        </div>

        <div className="mb-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_180px_180px_180px]">
          <label className="relative block">
            <span className="sr-only">Tìm biểu mẫu kế hoạch</span>
            <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
            <input
              className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Tìm biểu mẫu, node, bước"
              value={search}
              onChange={(event) => onSearchChange(event.target.value)}
            />
          </label>
          <select
            aria-label="Lọc loại biểu mẫu"
            className="rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
            value={typeFilter}
            onChange={(event) => onTypeFilterChange(event.target.value)}
          >
            {planningFormTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <select
            aria-label="Lọc trạng thái biểu mẫu"
            className="rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
            value={statusFilter}
            onChange={(event) => onStatusFilterChange(event.target.value)}
          >
            {planningFormStatusOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <select
            aria-label="Lọc mức bắt buộc"
            className="rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
            value={requirementFilter}
            onChange={(event) => onRequirementFilterChange(event.target.value)}
          >
            {planningFormRequirementOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
        </div>

        {!rows.length ? (
          <EmptyState title="Quy trình này chưa có biểu mẫu kế hoạch." />
        ) : filteredRows.length ? (
          <div className="space-y-3">
            {filteredRows.map((row) => <PlanningFormRowCard key={row.id} row={row} onOpen={() => onOpenRow(row)} />)}
          </div>
        ) : (
          <EmptyState title="Không có biểu mẫu khớp bộ lọc hiện tại." />
        )}
      </section>
    </section>
  );
}

function PlanningFormRowCard({ row, onOpen }: { row: PlanningFormRow; onOpen: () => void }) {
  const rowTone = row.missingInstance || row.blocked
    ? 'border-rose-200 bg-rose-50/40'
    : row.needsSubmit
      ? 'border-amber-200 bg-amber-50/30'
      : 'border-line bg-white';
  const disabled = !row.workItem || !row.hasInstance;

  return (
    <article className={`rounded-md border px-3 py-3 ${rowTone}`}>
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            {row.missingInstance ? (
              <span className="rounded-md bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700 ring-1 ring-inset ring-rose-200">Thiếu biểu mẫu</span>
            ) : row.blocked ? (
              <StatusBadge status="blocked" />
            ) : (
              <StatusBadge status={row.formStatus} />
            )}
            <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{row.formTypeLabel}</span>
            <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{row.required ? 'Bắt buộc' : 'Tùy chọn'}</span>
            <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Minh chứng: {row.evidenceCount}</span>
          </div>
          <p className="mt-2 text-sm font-semibold text-ink">{row.formTitle}</p>
          <p className="mt-1 text-xs text-slate-500">
            Node: {row.node.name ?? '-'} · Bước: {row.step.name ?? '-'}
          </p>
          <p className="mt-2 text-xs font-medium text-slate-600">Bước tiếp theo: {row.nextAction}</p>
          <p className="mt-1 text-xs text-slate-500">Đầu việc: {row.workItem?.title ?? 'Chưa có đầu việc'}</p>
        </div>
        <button
          className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-200 disabled:text-slate-500"
          disabled={disabled}
          onClick={onOpen}
          title={disabled ? 'Biểu mẫu chưa sẵn sàng để mở' : 'Mở biểu mẫu quy trình'}
        >
          <ClipboardList className="h-4 w-4" />
          Mở biểu mẫu
        </button>
      </div>
    </article>
  );
}

function PipelineRuntimeSection({
  showHeading = false,
  nodes,
  selectedNode,
  completed,
  percent,
  auditEvents,
  blockReason,
  onSelectNode,
  onOpenStepWorkItem,
  onSubmitStep,
  onAdvance,
  onRequestApproval,
  onBlock,
  onBlockReasonChange,
}: {
  showHeading?: boolean;
  nodes: Record<string, any>[];
  selectedNode?: Record<string, any>;
  completed: number;
  percent: number;
  auditEvents: Record<string, any>[];
  blockReason: string;
  onSelectNode: (nodeId: string) => void;
  onOpenStepWorkItem: (step: Record<string, any>) => void;
  onSubmitStep: (stepId: string) => void;
  onAdvance: (nodeId: string) => void;
  onRequestApproval: (nodeId: string) => void;
  onBlock: (nodeId: string) => void;
  onBlockReasonChange: (value: string) => void;
}) {
  return (
    <section className="space-y-3">
      {showHeading ? (
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="text-base font-semibold text-ink">Quy trình đang chạy</h2>
            <p className="mt-1 text-xs text-slate-500">Node, bước và hành động phê duyệt giữ nguyên để điều phối quy trình đang chạy.</p>
          </div>
        </div>
      ) : null}
      <div className="grid gap-4 xl:grid-cols-[1fr_420px]">
        <section className="space-y-4">
          <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
            <div className="mb-2 flex items-center justify-between text-sm">
              <span className="font-semibold text-ink">Tiến độ quy trình</span>
              <span className="font-semibold text-blue-700">{percent}% · {completed}/{nodes.length} node</span>
            </div>
            <div className="h-2 rounded-full bg-slate-100">
              <div className="h-2 rounded-full bg-blue-600" style={{ width: `${percent}%` }} />
            </div>
          </div>

          <div className="flex flex-wrap gap-3 rounded-lg border border-line bg-white p-4 shadow-panel">
            {nodes.map((node: Record<string, any>, index: number) => (
              <button
                key={node.id}
                className={`w-56 rounded-lg border p-4 text-left hover:border-blue-300 hover:bg-blue-50 ${selectedNode?.id === node.id ? 'border-blue-400 bg-blue-50' : 'border-line'}`}
                onClick={() => onSelectNode(String(node.id))}
              >
                <div className="mb-3 grid h-8 w-8 place-items-center rounded-md bg-slate-100 text-xs font-black text-slate-700">{index + 1}</div>
                <p className="text-sm font-semibold text-ink">{node.name}</p>
                <p className="mt-2 line-clamp-2 text-xs text-slate-500">{node.description}</p>
                <div className="mt-3 flex flex-wrap gap-1">
                  <StatusBadge status={node.status} />
                  <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{node.steps?.filter((s: Record<string, any>) => s.status === 'completed').length ?? 0}/{node.steps?.length ?? 0} bước</span>
                </div>
              </button>
            ))}
          </div>
        </section>

        <aside className="space-y-4">
          <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
            <div className="mb-4 flex items-start justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-ink">{selectedNode?.name ?? 'Chưa chọn node'}</h2>
                <p className="mt-1 text-sm leading-6 text-slate-600">{selectedNode?.description}</p>
              </div>
              <StatusBadge status={selectedNode?.status} />
            </div>
            <dl className="mb-4 grid gap-2 rounded-lg bg-slate-50 p-3 text-sm">
              <div className="flex justify-between gap-3"><dt className="text-slate-500">Vai trò</dt><dd className="font-medium text-ink">{selectedNode?.responsible_role_ids?.join?.(', ') ?? '-'}</dd></div>
              <div className="flex justify-between gap-3"><dt className="text-slate-500">Phê duyệt</dt><dd className="font-medium text-ink">{selectedNode?.approval_rule ?? '-'}</dd></div>
              <div className="flex justify-between gap-3"><dt className="text-slate-500">Nhánh khi không đạt</dt><dd className="font-medium text-ink"><EnumChip group="fail_path" value={selectedNode?.fail_path} /></dd></div>
            </dl>
            <div className="space-y-2">
              {(selectedNode?.steps ?? []).map((step: Record<string, any>) => {
                const hasForm = Boolean(step.workflow_form_instance_id);
                const hasFormSummary = hasWorkflowFormSummary(step);
                const formRequired = Boolean(step.workflow_form_required);
                const formSubmitted = step.workflow_form_status === 'submitted';
                const completedStep = step.status === 'completed';
                const canCompleteStep = !completedStep && (!formRequired || formSubmitted);
                return (
                  <div key={step.id} className="rounded-md border border-line px-3 py-2">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-ink">{step.name}</p>
                        <div className="mt-2 flex flex-wrap gap-1.5">
                          <EnumChip group="step_type" value={step.step_type} />
                          <StatusBadge status={step.status} />
                          {hasFormSummary ? (
                            <span className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">
                              {workflowFormChipText(step)}{formRequired ? ' · Bắt buộc' : ' · Tùy chọn'}
                            </span>
                          ) : null}
                          {hasForm ? <StatusBadge status={step.workflow_form_status ?? 'draft'} /> : null}
                          <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Minh chứng: {numberValue(step.evidence_count)}</span>
                        </div>
                        {hasFormSummary ? <p className="mt-2 text-xs font-medium text-slate-500">Bước tiếp theo: {workflowFormNextAction(step)}</p> : null}
                      </div>
                      {completedStep ? <CheckCircle2 className="mt-1 h-5 w-5 shrink-0 text-emerald-600" /> : null}
                    </div>
                    {!completedStep ? (
                      <div className="mt-3 flex flex-wrap justify-end gap-2">
                        {hasForm ? (
                          <button
                            className="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-blue-700"
                            onClick={() => onOpenStepWorkItem(step)}
                          >
                            <ClipboardList className="h-3.5 w-3.5" />
                            Mở biểu mẫu
                          </button>
                        ) : null}
                        {canCompleteStep ? (
                          <button
                            className="inline-flex items-center gap-1.5 rounded-md border border-line px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                            onClick={() => onSubmitStep(String(step.id))}
                          >
                            <FileCheck2 className="h-3.5 w-3.5" />
                            Hoàn tất bước
                          </button>
                        ) : null}
                        {formRequired && hasForm && !formSubmitted ? (
                          <span className="inline-flex items-center rounded-md bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-800 ring-1 ring-inset ring-amber-200">
                            Cần gửi biểu mẫu
                          </span>
                        ) : null}
                        {formRequired && !hasForm ? (
                          <span className="inline-flex items-center rounded-md bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700 ring-1 ring-inset ring-rose-200">
                            Thiếu biểu mẫu
                          </span>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
            <div className="mt-4 grid gap-2">
              <button
                className="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!selectedNode?.id}
                onClick={() => selectedNode?.id && onAdvance(String(selectedNode.id))}
              >
                <CheckCircle2 className="h-4 w-4" />
                Chuyển node tiếp theo
              </button>
              <button
                className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!selectedNode?.id}
                onClick={() => selectedNode?.id && onRequestApproval(String(selectedNode.id))}
              >
                <Send className="h-4 w-4" />
                Yêu cầu phê duyệt
              </button>
              <div className="flex gap-2">
                <input className="min-w-0 flex-1 rounded-md border border-line px-3 py-2 text-sm focus-ring" placeholder="Lý do chặn" value={blockReason} onChange={(event) => onBlockReasonChange(event.target.value)} />
                <button
                  className="rounded-md bg-rose-600 px-3 py-2 text-white hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={!selectedNode?.id}
                  onClick={() => selectedNode?.id && onBlock(String(selectedNode.id))}
                  title="Chặn node"
                >
                  <Ban className="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
          <div>
            <h2 className="mb-3 text-base font-semibold text-ink">Nhật ký / Xem lại</h2>
            <Timeline events={auditEvents} />
          </div>
        </aside>
      </div>
    </section>
  );
}

function buildPlanningFormRows(nodes: Record<string, any>[], run?: Record<string, any>): PlanningFormRow[] {
  const rows: PlanningFormRow[] = [];
  nodes.forEach((node) => {
    (node.steps ?? []).forEach((step: Record<string, any>) => {
      const hasSummary = hasWorkflowFormSummary(step);
      const required = Boolean(step.workflow_form_required);
      const hasInstance = Boolean(step.workflow_form_instance_id);
      if (!hasSummary && !required && !hasInstance) return;

      const workItem = stepToWorkItem(step, run);
      const formType = String(step.workflow_form_type ?? step.step_type ?? 'generic_submission');
      const formStatus = String(step.workflow_form_status ?? (hasInstance ? 'draft' : 'missing'));
      const missingInstance = required && !hasInstance;
      const blocked = step.status === 'blocked' || workItem?.status === 'blocked';
      const submitted = formStatus === 'submitted';
      const needsSubmit = !missingInstance && !blocked && !submitted;
      const formTitle = hasSummary ? workflowFormChipText(step) : workflowFormBusinessLabel(step);
      const nextAction = missingInstance ? 'Kiểm tra việc gắn biểu mẫu trước khi hoàn tất bước' : workflowFormNextAction(step);
      const searchText = [
        formTitle,
        formType,
        workflowFormTypeLabel(formType),
        node.name,
        node.description,
        step.name,
        step.description,
        workItem?.title,
      ].filter(Boolean).join(' ').toLowerCase();

      rows.push({
        id: String(step.id ?? `${node.id}:${rows.length}`),
        step,
        node,
        workItem,
        formTitle,
        formType,
        formTypeLabel: workflowFormTypeLabel(formType),
        formStatus,
        required,
        hasInstance,
        missingInstance,
        blocked,
        submitted,
        needsSubmit,
        evidenceCount: numberValue(step.evidence_count),
        nextAction,
        searchText,
      });
    });
  });
  return rows;
}

function buildPlanningFormMetrics(rows: PlanningFormRow[]): PlanningFormMetrics {
  return {
    total: rows.length,
    required: rows.filter((row) => row.required).length,
    needsSubmit: rows.filter((row) => row.needsSubmit).length,
    submitted: rows.filter((row) => row.submitted).length,
    blocked: rows.filter((row) => row.blocked).length,
    missing: rows.filter((row) => row.missingInstance).length,
  };
}

function planningFormRowMatchesFilters(row: PlanningFormRow, filters: PlanningFormFilters) {
  const search = filters.search.trim().toLowerCase();
  if (search && !row.searchText.includes(search)) return false;
  if (filters.type !== 'all' && row.formType !== filters.type) return false;
  if (filters.requirement === 'required' && !row.required) return false;
  if (filters.requirement === 'optional' && row.required) return false;
  if (filters.status === 'all') return true;
  if (filters.status === 'needs_submit') return row.needsSubmit;
  if (filters.status === 'blocked') return row.blocked;
  if (filters.status === 'missing') return row.missingInstance;
  return row.formStatus === filters.status;
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
