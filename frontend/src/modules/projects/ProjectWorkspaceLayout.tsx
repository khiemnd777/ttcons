import { useEffect, useMemo, useRef, useState } from 'react';
import type { RefObject } from 'react';
import { Link, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Circle,
  Clock3,
  Folder,
  FolderOpen,
  GitBranch,
  Loader2,
  LogOut,
  PanelLeftClose,
} from 'lucide-react';
import clsx from 'clsx';
import { BrandHomeLink } from '../../components/BrandLogo';
import { PoweredBySignature } from '../../components/PoweredBySignature';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  ProjectOperationsResponse,
  ProjectOperationsSidebarResponse,
  ProjectOperationsTimelineNode,
  WorkItem,
} from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';
import { workItemDetailPath } from '../work-items/workItemNavigation';
import { workflowFormStatusLabel } from '../work-items/workItemPresentation';
import {
  projectWorkspaceGroupPath,
  projectWorkspacePath,
  visibleProjectWorkspaceGroups,
  type ProjectWorkspaceGroup,
} from './projectWorkspaceNavigation';

type ProjectWorkspaceOperations = ProjectOperationsResponse | ProjectOperationsSidebarResponse;

export function ProjectWorkspaceLayout() {
  const { id = '' } = useParams();
  const { user, logout, hasAnyPermission } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const showWorkflowSidebar = isProjectWorkItemDetailPath(location.pathname);
  const operationsView = showWorkflowSidebar ? 'workflow-sidebar' : 'full';
  const operations = useQuery({
    queryKey: ['project-operations', id, operationsView],
    queryFn: () => get<ProjectOperationsResponse | ProjectOperationsSidebarResponse>(
      `/projects/${id}/operations${showWorkflowSidebar ? '?view=workflow-sidebar' : ''}`,
    ),
    enabled: Boolean(id),
    retry: false,
  });

  const project = operations.data?.project_summary ?? operations.data?.project;
  const visibleGroups = visibleProjectWorkspaceGroups(hasAnyPermission);
  const activeWorkItemId = showWorkflowSidebar ? activeWorkItemIdFromPath(location.pathname) : '';
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);

  useEffect(() => {
    setMobileSidebarOpen(false);
  }, [location.pathname]);

  return (
    <div className="min-h-screen bg-surface">
      <header className="sticky top-0 z-30 border-b border-line bg-white">
        <div className="lg:grid lg:grid-cols-[320px_minmax(0,1fr)]">
          <div className="hidden h-16 border-r border-line lg:block">
            <BrandHomeLink />
          </div>
          <div className="flex min-h-16 items-center justify-between gap-3 px-4 py-3 lg:py-0">
            <div className="flex min-w-0 items-center gap-3">
              <BrandHomeLink compact className="lg:hidden" />
              <button
                type="button"
                className="hidden items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 lg:inline-flex"
                onClick={() => navigate('/projects')}
              >
                <ArrowLeft className="h-4 w-4" />
                Công trình
              </button>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-ink">
                  {project ? `${project.code} · ${project.name}` : operations.isLoading ? 'Đang tải công trình...' : 'Không gian công trình'}
                </p>
                <p className="truncate text-xs text-slate-500">
                  {[
                    project?.client_name ? `Chủ đầu tư: ${project.client_name}` : null,
                    project?.pm_name ? `Phụ trách: ${project.pm_name}` : null,
                    project?.status ? `Trạng thái: ${statusLabel(project.status)}` : null,
                  ].filter(Boolean).join(' · ') || (showWorkflowSidebar ? 'Quy trình là dòng thời gian xử lý công việc' : 'Cây nghiệp vụ là menu chính trong phạm vi công trình')}
                </p>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-3">
              <div className="hidden min-w-0 text-right sm:block">
                <p className="truncate text-sm font-semibold text-ink">{user?.full_name}</p>
                <p className="truncate text-xs text-slate-500">{user?.roles?.join(', ')}</p>
              </div>
              <button
                aria-label="Đăng xuất"
                className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={() => {
                  void logout().finally(() => navigate('/login'));
                }}
              >
                <LogOut className="h-4 w-4" />
                <span className="hidden sm:inline">Đăng xuất</span>
              </button>
            </div>
          </div>
        </div>
      </header>

      <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-2 border-b border-line bg-white px-4 py-2 lg:hidden">
        <button
          type="button"
          className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          onClick={() => navigate('/projects')}
        >
          <ArrowLeft className="h-4 w-4" />
          Công trình
        </button>
        <button type="button" aria-expanded={mobileSidebarOpen} className="inline-flex min-w-0 items-center justify-between rounded-md border border-line bg-slate-50 px-3 py-2 text-sm font-semibold text-ink" onClick={() => setMobileSidebarOpen((current) => !current)}><span className="inline-flex min-w-0 items-center gap-2"><PanelLeftClose className="h-4 w-4 shrink-0 text-blue-700" /><span className="truncate">{showWorkflowSidebar ? 'Mở cây quy trình' : 'Mở cây nghiệp vụ'}</span></span>{mobileSidebarOpen ? <ChevronDown className="h-4 w-4 shrink-0" /> : <ChevronRight className="h-4 w-4 shrink-0" />}</button>
      </div>
      <div className="grid min-h-[calc(100vh-4rem)] lg:grid-cols-[320px_minmax(0,1fr)]">
        <aside className={`${mobileSidebarOpen ? 'flex' : 'hidden'} flex-col border-b border-line bg-white lg:sticky lg:top-16 lg:flex lg:h-[calc(100vh-4rem)] lg:overflow-hidden lg:border-b-0 lg:border-r`}>
          <div className="min-h-0 flex-1 overflow-auto">
            {showWorkflowSidebar ? (
              <WorkflowSidebar
                projectId={id}
                projectName={project?.name ?? 'Công trình'}
                projectCode={project?.code ?? id}
                operations={operations.data}
                loading={operations.isLoading}
                error={operations.error}
                activeWorkItemId={activeWorkItemId}
              />
            ) : (
              <ProjectNavigationSidebar
                projectId={id}
                projectName={project?.name ?? 'Công trình'}
                projectCode={project?.code ?? id}
                visibleGroups={visibleGroups}
              />
            )}
          </div>
          <div className="hidden border-t border-line px-4 py-3 lg:block">
            <PoweredBySignature />
          </div>
        </aside>

        <div className="flex min-w-0 flex-col">
          <main className="flex-1 px-4 py-5 md:px-6">
            <Outlet />
          </main>
          <footer className="border-t border-line bg-white px-4 py-3 lg:hidden">
            <PoweredBySignature />
          </footer>
        </div>
      </div>
    </div>
  );
}

