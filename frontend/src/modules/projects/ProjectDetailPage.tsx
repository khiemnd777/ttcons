import { ReactNode, useEffect, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
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
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { Timeline } from '../../components/Timeline';
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
import type { ActivityAttachmentOption } from '../work-items/ActivityComposer';
import { ActivityThread } from '../work-items/ActivityThread';
import { EvidenceGallery, type EvidenceReviewAction } from '../work-items/EvidenceGallery';
import { WorkItemCard } from '../work-items/WorkItemCard';
import { WorkItemDetailDialog } from '../work-items/WorkItemDetailDialog';
import {
  asArray,
  compactDateTime,
  departmentLabel,
  numberValue,
  orderWorkItemsByWorkflow,
  prioritizeWorkItems,
} from '../work-items/workItemPresentation';

type DepartmentSurface = {
  department?: string;
  total?: number;
  active?: number;
  blocked?: number;
  waiting_approval?: number;
  work_items?: WorkItem[];
};

type ProjectDetailTab = 'overview' | 'workflow' | 'work' | 'activity' | 'files' | 'audit' | 'members';

const ALL_WORK_SURFACE = '__all__';
const WORK_PAGE_SIZE = 10;

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

function isOpenWorkItem(item: WorkItem) {
  return item.status !== 'completed' && item.status !== 'cancelled';
}

function workOwner(item: WorkItem) {
  return item.assignee_name ?? item.owner_name ?? item.responsible_name ?? item.department_name ?? departmentLabel(item.department) ?? '-';
}

function workDueDate(item: WorkItem) {
  return item.due_date ?? item.due_at ?? item.deadline ?? item.planned_end_date;
}

export function ProjectDetailPage() {
  const { id = '' } = useParams();
  const [selectedWork, setSelectedWork] = useState<WorkItem | null>(null);
  const [selectedDepartment, setSelectedDepartment] = useState(ALL_WORK_SURFACE);
  const [selectedTimelineNodeId, setSelectedTimelineNodeId] = useState('');
  const [activeTab, setActiveTab] = useState<ProjectDetailTab>('overview');
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const toast = useToast();

  const operations = useQuery({
    queryKey: ['project-operations', id],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${id}/operations`),
    enabled: Boolean(id),
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

  const data = operations.data;
  const allWorkItems = useMemo(() => asArray<WorkItem>(data?.work_items), [data?.work_items]);
  const openWorkItems = useMemo(() => allWorkItems.filter(isOpenWorkItem), [allWorkItems]);
  const workflowWork = useMemo(() => orderWorkItemsByWorkflow(openWorkItems), [openWorkItems]);
  const priorityWork = useMemo(() => prioritizeWorkItems(openWorkItems), [openWorkItems]);
  const blockers = useMemo(() => prioritizeWorkItems(asArray<WorkItem>(data?.blockers)), [data?.blockers]);
  const departmentSurfaces = useMemo(() => asArray<DepartmentSurface>(data?.department_surfaces), [data?.department_surfaces]);
  const timeline = useMemo(() => asArray<ProjectOperationsTimelineNode>(data?.workflow_timeline), [data?.workflow_timeline]);
  const activityTargets = useMemo(() => allWorkItems.map((item) => ({
    scope_type: 'work_item',
    scope_id: item.id,
    work_item_id: item.id,
    label: `${item.project_code ? `${item.project_code} · ` : ''}${item.title}`,
  })), [allWorkItems]);
  const currentNode = useMemo(() => currentTimelineNode(timeline), [timeline]);

  useEffect(() => {
    if (!timeline.length) {
      if (selectedTimelineNodeId) setSelectedTimelineNodeId('');
      return;
    }
    const selectionExists = timeline.some((node) => node.id === selectedTimelineNodeId);
    if (!selectionExists) setSelectedTimelineNodeId(currentNode?.id ?? timeline[0].id);
  }, [currentNode?.id, selectedTimelineNodeId, timeline]);

  useEffect(() => {
    if (!departmentSurfaces.length) {
      if (selectedDepartment !== ALL_WORK_SURFACE) setSelectedDepartment(ALL_WORK_SURFACE);
      return;
    }
    const exists = selectedDepartment === ALL_WORK_SURFACE
      || departmentSurfaces.some((surface) => (surface.department ?? 'unassigned') === selectedDepartment);
    if (!exists) setSelectedDepartment(ALL_WORK_SURFACE);
  }, [departmentSurfaces, selectedDepartment]);

  useEffect(() => {
    const tab = searchParams.get('tab');
    if (tab === 'overview' || tab === 'workflow' || tab === 'work' || tab === 'activity' || tab === 'files' || tab === 'audit' || tab === 'members') {
      setActiveTab(tab);
    }
  }, [searchParams]);

  function openWorkItem(item: WorkItem) {
    setSelectedWork(item);
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
  const latestEvidence = asArray<EvidenceRecord>(data.latest_evidence);
  const attachmentOptions = buildActivityAttachmentOptions(latestEvidence);
  const latestActivity = asArray<WorkItemActivity>(data.latest_activity);
  const auditRecent = asArray<ProjectOperationsRecord>(data.audit_summary?.recent);
  const selectedTimelineNode = timeline.find((node) => node.id === selectedTimelineNodeId) ?? currentNode ?? timeline[0];
  const nodeActiveWork = selectedTimelineNode
    ? orderWorkItemsByWorkflow(openWorkItems.filter((item) => item.run_node_id === selectedTimelineNode.id))
    : workflowWork;
  const tabs: Array<{ id: ProjectDetailTab; label: string; count?: number }> = [
    { id: 'overview', label: 'Tổng quan', count: numberValue(data.health_counts?.active) },
    { id: 'workflow', label: 'Workflow', count: timeline.length },
    { id: 'work', label: 'Công việc', count: workflowWork.length },
    { id: 'activity', label: 'Trao đổi', count: latestActivity.length },
    { id: 'files', label: 'Hồ sơ', count: latestEvidence.length },
    { id: 'audit', label: 'Audit', count: auditRecent.length },
    { id: 'members', label: 'Thành viên', count: visibleMembers.length },
  ];

  return (
    <>
      <PageHeader
        title={`${project.code ?? data.project.code} · ${project.name ?? data.project.name}`}
        description={
          [
            project.client_name ? `Client: ${project.client_name}` : null,
            project.pm_name ? `Phụ trách: ${project.pm_name}` : null,
            project.status ? `Trạng thái: ${statusLabel(project.status)}` : null,
            project.planned_end_date ? `Kết thúc: ${dateText(project.planned_end_date)}` : null,
          ].filter(Boolean).join(' · ')
        }
        actions={
          <HeaderActions
            hasPipeline={hasPipeline}
            startPending={startPipeline.isPending}
            priorityWork={priorityWork}
            blockers={blockers}
            onStartPipeline={() => startPipeline.mutate()}
            onOpenWork={openWorkItem}
            projectId={id}
          />
        }
      />

      <ProjectDetailTabs tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />

      <div className="mt-5">
        {activeTab === 'overview' ? (
          <div className="space-y-4">
            <ProjectSummaryBand project={project} currentNode={currentNode} visibility={data.visibility} />
            <HealthStrip data={data} />
            <ProjectOverviewWorkPanel
              priorityWork={priorityWork}
              blockers={blockers}
              onOpenWork={openWorkItem}
              onShowAllWork={() => {
                setSelectedDepartment(ALL_WORK_SURFACE);
                setActiveTab('work');
              }}
            />
          </div>
        ) : null}

        {activeTab === 'workflow' ? (
          <div className="grid gap-4 xl:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)]">
            <WorkflowTimeline
              projectId={id}
              nodes={timeline}
              selectedNodeId={selectedTimelineNode?.id ?? ''}
              hasPipeline={hasPipeline}
              startPending={startPipeline.isPending}
              onStartPipeline={() => startPipeline.mutate()}
              onSelectNode={setSelectedTimelineNodeId}
            />
            <ActiveWorkPanel rows={nodeActiveWork} selectedNode={selectedTimelineNode} onOpenWork={openWorkItem} />
          </div>
        ) : null}

        {activeTab === 'work' ? (
          <DepartmentSurfaces
            surfaces={departmentSurfaces}
            allRows={workflowWork}
            selectedDepartment={selectedDepartment}
            onSelectDepartment={setSelectedDepartment}
            onOpenWork={openWorkItem}
          />
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

        {activeTab === 'files' ? <ProjectFilesPanel projectId={id} evidence={latestEvidence} /> : null}

        {activeTab === 'audit' ? <ProjectAuditPanel projectId={id} initialRows={auditRecent} /> : null}

        {activeTab === 'members' ? (
          visibleMembers.length ? <ProjectMembers rows={visibleMembers} /> : <EmptyState title="Chưa có thành viên công trình" />
        ) : null}
      </div>

      <WorkItemDetailDialog
        open={Boolean(selectedWork)}
        item={selectedWork}
        onClose={() => {
          setSelectedWork(null);
        }}
        onChanged={(item) => {
          setSelectedWork(item);
          queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
        }}
      />
    </>
  );
}

function HeaderActions({
  hasPipeline,
  startPending,
  priorityWork,
  blockers,
  onStartPipeline,
  onOpenWork,
  projectId,
}: {
  hasPipeline: boolean;
  startPending: boolean;
  priorityWork: WorkItem[];
  blockers: WorkItem[];
  onStartPipeline: () => void;
  onOpenWork: (item: WorkItem) => void;
  projectId: string;
}) {
  const approvalWork = priorityWork.find((item) => item.status === 'waiting_approval' || item.status === 'waiting_review');
  const primaryWork = blockers[0] ?? approvalWork ?? priorityWork[0];

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
  const items = [
    {
      label: 'Need action',
      value: numberValue(health.active),
      icon: <CircleDot className="h-4 w-4" />,
      className: 'text-blue-700',
    },
    {
      label: 'Overdue',
      value: numberValue(health.overdue),
      icon: <Clock3 className="h-4 w-4" />,
      className: numberValue(health.overdue) ? 'text-red-700' : 'text-slate-600',
    },
    {
      label: 'Blocked',
      value: numberValue(health.blocked),
      icon: <AlertTriangle className="h-4 w-4" />,
      className: numberValue(health.blocked) ? 'text-rose-700' : 'text-slate-600',
    },
    {
      label: 'Approval',
      value: numberValue(health.waiting_approval),
      icon: <CheckCircle2 className="h-4 w-4" />,
      className: numberValue(health.waiting_approval) ? 'text-amber-700' : 'text-slate-600',
    },
    {
      label: 'Evidence',
      value: numberValue(health.waiting_evidence),
      icon: <FileText className="h-4 w-4" />,
      className: numberValue(health.waiting_evidence) ? 'text-violet-700' : 'text-slate-600',
    },
    {
      label: 'Material',
      value: numberValue(health.material_risks),
      icon: <PackageCheck className="h-4 w-4" />,
      className: numberValue(health.material_risks) ? 'text-orange-700' : 'text-slate-600',
    },
  ];

  return (
    <section className="rounded-lg border border-line bg-white px-4 py-3 shadow-panel">
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        {items.map((item) => (
          <div key={item.label} className="inline-flex items-center gap-2 border-r border-line pr-5 last:border-r-0 last:pr-0">
            <span className={item.className}>{item.icon}</span>
            <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">{item.label}</span>
            <span className="text-sm font-semibold text-ink">{item.value}</span>
          </div>
        ))}
        <span className="text-xs text-slate-500">{numberValue(health.completed)} completed</span>
      </div>
    </section>
  );
}

function ProjectOverviewWorkPanel({
  priorityWork,
  blockers,
  onOpenWork,
  onShowAllWork,
}: {
  priorityWork: WorkItem[];
  blockers: WorkItem[];
  onOpenWork: (item: WorkItem) => void;
  onShowAllWork: () => void;
}) {
  const approvalCount = priorityWork.filter((item) => item.status === 'waiting_approval' || item.status === 'waiting_review').length;
  const blockerCount = priorityWork.filter((item) => item.status === 'blocked').length;

  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-col gap-3 border-b border-line pb-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tổng quan vận hành</p>
          <h2 className="mt-1 text-base font-semibold text-ink">Hàng ưu tiên</h2>
          <p className="mt-1 text-sm text-slate-600">Các việc cần can thiệp sớm, tách khỏi thứ tự workflow trong tab Workflow.</p>
        </div>
        <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          onClick={onShowAllWork}
        >
          <TimerReset className="h-4 w-4" />
          Mở danh sách việc
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 border-b border-line py-3 text-sm">
        <span><span className="font-semibold text-ink">{priorityWork.length}</span> việc đang mở</span>
        <span><span className="font-semibold text-rose-700">{blockerCount}</span> blocker</span>
        <span><span className="font-semibold text-amber-700">{approvalCount}</span> chờ duyệt</span>
        <span><span className="font-semibold text-slate-700">{blockers.length}</span> blocker toàn công trình</span>
      </div>

      <WorkspaceWorkTable rows={priorityWork.slice(0, 8)} onOpenWork={onOpenWork} />
    </section>
  );
}

function WorkspaceWorkTable({ rows, onOpenWork }: { rows: WorkItem[]; onOpenWork: (item: WorkItem) => void }) {
  if (!rows.length) {
    return <div className="pt-5"><EmptyState title="Không có việc đang mở trong folder này" /></div>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-line text-sm">
        <thead className="bg-white">
          <tr className="text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
            <th className="px-0 py-3 pr-4">Việc</th>
            <th className="px-3 py-3">Trạng thái</th>
            <th className="px-3 py-3">Phụ trách</th>
            <th className="px-3 py-3">Hạn</th>
            <th className="px-0 py-3 pl-3 text-right">Mở</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {rows.map((item) => (
            <tr key={item.id} className="hover:bg-slate-50">
              <td className="max-w-md px-0 py-3 pr-4">
                <p className="truncate font-semibold text-ink">{item.title}</p>
                <p className="mt-1 truncate text-xs text-slate-500">{item.workflow_name ?? item.node_name ?? departmentLabel(item.department)}</p>
              </td>
              <td className="px-3 py-3"><StatusBadge status={item.status} /></td>
              <td className="px-3 py-3 text-slate-700">{workOwner(item)}</td>
              <td className="px-3 py-3 text-slate-600">{dateText(workDueDate(item))}</td>
              <td className="px-0 py-3 pl-3 text-right">
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
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function NextWorkPanel({
  rows,
  onOpenWork,
  onShowAll,
}: {
  rows: WorkItem[];
  onOpenWork: (item: WorkItem) => void;
  onShowAll: () => void;
}) {
  const previewRows = rows.slice(0, 6);
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <TimerReset className="h-5 w-5 shrink-0 text-blue-700" />
          <h2 className="truncate font-semibold text-ink">Next Work</h2>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">
            {previewRows.length} / {rows.length} việc phải làm
          </span>
          {rows.length > previewRows.length ? (
            <button
              type="button"
              className="rounded-md border border-line px-2.5 py-1.5 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
              onClick={onShowAll}
            >
              Xem tất cả
            </button>
          ) : null}
        </div>
      </div>
      {previewRows.length ? (
        <div className="space-y-3">
          {previewRows.map((item) => (
            <WorkItemCard key={item.id} item={item} compact showWorkflowPrefix onOpen={() => onOpenWork(item)} />
          ))}
        </div>
      ) : (
        <EmptyState title="Không có work item đang mở" />
      )}
    </section>
  );
}

function WorkflowTimeline({
  projectId,
  nodes,
  selectedNodeId,
  hasPipeline,
  startPending,
  onStartPipeline,
  onSelectNode,
}: {
  projectId: string;
  nodes: ProjectOperationsTimelineNode[];
  selectedNodeId: string;
  hasPipeline: boolean;
  startPending: boolean;
  onStartPipeline: () => void;
  onSelectNode: (nodeId: string) => void;
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
          {nodes.map((node) => {
            const selected = node.id === selectedNodeId;
            return (
              <li key={node.id}>
                <button
                  type="button"
                  className={clsx(
                    'block w-full rounded-md border p-3 text-left transition hover:border-blue-300 hover:shadow-sm focus-ring',
                    nodeTone(node.status),
                    selected && 'ring-2 ring-blue-500 ring-offset-1',
                  )}
                  aria-pressed={selected}
                  onClick={() => onSelectNode(node.id)}
                >
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
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}

function ActiveWorkPanel({
  rows,
  selectedNode,
  onOpenWork,
}: {
  rows: WorkItem[];
  selectedNode?: ProjectOperationsTimelineNode;
  onOpenWork: (item: WorkItem) => void;
}) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <TimerReset className="h-5 w-5 shrink-0 text-blue-700" />
          <h2 className="truncate font-semibold text-ink">
            Công việc theo thứ tự workflow{selectedNode?.name ? ` · ${selectedNode.name}` : ''}
          </h2>
        </div>
        <span className="shrink-0 rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{rows.length} đang mở</span>
      </div>
      {rows.length ? (
        <div className="space-y-3">
          {rows.map((item) => <WorkItemCard key={item.id} item={item} onOpen={() => onOpenWork(item)} />)}
        </div>
      ) : (
        <EmptyState title={selectedNode?.name ? `Không có work item đang mở trong ${selectedNode.name}` : 'Không có work item đang mở'} />
      )}
    </section>
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
  const workByApproval = new Map(asArray<WorkItem>(data.work_items).map((item) => [item.approval_request_id, item]));

  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center gap-2">
        <AlertTriangle className="h-5 w-5 text-blue-700" />
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
                    <p className="truncate text-sm font-semibold text-ink">{approval.title ?? (approval.target_type ? enumLabel('target_type', approval.target_type) : 'Approval request')}</p>
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

        <CommandSection title="Material risks" count={numberValue(data.health_counts?.material_risks)}>
          {numberValue(data.health_counts?.material_risks) ? (
            <p className="rounded-md border border-orange-100 bg-orange-50 px-3 py-2 text-sm text-orange-800">
              Có rủi ro vật tư cần kiểm tra trong các module Kho hoặc Mua sắm.
            </p>
          ) : (
            <p className="text-sm text-slate-500">Không có rủi ro vật tư đang mở.</p>
          )}
        </CommandSection>
      </div>
    </section>
  );
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
        <h2 className="font-semibold text-ink">Trao đổi</h2>
      </div>
      <ActivityThread projectId={projectId} targetOptions={activityTargets} attachmentOptions={attachmentOptions} onChanged={onChanged} />
    </div>
  );
}

function ProjectFilesPanel({ projectId, evidence }: { projectId: string; evidence: EvidenceRecord[] }) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const reviewEvidence = useMutation({
    mutationFn: ({ record, action, comment }: { record: EvidenceRecord; action: EvidenceReviewAction; comment: string }) =>
      post<EvidenceRecord>(`/evidence/${record.id}/${action}`, { comment, reason: action === 'reject' ? comment : undefined }),
    onSuccess: (_record, variables) => {
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      toast.success(variables.action === 'accept' ? 'Đã accept evidence' : 'Đã reject evidence');
    },
    onError: (error) => toast.error('Không review được evidence', error),
  });

  return (
    <div className="space-y-4">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <FileText className="h-5 w-5 text-blue-700" />
          <h2 className="font-semibold text-ink">Hồ sơ</h2>
        </div>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{evidence.length}</span>
      </div>
      <EvidenceGallery
        evidence={evidence}
        emptyTitle="Chưa có evidence mới"
        showWorkItem
        onReview={(record, action, comment) => reviewEvidence.mutateAsync({ record, action, comment }).then(() => undefined)}
      />
    </div>
  );
}

function ProjectAuditPanel({ projectId, initialRows }: { projectId: string; initialRows: ProjectOperationsRecord[] }) {
  const path = `/projects/${projectId}/audit-logs`;
  const logs = useQuery({
    queryKey: ['audit', path],
    queryFn: () => get<ProjectOperationsRecord[]>(path),
    enabled: Boolean(projectId),
    retry: false,
  });
  const rows = logs.data ?? initialRows;

  return (
    <div className="space-y-4">
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex items-center gap-2">
          <ShieldAlert className="h-5 w-5 text-blue-700" />
          <h2 className="font-semibold text-ink">Audit Log</h2>
        </div>
        <Link to={`/projects/${projectId}/audit`} className="text-sm font-semibold text-blue-700 hover:underline">
          Mở trang Audit đầy đủ
        </Link>
      </div>

      {logs.isLoading ? <LoadingSpinner /> : null}
      {logs.error ? (
        <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          Không tải được audit đầy đủ. Đang hiển thị audit summary gần nhất nếu có.
        </div>
      ) : null}

      {!logs.isLoading && !rows.length ? <EmptyState title="Chưa có sự kiện audit" /> : null}
      {rows.length ? (
        <div className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
          <Timeline events={rows.slice(0, 20)} />
          <DataTable
            rows={rows}
            columns={[
              { key: 'created_at', header: 'Thời gian', render: (row) => compactDateTime(row.created_at) },
              { key: 'event_type', header: 'Event', render: (row) => <EnumChip group="event_type" value={row.event_type} /> },
              { key: 'actor_name', header: 'Actor' },
              { key: 'target_type', header: 'Target', render: (row) => <EnumChip group="target_type" value={row.target_type} /> },
              {
                key: 'description',
                header: 'Mô tả',
                render: (row) => <p className="max-w-xl text-sm leading-5 text-slate-700">{row.description ?? '-'}</p>,
              },
            ]}
          />
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
  allRows,
  selectedDepartment,
  onSelectDepartment,
  onOpenWork,
}: {
  surfaces: DepartmentSurface[];
  allRows: WorkItem[];
  selectedDepartment: string;
  onSelectDepartment: (department: string) => void;
  onOpenWork: (item: WorkItem) => void;
}) {
  const [page, setPage] = useState(1);
  const selectedSurface = selectedDepartment === ALL_WORK_SURFACE
    ? undefined
    : surfaces.find((surface) => (surface.department ?? 'unassigned') === selectedDepartment);
  const activeRows = useMemo(() => {
    if (selectedDepartment === ALL_WORK_SURFACE) return allRows;
    return orderWorkItemsByWorkflow(asArray<WorkItem>(selectedSurface?.work_items).filter(isOpenWorkItem));
  }, [allRows, selectedDepartment, selectedSurface?.work_items]);
  const totalPages = Math.max(1, Math.ceil(activeRows.length / WORK_PAGE_SIZE));
  const currentPage = Math.min(page, totalPages);
  const pageStartIndex = (currentPage - 1) * WORK_PAGE_SIZE;
  const pageRows = activeRows.slice(pageStartIndex, pageStartIndex + WORK_PAGE_SIZE);
  const rangeStart = activeRows.length ? pageStartIndex + 1 : 0;
  const rangeEnd = activeRows.length ? Math.min(pageStartIndex + pageRows.length, activeRows.length) : 0;
  const selectedLabel = selectedDepartment === ALL_WORK_SURFACE
    ? 'Tất cả'
    : departmentLabel(selectedSurface?.department ?? selectedDepartment);
  const selectedMetrics = selectedDepartment === ALL_WORK_SURFACE
    ? {
      total: allRows.length,
      active: allRows.length,
      blocked: allRows.filter((item) => item.status === 'blocked').length,
      waiting_approval: allRows.filter((item) => item.status === 'waiting_approval' || item.status === 'waiting_review').length,
    }
    : {
      total: numberValue(selectedSurface?.total),
      active: numberValue(selectedSurface?.active),
      blocked: numberValue(selectedSurface?.blocked),
      waiting_approval: numberValue(selectedSurface?.waiting_approval),
    };

  useEffect(() => {
    setPage(1);
  }, [selectedDepartment]);

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="font-semibold text-ink">Department Surfaces</h2>
          <p className="mt-1 text-sm text-slate-600">Các bề mặt hiển thị theo scope mà server trả về cho người dùng hiện tại.</p>
        </div>
        <div className="flex overflow-x-auto rounded-md border border-line bg-slate-50 p-1">
          <button
            className={clsx(
              'whitespace-nowrap rounded px-3 py-1.5 text-sm font-semibold',
              selectedDepartment === ALL_WORK_SURFACE ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600 hover:bg-white/70',
            )}
            onClick={() => onSelectDepartment(ALL_WORK_SURFACE)}
          >
            Tất cả
          </button>
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
          }) : null}
        </div>
      </div>

      {selectedDepartment === ALL_WORK_SURFACE || selectedSurface ? (
        <div className="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
          <div className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{selectedLabel}</p>
            <div className="mt-3 grid grid-cols-2 gap-2">
              <SurfaceMetric label="Total" value={selectedMetrics.total} />
              <SurfaceMetric label="Active" value={selectedMetrics.active} />
              <SurfaceMetric label="Blocked" value={selectedMetrics.blocked} />
              <SurfaceMetric label="Approval" value={selectedMetrics.waiting_approval} />
            </div>
          </div>
          <div>
            <div className="space-y-3">
              {activeRows.length ? (
                pageRows.map((item) => (
                  <WorkItemCard key={item.id} item={item} compact showWorkflowPrefix onOpen={() => onOpenWork(item)} />
                ))
              ) : (
                <EmptyState title="Không có work item đang mở trong scope hiện tại" />
              )}
            </div>

            {activeRows.length > WORK_PAGE_SIZE ? (
              <div className="mt-4 flex flex-col gap-3 border-t border-line pt-3 sm:flex-row sm:items-center sm:justify-between">
                <span className="text-sm font-semibold text-slate-600">{rangeStart}-{rangeEnd} / {activeRows.length}</span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    className="rounded-md border border-line px-3 py-1.5 text-sm font-semibold text-slate-700 hover:border-blue-300 hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={currentPage <= 1}
                    onClick={() => setPage((value) => Math.max(1, value - 1))}
                  >
                    Trước
                  </button>
                  <span className="rounded-md bg-slate-100 px-3 py-1.5 text-sm font-semibold text-slate-600">
                    Trang {currentPage} / {totalPages}
                  </span>
                  <button
                    type="button"
                    className="rounded-md border border-line px-3 py-1.5 text-sm font-semibold text-slate-700 hover:border-blue-300 hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={currentPage >= totalPages}
                    onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
                  >
                    Sau
                  </button>
                </div>
              </div>
            ) : null}
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

function ProjectMembers({ rows }: { rows: Record<string, any>[] }) {
  return (
    <section>
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
