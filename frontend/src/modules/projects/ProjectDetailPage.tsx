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
import { WorkItemCard } from '../work-items/WorkItemCard';
import { WorkItemDetailDialog } from '../work-items/WorkItemDetailDialog';
import {
  asArray,
  compactDateTime,
  departmentLabel,
  evidenceLabel,
  numberValue,
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

function isOpenWorkItem(item: WorkItem) {
  return item.status !== 'completed' && item.status !== 'cancelled';
}

export function ProjectDetailPage() {
  const { id = '' } = useParams();
  const [selectedWork, setSelectedWork] = useState<WorkItem | null>(null);
  const [selectedDepartment, setSelectedDepartment] = useState('');
  const [selectedTimelineNodeId, setSelectedTimelineNodeId] = useState('');
  const [activeTab, setActiveTab] = useState<ProjectDetailTab>('overview');
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
  const activeWork = useMemo(() => prioritizeWorkItems(allWorkItems.filter(isOpenWorkItem)), [allWorkItems]);
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
      setSelectedDepartment('');
      return;
    }
    const exists = departmentSurfaces.some((surface) => (surface.department ?? 'unassigned') === selectedDepartment);
    if (!exists) setSelectedDepartment(departmentSurfaces[0].department ?? 'unassigned');
  }, [departmentSurfaces, selectedDepartment]);

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
  const activeDepartment = departmentSurfaces.find((surface) => (surface.department ?? 'unassigned') === selectedDepartment)
    ?? departmentSurfaces[0];
  const selectedTimelineNode = timeline.find((node) => node.id === selectedTimelineNodeId) ?? currentNode ?? timeline[0];
  const nodeActiveWork = selectedTimelineNode
    ? prioritizeWorkItems(allWorkItems.filter((item) => item.run_node_id === selectedTimelineNode.id && isOpenWorkItem(item)))
    : activeWork;
  const tabs: Array<{ id: ProjectDetailTab; label: string; count?: number }> = [
    { id: 'overview', label: 'Tổng quan', count: numberValue(data.health_counts?.active) },
    { id: 'workflow', label: 'Workflow', count: timeline.length },
    { id: 'work', label: 'Công việc', count: activeWork.length },
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

      <ProjectDetailTabs tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />

      <div className="mt-5">
        {activeTab === 'overview' ? (
          <div className="space-y-4">
            <ProjectSummaryBand project={project} currentNode={currentNode} visibility={data.visibility} />
            <HealthStrip data={data} />
            <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
              <NextWorkPanel rows={activeWork} onOpenWork={openWorkItem} />
              <CommandCenter data={data} blockers={blockers} onOpenWork={openWorkItem} />
            </div>
            <ModuleShortcuts projectId={id} />
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
            activeSurface={activeDepartment}
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

        {activeTab === 'files' ? <ProjectFilesPanel evidence={latestEvidence} /> : null}

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
    <div className="mt-5 overflow-x-auto rounded-lg border border-line bg-white p-1 shadow-panel">
      <div className="flex min-w-max gap-1">
        {tabs.map((tab) => {
          const active = tab.id === activeTab;
          return (
            <button
              key={tab.id}
              type="button"
              className={clsx(
                'inline-flex items-center gap-2 whitespace-nowrap rounded-md px-3 py-2 text-sm font-semibold transition focus-ring',
                active ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:bg-slate-100 hover:text-ink',
              )}
              aria-pressed={active}
              onClick={() => onChange(tab.id)}
            >
              {tab.label}
              {typeof tab.count === 'number' ? (
                <span className={clsx(
                  'rounded px-1.5 py-0.5 text-xs',
                  active ? 'bg-white/20 text-white' : 'bg-slate-100 text-slate-500',
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

function NextWorkPanel({ rows, onOpenWork }: { rows: WorkItem[]; onOpenWork: (item: WorkItem) => void }) {
  const previewRows = rows.slice(0, 6);
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <TimerReset className="h-5 w-5 shrink-0 text-blue-700" />
          <h2 className="truncate font-semibold text-ink">Next Work</h2>
        </div>
        <span className="shrink-0 rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{rows.length} open</span>
      </div>
      {previewRows.length ? (
        <div className="space-y-3">
          {previewRows.map((item) => <WorkItemCard key={item.id} item={item} compact onOpen={() => onOpenWork(item)} />)}
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
            Active Work{selectedNode?.name ? ` · ${selectedNode.name}` : ''}
          </h2>
        </div>
        <span className="shrink-0 rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{rows.length} open</span>
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

function ProjectFilesPanel({ evidence }: { evidence: EvidenceRecord[] }) {
  return (
    <div className="space-y-4">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <FileText className="h-5 w-5 text-blue-700" />
          <h2 className="font-semibold text-ink">Hồ sơ</h2>
        </div>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{evidence.length}</span>
      </div>
      <DataTable
        rows={evidence}
        empty="Chưa có evidence mới"
        columns={[
          {
            key: 'name',
            header: 'Hồ sơ',
            render: (row) => (
              <div className="min-w-0">
                <p className="truncate font-semibold text-ink">{row.name ?? row.title ?? row.document_name ?? '-'}</p>
                <p className="mt-1 text-xs text-slate-500">{row.document_name ?? row.file_url ?? row.id}</p>
              </div>
            ),
          },
          { key: 'evidence_type', header: 'Loại', render: (row) => evidenceLabel(row.evidence_type) },
          { key: 'work_item_title', header: 'Work item', render: (row) => row.work_item_title ?? '-' },
          { key: 'visibility', header: 'Visibility' },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
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
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
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
    <section id="project-modules" className="rounded-lg border border-line bg-white p-4 shadow-panel">
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
