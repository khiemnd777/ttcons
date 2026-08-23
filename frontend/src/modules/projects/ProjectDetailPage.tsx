import { ReactNode, useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertCircle,
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  ChevronRight,
  CircleDot,
  Clock3,
  FileText,
  GitBranch,
  Loader2,
  MessageSquare,
  PackageCheck,
  Play,
  RefreshCw,
  Search,
  ShieldAlert,
  TimerReset,
} from 'lucide-react';
import clsx from 'clsx';
import { get, post, ApiError } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { EmptyState } from '../../components/EmptyState';
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { IdentityReferenceLabel, identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { dateText, percent } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import type {
  AuditLogPage,
  AuditLogRecord,
  EvidenceRecord,
  ProjectOperationsRecord,
  ProjectOperationsResponse,
  ProjectOperationsTimelineNode,
  ProjectMembersResponse,
  WorkItem,
} from '../../lib/types/domain';
import type { ActivityAttachmentOption } from '../work-items/ActivityComposer';
import { ActivityThread } from '../work-items/ActivityThread';
import { currentReturnTo } from '../work-items/workItemNavigation';
import { executionTargetPath } from '../work-items/workItemExecutionTarget';
import { ProjectEvidencePanel } from './ProjectEvidencePanel';
import { ProjectMembersPanel } from './ProjectMembersPanel';
import { ProjectWorkTab } from './ProjectWorkTab';
import {
  asArray,
  compactDateTime,
  departmentLabel,
  numberValue,
  orderWorkItemsByWorkflow,
  prioritizeWorkItems,
} from '../work-items/workItemPresentation';

type ProjectDetailTab = 'overview' | 'workflow' | 'work' | 'activity' | 'files' | 'audit' | 'members';
type WorkflowWorkFilter = 'all' | 'open' | 'waiting' | 'blocked' | 'completed' | 'skipped';

function currentTimelineNode(nodes: ProjectOperationsTimelineNode[]) {
  return nodes.find((node) => node.status === 'in_progress')
    ?? nodes.find((node) => node.status === 'blocked' || node.status === 'waiting_approval')
    ?? nodes.find((node) => !['completed', 'cancelled', 'skipped'].includes(node.status))
    ?? nodes[nodes.length - 1];
}

function nodeStatusDotTone(status?: string) {
  if (status === 'completed') return 'bg-emerald-500';
  if (status === 'blocked') return 'bg-rose-500';
  if (status === 'waiting_approval') return 'bg-amber-500';
  if (status === 'in_progress') return 'bg-blue-600';
  if (status === 'skipped' || status === 'cancelled') return 'bg-slate-300';
  return 'bg-slate-400';
}

function isOpenWorkItem(item: WorkItem) {
  return item.status !== 'completed' && item.status !== 'cancelled';
}

function workOwner(item: WorkItem) {
  if (item.assignee_name) return identityReferenceText(item.assignee_name, item.assignee_account_deleted);
  return item.owner_name ?? item.responsible_name ?? item.department_name ?? departmentLabel(item.department) ?? '-';
}

function workDueDate(item: WorkItem) {
  return item.due_date ?? item.due_at ?? item.deadline ?? item.planned_end_date;
}

export function ProjectDetailPage() {
  const { id = '' } = useParams();
  const [selectedTimelineNodeId, setSelectedTimelineNodeId] = useState('');
  const [activeTab, setActiveTab] = useState<ProjectDetailTab>('overview');
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const toast = useToast();

  const operations = useQuery({
    queryKey: ['project-operations', id],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${id}/operations`),
    enabled: Boolean(id),
    retry: false,
  });
  const members = useQuery({
    queryKey: ['project-members', id],
    queryFn: () => get<ProjectMembersResponse>(`/projects/${id}/members?include_inactive=true`),
    enabled: Boolean(id),
    retry: false,
  });
  const startPipeline = useMutation({
    mutationFn: () => post(`/projects/${id}/start-pipeline`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
      toast.success('Đã khởi động quy trình');
    },
    onError: (error) => toast.error('Không khởi động được quy trình', error),
  });

  const data = operations.data;
  const allWorkItems = useMemo(() => asArray<WorkItem>(data?.work_items), [data?.work_items]);
  const openWorkItems = useMemo(() => allWorkItems.filter(isOpenWorkItem), [allWorkItems]);
  const priorityWork = useMemo(() => prioritizeWorkItems(openWorkItems), [openWorkItems]);
  const blockers = useMemo(() => prioritizeWorkItems(asArray<WorkItem>(data?.blockers)), [data?.blockers]);
  const timeline = useMemo(() => asArray<ProjectOperationsTimelineNode>(data?.workflow_timeline), [data?.workflow_timeline]);
  const activityTargets = useMemo(() => allWorkItems.map((item) => ({
    scope_type: 'work_item',
    scope_id: item.id,
    work_item_id: item.id,
    label: `${item.project_code ? `${item.project_code} · ` : ''}${item.title}`,
  })), [allWorkItems]);
  const currentNode = useMemo(() => currentTimelineNode(timeline), [timeline]);
  const primaryWork = useMemo(() => selectProjectPriorityWork(priorityWork, blockers), [blockers, priorityWork]);

  useEffect(() => {
    if (!timeline.length) {
      if (selectedTimelineNodeId) setSelectedTimelineNodeId('');
      return;
    }
    const selectionExists = timeline.some((node) => node.id === selectedTimelineNodeId);
    if (!selectionExists) setSelectedTimelineNodeId(currentNode?.id ?? timeline[0].id);
  }, [currentNode?.id, selectedTimelineNodeId, timeline]);

  useEffect(() => {
    const tab = searchParams.get('tab');
    if (tab === 'overview' || tab === 'workflow' || tab === 'work' || tab === 'activity' || tab === 'files' || tab === 'audit' || tab === 'members') {
      setActiveTab(tab);
    }
  }, [searchParams]);

  function openWorkItem(item: WorkItem) {
    navigate(executionTargetPath(item, currentReturnTo(location)));
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
  const latestEvidence = asArray<EvidenceRecord>(data.latest_evidence);
  const attachmentOptions = buildActivityAttachmentOptions(latestEvidence);
  const activityTotalCount = numberValue(data.activity_summary?.total);
  const auditRecent = asArray<ProjectOperationsRecord>(data.audit_summary?.recent);
  const tabs: Array<{ id: ProjectDetailTab; label: string; count?: number }> = [
    { id: 'overview', label: 'Tổng quan' },
    { id: 'workflow', label: 'Quy trình', count: timeline.length },
    { id: 'work', label: 'Danh sách công việc', count: allWorkItems.length },
    { id: 'activity', label: 'Trao đổi', count: activityTotalCount },
    { id: 'files', label: 'Hồ sơ', count: numberValue(data.evidence_summary?.total ?? latestEvidence.length) },
    { id: 'audit', label: 'Nhật ký' },
    { id: 'members', label: 'Thành viên', count: members.data?.summary.active ?? 0 },
  ];

  return (
    <>
      <PageHeader
        title={`${project.code ?? data.project.code} · ${project.name ?? data.project.name}`}
        description={
          [
            project.client_name ? `Chủ đầu tư: ${project.client_name}` : null,
            project.pm_name ? `Phụ trách: ${identityReferenceText(project.pm_name, project.pm_account_deleted)}${project.pm_account_deleted ? ' · Cần phân công lại' : ''}` : null,
            project.status ? `Trạng thái: ${statusLabel(project.status)}` : null,
            project.is_sample ? 'Dữ liệu mẫu' : null,
            project.planned_end_date ? `Kết thúc: ${dateText(project.planned_end_date)}` : null,
          ].filter(Boolean).join(' · ')
        }
        actions={activeTab === 'workflow' ? undefined : (
          <HeaderActions
            hasPipeline={hasPipeline}
            startPending={startPipeline.isPending}
            primaryWork={primaryWork}
            onStartPipeline={() => startPipeline.mutate()}
            onOpenWork={openWorkItem}
            projectId={id}
          />
        )}
      />

      <ProjectDetailTabs tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />

      <div className="mt-5">
        {activeTab === 'overview' ? (
          <div className="space-y-4">
            <ProjectOverviewActionBand
              hasPipeline={hasPipeline}
              startPending={startPipeline.isPending}
              currentNode={currentNode}
              primaryWork={primaryWork}
              onStartPipeline={() => startPipeline.mutate()}
              onOpenWork={openWorkItem}
              onShowWorkflow={() => setActiveTab('workflow')}
            />
            <HealthStrip data={data} />
            <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
              <ProjectOverviewWorkPanel
                priorityWork={priorityWork}
                onOpenWork={openWorkItem}
                onShowAllWork={() => {
                  setActiveTab('work');
                }}
              />
              <ProjectInformationPanel project={project} visibility={data.visibility} />
            </div>
          </div>
        ) : null}

        {activeTab === 'workflow' ? (
          <ProjectWorkflowWorkspace
            projectId={id}
            nodes={timeline}
            workItems={allWorkItems}
            selectedNodeId={selectedTimelineNodeId}
            hasPipeline={hasPipeline}
            startPending={startPipeline.isPending}
            onSelectNode={setSelectedTimelineNodeId}
            onStartPipeline={() => startPipeline.mutate()}
            onOpenWork={openWorkItem}
          />
        ) : null}

        {activeTab === 'work' ? (
          <ProjectWorkTab rows={allWorkItems} onOpenWork={openWorkItem} />
        ) : null}

        {activeTab === 'activity' ? (
          <ProjectActivityPanel
            projectId={id}
            activityTargets={activityTargets}
            attachmentOptions={attachmentOptions}
            onChanged={() => {
              queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
            }}
          />
        ) : null}

        {activeTab === 'files' ? <ProjectEvidencePanel projectId={id} /> : null}

        {activeTab === 'audit' ? <ProjectAuditPanel projectId={id} initialRows={auditRecent} /> : null}

        {activeTab === 'members' ? (
          <ProjectMembersPanel
            projectId={id}
            data={members.data}
            loading={members.isLoading}
            error={members.error}
            onRetry={() => {
              void members.refetch();
            }}
          />
        ) : null}
      </div>
    </>
  );
}

function HeaderActions({
  hasPipeline,
  startPending,
  primaryWork,
  onStartPipeline,
  onOpenWork,
  projectId,
}: {
  hasPipeline: boolean;
  startPending: boolean;
  primaryWork?: WorkItem;
  onStartPipeline: () => void;
  onOpenWork: (item: WorkItem) => void;
  projectId: string;
}) {
  return (
    <>
      {!hasPipeline ? (
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={startPending}
          onClick={onStartPipeline}
        >
          {startPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
          Khởi động quy trình
        </button>
      ) : primaryWork ? (
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
          onClick={() => onOpenWork(primaryWork)}
        >
          {primaryWork.status === 'blocked' ? <AlertTriangle className="h-4 w-4" /> : <ArrowUpRight className="h-4 w-4" />}
          {projectPriorityActionLabel(primaryWork)}
        </button>
      ) : null}
      <Link
        to={`/projects/${projectId}/planning`}
        className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
      >
        <GitBranch className="h-4 w-4" />
        Mở kế hoạch
      </Link>
    </>
  );
}

function ProjectDetailTabs({
  tabs,
  activeTab,
  onChange,
}: {
  tabs: Array<{ id: ProjectDetailTab; label: string; count?: number }>;
  activeTab: ProjectDetailTab;
  onChange: (tab: ProjectDetailTab) => void;
}) {
  return (
    <div className="mt-5 overflow-x-auto border-b border-line">
      <div className="flex min-w-max gap-1">
        {tabs.map((tab) => {
          const active = tab.id === activeTab;
          return (
            <button
              key={tab.id}
              type="button"
              className={clsx(
                'inline-flex items-center gap-2 whitespace-nowrap border-b-2 px-3 py-2 text-sm font-semibold transition focus-ring',
                active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-600 hover:border-slate-300 hover:text-ink',
              )}
              aria-pressed={active}
              onClick={() => onChange(tab.id)}
            >
              {tab.label}
              {typeof tab.count === 'number' ? (
                <span className={clsx(
                  'rounded px-1.5 py-0.5 text-xs',
                  active ? 'bg-blue-50 text-blue-700' : 'bg-slate-100 text-slate-500',
                )}
                >
                  {tab.count}
                </span>
              ) : null}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function ProjectOverviewActionBand({
  hasPipeline,
  startPending,
  currentNode,
  primaryWork,
  onStartPipeline,
  onOpenWork,
  onShowWorkflow,
}: {
  hasPipeline: boolean;
  startPending: boolean;
  currentNode?: ProjectOperationsTimelineNode;
  primaryWork?: WorkItem;
  onStartPipeline: () => void;
  onOpenWork: (item: WorkItem) => void;
  onShowWorkflow: () => void;
}) {
  const currentProgress = Math.min(100, Math.max(0, numberValue(currentNode?.progress_percent)));

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="grid lg:grid-cols-[minmax(0,1.35fr)_minmax(280px,0.65fr)]">
        <div className={clsx('min-w-0 p-5 lg:border-r lg:border-line', primaryWork?.status === 'blocked' && 'bg-rose-50/40')}>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Cần làm tiếp theo</p>
          {!hasPipeline ? (
            <div className="mt-3">
              <h2 className="text-lg font-semibold text-ink">Khởi động quy trình công trình</h2>
              <p className="mt-1 text-sm leading-6 text-slate-600">Công trình chưa có quy trình vận hành để tạo và điều phối công việc.</p>
              <button
                type="button"
                className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 sm:w-auto"
                disabled={startPending}
                onClick={onStartPipeline}
              >
                {startPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
                Khởi động quy trình
              </button>
            </div>
          ) : primaryWork ? (
            <div className="mt-3">
              <div className="flex flex-wrap items-center gap-2">
                <StatusBadge status={primaryWork.status} />
                {primaryWork.status === 'blocked' ? <span className="text-xs font-semibold text-rose-700">Cần tháo gỡ vướng mắc</span> : null}
              </div>
              <h2 className="mt-3 text-lg font-semibold leading-7 text-ink">{primaryWork.title}</h2>
              <div className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-sm text-slate-600">
                <span>Phụ trách: <strong className="font-semibold text-slate-800">{workOwner(primaryWork)}</strong></span>
                <span>Hạn: <strong className="font-semibold text-slate-800">{dateText(workDueDate(primaryWork))}</strong></span>
              </div>
              {primaryWork.blocked_reason ? <p className="mt-3 rounded-md bg-rose-100/70 px-3 py-2 text-sm text-rose-800">{primaryWork.blocked_reason}</p> : null}
              <button
                type="button"
                className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 sm:w-auto"
                onClick={() => onOpenWork(primaryWork)}
              >
                {projectPriorityActionLabel(primaryWork)}
                <ArrowUpRight className="h-4 w-4" />
              </button>
            </div>
          ) : (
            <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4">
              <div className="flex items-start gap-3">
                <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-700" />
                <div>
                  <h2 className="font-semibold text-emerald-900">Hiện không có công việc cần can thiệp</h2>
                  <p className="mt-1 text-sm text-emerald-800">Có thể xem toàn bộ lịch sử và công việc sắp tới trong tab Danh sách công việc.</p>
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="min-w-0 bg-slate-50/70 p-5">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Giai đoạn hiện tại</p>
          {currentNode ? (
            <>
              <div className="mt-3 flex items-start justify-between gap-3">
                <h2 className="min-w-0 text-lg font-semibold text-ink">{currentNode.name}</h2>
                <StatusBadge status={currentNode.status} />
              </div>
              <div className="mt-5 flex items-center justify-between gap-3 text-xs font-semibold">
                <span className="text-slate-500">Tiến độ giai đoạn</span>
                <span className="text-slate-700">{currentProgress}%</span>
              </div>
              <div className="mt-2 h-2 overflow-hidden rounded-full bg-white ring-1 ring-slate-200">
                <div className="h-full rounded-full bg-blue-600" style={{ width: `${currentProgress}%` }} />
              </div>
              <p className="mt-2 text-sm text-slate-600">
                <strong className="font-semibold text-ink">{numberValue(currentNode.completed_steps)}/{numberValue(currentNode.step_count)}</strong> bước hoàn tất
              </p>
              <button
                type="button"
                className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700"
                onClick={onShowWorkflow}
              >
                Xem toàn bộ quy trình
                <ArrowUpRight className="h-4 w-4" />
              </button>
            </>
          ) : (
            <p className="mt-3 text-sm leading-6 text-slate-600">Chưa có giai đoạn vận hành.</p>
          )}
        </div>
      </div>
    </section>
  );
}

function ProjectInformationPanel({
  project,
  visibility,
}: {
  project: Record<string, any>;
  visibility: ProjectOperationsResponse['visibility'];
}) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Thông tin công trình</p>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <StatusBadge status={project.status} />
        {visibility?.scoped ? (
          <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Dữ liệu theo phạm vi phân quyền</span>
        ) : null}
        {visibility?.department ? (
          <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{departmentLabel(visibility.department)}</span>
        ) : null}
      </div>
      <div className="mt-4 grid gap-4 border-t border-line pt-4 sm:grid-cols-2 xl:grid-cols-1">
        <ProjectInformationMetric label="Ngân sách" value={<CurrencyText value={project.estimated_budget} />} />
        <ProjectInformationMetric label="Giá trị hợp đồng" value={<CurrencyText value={project.contract_amount} />} />
        <ProjectInformationMetric label="Tiến độ công trình" value={percent(project.progress_percent)} />
        <ProjectInformationMetric label="Thời gian" value={`${dateText(project.planned_start_date)} – ${dateText(project.planned_end_date)}`} />
      </div>
    </section>
  );
}

function ProjectInformationMetric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div>
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <div className="mt-1 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

function HealthStrip({ data }: { data: ProjectOperationsResponse }) {
  const health = data.health_counts ?? {};
  const items = [
    {
      label: 'Cần xử lý',
      value: numberValue(health.active),
      icon: <CircleDot className="h-4 w-4" />,
      className: 'text-blue-700',
    },
    {
      label: 'Quá hạn',
      value: numberValue(health.overdue),
      icon: <Clock3 className="h-4 w-4" />,
      className: numberValue(health.overdue) ? 'text-red-700' : 'text-slate-600',
    },
    {
      label: 'Bị chặn',
      value: numberValue(health.blocked),
      icon: <AlertTriangle className="h-4 w-4" />,
      className: numberValue(health.blocked) ? 'text-rose-700' : 'text-slate-600',
    },
    {
      label: 'Chờ duyệt',
      value: numberValue(health.waiting_approval),
      icon: <CheckCircle2 className="h-4 w-4" />,
      className: numberValue(health.waiting_approval) ? 'text-amber-700' : 'text-slate-600',
    },
    {
      label: 'Chờ hồ sơ',
      value: numberValue(health.waiting_evidence),
      icon: <FileText className="h-4 w-4" />,
      className: numberValue(health.waiting_evidence) ? 'text-violet-700' : 'text-slate-600',
    },
    {
      label: 'Rủi ro vật tư',
      value: numberValue(health.material_risks),
      icon: <PackageCheck className="h-4 w-4" />,
      className: numberValue(health.material_risks) ? 'text-orange-700' : 'text-slate-600',
    },
    {
      label: 'Đã hoàn tất',
      value: numberValue(health.completed),
      icon: <CheckCircle2 className="h-4 w-4" />,
      className: 'text-emerald-700',
    },
  ];

  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tình trạng cần chú ý</p>
      <div className="mt-3 grid grid-cols-2 gap-px overflow-hidden rounded-md border border-line bg-line sm:grid-cols-4 xl:grid-cols-7">
        {items.map((item) => (
          <div key={item.label} className="bg-white px-3 py-3">
            <div className={clsx('flex items-center gap-2', item.className)}>
              {item.icon}
              <span className="text-xl font-bold tabular-nums">{item.value}</span>
            </div>
            <p className="mt-1 text-xs font-semibold text-slate-500">{item.label}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function ProjectOverviewWorkPanel({
  priorityWork,
  onOpenWork,
  onShowAllWork,
}: {
  priorityWork: WorkItem[];
  onOpenWork: (item: WorkItem) => void;
  onShowAllWork: () => void;
}) {
  const previewRows = priorityWork.slice(0, 5);

  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-col gap-3 border-b border-line pb-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Công việc ưu tiên</p>
          <h2 className="mt-1 text-base font-semibold text-ink">Năm việc cần chú ý nhất</h2>
          <p className="mt-1 text-sm text-slate-600">Sắp xếp theo vướng mắc, yêu cầu duyệt và mức độ cần xử lý.</p>
        </div>
        <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          onClick={onShowAllWork}
        >
          <TimerReset className="h-4 w-4" />
          Xem toàn bộ {priorityWork.length} công việc
        </button>
      </div>
      <WorkspaceWorkTable rows={previewRows} onOpenWork={onOpenWork} />
    </section>
  );
}

function WorkspaceWorkTable({ rows, onOpenWork }: { rows: WorkItem[]; onOpenWork: (item: WorkItem) => void }) {
  if (!rows.length) {
    return <div className="pt-5"><EmptyState title="Hiện không có công việc cần can thiệp" /></div>;
  }

  return (
    <>
      <div className="hidden overflow-x-auto md:block">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="bg-white">
            <tr className="text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <th className="px-0 py-3 pr-3"><span className="sr-only">Thao tác</span></th>
              <th className="px-0 py-3 pr-4">Việc</th>
              <th className="px-3 py-3">Trạng thái</th>
              <th className="px-3 py-3">Phụ trách</th>
              <th className="px-3 py-3">Hạn</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {rows.map((item) => (
              <tr key={item.id} className="hover:bg-slate-50">
                <td className="px-0 py-3 pr-3">
                  <button
                    type="button"
                    className="inline-flex items-center justify-center rounded-md border border-line p-1.5 text-blue-700 hover:bg-blue-50"
                    title="Mở việc"
                    aria-label="Mở việc"
                    onClick={() => onOpenWork(item)}
                  >
                    <ArrowUpRight className="h-4 w-4" />
                  </button>
                </td>
                <td className="max-w-md px-0 py-3 pr-4">
                  <p className="truncate font-semibold text-ink">{item.title}</p>
                  <p className="mt-1 truncate text-xs text-slate-500">{item.workflow_name ?? item.node_name ?? departmentLabel(item.department)}</p>
                </td>
                <td className="px-3 py-3"><StatusBadge status={item.status} /></td>
                <td className="px-3 py-3 text-slate-700">{workOwner(item)}</td>
                <td className="px-3 py-3 text-slate-600">{dateText(workDueDate(item))}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="divide-y divide-line md:hidden">
        {rows.map((item) => (
          <article key={item.id} className="py-3 first:pt-4 last:pb-0">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="font-semibold leading-6 text-ink">{item.title}</p>
                <div className="mt-2"><StatusBadge status={item.status} /></div>
              </div>
              <button
                type="button"
                className="inline-flex shrink-0 items-center gap-1 rounded-md border border-line px-2.5 py-1.5 text-sm font-semibold text-blue-700 hover:bg-blue-50"
                onClick={() => onOpenWork(item)}
                aria-label={`Mở ${item.title}`}
              >
                Mở
                <ArrowUpRight className="h-3.5 w-3.5" />
              </button>
            </div>
            <div className="mt-2 grid gap-1 text-xs text-slate-500">
              <span>Phụ trách: {workOwner(item)}</span>
              <span>Hạn: {dateText(workDueDate(item))}</span>
            </div>
          </article>
        ))}
      </div>
    </>
  );
}

export function ProjectWorkflowWorkspace({
  projectId,
  nodes,
  workItems,
  selectedNodeId,
  hasPipeline,
  startPending,
  onSelectNode,
  onStartPipeline,
  onOpenWork,
  renderStageContext,
}: {
  projectId: string;
  nodes: ProjectOperationsTimelineNode[];
  workItems: WorkItem[];
  selectedNodeId: string;
  hasPipeline: boolean;
  startPending: boolean;
  onSelectNode: (nodeId: string) => void;
  onStartPipeline: () => void;
  onOpenWork: (item: WorkItem) => void;
  renderStageContext?: (node: ProjectOperationsTimelineNode) => ReactNode;
}) {
  const currentNode = useMemo(() => currentTimelineNode(nodes), [nodes]);
  const currentNodeWorkItems = useMemo(
    () => currentNode ? workItems.filter((item) => item.run_node_id === currentNode.id) : [],
    [currentNode, workItems],
  );
  const nextWork = useMemo(() => selectNextWorkflowWork(currentNodeWorkItems), [currentNodeWorkItems]);
  const selectedNode = nodes.find((node) => node.id === selectedNodeId) ?? currentNode ?? nodes[0];
  const nodeWorkItems = useMemo(
    () => selectedNode
      ? orderWorkItemsByWorkflow(workItems.filter((item) => item.run_node_id === selectedNode.id))
      : orderWorkItemsByWorkflow(workItems),
    [selectedNode, workItems],
  );

  if (!hasPipeline || !nodes.length) {
    return (
      <WorkflowEmptyState
        startPending={startPending}
        onStartPipeline={onStartPipeline}
      />
    );
  }

  return (
    <div className="space-y-4">
      <WorkflowExecutionSummary
        projectId={projectId}
        nodes={nodes}
        rows={currentNodeWorkItems}
        currentNode={currentNode}
        nextWork={nextWork}
        onOpenWork={onOpenWork}
      />
      <div className="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
        <WorkflowStageRail
          nodes={nodes}
          selectedNodeId={selectedNode?.id ?? ''}
          onSelectNode={onSelectNode}
        />
        <WorkflowWorkPanel
          rows={nodeWorkItems}
          selectedNode={selectedNode}
          onOpenWork={onOpenWork}
          stageContext={selectedNode && renderStageContext ? renderStageContext(selectedNode) : undefined}
        />
      </div>
    </div>
  );
}

function WorkflowEmptyState({ startPending, onStartPipeline }: { startPending: boolean; onStartPipeline: () => void }) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <EmptyState
        title="Chưa có quy trình vận hành"
        action={
          <button
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
            disabled={startPending}
            onClick={onStartPipeline}
          >
            {startPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
            Khởi động quy trình
          </button>
        }
      />
    </section>
  );
}

function WorkflowExecutionSummary({
  projectId,
  nodes,
  rows,
  currentNode,
  nextWork,
  onOpenWork,
}: {
  projectId: string;
  nodes: ProjectOperationsTimelineNode[];
  rows: WorkItem[];
  currentNode?: ProjectOperationsTimelineNode;
  nextWork?: WorkItem;
  onOpenWork: (item: WorkItem) => void;
}) {
  const progress = workflowProgressSummary(nodes);
  const counts = workflowWorkCounts(rows);

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="flex items-center gap-2 border-b border-line px-4 py-3">
        <GitBranch className="h-5 w-5 text-blue-700" />
        <h2 className="font-semibold text-ink">Quy trình</h2>
      </div>
      <div className="grid gap-4 p-4 sm:grid-cols-2 xl:grid-cols-[minmax(180px,1.15fr)_minmax(210px,1.25fr)_110px_110px_110px_minmax(250px,1.3fr)] xl:items-center xl:gap-0 xl:p-0">
        <div className="min-w-0 xl:border-r xl:border-line xl:px-5 xl:py-4">
          <p className="text-xs font-semibold text-slate-500">Đang ở</p>
          <p className="mt-1 truncate text-lg font-bold text-blue-700">
            {currentNode?.order_index ? `${currentNode.order_index}. ` : ''}{currentNode?.name ?? 'Chưa xác định'}
          </p>
          {nextWork ? <p className="mt-1 truncate text-xs text-slate-500" title={nextWork.title}>Tiếp theo: {nextWork.title}</p> : null}
        </div>

        <div className="xl:border-r xl:border-line xl:px-5 xl:py-4">
          <div className="flex items-center justify-between gap-3 text-xs font-semibold">
            <span className="text-slate-500">Tiến độ tổng thể</span>
            <span className="text-slate-700">{progress.completed}/{progress.total} bước · {progress.percent}%</span>
          </div>
          <div className="mt-2 h-2 overflow-hidden rounded-full bg-slate-100">
            <div className="h-full rounded-full bg-blue-600" style={{ width: `${progress.percent}%` }} />
          </div>
          {progress.skipped ? <p className="mt-1.5 text-right text-[11px] text-slate-500">{progress.skipped} bước đã bỏ qua</p> : null}
        </div>

        <WorkflowSummaryMetric value={counts.open} label="cần xử lý" tone="amber" />
        <WorkflowSummaryMetric value={counts.waiting} label="chờ duyệt" tone="slate" />
        <WorkflowSummaryMetric value={counts.blocked} label="bị chặn" tone="rose" />

        <div className="flex flex-col gap-2 sm:col-span-2 xl:col-span-1 xl:px-5 xl:py-4">
          <button
            type="button"
            className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-300"
            disabled={!nextWork}
            onClick={() => nextWork && onOpenWork(nextWork)}
          >
            Mở việc tiếp theo
            <ArrowUpRight className="h-4 w-4" />
          </button>
          <Link
            to={`/projects/${projectId}/planning`}
            className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50"
          >
            <GitBranch className="h-4 w-4" />
            Mở kế hoạch
          </Link>
        </div>
      </div>
    </section>
  );
}

function WorkflowSummaryMetric({ value, label, tone }: { value: number; label: string; tone: 'amber' | 'slate' | 'rose' }) {
  const valueTone = tone === 'amber' ? 'text-amber-600' : tone === 'rose' ? 'text-rose-600' : 'text-slate-700';
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-3 text-center xl:rounded-none xl:border-y-0 xl:border-l-0 xl:bg-white xl:py-4">
      <p className={clsx('text-2xl font-bold', valueTone)}>{value}</p>
      <p className="mt-1 text-xs font-medium text-slate-500">{label}</p>
    </div>
  );
}

function WorkflowStageRail({
  nodes,
  selectedNodeId,
  onSelectNode,
}: {
  nodes: ProjectOperationsTimelineNode[];
  selectedNodeId: string;
  onSelectNode: (nodeId: string) => void;
}) {
  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="border-b border-line px-4 py-3">
        <h2 className="text-sm font-semibold text-ink">Giai đoạn</h2>
      </div>

      <div className="p-3 xl:hidden">
        <label className="block text-xs font-semibold text-slate-500" htmlFor="workflow-stage-select">Chọn giai đoạn quy trình</label>
        <select
          id="workflow-stage-select"
          className="mt-2 w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100"
          value={selectedNodeId}
          onChange={(event) => onSelectNode(event.target.value)}
        >
          {nodes.map((node) => (
            <option key={node.id} value={node.id}>
              {node.order_index ? `${node.order_index}. ` : ''}{node.name} · {numberValue(node.completed_steps)}/{numberValue(node.step_count)}
            </option>
          ))}
        </select>
      </div>

      <ol className="hidden divide-y divide-line xl:block">
        {nodes.map((node) => {
          const selected = node.id === selectedNodeId;
          const progressPercent = Math.min(100, Math.max(0, numberValue(node.progress_percent)));
          return (
            <li key={node.id}>
              <button
                type="button"
                className={clsx(
                  'group block w-full border-l-2 px-3 py-2.5 text-left transition focus-ring',
                  selected ? 'border-blue-600 bg-blue-50/80' : 'border-transparent hover:bg-slate-50',
                )}
                aria-current={selected ? 'step' : undefined}
                onClick={() => onSelectNode(node.id)}
                title={`${node.name} · ${statusLabel(node.status)}`}
              >
                <div className="flex items-center gap-2">
                  <span className={clsx('h-2 w-2 shrink-0 rounded-full', nodeStatusDotTone(node.status))} />
                  <span className="w-5 shrink-0 text-xs font-semibold tabular-nums text-slate-500">
                    {String(node.order_index ?? '').padStart(2, '0')}
                  </span>
                  <span className={clsx('min-w-0 flex-1 truncate text-sm font-semibold', selected ? 'text-blue-700' : 'text-slate-700')}>
                    {node.name}
                  </span>
                  <span className="shrink-0 text-xs font-semibold tabular-nums text-slate-500">
                    {numberValue(node.completed_steps)}/{numberValue(node.step_count)}
                  </span>
                </div>
                <div className="ml-12 mt-1.5 h-1 overflow-hidden rounded-full bg-slate-100">
                  <div className={clsx('h-full rounded-full', selected ? 'bg-blue-600' : 'bg-slate-300')} style={{ width: `${progressPercent}%` }} />
                </div>
              </button>
            </li>
          );
        })}
      </ol>
    </section>
  );
}

function WorkflowWorkPanel({
  rows,
  selectedNode,
  onOpenWork,
  stageContext,
}: {
  rows: WorkItem[];
  selectedNode?: ProjectOperationsTimelineNode;
  onOpenWork: (item: WorkItem) => void;
  stageContext?: ReactNode;
}) {
  const [filter, setFilter] = useState<WorkflowWorkFilter>('open');
  const [search, setSearch] = useState('');
  const selectedNodeSkipped = selectedNode?.status === 'skipped' || selectedNode?.status === 'cancelled';
  const defaultFilter: WorkflowWorkFilter = selectedNodeSkipped ? 'skipped' : 'open';
  const counts = useMemo(() => workflowWorkCounts(rows), [rows]);
  const visibleRows = useMemo(() => {
    const query = search.trim().toLocaleLowerCase('vi');
    return rows.filter((item) => {
      if (!matchesWorkflowWorkFilter(item, filter)) return false;
      if (!query) return true;
      return [
        item.title,
        item.description,
        item.assignee_name,
        item.owner_name,
        item.responsible_name,
        asArray<string>(item.role_ids).join(' '),
        item.run_step_name,
        item.workflow_form_template_name,
      ]
        .some((value) => String(value ?? '').toLocaleLowerCase('vi').includes(query));
    });
  }, [filter, rows, search]);

  useEffect(() => {
    setFilter(defaultFilter);
    setSearch('');
  }, [defaultFilter, selectedNode?.id]);

  return (
    <section className="min-w-0 rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <h2 className="truncate text-lg font-semibold text-ink">
              {selectedNode?.order_index ? `${selectedNode.order_index}. ` : ''}{selectedNode?.name ?? 'Công việc quy trình'}
            </h2>
            {selectedNode?.status ? <StatusBadge status={selectedNode.status} /> : null}
          </div>
          <p className="mt-1 text-sm text-slate-500">
            {selectedNode ? `${numberValue(selectedNode.completed_steps)}/${numberValue(selectedNode.step_count)} bước hoàn tất · ` : ''}{counts.open} việc cần xử lý
          </p>
        </div>
        <span className="shrink-0 rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{rows.length} công việc</span>
      </div>

      {stageContext ? <div className="mb-4">{stageContext}</div> : null}

      {rows.length ? (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2" aria-label="Lọc công việc quy trình">
            {workflowWorkFilterOptions(counts).map((option) => (
              <button
                key={option.id}
                type="button"
                className={clsx(
                  'rounded-md border px-2.5 py-1.5 text-xs font-semibold transition focus-ring',
                  filter === option.id ? 'border-blue-600 bg-blue-50 text-blue-700' : 'border-line bg-white text-slate-600 hover:border-blue-300',
                )}
                aria-pressed={filter === option.id}
                onClick={() => setFilter(option.id)}
              >
                {option.label} {option.count}
              </button>
            ))}
          </div>

          <label className="relative block">
            <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
            <input
              className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Tìm công việc, hồ sơ, người thực hiện..."
            />
          </label>

          {selectedNodeSkipped ? (
            <div className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-700">
              <span className="font-semibold">Nhánh “{selectedNode?.name}” không được lựa chọn.</span>{' '}
              {counts.skipped} công việc được lưu ở chế độ chỉ đọc để truy vết quy trình.
            </div>
          ) : null}

          {visibleRows.length ? (
            <>
              <div className="hidden overflow-x-auto rounded-md border border-line md:block">
                <div className="min-w-[920px]">
                  <div className="grid grid-cols-[72px_minmax(260px,1.6fr)_120px_minmax(140px,0.8fr)_110px_minmax(170px,1fr)] gap-3 bg-slate-50 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-500">
                    <span><span className="sr-only">Thao tác</span></span>
                    <span>Công việc</span>
                    <span>Trạng thái</span>
                    <span>Phụ trách</span>
                    <span>Hạn</span>
                    <span>Hồ sơ / Minh chứng</span>
                  </div>
                  <div className="divide-y divide-line">
                    {visibleRows.map((item, index) => (
                      <WorkflowWorkTableRow key={item.id} item={item} fallbackIndex={index} onOpen={() => onOpenWork(item)} />
                    ))}
                  </div>
                </div>
              </div>
              <div className="space-y-2 md:hidden">
                {visibleRows.map((item, index) => (
                  <WorkflowWorkMobileCard key={item.id} item={item} fallbackIndex={index} onOpen={() => onOpenWork(item)} />
                ))}
              </div>
            </>
          ) : (
            <EmptyState
              title="Không có công việc phù hợp bộ lọc"
              action={(filter !== defaultFilter || search) ? (
                <button
                  type="button"
                  className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50"
                  onClick={() => {
                    setFilter(defaultFilter);
                    setSearch('');
                  }}
                >
                  Xóa bộ lọc
                </button>
              ) : undefined}
            />
          )}
        </div>
      ) : (
        <EmptyState title={selectedNode?.name ? `Giai đoạn ${selectedNode.name} chưa phát sinh công việc` : 'Chưa phát sinh công việc'} />
      )}
    </section>
  );
}

function WorkflowWorkTableRow({
  item,
  fallbackIndex,
  onOpen,
}: {
  item: WorkItem;
  fallbackIndex: number;
  onOpen: () => void;
}) {
  const displayStatus = item.status === 'cancelled' ? 'skipped' : item.status;
  return (
    <div className="grid grid-cols-[72px_minmax(260px,1.6fr)_120px_minmax(140px,0.8fr)_110px_minmax(170px,1fr)] items-start gap-3 px-3 py-3 text-sm hover:bg-slate-50/80">
      <button
        type="button"
        className="inline-flex items-center justify-center gap-1 rounded-md border border-line px-2 py-1.5 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
        onClick={onOpen}
        aria-label={`Mở ${item.title}`}
      >
        Mở
        <ArrowUpRight className="h-3.5 w-3.5" />
      </button>
      <div className="flex min-w-0 items-start gap-3">
        <span className="w-6 shrink-0 pt-0.5 text-xs font-semibold tabular-nums text-slate-500">
          {String(item.run_step_order_index ?? fallbackIndex + 1).padStart(2, '0')}
        </span>
        <div className="min-w-0">
          <p className="font-semibold text-ink">{item.title}</p>
          {item.description ? <p className="mt-1 line-clamp-1 text-xs text-slate-500">{item.description}</p> : null}
          {item.blocked_reason ? <p className="mt-1 line-clamp-2 text-xs font-medium text-rose-700">{item.blocked_reason}</p> : null}
        </div>
      </div>
      <div><StatusBadge status={displayStatus} /></div>
      <p className="text-sm text-slate-700">{workflowWorkOwnerLabel(item)}</p>
      <p className="text-sm tabular-nums text-slate-600">{dateText(workDueDate(item))}</p>
      <div className="text-xs text-slate-600">
        <p className="font-medium text-slate-700">{workflowFormRequirementLabel(item)}</p>
        <p className="mt-1">{numberValue(item.evidence_count)} minh chứng</p>
      </div>
    </div>
  );
}

function WorkflowWorkMobileCard({
  item,
  fallbackIndex,
  onOpen,
}: {
  item: WorkItem;
  fallbackIndex: number;
  onOpen: () => void;
}) {
  const displayStatus = item.status === 'cancelled' ? 'skipped' : item.status;
  return (
    <article className="rounded-md border border-line bg-white p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-2">
          <span className="w-6 shrink-0 pt-0.5 text-xs font-semibold tabular-nums text-slate-500">
            {String(item.run_step_order_index ?? fallbackIndex + 1).padStart(2, '0')}
          </span>
          <div className="min-w-0">
            <p className="font-semibold text-ink">{item.title}</p>
            <div className="mt-2"><StatusBadge status={displayStatus} /></div>
          </div>
        </div>
        <button
          type="button"
          className="inline-flex shrink-0 items-center gap-1 rounded-md border border-line px-2.5 py-1.5 text-sm font-semibold text-blue-700 hover:bg-blue-50"
          onClick={onOpen}
          aria-label={`Mở ${item.title}`}
        >
          Mở
          <ArrowUpRight className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="mt-3 grid gap-1 text-xs text-slate-500 sm:grid-cols-2">
        <span>Phụ trách: {workflowWorkOwnerLabel(item)}</span>
        <span>Hạn: {dateText(workDueDate(item))}</span>
        <span>{workflowFormRequirementLabel(item)}</span>
        <span>{numberValue(item.evidence_count)} minh chứng</span>
      </div>
      {item.blocked_reason ? <p className="mt-3 rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{item.blocked_reason}</p> : null}
    </article>
  );
}

function workflowWorkOwnerLabel(item: WorkItem) {
  const roleLabels = asArray<string>(item.role_ids).filter(Boolean).join(', ');
  return (item.assignee_name ? identityReferenceText(item.assignee_name, item.assignee_account_deleted) : undefined)
    ?? item.owner_name
    ?? item.responsible_name
    ?? (roleLabels || item.department_name || departmentLabel(item.department) || '-');
}

function workflowFormRequirementLabel(item: WorkItem) {
  if (item.workflow_form_required) return 'Hồ sơ bắt buộc';
  if (item.workflow_form_instance_id || item.workflow_form_template_name) return 'Hồ sơ tùy chọn';
  return 'Không yêu cầu hồ sơ';
}

function selectProjectPriorityWork(priorityWork: WorkItem[], blockers: WorkItem[]) {
  return blockers[0]
    ?? priorityWork.find((item) => item.status === 'waiting_approval' || item.status === 'waiting_review')
    ?? priorityWork[0];
}

function projectPriorityActionLabel(item: WorkItem) {
  if (item.status === 'blocked') return 'Xử lý vướng mắc';
  if (item.status === 'waiting_approval' || item.status === 'waiting_review') return 'Mở yêu cầu duyệt';
  return 'Xử lý việc ưu tiên';
}

function selectNextWorkflowWork(rows: WorkItem[]) {
  const prioritized = prioritizeWorkItems(rows.filter(isOpenWorkItem));
  return prioritized.find((item) => item.status === 'blocked')
    ?? prioritized.find((item) => item.status === 'waiting_approval' || item.status === 'waiting_review')
    ?? prioritized[0];
}

function workflowProgressSummary(nodes: ProjectOperationsTimelineNode[]) {
  const summary = nodes.reduce((acc, node) => {
    const total = numberValue(node.step_count);
    const skipped = Math.min(total, numberValue(node.skipped_steps));
    const executable = Math.max(0, total - skipped);
    const completed = Math.min(executable, numberValue(node.completed_steps));
    return {
      total: acc.total + executable,
      completed: acc.completed + completed,
      skipped: acc.skipped + skipped,
    };
  }, { total: 0, completed: 0, skipped: 0 });

  return {
    ...summary,
    percent: summary.total ? Math.round((summary.completed / summary.total) * 100) : 0,
  };
}

function workflowWorkCounts(rows: WorkItem[]) {
  return {
    all: rows.length,
    open: rows.filter((item) => isOpenWorkItem(item)).length,
    waiting: rows.filter((item) => item.status === 'waiting_approval' || item.status === 'waiting_review').length,
    blocked: rows.filter((item) => item.status === 'blocked').length,
    completed: rows.filter((item) => item.status === 'completed').length,
    skipped: rows.filter((item) => item.status === 'cancelled').length,
  };
}

function workflowWorkFilterOptions(counts: ReturnType<typeof workflowWorkCounts>): Array<{ id: WorkflowWorkFilter; label: string; count: number }> {
  return [
    { id: 'open', label: 'Cần xử lý', count: counts.open },
    { id: 'waiting', label: 'Chờ duyệt', count: counts.waiting },
    { id: 'blocked', label: 'Bị chặn', count: counts.blocked },
    { id: 'completed', label: 'Hoàn tất', count: counts.completed },
    { id: 'skipped', label: 'Đã bỏ qua', count: counts.skipped },
    { id: 'all', label: 'Tất cả', count: counts.all },
  ];
}

function matchesWorkflowWorkFilter(item: WorkItem, filter: WorkflowWorkFilter) {
  if (filter === 'all') return true;
  if (filter === 'open') return isOpenWorkItem(item);
  if (filter === 'waiting') return item.status === 'waiting_approval' || item.status === 'waiting_review';
  if (filter === 'blocked') return item.status === 'blocked';
  if (filter === 'completed') return item.status === 'completed';
  return item.status === 'cancelled';
}

function ProjectActivityPanel({
  projectId,
  activityTargets,
  attachmentOptions,
  onChanged,
}: {
  projectId: string;
  activityTargets: Array<{ scope_type: string; scope_id: string; label: string; work_item_id?: string }>;
  attachmentOptions: ActivityAttachmentOption[];
  onChanged: () => void;
}) {
  return (
    <div className="space-y-4">
      <div className="mb-4 flex items-center gap-2">
        <MessageSquare className="h-5 w-5 text-blue-700" />
        <div>
          <h2 className="font-semibold text-ink">Trao đổi</h2>
          <p className="mt-0.5 text-sm text-slate-500">Phối hợp, quyết định và theo dõi các vấn đề của công trình.</p>
        </div>
      </div>
      <ActivityThread projectId={projectId} targetOptions={activityTargets} attachmentOptions={attachmentOptions} onChanged={onChanged} />
    </div>
  );
}

function ProjectAuditPanel({ projectId, initialRows }: { projectId: string; initialRows: ProjectOperationsRecord[] }) {
  const path = `/projects/${projectId}/audit-logs?page=1&page_size=5`;
  const logs = useQuery({
    queryKey: ['audit', path],
    queryFn: () => get<AuditLogPage>(path),
    enabled: Boolean(projectId),
    retry: false,
  });
  const rows = logs.data?.items ?? initialRows.slice(0, 5) as AuditLogRecord[];

  return (
    <div className="space-y-4">
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex items-center gap-2">
          <ShieldAlert className="h-5 w-5 text-blue-700" />
          <div>
            <h2 className="font-semibold text-ink">Audit gần đây</h2>
            <p className="mt-0.5 text-sm text-slate-500">{logs.data ? `${logs.data.total} sự kiện trong công trình` : 'Năm sự kiện gần nhất trong công trình'}</p>
          </div>
        </div>
        <Link to={`/projects/${projectId}/audit`} className="text-sm font-semibold text-blue-700 hover:underline">
          Mở toàn bộ nhật ký
        </Link>
      </div>

      {logs.isLoading && !rows.length ? <LoadingSpinner /> : null}
      {logs.error ? (
        <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          Không tải được toàn bộ nhật ký. Đang hiển thị các sự kiện gần nhất nếu có.
        </div>
      ) : null}

      {!logs.isLoading && !rows.length ? <EmptyState title="Chưa có sự kiện hệ thống" /> : null}
      {rows.length ? (
        <div className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
          <div className="hidden grid-cols-[32px_120px_180px_minmax(140px,1fr)_150px] gap-3 border-b border-line bg-slate-50 px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500 md:grid">
            <span><span className="sr-only">Thao tác</span></span><span>Thời gian</span><span>Sự kiện</span><span>Người thực hiện</span><span>Đối tượng</span>
          </div>
          <div className="divide-y divide-line">
            {rows.map((row) => (
              <Link key={row.id} to={`/projects/${projectId}/audit`} className="grid gap-2 px-4 py-3 hover:bg-slate-50 md:grid-cols-[32px_120px_180px_minmax(140px,1fr)_150px] md:items-center md:gap-3">
                <ChevronRight className="hidden h-4 w-4 text-blue-700 md:block" />
                <span className="text-xs text-slate-500">{compactDateTime(row.created_at)}</span>
                <span><EnumChip group="event_type" value={row.event_type} /></span>
                <IdentityReferenceLabel name={row.actor_name} deleted={row.actor_account_deleted} className="text-sm text-slate-700" />
                <span><EnumChip group="target_type" value={row.target_type} /></span>
                <p className="text-sm leading-5 text-slate-600 md:col-span-5">{row.description ?? '-'}</p>
              </Link>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}

function buildActivityAttachmentOptions(evidence: EvidenceRecord[]): ActivityAttachmentOption[] {
  const options: ActivityAttachmentOption[] = [];
  evidence.forEach((record) => {
    const label = record.name ?? record.title ?? record.document_name ?? record.id;
    options.push({
      type: 'evidence',
      id: record.id,
      label,
      status: record.status,
      visibility: record.visibility,
    });
    if (record.document_id) {
      options.push({
        type: 'document',
        id: record.document_id,
        label: record.document_name ?? label,
        status: record.status,
        visibility: record.visibility,
      });
    }
  });
  return options;
}

function ProjectOperationsSkeleton() {
  return (
    <div className="space-y-4">
      <div className="grid overflow-hidden rounded-lg border border-line bg-white shadow-panel lg:grid-cols-[minmax(0,1.35fr)_minmax(280px,0.65fr)]">
        <div className="h-56 animate-pulse bg-white p-5 lg:border-r lg:border-line" />
        <div className="h-56 animate-pulse bg-slate-50 p-5" />
      </div>
      <div className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-line bg-line sm:grid-cols-4 xl:grid-cols-7">
        {Array.from({ length: 7 }).map((_, index) => (
          <div key={index} className="h-20 animate-pulse bg-white" />
        ))}
      </div>
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="h-80 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        <div className="h-80 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
      </div>
    </div>
  );
}

function ProjectOperationsError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const apiError = error instanceof ApiError ? error : null;
  const forbidden = apiError?.status === 403;
  const title = forbidden ? 'Bạn không có quyền xem điều hành công trình này' : 'Không tải được dữ liệu điều hành công trình';
  const description = error instanceof Error ? error.message : 'Vui lòng thử lại hoặc kiểm tra quyền truy cập.';

  return (
    <>
      <PageHeader
        title="Điều hành công trình"
        description="Tổng quan vận hành và công việc cần xử lý"
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
                Thử lại
              </button>
            ) : null}
          </div>
        </div>
      </section>
    </>
  );
}
