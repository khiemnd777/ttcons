import { ReactNode, useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertCircle,
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  CircleDot,
  Clock3,
  FileText,
  GitBranch,
  Layers3,
  Loader2,
  MessageSquare,
  PackageCheck,
  Play,
  RefreshCw,
  ShieldAlert,
  TimerReset,
  Users,
} from 'lucide-react';
import clsx from 'clsx';
import { get, post, ApiError } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { EmptyState } from '../../components/EmptyState';
import { Dialog } from '../../components/Dialog';
import { DataTable } from '../../components/DataTable';
import { useToast } from '../../components/ToastProvider';
import { dateText, percent } from '../../lib/utils/format';
import { enumLabel, statusLabel } from '../../lib/utils/status';
import type {
  EvidenceRecord,
  ProjectOperationsRecord,
  ProjectOperationsResponse,
  ProjectOperationsTimelineNode,
  WorkItem,
  WorkItemActivity,
} from '../../lib/types/domain';

type DepartmentSurface = {
  department?: string;
  total?: number;
  active?: number;
  blocked?: number;
  waiting_approval?: number;
  work_items?: WorkItem[];
};

type WorkItemActionKey =
  | 'start'
  | 'complete'
  | 'block'
  | 'resolve-blocker'
  | 'request-review'
  | 'request-approval'
  | 'approve'
  | 'reject'
  | 'request-change';

const moduleLinks = [
  ['Pipeline', 'pipeline'],
  ['Planning', 'planning'],
  ['BOQ / Dự toán', 'boq'],
  ['Mua sắm', 'procurement'],
  ['Kho', 'warehouse'],
  ['Thi công', 'operations'],
  ['Nghiệm thu', 'acceptance'],
  ['Rework', 'rework'],
  ['Bàn giao', 'handover'],
  ['Kế toán', 'accounting'],
  ['Công nợ', 'debt'],
  ['Audit', 'audit'],
];

const actionLabels: Record<WorkItemActionKey, string> = {
  start: 'Bắt đầu',
  complete: 'Hoàn tất',
  block: 'Báo blocker',
  'resolve-blocker': 'Resolve blocker',
  'request-review': 'Gửi review',
  'request-approval': 'Gửi duyệt',
  approve: 'Duyệt',
  reject: 'Từ chối',
  'request-change': 'Yêu cầu sửa',
};

const actionDescriptions: Record<WorkItemActionKey, string> = {
  start: 'Chuyển work item sang đang xử lý.',
  complete: 'Hoàn tất work item khi đủ evidence bắt buộc.',
  block: 'Ghi nhận blocker và lý do dừng việc.',
  'resolve-blocker': 'Gỡ blocker và đưa work item về trạng thái làm tiếp.',
  'request-review': 'Gửi work item sang hàng chờ review.',
  'request-approval': 'Tạo hoặc đồng bộ approval request cho work item.',
  approve: 'Duyệt và hoàn tất work item.',
  reject: 'Từ chối và trả work item về cần chỉnh sửa.',
  'request-change': 'Yêu cầu chỉnh sửa trước khi duyệt.',
};

const departmentLabels: Record<string, string> = {
  design: 'Thiết kế',
  qs: 'QS',
  procurement: 'Mua hàng',
  warehouse: 'Kho',
  construction: 'Thi công',
  operations: 'Thi công',
  qa: 'QA/QC',
  qaqc: 'QA/QC',
  accounting: 'Kế toán',
  client: 'Client',
  unassigned: 'Chưa phân công',
};

const activityLabels: Record<string, string> = {
  created: 'Tạo work item',
  started: 'Bắt đầu',
  completed: 'Hoàn tất',
  blocked: 'Báo blocker',
  blocker_resolved: 'Resolve blocker',
  review_requested: 'Gửi review',
  approval_requested: 'Gửi duyệt',
  approved: 'Đã duyệt',
  rejected: 'Từ chối',
  changes_requested: 'Yêu cầu sửa',
  evidence_added: 'Thêm evidence',
  evidence_accepted: 'Duyệt evidence',
  evidence_rejected: 'Từ chối evidence',
  comment: 'Ghi chú',
};

const evidenceLabels: Record<string, string> = {
  document: 'Tài liệu',
  photo: 'Ảnh',
  checklist: 'Checklist',
  link: 'Link',
  note: 'Ghi chú',
  other: 'Khác',
};

function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? value as T[] : [];
}

function numberValue(value: unknown) {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? number : 0;
}