function ProjectNavigationSidebar({
  projectId,
  projectName,
  projectCode,
  visibleGroups,
}: {
  projectId: string;
  projectName: string;
  projectCode: string;
  visibleGroups: ProjectWorkspaceGroup[];
}) {
  return (
    <>
      <div className="border-b border-line px-4 py-3">
        <div className="flex items-center gap-2">
          <PanelLeftClose className="h-4 w-4 text-blue-700" />
          <p className="text-sm font-semibold text-ink">Cây nghiệp vụ</p>
        </div>
      </div>
      <nav className="px-3 py-3">
        <div className="mb-2 rounded-md border border-line bg-slate-50 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <ChevronDown className="h-4 w-4 shrink-0 text-slate-500" />
            <FolderOpen className="h-4 w-4 shrink-0 text-blue-700" />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-ink">{projectName}</p>
              <p className="truncate text-xs text-slate-500">{projectCode}</p>
            </div>
          </div>
        </div>

        <div className="space-y-1">
          {visibleGroups.map((group) => (
            <WorkspaceGroupNode key={group.key} group={group} projectId={projectId} />
          ))}
        </div>
      </nav>
    </>
  );
}

function WorkflowSidebar({
  projectId,
  projectName,
  projectCode,
  operations,
  loading,
  error,
  activeWorkItemId,
}: {
  projectId: string;
  projectName: string;
  projectCode: string;
  operations?: ProjectWorkspaceOperations;
  loading: boolean;
  error: unknown;
  activeWorkItemId: string;
}) {
  const run = operations?.pipeline_run as Record<string, any> | undefined;
  const timelineNodes = asRecords<ProjectOperationsTimelineNode>(operations?.workflow_timeline);
  const workItems = asRecords<WorkItem>(operations?.work_items);
  const nodes = useMemo(
    () => buildWorkflowSidebarNodes(asRecords(run?.nodes), timelineNodes, workItems),
    [run?.nodes, timelineNodes, workItems],
  );
  const timelineByNodeId = useMemo(
    () => new Map(timelineNodes.map((node) => [String(node.id), node])),
    [timelineNodes],
  );
  const allSteps = nodes.flatMap((node) => asRecords(node.steps));
  const skippedSteps = allSteps.filter((step) => isSkippedStatus(step.status)).length;
  const executableSteps = allSteps.filter((step) => !isSkippedStatus(step.status));
  const completedSteps = executableSteps.filter((step) => isCompletedStatus(step.status)).length;
  const overallProgress = progressPercent(completedSteps, executableSteps.length);
  const hasData = nodes.length > 0;
  const activeNodeId = activeNodeIdForWorkItem(nodes, activeWorkItemId);
  const [expandedNodeIds, setExpandedNodeIds] = useState<string[]>([]);
  const activeStepRef = useRef<HTMLAnchorElement>(null);

  useEffect(() => {
    setExpandedNodeIds(activeNodeId ? [activeNodeId] : []);
  }, [activeNodeId, activeWorkItemId]);

  useEffect(() => {
    if (!activeNodeId || !activeWorkItemId) return;
    const scrollTimer = window.setTimeout(() => {
      activeStepRef.current?.scrollIntoView({ block: 'center' });
    }, 80);
    return () => window.clearTimeout(scrollTimer);
  }, [activeNodeId, activeWorkItemId, expandedNodeIds]);

  function toggleNode(nodeId: string) {
    setExpandedNodeIds((current) => (
      current.includes(nodeId)
        ? current.filter((id) => id !== nodeId)
        : [...current, nodeId]
    ));
  }

  return (
    <>
      <div className="border-b border-line px-4 py-3">
        <div className="flex items-start gap-2">
          <GitBranch className="mt-0.5 h-4 w-4 shrink-0 text-blue-700" />
          <div className="min-w-0">
            <p className="text-sm font-semibold text-ink">Quy trình</p>
            <p className="mt-0.5 text-xs font-medium text-slate-500">Danh sách kiểm tra / Bước</p>
          </div>
        </div>
      </div>

      <nav className="px-3 py-3" aria-label="Dòng thời gian quy trình vận hành">
        <div className="mb-3 rounded-md border border-line bg-slate-50 px-3 py-3">
          <div className="flex min-w-0 items-center gap-2">
            <ChevronDown className="h-4 w-4 shrink-0 text-slate-500" />
            <FolderOpen className="h-4 w-4 shrink-0 text-blue-700" />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-ink">{projectName}</p>
              <p className="truncate text-xs text-slate-500">{projectCode}</p>
            </div>
          </div>
          <div className="mt-3">
            <div className="mb-1.5 flex items-center justify-between gap-2 text-xs font-semibold">
              <span className="text-slate-600">Tiến độ</span>
              <span className="text-blue-700">{overallProgress}% · {completedSteps}/{executableSteps.length} bước thực hiện</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-white ring-1 ring-inset ring-line">
              <div className="h-full rounded-full bg-blue-600" style={{ width: `${overallProgress}%` }} />
            </div>
            {skippedSteps ? <p className="mt-1.5 text-right text-[11px] font-medium text-slate-500">{skippedSteps} bước đã bỏ qua · {allSteps.length} bước toàn quy trình</p> : null}
          </div>
        </div>

        {loading && !operations ? <WorkflowSidebarLoading /> : null}

        {error && !operations ? (
          <div className="rounded-md border border-rose-100 bg-rose-50 px-3 py-3 text-sm text-rose-700">
            Không tải được quy trình.
          </div>
        ) : null}

        {!loading && !error && !hasData ? (
          <div className="rounded-md border border-line bg-white px-3 py-4 text-sm">
            <p className="font-semibold text-ink">Chưa có quy trình đang chạy</p>
            <p className="mt-1 text-xs leading-5 text-slate-500">Mở Kế hoạch để khởi động hoặc kiểm tra quy trình vận hành.</p>
            <Link
              to={`/projects/${projectId}/planning`}
              className="mt-3 inline-flex w-full items-center justify-center rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
            >
              Mở kế hoạch
            </Link>
          </div>
        ) : null}

        {hasData ? (
          <ol className="space-y-3">
            {nodes.map((node, index) => (
              <WorkflowNodeItem
                key={String(node.id ?? index)}
                node={node}
                fallbackIndex={index}
                timelineNode={timelineByNodeId.get(String(node.id))}
                projectId={projectId}
                activeWorkItemId={activeWorkItemId}
                expanded={expandedNodeIds.includes(cleanText(node.id))}
                activeStepRef={activeStepRef}
                onToggle={toggleNode}
              />
            ))}
          </ol>
        ) : null}
      </nav>
    </>
  );
}

function WorkflowSidebarLoading() {
  return (
    <div className="space-y-2 rounded-md border border-line bg-white px-3 py-3 text-sm text-slate-600">
      <div className="flex items-center gap-2 font-semibold">
        <Loader2 className="h-4 w-4 animate-spin text-blue-700" />
        Đang tải quy trình
      </div>
      <div className="h-2 rounded-full bg-slate-100" />
      <div className="h-2 w-4/5 rounded-full bg-slate-100" />
      <div className="h-2 w-2/3 rounded-full bg-slate-100" />
    </div>
  );
}

function WorkflowNodeItem({
  node,
  fallbackIndex,
  timelineNode,
  projectId,
  activeWorkItemId,
  expanded,
  activeStepRef,
  onToggle,
}: {
  node: Record<string, any>;
  fallbackIndex: number;
  timelineNode?: ProjectOperationsTimelineNode;
  projectId: string;
  activeWorkItemId: string;
  expanded: boolean;
  activeStepRef: RefObject<HTMLAnchorElement>;
  onToggle: (nodeId: string) => void;
}) {
  const nodeId = cleanText(node.id);
  const steps = asRecords(node.steps);
  const derivedCompletedSteps = steps.filter((step) => isCompletedStatus(step.status)).length;
  const completedSteps = clampStepCount(Number(timelineNode?.completed_steps ?? derivedCompletedSteps), steps.length);
  const nodeProgress = clampPercent(Number(timelineNode?.progress_percent ?? progressPercent(completedSteps, steps.length)));
  const orderIndex = cleanText(node.order_index) || cleanText(timelineNode?.order_index) || String(fallbackIndex + 1);
  const isActiveNode = steps.some((step) => cleanText(step.work_item_id) === activeWorkItemId);

  return (
    <li
      className={clsx(
        'rounded-md border transition-colors',
        isActiveNode ? 'border-blue-300 bg-blue-50/30 ring-1 ring-blue-200' : 'border-line bg-white',
      )}
    >
      <button
        type="button"
        className={clsx(
          'block w-full border-b px-3 py-3 text-left transition focus-ring',
          isActiveNode ? 'border-blue-100 bg-blue-50/40 hover:bg-blue-50' : 'border-line hover:bg-slate-50',
        )}
        aria-expanded={expanded}
        onClick={() => nodeId && onToggle(nodeId)}
        disabled={!nodeId}
      >
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-start gap-2">
            <span className={clsx('mt-0.5 shrink-0', isActiveNode ? 'text-blue-700' : 'text-slate-500')}>
              {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </span>
            <div className="min-w-0">
              <p className={clsx('truncate text-sm font-semibold', isActiveNode ? 'text-blue-800' : 'text-ink')}>
                {orderIndex}. {cleanText(node.name) || 'Giai đoạn quy trình'}
              </p>
              <p className="mt-1 text-xs text-slate-500">{completedSteps}/{steps.length} bước</p>
            </div>
          </div>
          <StatusBadge status={cleanText(node.status) || cleanText(timelineNode?.status) || 'pending'} />
        </div>
        <div className={clsx('mt-3 h-1.5 overflow-hidden rounded-full', isActiveNode ? 'bg-blue-100' : 'bg-slate-100')}>
          <div className="h-full rounded-full bg-blue-600" style={{ width: `${nodeProgress}%` }} />
        </div>
      </button>

      {expanded ? (
        <ol className={clsx('divide-y', isActiveNode ? 'divide-blue-100' : 'divide-line')}>
          {steps.length ? steps.map((step, index) => (
            <WorkflowStepItem
              key={String(step.id ?? `${node.id}-${index}`)}
              step={step}
              projectId={projectId}
              active={cleanText(step.work_item_id) === activeWorkItemId}
              activeStepRef={activeStepRef}
            />
          )) : (
            <li className="px-3 py-3 text-xs text-slate-500">Giai đoạn chưa có danh sách kiểm tra hoặc bước.</li>
          )}
        </ol>
      ) : null}
    </li>
  );
}

function WorkflowStepItem({
  step,
  projectId,
  active,
  activeStepRef,
}: {
  step: Record<string, any>;
  projectId: string;
  active: boolean;
  activeStepRef: RefObject<HTMLAnchorElement>;
}) {
  const workItemId = cleanText(step.work_item_id);
  const status = cleanText(step.work_item_status) || cleanText(step.status) || 'pending';
  const disabled = !workItemId;
  const content = (
    <>
      <span className="mt-0.5 shrink-0">{stepStatusIcon(status, active)}</span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-semibold">{cleanText(step.name) || 'Danh sách kiểm tra / Bước'}</span>
        <span className="mt-1 flex flex-wrap gap-1.5 text-[11px] font-semibold">
          <span className="rounded-md bg-slate-100 px-1.5 py-0.5 text-slate-600">{statusLabel(status)}</span>
          {step.workflow_form_instance_id ? (
            <span className="rounded-md bg-emerald-50 px-1.5 py-0.5 text-emerald-700">
              Hồ sơ {workflowFormStatusLabel(step.workflow_form_status)}
            </span>
          ) : null}
          {Number(step.evidence_count ?? 0) > 0 ? (
            <span className="rounded-md bg-blue-50 px-1.5 py-0.5 text-blue-700">Minh chứng {Number(step.evidence_count)}</span>
          ) : null}
          {disabled ? <span className="rounded-md bg-amber-50 px-1.5 py-0.5 text-amber-700">Chưa có đầu việc</span> : null}
        </span>
      </span>
    </>
  );

  if (disabled) {
    return (
      <li className="flex cursor-not-allowed items-start gap-2 px-3 py-2.5 text-slate-400" aria-disabled="true">
        {content}
      </li>
    );
  }

  return (
    <li>
      <Link
        ref={active ? activeStepRef : undefined}
        to={workItemDetailPath(workItemId, `/projects/${projectId}`, projectId)}
        aria-current={active ? 'page' : undefined}
        className={clsx(
          'flex items-start gap-2 px-3 py-2.5 focus-ring',
          active ? 'bg-blue-50 text-blue-800' : 'text-slate-700 hover:bg-slate-50 hover:text-blue-700',
        )}
      >
        {content}
      </Link>
    </li>
  );
}

function buildWorkflowSidebarNodes(
  runNodes: Record<string, any>[],
  timelineNodes: ProjectOperationsTimelineNode[],
  workItems: WorkItem[],
): Record<string, any>[] {
  const workItemsByNodeId = new Map<string, WorkItem[]>();
  workItems.forEach((item) => {
    const nodeId = cleanText(item.run_node_id);
    const stepId = cleanText(item.run_step_id);
    if (!nodeId || !stepId) return;
    const rows = workItemsByNodeId.get(nodeId) ?? [];
    rows.push(item);
    workItemsByNodeId.set(nodeId, rows);
  });

  const timelineByNodeId = new Map(timelineNodes.map((node) => [cleanText(node.id), node]));
  const mergedNodes: Record<string, any>[] = runNodes.length ? runNodes : timelineNodes.map((node) => ({
    id: node.id,
    name: node.name,
    type: node.type,
    status: node.status,
    order_index: node.order_index,
    started_at: node.started_at,
    completed_at: node.completed_at,
    steps: [],
  }));
  const seenNodeIds = new Set(mergedNodes.map((node) => cleanText(node.id)).filter(Boolean));

  workItemsByNodeId.forEach((nodeWorkItems, nodeId) => {
    if (seenNodeIds.has(nodeId)) return;
    const firstItem = nodeWorkItems[0];
    mergedNodes.push({
      id: nodeId,
      name: firstItem?.run_node_name ?? 'Giai đoạn quy trình',
      status: firstItem?.status ?? 'pending',
      order_index: firstItem?.run_node_order_index,
      steps: [],
    });
    seenNodeIds.add(nodeId);
  });

  return mergedNodes
    .map((node) => {
      const nodeId = cleanText(node.id);
      const timelineNode = timelineByNodeId.get(nodeId);
      const fallbackSteps = (workItemsByNodeId.get(nodeId) ?? []).map(stepFromWorkItem);
      const steps = mergeStepRecords(asRecords(node.steps), fallbackSteps);
      return {
        ...node,
        name: cleanText(node.name) || timelineNode?.name,
        status: cleanText(node.status) || timelineNode?.status,
        order_index: node.order_index ?? timelineNode?.order_index,
        steps,
      };
    })
    .sort((a, b) => sortIndex(a.order_index) - sortIndex(b.order_index));
}

function stepFromWorkItem(item: WorkItem): Record<string, any> {
  return {
    id: item.run_step_id,
    run_step_id: item.run_step_id,
    name: item.run_step_name ?? item.title,
    description: item.description,
    step_type: item.work_type,
    status: item.status,
    work_item_id: item.id,
    work_item_status: item.status,
    work_item_title: item.title,
    work_item_role_ids: item.role_ids,
    order_index: item.run_step_order_index,
    evidence_count: item.evidence_count,
    evidence_requirement_json: item.evidence_requirement_json,
    workflow_form_instance_id: item.workflow_form_instance_id,
    workflow_form_status: item.workflow_form_status,
    workflow_form_type: item.workflow_form_type,
    workflow_form_revision: item.workflow_form_revision,
    workflow_form_submitted_at: item.workflow_form_submitted_at,
    workflow_form_template_id: item.workflow_form_template_id,
    workflow_form_version_id: item.workflow_form_version_id,
    workflow_form_version_no: item.workflow_form_version_no,
    workflow_form_version_status: item.workflow_form_version_status,
    workflow_form_template_code: item.workflow_form_template_code,
    workflow_form_template_name: item.workflow_form_template_name,
    workflow_form_required: item.workflow_form_required,
  };
}

function mergeStepRecords(runSteps: Record<string, any>[], fallbackSteps: Record<string, any>[]) {
  const merged = new Map<string, Record<string, any>>();
  runSteps.forEach((step, index) => {
    const key = stepKey(step) || `run-step-${index}`;
    merged.set(key, step);
  });
  fallbackSteps.forEach((step, index) => {
    const key = stepKey(step) || `fallback-step-${index}`;
    const existing = merged.get(key);
    merged.set(key, existing ? { ...existing, ...step, status: existing.status ?? step.status } : step);
  });
  return Array.from(merged.values()).sort((a, b) => {
    const orderDiff = sortIndex(a.order_index ?? a.run_step_order_index) - sortIndex(b.order_index ?? b.run_step_order_index);
    if (orderDiff !== 0) return orderDiff;
    return cleanText(a.name).localeCompare(cleanText(b.name));
  });
}

function stepKey(step: Record<string, any>) {
  return cleanText(step.id) || cleanText(step.run_step_id) || cleanText(step.source_id);
}

function activeNodeIdForWorkItem(nodes: Record<string, any>[], activeWorkItemId: string) {
  if (!activeWorkItemId) return '';
  for (const node of nodes) {
    const activeStep = asRecords(node.steps).some((step) => cleanText(step.work_item_id) === activeWorkItemId);
    if (activeStep) return cleanText(node.id);
  }
  return '';
}

function WorkspaceGroupNode({ group, projectId }: { group: ProjectWorkspaceGroup; projectId: string }) {
  const location = useLocation();
  const groupTarget = projectWorkspaceGroupPath(projectId, group);
  const currentTarget = `${location.pathname}${location.search}`;
  const groupActive = group.children.length
    ? group.children.some((child) => {
      const childTarget = projectWorkspacePath(projectId, child.to);
      return childTarget.includes('?') ? currentTarget === childTarget : location.pathname === childTarget;
    })
    : currentTarget === groupTarget;
  return (
    <div>
      <Link
        to={groupTarget}
        className={clsx(
          'flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold focus-ring',
          groupActive ? 'bg-blue-50 text-blue-700' : 'text-slate-700 hover:bg-slate-50 hover:text-ink',
        )}
      >
        <ChevronDown className="h-4 w-4 shrink-0" />
        <Folder className="h-4 w-4 shrink-0" />
        <span className="min-w-0 truncate">{group.label}</span>
      </Link>
      {group.children.length ? (
        <div className="mb-1">
          {group.children.map((child) => {
            const childTarget = projectWorkspacePath(projectId, child.to);
            const active = childTarget.includes('?') ? currentTarget === childTarget : location.pathname === childTarget;
            return (
              <Link
                key={`${group.key}-${child.label}`}
                to={childTarget}
                className={clsx(
                  'ml-8 flex w-[calc(100%-2rem)] items-center gap-2 border-l border-slate-200 px-4 py-1.5 text-sm focus-ring',
                  active ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-50 hover:text-blue-700',
                )}
              >
                <Folder className="h-3.5 w-3.5 shrink-0" />
                <span className="min-w-0 truncate">{child.label}</span>
              </Link>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

function isProjectWorkItemDetailPath(pathname: string) {
  return /^\/projects\/[^/]+\/work-items\/[^/]+/.test(pathname);
}

function activeWorkItemIdFromPath(pathname: string) {
  const match = pathname.match(/^\/projects\/[^/]+\/work-items\/([^/]+)/);
  return match ? decodeURIComponent(match[1]) : '';
}

function asRecords<T extends Record<string, any> = Record<string, any>>(value: unknown): T[] {
  return Array.isArray(value) ? value as T[] : [];
}

function cleanText(value: unknown) {
  const text = String(value ?? '').trim();
  return text === '<nil>' ? '' : text;
}

function isCompletedStatus(status: unknown) {
  return ['submitted', 'completed'].includes(cleanText(status));
}

function isSkippedStatus(status: unknown) {
  return ['skipped', 'cancelled'].includes(cleanText(status));
}

function clampStepCount(value: number, total: number) {
  if (!Number.isFinite(value)) return 0;
  return Math.min(total, Math.max(0, Math.round(value)));
}

function progressPercent(completed: number, total: number) {
  if (!total) return 0;
  return clampPercent(Math.round((completed / total) * 100));
}

function clampPercent(value: number) {
  if (!Number.isFinite(value)) return 0;
  return Math.min(100, Math.max(0, value));
}

function sortIndex(value: unknown) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : Number.MAX_SAFE_INTEGER;
}

function stepStatusIcon(status: string, active: boolean) {
  if (isCompletedStatus(status)) return <CheckCircle2 className="h-4 w-4 text-emerald-600" />;
  if (status === 'blocked') return <AlertTriangle className="h-4 w-4 text-rose-600" />;
  if (status === 'waiting_approval' || status === 'waiting_review') return <Clock3 className="h-4 w-4 text-amber-600" />;
  return <Circle className={clsx('h-4 w-4', active ? 'text-blue-700' : 'text-slate-400')} />;
}