function compactDateTime(value: unknown) {
  if (!value) return '-';
  const date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat('vi-VN', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

function departmentLabel(value?: string) {
  const key = String(value || 'unassigned').toLowerCase();
  return departmentLabels[key] ?? value ?? 'Chưa phân công';
}

function activityLabel(value?: string) {
  const key = String(value ?? '');
  return activityLabels[key] ?? enumLabel('event_type', key);
}

function evidenceLabel(value?: string) {
  const key = String(value ?? '');
  return evidenceLabels[key] ?? (key || '-');
}

function isWorkOverdue(item: WorkItem) {
  if (!item.due_at || ['completed', 'cancelled'].includes(item.status)) return false;
  const due = new Date(item.due_at);
  return !Number.isNaN(due.getTime()) && due.getTime() < Date.now();
}

function workItemRank(item: WorkItem) {
  if (item.status === 'blocked') return 0;
  if (isWorkOverdue(item)) return 1;
  if (item.status === 'waiting_approval' || item.status === 'waiting_review') return 2;
  if (item.status === 'waiting_evidence') return 3;
  if (item.due_at) return 4;
  return 5;
}

function prioritizeWorkItems(items: WorkItem[]) {
  return [...items].sort((a, b) => {
    const rankDiff = workItemRank(a) - workItemRank(b);
    if (rankDiff !== 0) return rankDiff;
    const aDue = a.due_at ? new Date(a.due_at).getTime() : Number.MAX_SAFE_INTEGER;
    const bDue = b.due_at ? new Date(b.due_at).getTime() : Number.MAX_SAFE_INTEGER;
    return aDue - bDue;
  });
}

function availableActions(status?: string): WorkItemActionKey[] {
  switch (status) {
    case 'pending':
      return ['start', 'block'];
    case 'ready':
    case 'needs_changes':
      return ['start', 'request-review', 'request-approval', 'block'];
    case 'in_progress':
    case 'waiting_evidence':
      return ['complete', 'request-review', 'request-approval', 'block'];
    case 'waiting_review':
    case 'waiting_approval':
      return ['approve', 'request-change', 'reject', 'block'];
    case 'blocked':
      return ['resolve-blocker'];
    default:
      return [];
  }
}

function actionRequiresReason(action: WorkItemActionKey) {
  return action === 'block' || action === 'reject' || action === 'request-change';
}

function currentTimelineNode(nodes: ProjectOperationsTimelineNode[]) {
  return nodes.find((node) => node.status === 'in_progress')
    ?? nodes.find((node) => node.status === 'blocked' || node.status === 'waiting_approval')
    ?? nodes.find((node) => node.status !== 'completed' && node.status !== 'cancelled')
    ?? nodes[nodes.length - 1];
}

function nodeTone(status?: string) {
  if (status === 'completed') return 'border-emerald-200 bg-emerald-50 text-emerald-700';
  if (status === 'blocked') return 'border-rose-200 bg-rose-50 text-rose-700';
  if (status === 'waiting_approval') return 'border-amber-200 bg-amber-50 text-amber-800';
  if (status === 'in_progress') return 'border-blue-200 bg-blue-50 text-blue-700';
  return 'border-slate-200 bg-slate-50 text-slate-600';
}

function healthTone(value: number, alertTone: string, quietTone = 'border-line bg-white') {
  return value > 0 ? alertTone : quietTone;
}

export function ProjectDetailPage() {
  const { id = '' } = useParams();
  const [selectedWork, setSelectedWork] = useState<WorkItem | null>(null);
  const [selectedDepartment, setSelectedDepartment] = useState('');
  const [actionText, setActionText] = useState('');
  const [pendingAction, setPendingAction] = useState<WorkItemActionKey | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();

  const operations = useQuery({
    queryKey: ['project-operations', id],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${id}/operations`),
    enabled: Boolean(id),
    retry: false,
  });
  const selectedWorkId = selectedWork?.id ?? '';
  const workDetail = useQuery({
    queryKey: ['work-item', selectedWorkId],
    queryFn: () => get<WorkItem>(`/work-items/${selectedWorkId}`),
    enabled: Boolean(selectedWorkId),
    retry: false,
  });

  const startPipeline = useMutation({
    mutationFn: () => post(`/projects/${id}/start-pipeline`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
      toast.success('Đã start pipeline');
    },
    onError: (error) => toast.error('Không start được pipeline', error),
  });

  const workAction = useMutation({
    mutationFn: ({ workItemId, action, comment }: { workItemId: string; action: WorkItemActionKey; comment: string }) =>
      post<WorkItem>(`/work-items/${workItemId}/${action}`, { comment, reason: comment }),
    onSuccess: (item, variables) => {
      setSelectedWork(item);
      setActionText('');
      setPendingAction(null);
      queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
      queryClient.invalidateQueries({ queryKey: ['work-item', variables.workItemId] });
      toast.success(`Đã ${actionLabels[variables.action].toLowerCase()}`);
    },
    onError: (error) => toast.error('Không thao tác được work item', error),
  });

  const data = operations.data;
  const activeWork = useMemo(() => prioritizeWorkItems(asArray<WorkItem>(data?.active_work)), [data?.active_work]);
  const blockers = useMemo(() => prioritizeWorkItems(asArray<WorkItem>(data?.blockers)), [data?.blockers]);
  const departmentSurfaces = useMemo(() => asArray<DepartmentSurface>(data?.department_surfaces), [data?.department_surfaces]);
  const timeline = useMemo(() => asArray<ProjectOperationsTimelineNode>(data?.workflow_timeline), [data?.workflow_timeline]);
  const currentNode = useMemo(() => currentTimelineNode(timeline), [timeline]);
  const detailItem = workDetail.data ?? selectedWork;

  useEffect(() => {
    if (!departmentSurfaces.length) {
      setSelectedDepartment('');
      return;
    }
    const exists = departmentSurfaces.some((surface) => (surface.department ?? 'unassigned') === selectedDepartment);
    if (!exists) setSelectedDepartment(departmentSurfaces[0].department ?? 'unassigned');
  }, [departmentSurfaces, selectedDepartment]);

  function openWorkItem(item: WorkItem) {
    setSelectedWork(item);
    setActionText('');
    setPendingAction(null);
  }

  function runAction(action: WorkItemActionKey) {
    if (!detailItem?.id) return;
    const comment = actionText.trim();
    if (actionRequiresReason(action) && !comment) {
      setPendingAction(action);
      toast.warn({ title: 'Cần nhập lý do', description: actionDescriptions[action] });
      return;
    }
    workAction.mutate({ workItemId: detailItem.id, action, comment });
  }

  if (operations.isLoading) return <ProjectOperationsSkeleton />;
  if (operations.error) {
    return (
      <ProjectOperationsError
        error={operations.error}
        onRetry={() => {
          void operations.refetch();
        }}
      />
    );
  }
  if (!data) return <EmptyState title="Không có dữ liệu điều hành công trình" />;

  const project = data.project_summary ?? data.project;
  const hasPipeline = Boolean(data.pipeline_run) || timeline.length > 0;
  const visibleMembers = asArray<Record<string, any>>(data.project?.members);
  const activeDepartment = departmentSurfaces.find((surface) => (surface.department ?? 'unassigned') === selectedDepartment)
    ?? departmentSurfaces[0];

  return (
    <>
      <PageHeader
        title={`${project.code ?? data.project.code} · ${project.name ?? data.project.name}`}
        description={
          [
            project.client_name ? `Client: ${project.client_name}` : null,
            project.pm_name ? `PM: ${project.pm_name}` : null,
            project.status ? `Trạng thái: ${statusLabel(project.status)}` : null,
            project.planned_end_date ? `Kết thúc: ${dateText(project.planned_end_date)}` : null,
          ].filter(Boolean).join(' · ')
        }
        actions={
          <HeaderActions
            hasPipeline={hasPipeline}
            startPending={startPipeline.isPending}
            activeWork={activeWork}
            blockers={blockers}
            onStartPipeline={() => startPipeline.mutate()}
            onOpenWork={openWorkItem}
            projectId={id}
          />
        }
      />

      <ProjectSummaryBand project={project} currentNode={currentNode} visibility={data.visibility} />
      <HealthStrip data={data} />

      <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)_360px]">
        <WorkflowTimeline projectId={id} nodes={timeline} hasPipeline={hasPipeline} startPending={startPipeline.isPending} onStartPipeline={() => startPipeline.mutate()} />
        <ActiveWorkPanel rows={activeWork} onOpenWork={openWorkItem} />
        <CommandCenter data={data} blockers={blockers} onOpenWork={openWorkItem} />
      </div>

      <DepartmentSurfaces
        surfaces={departmentSurfaces}
        activeSurface={activeDepartment}
        selectedDepartment={selectedDepartment}
        onSelectDepartment={setSelectedDepartment}
        onOpenWork={openWorkItem}
      />

      <ModuleShortcuts projectId={id} />

      {visibleMembers.length ? <ProjectMembers rows={visibleMembers} /> : null}

      <WorkItemDetailDialog
        open={Boolean(selectedWork)}
        item={detailItem}
        loading={workDetail.isLoading}
        actionText={actionText}
        pendingAction={pendingAction}
        actionPending={workAction.isPending}
        onActionTextChange={setActionText}
        onRunAction={runAction}
        onClose={() => {
          setSelectedWork(null);
          setActionText('');
          setPendingAction(null);
        }}
      />
    </>
  );
}

function HeaderActions({
  hasPipeline,
  startPending,
  activeWork,
  blockers,
  onStartPipeline,
  onOpenWork,
  projectId,
}: {
  hasPipeline: boolean;
  startPending: boolean;
  activeWork: WorkItem[];
  blockers: WorkItem[];
  onStartPipeline: () => void;
  onOpenWork: (item: WorkItem) => void;
  projectId: string;
}) {
  const approvalWork = activeWork.find((item) => item.status === 'waiting_approval' || item.status === 'waiting_review');
  const primaryWork = blockers[0] ?? approvalWork ?? activeWork[0];

  return (
    <>
      {!hasPipeline ? (
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={startPending}
          onClick={onStartPipeline}
        >
          {startPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
          Start pipeline
        </button>
      ) : primaryWork ? (
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
          onClick={() => onOpenWork(primaryWork)}
        >
          {primaryWork.status === 'blocked' ? <AlertTriangle className="h-4 w-4" /> : <ArrowUpRight className="h-4 w-4" />}
          {primaryWork.status === 'blocked' ? 'Mở blocker' : primaryWork.status === 'waiting_approval' ? 'Mở duyệt' : 'Mở work'}
        </button>
      ) : null}
      <a
        href="#project-modules"
        className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
      >
        <Layers3 className="h-4 w-4" />
        Modules
      </a>
      <Link
        to={`/projects/${projectId}/planning`}
        className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
      >
        <GitBranch className="h-4 w-4" />
        Planning
      </Link>
    </>
  );
}

function ProjectSummaryBand({
  project,
  currentNode,
  visibility,
}: {
  project: Record<string, any>;
  currentNode?: ProjectOperationsTimelineNode;
  visibility: ProjectOperationsResponse['visibility'];
}) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,0.7fr)]">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={project.status} />
            {visibility?.scoped ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Scoped view</span> : null}
            {visibility?.department ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{departmentLabel(visibility.department)}</span> : null}
          </div>
          <div className="mt-3 grid gap-3 text-sm text-slate-700 sm:grid-cols-2 xl:grid-cols-4">
            <SummaryMetric label="Ngân sách" value={<CurrencyText value={project.estimated_budget} />} />
            <SummaryMetric label="Hợp đồng" value={<CurrencyText value={project.contract_amount} />} />
            <SummaryMetric label="Tiến độ" value={percent(project.progress_percent)} />
            <SummaryMetric label="Lịch" value={`${dateText(project.planned_start_date)} - ${dateText(project.planned_end_date)}`} />
          </div>
        </div>
        <div className="rounded-md border border-line bg-slate-50 p-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Current workflow</p>
          {currentNode ? (
            <>
              <div className="mt-2 flex items-center justify-between gap-3">
                <p className="min-w-0 truncate text-sm font-semibold text-ink">{currentNode.name}</p>
                <StatusBadge status={currentNode.status} />
              </div>
              <div className="mt-3 h-2 overflow-hidden rounded-full bg-white">
                <div className="h-full rounded-full bg-blue-600" style={{ width: `${Math.min(100, Math.max(0, numberValue(currentNode.progress_percent)))}%` }} />
              </div>
              <p className="mt-2 text-xs text-slate-500">
                {numberValue(currentNode.completed_steps)} / {numberValue(currentNode.step_count)} steps completed
              </p>
            </>
          ) : (
            <p className="mt-2 text-sm text-slate-600">Chưa có pipeline runtime.</p>
          )}
        </div>
      </div>
    </section>
  );
}

function SummaryMetric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div>
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <div className="mt-1 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

function HealthStrip({ data }: { data: ProjectOperationsResponse }) {
  const health = data.health_counts ?? {};
  const cards = [
    {
      label: 'Need action',
      value: numberValue(health.active),
      sub: `${numberValue(health.completed)} completed`,
      icon: <CircleDot className="h-4 w-4" />,
      className: 'border-blue-200 bg-blue-50',
    },
    {
      label: 'Overdue',
      value: numberValue(health.overdue),
      sub: 'Quá hạn',
      icon: <Clock3 className="h-4 w-4" />,
      className: healthTone(numberValue(health.overdue), 'border-red-200 bg-red-50'),
    },
    {
      label: 'Blocked',
      value: numberValue(health.blocked),
      sub: 'Đang bị chặn',
      icon: <AlertTriangle className="h-4 w-4" />,
      className: healthTone(numberValue(health.blocked), 'border-rose-200 bg-rose-50'),
    },
    {
      label: 'Approval',
      value: numberValue(health.waiting_approval),
      sub: 'Chờ quyết định',
      icon: <CheckCircle2 className="h-4 w-4" />,
      className: healthTone(numberValue(health.waiting_approval), 'border-amber-200 bg-amber-50'),
    },
    {
      label: 'Evidence',
      value: numberValue(health.waiting_evidence),
      sub: 'Chờ hồ sơ',
      icon: <FileText className="h-4 w-4" />,
      className: healthTone(numberValue(health.waiting_evidence), 'border-violet-200 bg-violet-50'),
    },
    {
      label: 'Material',
      value: numberValue(health.material_risks),
      sub: 'Rủi ro vật tư',
      icon: <PackageCheck className="h-4 w-4" />,
      className: healthTone(numberValue(health.material_risks), 'border-orange-200 bg-orange-50'),
    },
  ];

  return (
    <section className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
      {cards.map((card) => (
        <div key={card.label} className={clsx('rounded-lg border p-4 shadow-panel', card.className)}>
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{card.label}</p>
              <p className="mt-2 text-2xl font-semibold text-ink">{card.value}</p>
            </div>
            <div className="rounded-md bg-white/75 p-2 text-slate-700">{card.icon}</div>
          </div>
          <p className="mt-1 text-xs text-slate-500">{card.sub}</p>
        </div>
      ))}
    </section>
  );
}

function WorkflowTimeline({
  projectId,
  nodes,
  hasPipeline,
  startPending,
  onStartPipeline,
}: {
  projectId: string;
  nodes: ProjectOperationsTimelineNode[];
  hasPipeline: boolean;
  startPending: boolean;
  onStartPipeline: () => void;
}) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <GitBranch className="h-5 w-5 text-blue-700" />
          <h2 className="font-semibold text-ink">Workflow Timeline</h2>
        </div>
        <Link to={`/projects/${projectId}/planning`} className="text-sm font-semibold text-blue-700 hover:underline">Planning</Link>
      </div>
      {!hasPipeline || !nodes.length ? (
        <EmptyState
          title="Chưa có pipeline runtime"
          action={
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
              disabled={startPending}
              onClick={onStartPipeline}
            >
              {startPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
              Start pipeline
            </button>
          }
        />
      ) : (
        <ol className="space-y-3">
          {nodes.map((node) => (
            <li key={node.id} className={clsx('rounded-md border p-3', nodeTone(node.status))}>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink">
                    {node.order_index ? `${node.order_index}. ` : null}{node.name}
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    {enumLabel('node_type', node.type)} · {numberValue(node.completed_steps)}/{numberValue(node.step_count)} steps
                  </p>
                </div>
                <StatusBadge status={node.status} />
              </div>
              <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-white">
                <div className="h-full rounded-full bg-current" style={{ width: `${Math.min(100, Math.max(0, numberValue(node.progress_percent)))}%` }} />
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

function ActiveWorkPanel({ rows, onOpenWork }: { rows: WorkItem[]; onOpenWork: (item: WorkItem) => void }) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <TimerReset className="h-5 w-5 text-blue-700" />
          <h2 className="font-semibold text-ink">Active Work</h2>
        </div>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{rows.length} open</span>
      </div>
      {rows.length ? (
        <div className="space-y-3">
          {rows.slice(0, 10).map((item) => <WorkItemCard key={item.id} item={item} onOpen={() => onOpenWork(item)} />)}
        </div>
      ) : (
        <EmptyState title="Không có work item đang mở" />
      )}
    </section>
  );
}

function WorkItemCard({ item, onOpen, compact = false }: { item: WorkItem; onOpen: () => void; compact?: boolean }) {
  const overdue = isWorkOverdue(item);
  return (
    <article className={clsx('rounded-md border border-line bg-white p-3', overdue && 'border-red-200 bg-red-50/50')}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={overdue ? 'overdue' : item.status} />
            {item.department ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{departmentLabel(item.department)}</span> : null}
          </div>
          <p className={clsx('mt-2 font-semibold text-ink', compact ? 'text-sm' : 'text-base')}>{item.title}</p>
          {item.description && !compact ? <p className="mt-1 line-clamp-2 text-sm text-slate-600">{item.description}</p> : null}
          <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-slate-500">
            <span>{item.run_node_name ?? item.run_step_name ?? item.work_type ?? 'Runtime work'}</span>
            <span>Owner: {item.assignee_name ?? (asArray<string>(item.role_ids).join(', ') || '-')}</span>
            <span>Due: {dateText(item.due_at)}</span>
            <span>Evidence: {numberValue(item.evidence_count)}</span>
          </div>
        </div>
        <button
          className="shrink-0 rounded-md border border-line px-2.5 py-1.5 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
          onClick={onOpen}
        >
          Open
        </button>
      </div>
      {item.blocked_reason ? <p className="mt-3 rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{item.blocked_reason}</p> : null}
    </article>
  );
}

function CommandCenter({
  data,
  blockers,
  onOpenWork,
}: {
  data: ProjectOperationsResponse;
  blockers: WorkItem[];
  onOpenWork: (item: WorkItem) => void;
}) {
  const approvals = asArray<ProjectOperationsRecord>(data.approvals);
  const latestEvidence = asArray<EvidenceRecord>(data.latest_evidence);
  const latestActivity = asArray<WorkItemActivity>(data.latest_activity);
  const auditRecent = asArray<ProjectOperationsRecord>(data.audit_summary?.recent);
  const workByApproval = new Map(asArray<WorkItem>(data.work_items).map((item) => [item.approval_request_id, item]));

  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center gap-2">
        <MessageSquare className="h-5 w-5 text-blue-700" />
        <h2 className="font-semibold text-ink">Command Center</h2>
      </div>
      <div className="space-y-4">
        <CommandSection title="Blockers" count={blockers.length}>
          {blockers.length ? blockers.slice(0, 4).map((item) => (
            <button key={item.id} className="block w-full rounded-md border border-rose-100 bg-rose-50 px-3 py-2 text-left hover:border-rose-200" onClick={() => onOpenWork(item)}>
              <p className="text-sm font-semibold text-ink">{item.title}</p>
              <p className="mt-1 line-clamp-2 text-xs text-rose-700">{item.blocked_reason ?? 'Đang bị chặn'}</p>
            </button>
          )) : <p className="text-sm text-slate-500">Không có blocker đang mở.</p>}
        </CommandSection>

        <CommandSection title="Approvals" count={approvals.length}>
          {approvals.length ? approvals.slice(0, 4).map((approval) => {
            const work = workByApproval.get(String(approval.id));
            return (
              <div key={approval.id} className="rounded-md border border-line px-3 py-2">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-ink">{approval.title ?? approval.target_type ?? 'Approval request'}</p>
                    <p className="mt-1 text-xs text-slate-500">{approval.requested_by_name ?? 'Requester'} · {compactDateTime(approval.created_at)}</p>
                  </div>
                  <StatusBadge status={String(approval.status ?? 'pending')} />
                </div>
                {work ? (
                  <button className="mt-2 text-xs font-semibold text-blue-700 hover:underline" onClick={() => onOpenWork(work)}>
                    Open work item
                  </button>
                ) : null}
              </div>
            );
          }) : <p className="text-sm text-slate-500">Không có approval trong scope hiện tại.</p>}
        </CommandSection>

        <CommandSection title="Latest Files" count={latestEvidence.length}>
          {latestEvidence.length ? latestEvidence.slice(0, 4).map((evidence) => (
            <div key={evidence.id} className="rounded-md border border-line px-3 py-2">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink">{evidence.name ?? evidence.title ?? evidence.document_name}</p>
                  <p className="mt-1 text-xs text-slate-500">{evidenceLabel(evidence.evidence_type)} · {evidence.work_item_title ?? 'Work item'}</p>
                </div>
                <StatusBadge status={evidence.status} />
              </div>
            </div>
          )) : <p className="text-sm text-slate-500">Chưa có evidence mới.</p>}
        </CommandSection>

        <CommandSection title="Activity / Audit" count={latestActivity.length + auditRecent.length}>
          {latestActivity.length ? latestActivity.slice(0, 5).map((activity) => (
            <div key={activity.id} className="rounded-md border border-line px-3 py-2">
              <p className="text-sm font-semibold text-ink">{activityLabel(activity.activity_type)}</p>
              <p className="mt-1 line-clamp-2 text-xs text-slate-500">
                {activity.actor_name ?? 'System'} · {activity.work_item_title ?? 'Work item'} · {compactDateTime(activity.created_at)}
              </p>
            </div>
          )) : <p className="text-sm text-slate-500">Chưa có activity mới.</p>}
        </CommandSection>
      </div>
    </section>
  );
}

function CommandSection({ title, count, children }: { title: string; count: number; children: ReactNode }) {
  return (
    <div>
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">{title}</h3>
        <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{count}</span>
      </div>
      <div className="space-y-2">{children}</div>
    </div>
  );
}

function DepartmentSurfaces({
  surfaces,
  activeSurface,
  selectedDepartment,
  onSelectDepartment,
  onOpenWork,
}: {
  surfaces: DepartmentSurface[];
  activeSurface?: DepartmentSurface;
  selectedDepartment: string;
  onSelectDepartment: (department: string) => void;
  onOpenWork: (item: WorkItem) => void;
}) {
  return (
    <section className="mt-5 rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="font-semibold text-ink">Department Surfaces</h2>
          <p className="mt-1 text-sm text-slate-600">Các bề mặt hiển thị theo scope mà server trả về cho người dùng hiện tại.</p>
        </div>
        <div className="flex overflow-x-auto rounded-md border border-line bg-slate-50 p-1">
          {surfaces.length ? surfaces.map((surface) => {
            const department = surface.department ?? 'unassigned';
            const selected = department === selectedDepartment;
            return (
              <button
                key={department}
                className={clsx(
                  'whitespace-nowrap rounded px-3 py-1.5 text-sm font-semibold',
                  selected ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600 hover:bg-white/70',
                )}
                onClick={() => onSelectDepartment(department)}
              >
                {departmentLabel(department)}
              </button>
            );
          }) : <span className="px-3 py-1.5 text-sm text-slate-500">Không có surface</span>}
        </div>
      </div>

      {activeSurface ? (
        <div className="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
          <div className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{departmentLabel(activeSurface.department)}</p>
            <div className="mt-3 grid grid-cols-2 gap-2">
              <SurfaceMetric label="Total" value={numberValue(activeSurface.total)} />
              <SurfaceMetric label="Active" value={numberValue(activeSurface.active)} />
              <SurfaceMetric label="Blocked" value={numberValue(activeSurface.blocked)} />
              <SurfaceMetric label="Approval" value={numberValue(activeSurface.waiting_approval)} />
            </div>
          </div>
          <div className="space-y-3">
            {asArray<WorkItem>(activeSurface.work_items).length ? (
              prioritizeWorkItems(asArray<WorkItem>(activeSurface.work_items)).map((item) => (
                <WorkItemCard key={item.id} item={item} compact onOpen={() => onOpenWork(item)} />
              ))
            ) : (
              <EmptyState title="Department này chưa có work item trong scope hiện tại" />
            )}
          </div>
        </div>
      ) : (
        <EmptyState title="Không có department surface trong scope hiện tại" />
      )}
    </section>
  );
}

function SurfaceMetric({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md bg-white p-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1 text-xl font-semibold text-ink">{value}</p>
    </div>
  );
}

function ModuleShortcuts({ projectId }: { projectId: string }) {
  return (
    <section id="project-modules" className="mt-5 rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-3 flex items-center gap-2">
        <Layers3 className="h-5 w-5 text-blue-700" />
        <h2 className="font-semibold text-ink">Secondary Module Shortcuts</h2>
      </div>
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-6">
        {moduleLinks.map(([label, path]) => (
          <Link
            key={path}
            to={`/projects/${projectId}/${path}`}
            className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700"
          >
            {label}
          </Link>
        ))}
      </div>
    </section>
  );
}

function ProjectMembers({ rows }: { rows: Record<string, any>[] }) {
  return (
    <section className="mt-5">
      <div className="mb-3 flex items-center gap-2">
        <Users className="h-5 w-5 text-blue-700" />
        <h2 className="font-semibold text-ink">Thành viên công trình</h2>
      </div>
      <DataTable
        rows={rows}
        columns={[
          { key: 'full_name', header: 'Nhân sự' },
          { key: 'department', header: 'Phòng ban' },
          { key: 'position', header: 'Chức vụ' },
          { key: 'role_ids', header: 'Role', render: (row) => Array.isArray(row.role_ids) ? row.role_ids.join(', ') : '-' },
        ]}
      />
    </section>
  );
}

function WorkItemDetailDialog({
  open,
  item,
  loading,
  actionText,
  pendingAction,
  actionPending,
  onActionTextChange,
  onRunAction,
  onClose,
}: {
  open: boolean;
  item: WorkItem | null;
  loading: boolean;
  actionText: string;
  pendingAction: WorkItemActionKey | null;
  actionPending: boolean;
  onActionTextChange: (value: string) => void;
  onRunAction: (action: WorkItemActionKey) => void;
  onClose: () => void;
}) {
  const actions = availableActions(item?.status);
  const evidence = asArray<EvidenceRecord>(item?.evidence);
  const activities = asArray<WorkItemActivity>(item?.activities);
  const requirement = item?.evidence_requirement_json ?? {};

  return (
    <Dialog
      open={open}
      title={item?.title ?? 'Work item'}
      description={item ? `${departmentLabel(item.department)} · ${item.run_node_name ?? item.run_step_name ?? item.work_type ?? 'Runtime work'}` : undefined}
      onClose={onClose}
      size="xl"
      footer={
        item ? (
          <div className="space-y-3">
            {actions.length ? (
              <textarea
                className={clsx(
                  'min-h-20 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
                  pendingAction && actionRequiresReason(pendingAction) && !actionText.trim() ? 'border-amber-300 bg-amber-50' : 'border-line bg-white',
                )}
                value={actionText}
                onChange={(event) => onActionTextChange(event.target.value)}
                placeholder="Ghi chú / lý do cho thao tác"
              />
            ) : null}
            <div className="flex flex-wrap justify-end gap-2">
              <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>
                Đóng
              </button>
              {actions.map((action, index) => (
                <button
                  key={action}
                  className={clsx(
                    'inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60',
                    index === 0
                      ? 'bg-blue-600 text-white hover:bg-blue-700'
                      : action === 'reject' || action === 'block'
                        ? 'border border-rose-200 text-rose-700 hover:bg-rose-50'
                        : 'border border-line text-slate-700 hover:bg-slate-100',
                  )}
                  disabled={actionPending}
                  onClick={() => onRunAction(action)}
                  title={actionDescriptions[action]}
                >
                  {actionPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                  {actionLabels[action]}
                </button>
              ))}
            </div>
          </div>
        ) : null
      }
    >
      {loading && !item ? <LoadingSpinner /> : null}
      {item ? (
        <div className="space-y-5">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />
            {item.priority ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Priority: {item.priority}</span> : null}
            {item.work_type ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{item.work_type}</span> : null}
          </div>

          <div className="grid gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4">
            <DetailMetric label="Owner" value={item.assignee_name ?? (asArray<string>(item.role_ids).join(', ') || '-')} />
            <DetailMetric label="Department" value={departmentLabel(item.department)} />
            <DetailMetric label="Due" value={dateText(item.due_at)} />
            <DetailMetric label="Evidence" value={numberValue(item.evidence_count)} />
          </div>

          {item.description ? <p className="rounded-md border border-line bg-slate-50 p-3 text-sm leading-6 text-slate-700">{item.description}</p> : null}
          {item.blocked_reason ? <p className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm leading-6 text-rose-700">{item.blocked_reason}</p> : null}

          <section>
            <h3 className="mb-2 text-sm font-semibold text-ink">Required Evidence</h3>
            <div className="flex flex-wrap gap-2">
              {requirement.checklist_required ? <RequirementChip label="Checklist" /> : null}
              {requirement.document_required ? <RequirementChip label="Document" /> : null}
              {requirement.photo_required ? <RequirementChip label="Photo" /> : null}
              {requirement.required_permission ? <RequirementChip label={`Permission: ${requirement.required_permission}`} /> : null}
              {!requirement.checklist_required && !requirement.document_required && !requirement.photo_required && !requirement.required_permission ? (
                <span className="text-sm text-slate-500">Không có requirement đặc biệt.</span>
              ) : null}
            </div>
          </section>

          <div className="grid gap-4 xl:grid-cols-2">
            <section>
              <h3 className="mb-2 text-sm font-semibold text-ink">Evidence</h3>
              {evidence.length ? (
                <div className="space-y-2">
                  {evidence.map((record) => (
                    <div key={record.id} className="rounded-md border border-line px-3 py-2">
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold text-ink">{record.name ?? record.title ?? record.document_name}</p>
                          <p className="mt-1 text-xs text-slate-500">{evidenceLabel(record.evidence_type)} · {record.visibility ?? 'project'} · {compactDateTime(record.created_at)}</p>
                        </div>
                        <StatusBadge status={record.status} />
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <EmptyState title="Chưa có evidence cho work item này" />
              )}
            </section>
            <section>
              <h3 className="mb-2 text-sm font-semibold text-ink">Activity</h3>
              {activities.length ? (
                <div className="space-y-2">
                  {activities.map((activity) => (
                    <div key={activity.id} className="rounded-md border border-line px-3 py-2">
                      <p className="text-sm font-semibold text-ink">{activityLabel(activity.activity_type)}</p>
                      <p className="mt-1 text-xs text-slate-500">
                        {activity.actor_name ?? 'System'} · {compactDateTime(activity.created_at)}
                      </p>
                      {activity.comment ? <p className="mt-2 text-sm text-slate-700">{activity.comment}</p> : null}
                    </div>
                  ))}
                </div>
              ) : (
                <EmptyState title="Chưa có activity cho work item này" />
              )}
            </section>
          </div>
        </div>
      ) : null}
    </Dialog>
  );
}

function DetailMetric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-white p-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <div className="mt-1 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

function RequirementChip({ label }: { label: string }) {
  return <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700 ring-1 ring-inset ring-blue-200">{label}</span>;
}

function ProjectOperationsSkeleton() {
  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-line bg-white p-5 shadow-panel">
        <LoadingSpinner />
      </div>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
        {Array.from({ length: 6 }).map((_, index) => (
          <div key={index} className="h-28 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        ))}
      </div>
      <div className="grid gap-4 xl:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)_360px]">
        <div className="h-96 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        <div className="h-96 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        <div className="h-96 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
      </div>
    </div>
  );
}

function ProjectOperationsError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const apiError = error instanceof ApiError ? error : null;
  const forbidden = apiError?.status === 403;
  const title = forbidden ? 'Bạn không có quyền xem điều hành công trình này' : 'Không tải được Project Operations';
  const description = error instanceof Error ? error.message : 'Vui lòng thử lại hoặc kiểm tra quyền truy cập.';

  return (
    <>
      <PageHeader
        title="Điều hành công trình"
        description="Project Operations Dashboard"
        actions={
          <Link to="/projects" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100">
            Quay lại công trình
          </Link>
        }
      />
      <section className="rounded-lg border border-line bg-white p-6 shadow-panel">
        <div className="flex items-start gap-3">
          {forbidden ? <ShieldAlert className="mt-0.5 h-6 w-6 text-amber-600" /> : <AlertCircle className="mt-0.5 h-6 w-6 text-rose-600" />}
          <div className="min-w-0 flex-1">
            <h2 className="text-base font-semibold text-ink">{title}</h2>
            <p className="mt-1 text-sm leading-6 text-slate-600">{description}</p>
            {!forbidden ? (
              <button
                className="mt-4 inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
                onClick={onRetry}
              >
                <RefreshCw className="h-4 w-4" />
                Retry
              </button>
            ) : null}
          </div>
        </div>
      </section>
    </>
  );
}
