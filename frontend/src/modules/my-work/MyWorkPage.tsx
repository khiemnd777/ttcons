import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertCircle,
  AtSign,
  Bell,
  BriefcaseBusiness,
  CheckCircle2,
  ChevronRight,
  Clock3,
  MessageSquare,
  PlayCircle,
  RefreshCw,
  Search,
  SlidersHorizontal,
  UserCog,
  X,
} from 'lucide-react';
import clsx from 'clsx';
import './myWorkFilters.css';
import { get, post, type StreamStatus } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import type { AccountingWorkPage, AccountingWorkStatus, ActivityAttentionItem, ActivityAttentionSummary, ApprovalRequest, MyWorkflowAction, ProjectStartupAction, ProjectStartupActionsResponse, WorkItem, WorkReportListItem, WorkReportSummary } from '../../lib/types/domain';
import { ApprovalPanel } from '../../components/ApprovalPanel';
import { statusLabel } from '../../lib/utils/status';
import { roleSummary as summarizeRoles } from '../../lib/utils/roles';
import { ActivityAttachments } from '../work-items/ActivityAttachments';
import { ActivityReactionBar, ActivityReplyQuote } from '../work-items/ActivityConversationMeta';
import { WorkItemCard } from '../work-items/WorkItemCard';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import { executionTargetActionLabel, executionTargetPath, executionTargetWorkspaceLabel } from '../work-items/workItemExecutionTarget';
import { useActorRealtimeStatus } from '../notifications/ActorRealtimeProvider';
import {
  asArray,
  activityLabel,
  compactDateTime,
  isWorkOverdue,
  priorityLabel,
  scopeTypeLabel,
  workItemIsUpcoming,
  workItemWorkflowContext,
  workTypeLabel,
} from '../work-items/workItemPresentation';
import { ProjectStartupWizard } from '../projects/ProjectStartupWizard';
import { AccountingWorkLane } from './AccountingWorkLane';
import { AccountingRequestQueue } from '../project-finance/AccountingRequestQueue';
import { Dialog } from '../../components/Dialog';
import { AccountingWorkCard, AccountingWorkDetails } from './AccountingWorkCard';
import { AccountingHandoffActionDialog } from '../project-finance/AccountingHandoffActionDialog';
import { useAccountingWorkActions } from '../project-finance/accountingWorkActions';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import type { AccountingWorkItem } from '../../lib/types/domain';
import {
  buildAccountingWorkPath,
  buildProjectStartupActionsPath,
  buildWorkPath,
  buildWorkQueuePath,
  dedupeWorkQueue,
  workQueueEntryKey,
  orderedWorkListEntries,
  projectStartupVisibility,
  type WorkQueueEntry,
  type WorkQueueKind,
  buildWorkScopeSummaryPath,
  buildWorkflowActionsPath,
  dedupeWorkItems,
  firstPopulatedApprovalLane,
  nextActionCopy,
  validValue,
  workflowActionSuccessMessage,
  workReportDateText,
  type ApprovalLaneKey,
  type SummaryBucket,
  type WorkScopeKey,
} from './myWorkDomain';

type ModeKey = 'work' | 'approval' | 'accounting' | 'attention';
type LegacyQueueKey = WorkScopeKey | 'pending_approval';
type AttentionQueueKey = 'all' | 'mentions' | 'questions' | 'blockers' | 'reminders' | 'unread';
type WorkQuickBucket = 'all' | 'need_action' | 'in_progress' | 'blocked' | 'upcoming' | 'completed';

type FacetOption = string | {
  id?: string;
  value?: string;
  label?: string;
  code?: string;
  name?: string;
  project_id?: string;
  project_code?: string;
  project_name?: string;
};
type MyWorkFacets = {
  projects?: FacetOption[];
  statuses?: FacetOption[];
  work_types?: FacetOption[];
  priorities?: FacetOption[];
};
type MyWorkSummary = {
  total: number;
  need_action: number;
  in_progress: number;
  overdue: number;
  blocked: number;
  waiting_decision: number;
  waiting_approval: number;
  upcoming: number;
  completed: number;
  done_today: number;
  waiting_supplement?: number;
};
type MyWorkPageResponse = {
  items: WorkItem[];
  entries?: WorkQueueEntry[];
  summary: MyWorkSummary;
  facets: MyWorkFacets;
  total: number;
  has_more: boolean;
  next_cursor?: string;
};
type MyWorkflowActionsResponse = {
  items: MyWorkflowAction[];
  total: number;
  summary_total: number;
};
type MyWorkScopeSummaryRow = {
  work_items: number;
  workflow_actions: number;
  project_startups: number;
  total: number;
};
type MyWorkScopeSummary = Record<WorkScopeKey, MyWorkScopeSummaryRow>;

const scopes: Array<{ key: WorkScopeKey; label: string }> = [
  { key: 'responsible', label: 'Tôi phụ trách' },
  { key: 'department', label: 'Phòng ban' },
  { key: 'watching', label: 'Đang theo dõi' },
  { key: 'all', label: 'Tất cả' },
];

const attentionQueues: Array<{ key: AttentionQueueKey; label: string; icon: typeof MessageSquare }> = [
  { key: 'all', label: 'Tất cả', icon: MessageSquare },
  { key: 'mentions', label: 'Nhắc đến', icon: AtSign },
  { key: 'questions', label: 'Câu hỏi', icon: MessageSquare },
  { key: 'blockers', label: 'Điểm chặn', icon: AlertCircle },
  { key: 'reminders', label: 'Nhắc hạn', icon: Bell },
  { key: 'unread', label: 'Chưa đọc', icon: CheckCircle2 },
];

const statusOptions = [
  { value: 'ready', label: 'Sẵn sàng' },
  { value: 'in_progress', label: 'Đang xử lý' },
  { value: 'blocked', label: 'Bị chặn' },
  { value: 'waiting_review', label: 'Chờ rà soát' },
  { value: 'waiting_approval', label: 'Chờ duyệt' },
  { value: 'waiting_evidence', label: 'Chờ minh chứng' },
  { value: 'completed', label: 'Hoàn tất' },
];

const deadlineOptions = [
  { value: 'overdue', label: 'Quá hạn' },
  { value: 'due_7_days', label: 'Đến hạn trong 7 ngày' },
  { value: 'no_due', label: 'Chưa có hạn' },
];

const sortOptions = [
  { value: 'priority', label: 'Ưu tiên xử lý' },
  { value: 'due_asc', label: 'Hạn gần nhất' },
  { value: 'newest', label: 'Mới cập nhật' },
];

export function MyWorkPage() {
  const { user, hasPermission } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const requestedTask = searchParams.get('task')?.trim() ?? '';
  const requestedSetup = searchParams.get('setup')?.trim() ?? '';
  const requestedApproval = searchParams.get('approval')?.trim() ?? '';
  const requestedApprovalLaneValue = searchParams.get('approval_type');
  const [selectedEntryId, setSelectedEntryId] = useState<string | null>(() => requestedTask || (requestedSetup ? `project-startup:${requestedSetup}` : null));
  const [startupDialogProjectID, setStartupDialogProjectID] = useState<string | null>(() => requestedSetup || null);
  const [autoApprovalLane, setAutoApprovalLane] = useState<ApprovalLaneKey | null>(null);
  const [searchDraft, setSearchDraft] = useState(searchParams.get('search') ?? '');
  const streamStatus = useActorRealtimeStatus();
  const [advancedFiltersOpen, setAdvancedFiltersOpen] = useState(false);
  const [accountingDetailsOpen, setAccountingDetailsOpen] = useState(false);
  const accountingActions = useAccountingWorkActions();

  const canViewAccountingWork = hasPermission('accounting_handoff.view') && hasPermission('accounting_handoff.acknowledge');
  const canAcknowledgeAccounting = hasPermission('accounting_handoff.acknowledge');
  const requestedMode = validValue<ModeKey>(searchParams.get('mode'), ['work', 'approval', 'accounting', 'attention'], 'work');
  const requestedQueue = validValue<LegacyQueueKey>(searchParams.get('queue'), ['responsible', 'mine', 'department', 'pending_approval', 'watching', 'all'], 'responsible');
  const requestedBucket = validValue<SummaryBucket>(searchParams.get('bucket'), ['all', 'need_action', 'in_progress', 'overdue', 'blocked', 'waiting_decision', 'upcoming', 'completed', 'done_today', 'waiting_supplement'], 'need_action');
  const legacyApprovalMode = requestedQueue === 'pending_approval' || requestedBucket === 'waiting_decision';
  const legacyReturnedAccountingMode = requestedMode === 'work' && requestedBucket === 'waiting_supplement' && canViewAccountingWork;
  const mode: ModeKey = requestedMode === 'accounting'
    ? canViewAccountingWork ? 'accounting' : 'work'
    : requestedMode === 'attention'
      ? 'attention'
      : legacyReturnedAccountingMode
        ? 'accounting'
        : requestedMode === 'approval' || legacyApprovalMode
          ? 'approval'
          : 'work';
  const queue: WorkScopeKey = (requestedQueue === 'pending_approval' || requestedQueue === 'mine') ? 'responsible' : requestedQueue;
  const attentionQueue = validValue<AttentionQueueKey>(searchParams.get('attention'), ['all', 'mentions', 'questions', 'blockers', 'reminders', 'unread'], 'all');
  const bucket: SummaryBucket = requestedBucket === 'waiting_decision' || requestedBucket === 'waiting_supplement' ? 'need_action' : requestedBucket;
  const project = searchParams.get('project') ?? '';
  const status = searchParams.get('status') ?? '';
  const deadline = searchParams.get('deadline') ?? '';
  const workType = searchParams.get('type') ?? '';
  const entryKind = validValue<WorkQueueKind>(searchParams.get('entry_kind'), ['', 'work_item', 'accounting_handoff'], '');
  const priority = searchParams.get('priority') ?? '';
  const search = searchParams.get('search') ?? '';
  const accountingStatus = validValue<AccountingWorkStatus>(searchParams.get('accounting_status'), ['handed_off', 'returned', 'acknowledged', 'all'], legacyReturnedAccountingMode ? 'returned' : 'handed_off');
  const sort = searchParams.get('sort') ?? 'priority';
  const roleSummary = summarizeRoles(user?.roles);
  const canReviewWorkReports = hasPermission('work_report.view') && (hasPermission('work_report.review') || hasPermission('work_report.manage'));
  const requestedApprovalLane = requestedApprovalLaneValue && ['work', 'node', 'report'].includes(requestedApprovalLaneValue)
    ? requestedApprovalLaneValue as ApprovalLaneKey
    : null;
  const permittedRequestedApprovalLane = requestedApprovalLane === 'report' && !canReviewWorkReports ? null : requestedApprovalLane;

  useEffect(() => {
    if (requestedQueue !== 'mine') return;
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.delete('queue');
      return next;
    }, { replace: true });
  }, [requestedQueue, setSearchParams]);

  useEffect(() => {
    if (requestedMode !== 'work' || requestedBucket !== 'waiting_supplement') return;
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.delete('bucket');
      next.delete('queue');
      next.delete('entry_kind');
      next.delete('status');
      if (canViewAccountingWork) {
        next.set('mode', 'accounting');
        next.set('accounting_status', 'returned');
      } else {
        next.delete('mode');
        next.delete('accounting_status');
      }
      return next;
    }, { replace: true });
  }, [canViewAccountingWork, requestedBucket, requestedMode, setSearchParams]);

  useEffect(() => setSearchDraft(search), [search]);
  useEffect(() => {
    if (requestedTask) setSelectedEntryId(requestedTask);
  }, [requestedTask]);
  useEffect(() => {
    if (!requestedSetup) return;
    setSelectedEntryId(`project-startup:${requestedSetup}`);
    setStartupDialogProjectID(requestedSetup);
  }, [requestedSetup]);

  const filterKey = useMemo(() => ({ queue, bucket, project, status, deadline, workType, priority, search, sort, entryKind }), [bucket, deadline, priority, project, queue, search, sort, status, workType, entryKind]);
  const workItems = useInfiniteQuery({
    queryKey: ['my-work-items', user?.id, 'unified', filterKey],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<MyWorkPageResponse>(buildWorkQueuePath(filterKey, String(pageParam ?? ''))),
    getNextPageParam: (lastPage) => lastPage.has_more ? lastPage.next_cursor : undefined,
    enabled: Boolean(user?.id),
    retry: false,
  });
  const scopeSummary = useQuery({
    queryKey: ['my-work-scope-summary', user?.id, project],
    queryFn: () => get<MyWorkScopeSummary>(buildWorkScopeSummaryPath(project)),
    enabled: Boolean(user?.id),
    retry: false,
  });
  const workflowActionsMatchCurrentWorkFilters = (bucket === 'all' || bucket === 'need_action')
    && entryKind !== 'accounting_handoff'
    && !status && !deadline && !workType && !priority;
  const workflowActionsVisible = mode === 'work' && workflowActionsMatchCurrentWorkFilters;
  const workflowActions = useQuery({
    queryKey: ['my-workflow-actions', user?.id, queue, project, search],
    queryFn: () => get<MyWorkflowActionsResponse>(buildWorkflowActionsPath({ queue, project, search })),
    enabled: Boolean(user?.id),
    retry: false,
  });
  const startupVisibility = projectStartupVisibility({ bucket, queue, entryKind, status, deadline, workType, priority });
  const startupActionsMatchCurrentWorkFilters = startupVisibility.rowsVisible;
  const startupActions = useQuery({
    queryKey: ['my-project-startup-actions', user?.id, project, search],
    queryFn: () => get<ProjectStartupActionsResponse>(buildProjectStartupActionsPath({ project, search })),
    enabled: Boolean(user?.id),
    retry: false,
  });
  const accountingFilterKey = useMemo(() => ({ project, status: accountingStatus, search }), [accountingStatus, project, search]);
  const accountingWork = useInfiniteQuery({
    queryKey: ['my-accounting-work', user?.id, accountingFilterKey],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<AccountingWorkPage>(buildAccountingWorkPath(accountingFilterKey, String(pageParam ?? ''))),
    getNextPageParam: (lastPage) => lastPage.has_more ? lastPage.next_cursor : undefined,
    enabled: Boolean(user?.id) && canViewAccountingWork,
    retry: false,
  });
  const approvalFilterKey = useMemo(() => ({
    queue: 'responsible' as WorkScopeKey,
    bucket: 'waiting_decision' as SummaryBucket,
    project,
    status: '',
    deadline: '',
    workType: '',
    priority: '',
    search: '',
    sort: 'priority',
  }), [project]);
  const approvalWorkItems = useInfiniteQuery({
    queryKey: ['my-work-items', user?.id, 'approval', project],
    initialPageParam: '',
    queryFn: ({ pageParam }) => get<MyWorkPageResponse>(buildWorkPath(approvalFilterKey, String(pageParam ?? ''))),
    getNextPageParam: (lastPage) => lastPage.has_more ? lastPage.next_cursor : undefined,
    enabled: Boolean(user?.id),
    retry: false,
  });
  const pendingNodeApprovals = useQuery({
    queryKey: ['approvals', 'pending', 'decidable', user?.id, project],
    queryFn: () => {
      const params = new URLSearchParams({ decidable_only: 'true' });
      if (project) params.set('project_id', project);
      return get<ApprovalRequest[]>(`/approvals/pending?${params.toString()}`);
    },
    enabled: Boolean(user?.id),
    retry: false,
  });
  const reportReviewSummary = useQuery({
    queryKey: ['work-report-summary', user?.id, 'my-work', project],
    queryFn: () => get<WorkReportSummary>(`/work-reports/summary${project ? `?project_id=${encodeURIComponent(project)}` : ''}`),
    enabled: Boolean(user?.id) && canReviewWorkReports,
    retry: false,
  });
  const reportReviewList = useQuery({
    queryKey: ['work-reports', 'review', user?.id, 'my-work', project],
    queryFn: () => {
      const params = new URLSearchParams({ scope: 'review' });
      if (project) params.set('project_id', project);
      return get<WorkReportListItem[]>(`/work-reports?${params.toString()}`);
    },
    enabled: Boolean(user?.id) && canReviewWorkReports && mode === 'approval',
    retry: false,
  });

  const attentionPath = useMemo(() => {
    const params = new URLSearchParams({ limit: '120' });
    if (project) params.set('project_id', project);
    return `/me/activity-attention?${params.toString()}`;
  }, [project]);
  const attentionSummaryPath = useMemo(() => {
    const params = new URLSearchParams();
    if (project) params.set('project_id', project);
    const query = params.toString();
    return `/me/activity-attention/summary${query ? `?${query}` : ''}`;
  }, [project]);
  const activityAttentionSummary = useQuery({
    queryKey: ['activity-attention', 'summary', user?.id, project],
    queryFn: () => get<ActivityAttentionSummary>(attentionSummaryPath),
    enabled: Boolean(user?.id),
    retry: false,
  });
  const activityAttention = useQuery({
    queryKey: ['activity-attention', 'list', user?.id, project],
    queryFn: () => get<ActivityAttentionItem[]>(attentionPath),
    enabled: Boolean(user?.id) && mode === 'attention',
    retry: false,
  });
  const markAttentionRead = useMutation({
    mutationFn: (activityId: string) => post(`/activities/${activityId}/read`),
    onSuccess: (_row, activityId) => {
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      queryClient.invalidateQueries({ queryKey: ['project-activity'] });
      const item = activityAttention.data?.find((row) => row.id === activityId);
      if (item?.project_id) queryClient.invalidateQueries({ queryKey: ['project-operations', item.project_id] });
    },
  });
  const advanceWorkflowAction = useMutation({
    mutationFn: (action: MyWorkflowAction) => post(`/pipeline-run-nodes/${action.run_node_id}/advance`, {}),
    onSuccess: async (_result, action) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['my-workflow-actions'] }),
        queryClient.invalidateQueries({ queryKey: ['my-work-items'] }),
        queryClient.invalidateQueries({ queryKey: ['my-work-scope-summary'] }),
        queryClient.invalidateQueries({ queryKey: ['pipeline-run', action.project_id] }),
        queryClient.invalidateQueries({ queryKey: ['project-operations', action.project_id] }),
        queryClient.invalidateQueries({ queryKey: ['project-progress'] }),
      ]);
      toast.success(workflowActionSuccessMessage(action));
    },
    onError: (error) => toast.error('Không chuyển được giai đoạn', error),
  });

  const firstPage = workItems.data?.pages[0];
  const queueEntries = useMemo(() => dedupeWorkQueue(workItems.data?.pages.flatMap((page) => page.entries ?? []) ?? []), [workItems.data]);
  const rows = useMemo(() => dedupeWorkItems(workItems.data?.pages.flatMap((page) => page.items) ?? []), [workItems.data]);
  const workflowActionRows = useMemo(() => workflowActionsVisible ? workflowActions.data?.items ?? [] : [], [workflowActions.data?.items, workflowActionsVisible]);
  const workflowActionTotal = workflowActionsMatchCurrentWorkFilters ? workflowActions.data?.total ?? 0 : 0;
  const workflowActionSummaryTotal = entryKind === 'accounting_handoff' ? 0 : workflowActions.data?.summary_total ?? 0;
  const startupActionRows = useMemo(() => {
    if (!startupActionsMatchCurrentWorkFilters) return [];
    const items = startupActions.data?.items ?? [];
    if (bucket === 'need_action') return items.filter((item) => item.action_kind === 'setup_required');
    if (bucket === 'upcoming') return items.filter((item) => item.action_kind === 'upcoming');
    return items;
  }, [bucket, startupActions.data?.items, startupActionsMatchCurrentWorkFilters]);
  const startupActionTotal = startupActionRows.length;
  const startupSummary = startupActions.data?.summary ?? { need_action: 0, upcoming: 0, total: 0 };
  const visibleStartupSummary = startupVisibility.contributesToSummary
    ? startupSummary
    : { need_action: 0, upcoming: 0, total: 0 };
  const approvalFirstPage = approvalWorkItems.data?.pages[0];
  const approvalRows = useMemo(() => dedupeWorkItems(approvalWorkItems.data?.pages.flatMap((page) => page.items) ?? []), [approvalWorkItems.data]);
  const summary = firstPage?.summary ?? emptySummary;
  const nodeApprovalRows = useMemo(() => (pendingNodeApprovals.data ?? []).filter((request) => request.target_type === 'pipeline_run_node'), [pendingNodeApprovals.data]);
  const nodeApprovalCount = nodeApprovalRows.length;
  const reportReviewRows = reportReviewList.data ?? [];
  const reportReviewCount = canReviewWorkReports ? reportReviewSummary.data?.scopes.review ?? 0 : 0;
  const facets = firstPage?.facets ?? {};
  const approvalProjectOptions = useMemo(() => mergeApprovalProjectOptions(normalizeFacetOptions(facets.projects, 'Công trình'), nodeApprovalRows, reportReviewRows), [facets.projects, nodeApprovalRows, reportReviewRows]);
  const attentionRows = useMemo(() => asArray<ActivityAttentionItem>(activityAttention.data), [activityAttention.data]);
  const accountingFirstPage = accountingWork.data?.pages[0];
  const accountingRows = useMemo(() => accountingWork.data?.pages.flatMap((page) => page.items) ?? [], [accountingWork.data]);
  const attentionTabCount = activityAttentionSummary.isLoading
    ? '…'
    : activityAttentionSummary.isError
      ? '—'
      : activityAttentionSummary.data?.has_more
        ? '120+'
        : activityAttentionSummary.data?.count ?? 0;
  const workTabCount = scopeSummary.isLoading
    ? '…'
    : scopeSummary.isError && !scopeSummary.data
      ? '—'
      : scopeSummary.data?.[queue]?.total ?? 0;
  const approvalTabCount = approvalWorkItems.isLoading || pendingNodeApprovals.isLoading || (canReviewWorkReports && reportReviewSummary.isLoading)
    ? '…'
    : (approvalWorkItems.isError && !approvalFirstPage) || (pendingNodeApprovals.isError && !pendingNodeApprovals.data) || (canReviewWorkReports && reportReviewSummary.isError && !reportReviewSummary.data)
      ? '—'
      : (approvalFirstPage?.total ?? 0) + nodeApprovalCount + reportReviewCount;
  const accountingTabCount = accountingWork.isLoading
    ? '…'
    : accountingWork.isError && !accountingFirstPage
      ? '—'
      : accountingFirstPage?.summary.handed_off_count ?? 0;
  const approvalWorkLaneCount = approvalWorkItems.isLoading
    ? '…'
    : approvalWorkItems.isError && !approvalFirstPage
      ? '—'
      : approvalFirstPage?.total ?? approvalRows.length;
  const nodeApprovalLaneCount = pendingNodeApprovals.isLoading
    ? '…'
    : pendingNodeApprovals.isError && !pendingNodeApprovals.data
      ? '—'
      : nodeApprovalCount;
  const reportReviewLaneCount = reportReviewSummary.isLoading
    ? '…'
    : reportReviewSummary.isError && !reportReviewSummary.data
      ? '—'
      : reportReviewCount;
  const approvalSourcesSettled = !approvalWorkItems.isLoading
    && !pendingNodeApprovals.isLoading
    && (!canReviewWorkReports || !reportReviewSummary.isLoading);
  const approvalLane = requestedApproval
    ? 'node'
    : permittedRequestedApprovalLane ?? autoApprovalLane;
  const activeWorkRows = mode === 'approval' ? approvalRows : rows;
  const selectedStartupAction = mode === 'work' ? startupActionRows.find((action) => action.id === selectedEntryId) ?? null : null;
  const selectedWorkflowAction = mode === 'work' && !selectedStartupAction ? workflowActionRows.find((action) => action.id === selectedEntryId) ?? null : null;
  const selectedAccountingEntry = mode === 'work' ? queueEntries.find((entry) => entry.kind === 'accounting_handoff' && workQueueEntryKey(entry) === selectedEntryId) : undefined;
  const selectedAccounting = selectedAccountingEntry?.kind === 'accounting_handoff' ? selectedAccountingEntry.item : null;
  const selectedItem = selectedStartupAction || selectedWorkflowAction || selectedAccounting ? null : activeWorkRows.find((item) => item.id === selectedEntryId) ?? activeWorkRows[0] ?? null;
  const orderedEntries = useMemo(() => orderedWorkListEntries(queueEntries, startupActionRows, workflowActionRows, sort === 'priority'), [queueEntries, startupActionRows, workflowActionRows, sort]);

  useEffect(() => {
    // Do not let an earlier startup/action response preselect a non-accounting row.
    if (mode === 'work' && workItems.isLoading) return;
    const entryIds = mode === 'work'
      ? orderedEntries.map(workQueueEntryKey)
      : activeWorkRows.map((item) => item.id);
    if (!entryIds.length) setSelectedEntryId(null);
    else if (!selectedEntryId || !entryIds.includes(selectedEntryId)) setSelectedEntryId(entryIds[0]);
  }, [activeWorkRows, mode, selectedEntryId, orderedEntries, workItems.isLoading]);

  useEffect(() => {
    if (mode !== 'approval' || requestedApproval || permittedRequestedApprovalLane || autoApprovalLane || !approvalSourcesSettled) return;
    setAutoApprovalLane(firstPopulatedApprovalLane(
      approvalFirstPage?.total ?? approvalRows.length,
      nodeApprovalCount,
      reportReviewCount,
      canReviewWorkReports,
    ));
  }, [approvalFirstPage?.total, approvalRows.length, approvalSourcesSettled, autoApprovalLane, canReviewWorkReports, mode, nodeApprovalCount, permittedRequestedApprovalLane, reportReviewCount, requestedApproval]);

  function updateParams(changes: Record<string, string | null>) {
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      Object.entries(changes).forEach(([key, value]) => {
        if (!value || (key === 'mode' && value === 'work') || (key === 'queue' && value === 'responsible') || (key === 'bucket' && value === 'need_action') || (key === 'sort' && value === 'priority')) next.delete(key);
        else next.set(key, value);
      });
      return next;
    });
  }

  function resetFilters() {
    setSearchDraft('');
    updateParams({ queue: 'responsible', bucket: 'need_action', project: null, status: null, deadline: null, type: null, entry_kind: null, priority: null, search: null, sort: 'priority' });
  }

  function changeWorkFilter(key: string, value: string) {
    updateParams(key === 'entry_kind'
      ? { entry_kind: value || null, status: null, deadline: null, type: null, priority: null, bucket: 'all' }
      : { [key]: value || null, bucket: 'all' });
  }

  function selectBucket(nextBucket: WorkQuickBucket) {
    updateParams({
      mode: 'work',
      bucket: nextBucket,
      status: null,
      deadline: null,
      type: null,
      priority: null,
      search: null,
    });
  }

  function refreshAll() {
    void workItems.refetch();
    void scopeSummary.refetch();
    void workflowActions.refetch();
    void startupActions.refetch();
    void activityAttentionSummary.refetch();
    if (mode === 'attention') void activityAttention.refetch();
    if (mode === 'approval') void approvalWorkItems.refetch();
    void pendingNodeApprovals.refetch();
    if (canReviewWorkReports) void reportReviewSummary.refetch();
    if (mode === 'approval' && canReviewWorkReports) void reportReviewList.refetch();
    if (canViewAccountingWork) void accountingWork.refetch();
  }

  function openWorkItem(item: WorkItem) {
    navigate(executionTargetPath(item, currentReturnTo(location)));
  }

  function openAccountingAction(item: AccountingWorkItem, action: 'acknowledge' | 'return') {
    setAccountingDetailsOpen(false);
    accountingActions.setDialog({ item, action });
  }

  const hasFilters = Boolean(bucket !== 'need_action' || queue !== 'responsible' || project || status || deadline || workType || entryKind || priority || search || sort !== 'priority');
  const total = (firstPage?.total ?? summary.total ?? rows.length) + workflowActionTotal + startupActionTotal;
  const approvalTotal = approvalFirstPage?.total ?? approvalRows.length;
  const scopeLabel = scopes.find((item) => item.key === queue)?.label ?? 'Tôi phụ trách';
  const actionAwareSummary = {
    ...summary,
    total: summary.total + workflowActionSummaryTotal + visibleStartupSummary.total,
    need_action: summary.need_action + workflowActionSummaryTotal + visibleStartupSummary.need_action,
    upcoming: summary.upcoming + visibleStartupSummary.upcoming,
  };

  return (
    <>
      <PageHeader
        title="Bàn làm việc"
        description={`${user?.full_name ?? 'Người dùng'}${roleSummary ? ` · ${roleSummary}` : ''} · Mở thẳng đúng sổ nghiệp vụ cần xử lý`}
        compactMobile
        actions={(
          <div className="flex items-center gap-2">
            <LiveStatusBadge status={streamStatus} />
            <button className="inline-flex h-10 items-center gap-2 rounded-md border border-line px-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 md:px-3" onClick={refreshAll} aria-label="Làm mới Bàn làm việc">
            <RefreshCw className={clsx('h-4 w-4', (workItems.isFetching || scopeSummary.isFetching || workflowActions.isFetching || startupActions.isFetching || activityAttentionSummary.isFetching || pendingNodeApprovals.isFetching || (canViewAccountingWork && accountingWork.isFetching) || (canReviewWorkReports && reportReviewSummary.isFetching) || (mode === 'approval' && canReviewWorkReports && reportReviewList.isFetching) || (mode === 'attention' && activityAttention.isFetching)) && 'animate-spin')} />
              <span className="hidden md:inline">Làm mới</span>
            </button>
          </div>
        )}
      />

      <div className="mt-2 grid grid-cols-2 gap-1.5 md:mt-4 md:flex md:gap-1 md:overflow-x-auto md:border-b md:border-line" role="group" aria-label="Chế độ Bàn làm việc">
        <ModeTab active={mode === 'work'} label="Công việc" count={workTabCount} onClick={() => updateParams({ mode: 'work', queue, bucket })} />
        <ModeTab active={mode === 'approval'} label="Chờ tôi duyệt" count={approvalTabCount} onClick={() => updateParams({ mode: 'approval', queue, bucket })} />
        {canViewAccountingWork ? <ModeTab active={mode === 'accounting'} label="Nghiệp vụ Kế toán" count={accountingTabCount} onClick={() => updateParams({ mode: 'accounting' })} /> : null}
        <ModeTab active={mode === 'attention'} label="Trao đổi cần phản hồi" count={attentionTabCount} wideMobile={!canViewAccountingWork} onClick={() => updateParams({ mode: 'attention' })} />
      </div>

      {mode === 'work' && hasPermission('accounting_request.view') && <AccountingRequestQueue projectId={project}/>}
      {mode === 'work' ? (
        <section className="mt-4">
          <div className="grid gap-3 xl:grid-cols-[208px_minmax(0,1fr)_320px]">
            <ScopeRail
              value={queue}
              counts={scopeSummary.data}
              loading={scopeSummary.isLoading}
              unavailable={scopeSummary.isError && !scopeSummary.data}
              onChange={(value) => updateParams({ queue: value, bucket: 'all' })}
            />

            <div className="min-w-0">
              <WorkQuickFilters
                summary={actionAwareSummary}
                selected={bucket}
                scopeLabel={scopeLabel}
                loading={workItems.isLoading || workflowActions.isLoading || startupActions.isLoading}
                unavailable={(workItems.isError && !firstPage) || (workflowActions.isError && !workflowActions.data) || (startupActions.isError && !startupActions.data)}
                onSelect={selectBucket}
              />

              <FilterBar
                accounting={canViewAccountingWork}
                entryKind={entryKind}
                searchDraft={searchDraft}
                project={project}
                status={status}
                deadline={deadline}
                workType={workType}
                priority={priority}
                sort={sort}
                facets={facets}
                hasFilters={hasFilters}
                advancedOpen={advancedFiltersOpen}
                onSearchDraft={setSearchDraft}
                onSearch={() => updateParams({ search: searchDraft.trim() || null, bucket: 'all' })}
                onChange={changeWorkFilter}
                onReset={resetFilters}
                onToggleAdvanced={() => setAdvancedFiltersOpen((open) => !open)}
              />

              <MobileFilterBar
                searchDraft={searchDraft}
                project={project}
                status={status}
                deadline={deadline}
                workType={workType}
                priority={priority}
                sort={sort}
                entryKind={entryKind}
                facets={facets}
                accounting={canViewAccountingWork}
                onSearchDraft={setSearchDraft}
                onSearch={() => updateParams({ search: searchDraft.trim() || null, bucket: 'all' })}
                onChange={changeWorkFilter}
                onApply={(values) => updateParams({ project: values.project || null, status: values.status || null, deadline: values.deadline || null, type: values.workType || null, priority: values.priority || null, sort: values.sort, entry_kind: values.entryKind || null, bucket: 'all' })}
              />

              <div className="mt-3">
                <WorkList
                  entries={queueEntries}
                  prioritizeAccounting={sort === 'priority'}
                  selectedAccountingId={selectedAccounting?.handoff_id}
                  onSelectAccounting={(item) => {
                    setSelectedEntryId(`accounting:${item.handoff_id}`);
                    if (!window.matchMedia('(min-width: 1280px)').matches) setAccountingDetailsOpen(true);
                  }}
                  onAcknowledgeAccounting={canAcknowledgeAccounting ? (item) => openAccountingAction(item, 'acknowledge') : undefined}
                  rows={rows}
                  selectedId={selectedItem?.id}
                  workflowActions={workflowActionRows}
                  startupActions={startupActionRows}
                  selectedStartupActionId={selectedStartupAction?.id}
                  startupActionsLoading={startupActionsMatchCurrentWorkFilters && startupActions.isLoading}
                  startupActionsError={startupActionsMatchCurrentWorkFilters ? startupActions.error : null}
                  selectedWorkflowActionId={selectedWorkflowAction?.id}
                  workflowActionsLoading={workflowActionsVisible && workflowActions.isLoading}
                  workflowActionsError={workflowActionsVisible ? workflowActions.error : null}
                  pendingWorkflowActionId={advanceWorkflowAction.isPending ? advanceWorkflowAction.variables?.id : undefined}
                  loading={workItems.isLoading}
                  fetchingMore={workItems.isFetchingNextPage}
                  error={workItems.error}
                  total={total}
                  hasMore={Boolean(workItems.hasNextPage)}
                  filtered={hasFilters}
                  onSelect={setSelectedEntryId}
                  onSelectWorkflowAction={setSelectedEntryId}
                  onSelectStartupAction={setSelectedEntryId}
                  onOpenStartupAction={(action) => setStartupDialogProjectID(action.project_id)}
                  onRunWorkflowAction={(action) => advanceWorkflowAction.mutate(action)}
                  onOpen={openWorkItem}
                  onLoadMore={() => void workItems.fetchNextPage()}
                  onReset={resetFilters}
                  onRetry={() => void workItems.refetch()}
                  onRetryWorkflowActions={() => void workflowActions.refetch()}
                  onRetryStartupActions={() => void startupActions.refetch()}
                />
              </div>
            </div>

            {selectedAccounting ? <aside className="hidden self-start rounded-lg border border-line bg-white p-4 xl:block">
              <AccountingWorkDetails item={selectedAccounting} canAcknowledge={canAcknowledgeAccounting} onAction={(action) => openAccountingAction(selectedAccounting, action)} />
            </aside> : <NextQueuePanel
              item={selectedItem}
              workflowAction={selectedWorkflowAction}
              startupAction={selectedStartupAction}
              pending={advanceWorkflowAction.isPending && advanceWorkflowAction.variables?.id === selectedWorkflowAction?.id}
              onOpen={openWorkItem}
              onRunWorkflowAction={(action) => advanceWorkflowAction.mutate(action)}
              onOpenStartupAction={(action) => setStartupDialogProjectID(action.project_id)}
            />}
          </div>
        </section>
      ) : mode === 'approval' ? (
        <ApprovalWorkspace
          activeLane={approvalLane}
          workCount={approvalWorkLaneCount}
          nodeCount={nodeApprovalLaneCount}
          reportTabCount={reportReviewLaneCount}
          workRows={approvalRows}
          selectedItem={selectedItem}
          workLoading={approvalWorkItems.isLoading}
          workFetchingMore={approvalWorkItems.isFetchingNextPage}
          workError={approvalWorkItems.error}
          workTotal={approvalTotal}
          workHasMore={Boolean(approvalWorkItems.hasNextPage)}
          nodeRows={nodeApprovalRows}
          nodeLoading={pendingNodeApprovals.isLoading}
          nodeError={pendingNodeApprovals.error}
          requestedApprovalId={requestedApproval}
          reportVisible={canReviewWorkReports}
          reportRows={reportReviewRows}
          reportCount={reportReviewCount}
          reportLoading={reportReviewList.isLoading}
          reportError={reportReviewList.error}
          project={project}
          projects={approvalProjectOptions}
          onProjectChange={(value) => updateParams({ project: value || null })}
          onLaneChange={(value) => updateParams({ approval_type: value, approval: value === 'node' ? requestedApproval || null : null })}
          onSelect={setSelectedEntryId}
          onOpen={openWorkItem}
          onLoadMore={() => void approvalWorkItems.fetchNextPage()}
          onRetryWork={() => void approvalWorkItems.refetch()}
          onRetryNodes={() => void pendingNodeApprovals.refetch()}
          onRetryReports={() => void reportReviewList.refetch()}
          onOpenReport={(report) => navigate(`/reports?scope=review&report=${report.source_id}`)}
        />
      ) : mode === 'accounting' ? (
        <AccountingWorkLane
          rows={accountingRows}
          summary={accountingFirstPage?.summary}
          total={accountingFirstPage?.total ?? accountingRows.length}
          loading={accountingWork.isLoading}
          fetchingMore={accountingWork.isFetchingNextPage}
          error={accountingWork.error}
          hasMore={Boolean(accountingWork.hasNextPage)}
          project={project}
          status={accountingStatus}
          search={search}
          canAcknowledge={canAcknowledgeAccounting}
          onFilterChange={(changes) => updateParams(changes)}
          onLoadMore={() => void accountingWork.fetchNextPage()}
          onRetry={() => void accountingWork.refetch()}
        />
      ) : (
        <ActivityAttentionPanel
          rows={attentionRows}
          loading={activityAttention.isLoading}
          error={activityAttention.error}
          selectedQueue={attentionQueue}
          streamStatus={streamStatus}
          returnTo={currentReturnTo(location)}
          markingId={markAttentionRead.isPending ? markAttentionRead.variables : undefined}
          onSelectQueue={(value) => updateParams({ attention: value === 'all' ? null : value })}
          onMarkRead={(activityId) => markAttentionRead.mutate(activityId)}
          onRetry={() => void activityAttention.refetch()}
        />
      )}

      <Dialog open={mode === 'work' && accountingDetailsOpen && Boolean(selectedAccounting)} title="Hồ sơ kế toán" onClose={() => setAccountingDetailsOpen(false)} size="md">
        {selectedAccounting ? <AccountingWorkDetails item={selectedAccounting} canAcknowledge={canAcknowledgeAccounting} onAction={(action) => openAccountingAction(selectedAccounting, action)} /> : null}
      </Dialog>
      <AccountingHandoffActionDialog
        dialog={accountingActions.dialog}
        pending={accountingActions.mutation.isPending}
        onClose={() => { if (!accountingActions.mutation.isPending) accountingActions.setDialog(null); }}
        onSubmit={(values) => {
          const target = accountingActions.dialog;
          if (target && target.action !== 'handoff') accountingActions.mutation.mutate({ action: target.action, item: target.item, ...values });
        }}
      />

      <ProjectStartupWizard
        open={Boolean(startupDialogProjectID)}
        projectId={startupDialogProjectID}
        onClose={() => {
          setStartupDialogProjectID(null);
          if (searchParams.get('setup')) updateParams({ setup: null });
        }}
        onChanged={() => {
          void startupActions.refetch();
          void workItems.refetch();
          void scopeSummary.refetch();
          void queryClient.invalidateQueries({ queryKey: ['my-work-navigation-summary'] });
        }}
      />

    </>
  );
}

function ApprovalWorkspace({ activeLane, workCount, nodeCount, reportTabCount, workRows, selectedItem, workLoading, workFetchingMore, workError, workTotal, workHasMore, nodeRows, nodeLoading, nodeError, requestedApprovalId, reportVisible, reportRows, reportCount, reportLoading, reportError, project, projects, onProjectChange, onLaneChange, onSelect, onOpen, onLoadMore, onRetryWork, onRetryNodes, onRetryReports, onOpenReport }: {
  activeLane: ApprovalLaneKey | null;
  workCount: number | string;
  nodeCount: number | string;
  reportTabCount: number | string;
  workRows: WorkItem[];
  selectedItem: WorkItem | null;
  workLoading: boolean;
  workFetchingMore: boolean;
  workError: unknown;
  workTotal: number;
  workHasMore: boolean;
  nodeRows: ApprovalRequest[];
  nodeLoading: boolean;
  nodeError: unknown;
  requestedApprovalId: string;
  reportVisible: boolean;
  reportRows: WorkReportListItem[];
  reportCount: number;
  reportLoading: boolean;
  reportError: unknown;
  project: string;
  projects: Array<{ value: string; label: string }>;
  onProjectChange: (value: string) => void;
  onLaneChange: (value: ApprovalLaneKey) => void;
  onSelect: (id: string) => void;
  onOpen: (item: WorkItem) => void;
  onLoadMore: () => void;
  onRetryWork: () => void;
  onRetryNodes: () => void;
  onRetryReports: () => void;
  onOpenReport: (report: WorkReportListItem) => void;
}) {
  return (
    <section className="mt-4" aria-label="Công việc chờ tôi duyệt">
      <div className="flex flex-col gap-3 rounded-lg border border-line bg-white p-3 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-sm font-semibold text-ink">Chờ tôi duyệt</h2>
        <FilterSelect value={project} label="Tất cả công trình" options={projects} onChange={onProjectChange} />
      </div>

      <div className="mt-3 flex gap-1 overflow-x-auto border-b border-line" role="tablist" aria-label="Nhóm yêu cầu chờ duyệt">
        <ApprovalLaneTab lane="work" active={activeLane === 'work'} label="Phê duyệt công việc" count={workCount} onSelect={onLaneChange} />
        <ApprovalLaneTab lane="node" active={activeLane === 'node'} label="Phê duyệt giai đoạn quy trình" count={nodeCount} onSelect={onLaneChange} />
        {reportVisible ? <ApprovalLaneTab lane="report" active={activeLane === 'report'} label="Báo cáo chờ xác nhận" count={reportTabCount} onSelect={onLaneChange} /> : null}
      </div>

      {!activeLane ? <div className="mt-3 rounded-lg border border-line bg-white p-8" aria-label="Đang tải nhóm yêu cầu chờ duyệt"><LoadingSpinner /></div> : null}

      {activeLane === 'work' ? (
        <div id="approval-lane-panel-work" role="tabpanel" aria-labelledby="approval-lane-tab-work" className="mt-3 grid gap-3 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div className="min-w-0">
            <WorkList
              rows={workRows}
              selectedId={selectedItem?.id}
              loading={workLoading}
              fetchingMore={workFetchingMore}
              error={workError}
              total={workTotal}
              hasMore={workHasMore}
              filtered={Boolean(project)}
              emptyTitle="Không có công việc nào đang chờ anh duyệt."
              onSelect={onSelect}
              onOpen={onOpen}
              onLoadMore={onLoadMore}
              onReset={() => onProjectChange('')}
              onRetry={onRetryWork}
            />
          </div>
          <NextWorkPanel item={selectedItem} onOpen={onOpen} />
        </div>
      ) : null}

      {activeLane === 'node' ? (
        <div id="approval-lane-panel-node" role="tabpanel" aria-labelledby="approval-lane-tab-node" className="mt-3">
          <NodeApprovalLane rows={nodeRows} loading={nodeLoading} error={nodeError} requestedApprovalId={requestedApprovalId} onRetry={onRetryNodes} />
        </div>
      ) : null}

      {reportVisible && activeLane === 'report' ? (
        <div id="approval-lane-panel-report" role="tabpanel" aria-labelledby="approval-lane-tab-report" className="mt-3">
          <WorkReportReviewLane rows={reportRows} total={reportCount} loading={reportLoading} error={reportError} onRetry={onRetryReports} onOpen={onOpenReport} />
        </div>
      ) : null}
    </section>
  );
}

function ApprovalLaneTab({ lane, active, label, count, onSelect }: { lane: ApprovalLaneKey; active: boolean; label: string; count: number | string; onSelect: (lane: ApprovalLaneKey) => void }) {
  return (
    <button
      id={`approval-lane-tab-${lane}`}
      type="button"
      role="tab"
      aria-selected={active}
      aria-controls={`approval-lane-panel-${lane}`}
      className={clsx(
        'inline-flex min-w-fit items-center gap-2 border-b-2 px-4 py-3 text-sm font-semibold transition',
        active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:border-slate-300 hover:text-slate-800',
      )}
      onClick={() => onSelect(lane)}
    >
      {label}
      <span className={clsx('rounded-md px-2 py-0.5 text-xs', active ? 'bg-blue-50 text-blue-700' : 'bg-slate-100 text-slate-600')}>{count}</span>
    </button>
  );
}

function NodeApprovalLane({ rows, loading, error, requestedApprovalId, onRetry }: { rows: ApprovalRequest[]; loading: boolean; error: unknown; requestedApprovalId: string; onRetry: () => void }) {
  const requestedRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!requestedApprovalId || !requestedRef.current) return;
    requestedRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' });
    requestedRef.current.focus({ preventScroll: true });
  }, [requestedApprovalId, rows]);
  return (
    <section className="rounded-md border border-amber-200 bg-amber-50/50 p-3" aria-label="Phê duyệt giai đoạn quy trình">
      <p className="mb-3 text-xs text-slate-600">Quyết định ở cấp node; không tạo đầu việc giả trong danh sách công việc.</p>
      {loading ? <LoadingSpinner /> : error ? <SourceError title="Không tải được yêu cầu phê duyệt giai đoạn" onRetry={onRetry} /> : rows.length ? <div className="space-y-2">{rows.map((request) => (
        <div
          key={request.id}
          ref={request.id === requestedApprovalId ? requestedRef : undefined}
          data-approval-id={request.id}
          tabIndex={-1}
          className={clsx('rounded-md outline-none', request.id === requestedApprovalId && 'ring-2 ring-blue-500 ring-offset-2')}
        >
          <ApprovalPanel request={request} />
        </div>
      ))}</div> : <p className="rounded-md border border-dashed border-amber-200 bg-white p-4 text-sm text-slate-600">Không có giai đoạn nào đang chờ anh phê duyệt.</p>}
    </section>
  );
}

function WorkReportReviewLane({ rows, total, loading, error, onRetry, onOpen }: { rows: WorkReportListItem[]; total: number; loading: boolean; error: unknown; onRetry: () => void; onOpen: (report: WorkReportListItem) => void }) {
  const visibleRows = rows.slice(0, 5);
  return (
    <section className="rounded-md border border-blue-200 bg-blue-50/40 p-3" aria-label="Báo cáo chờ xác nhận">
      <p className="mb-3 text-xs text-slate-600">Báo cáo đã gửi đích danh đến anh/chị và còn ở trạng thái chờ xác nhận.</p>
      {loading ? <LoadingSpinner /> : error ? <SourceError title="Không tải được báo cáo chờ xác nhận" onRetry={onRetry} /> : visibleRows.length ? (
        <div className="space-y-2">
          {visibleRows.map((report) => (
            <article key={report.source_id} className="flex flex-col gap-3 rounded-md border border-blue-100 bg-white p-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0">
                <p className="text-sm font-semibold text-ink">Báo cáo ngày {workReportDateText(report.report_date)}</p>
                <p className="mt-1 truncate text-xs font-medium text-slate-600" title={`${report.project_code} · ${report.project_name}`}>{report.project_code} · {report.project_name}</p>
                <p className="mt-1 text-xs text-slate-500">{identityReferenceText(report.author_name, report.author_account_deleted, 'Người lập')} · Lần {report.current_revision_no}</p>
              </div>
              <button type="button" className="shrink-0 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => onOpen(report)}>Xem và xác nhận</button>
            </article>
          ))}
          {total > visibleRows.length ? <Link className="inline-flex w-full justify-center rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to="/reports?scope=review">Mở tất cả {total} báo cáo</Link> : null}
        </div>
      ) : <p className="rounded-md border border-dashed border-blue-200 bg-white p-4 text-sm text-slate-600">Không có báo cáo nào đang chờ anh/chị xác nhận.</p>}
    </section>
  );
}

const emptySummary: MyWorkSummary = { total: 0, need_action: 0, in_progress: 0, overdue: 0, blocked: 0, waiting_decision: 0, waiting_approval: 0, upcoming: 0, completed: 0, done_today: 0 };

function WorkQuickFilters({ summary, selected, scopeLabel, loading, unavailable, onSelect }: {
  summary: MyWorkSummary;
  selected: SummaryBucket;
  scopeLabel: string;
  loading: boolean;
  unavailable: boolean;
  onSelect: (bucket: WorkQuickBucket) => void;
}) {
  const cards: Array<{ bucket: WorkQuickBucket; label: string; value: number; icon: typeof BriefcaseBusiness; activeClass: string }> = [
    { bucket: 'need_action', label: 'Làm ngay', value: summary.need_action, icon: BriefcaseBusiness, activeClass: 'border-blue-300 bg-blue-50 ring-blue-200' },
    { bucket: 'in_progress', label: 'Đang làm', value: summary.in_progress, icon: Clock3, activeClass: 'border-emerald-300 bg-emerald-50 ring-emerald-200' },
    { bucket: 'blocked', label: 'Bị chặn', value: summary.blocked, icon: AlertCircle, activeClass: 'border-rose-300 bg-rose-50 ring-rose-200' },
    { bucket: 'upcoming', label: 'Sắp tới · chỉ xem', value: summary.upcoming, icon: Clock3, activeClass: 'border-slate-300 bg-slate-50 ring-slate-200' },
    { bucket: 'completed', label: 'Hoàn tất', value: summary.completed, icon: CheckCircle2, activeClass: 'border-emerald-300 bg-emerald-50 ring-emerald-200' },
  ];
  const allActive = selected === 'all';
  return (
    <section className="mb-3 rounded-lg border border-line bg-white p-2 md:p-3" aria-label="Bộ lọc nhanh công việc" aria-busy={loading}>
      <div className="mb-2 hidden flex-col gap-2 md:flex md:flex-row md:items-center md:justify-between">
        <h2 className="text-sm font-semibold text-ink">Bộ lọc nhanh · {scopeLabel}</h2>
        <button
          type="button"
          aria-pressed={allActive}
          className={clsx(
            'inline-flex w-full items-center justify-between gap-3 rounded-md border px-3 py-2 text-left text-sm transition sm:w-auto sm:min-w-[9rem]',
            allActive ? 'border-blue-300 bg-blue-50 text-blue-800 ring-1 ring-blue-200' : 'border-line bg-white text-slate-700 hover:border-blue-200 hover:bg-slate-50',
          )}
          onClick={() => onSelect('all')}
        >
          <span className="font-semibold">Xem tất cả</span>
          <span className="text-base font-semibold">{loading ? '…' : unavailable ? '—' : summary.total}</span>
        </button>
      </div>
      <div className="grid grid-cols-2 gap-1.5 min-[360px]:grid-cols-3 md:hidden">
        <button type="button" aria-pressed={allActive} className={clsx('flex min-h-11 min-w-0 items-center justify-between gap-1 rounded-md border px-2 text-left text-xs font-semibold', allActive ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700')} onClick={() => onSelect('all')}>
          <span>Tất cả</span><span className="tabular-nums">{loading ? '…' : unavailable ? '—' : summary.total}</span>
        </button>
        {cards.map((card) => <button key={card.bucket} type="button" aria-pressed={selected === card.bucket} aria-label={card.label} className={clsx('flex min-h-11 min-w-0 items-center justify-between gap-1 rounded-md border px-2 text-left text-xs font-semibold', selected === card.bucket ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700')} onClick={() => onSelect(card.bucket)}>
          <span className="min-w-0">{card.bucket === 'upcoming' ? 'Sắp tới' : card.label}</span><span className="shrink-0 tabular-nums">{loading ? '…' : unavailable ? '—' : card.value}</span>
        </button>)}
      </div>
      <div className="hidden gap-2 overflow-x-auto pb-1 md:flex">
        {cards.map((card) => {
          const Icon = card.icon;
          const active = selected === card.bucket;
          return (
            <button
              key={card.bucket}
              type="button"
              aria-pressed={active}
              className={clsx(
                'flex min-w-[8.5rem] flex-1 items-center justify-between gap-3 rounded-md border bg-white px-3 py-2 text-left transition',
                active ? `ring-1 ${card.activeClass}` : 'border-line hover:border-blue-200 hover:bg-slate-50',
              )}
              onClick={() => onSelect(card.bucket)}
            >
              <span><span className="block text-xs font-semibold text-slate-500">{card.label}</span><span className="mt-0.5 block text-lg font-semibold text-ink">{loading ? '…' : unavailable ? '—' : card.value}</span></span>
              <Icon className={clsx('h-4 w-4', active ? 'text-blue-700' : 'text-slate-400')} />
            </button>
          );
        })}
      </div>
    </section>
  );
}

function ModeTab({ active, label, count, wideMobile = false, onClick }: { active: boolean; label: string; count: number | string; wideMobile?: boolean; onClick: () => void }) {
  return (
    <button type="button" aria-pressed={active} className={clsx('flex min-h-11 min-w-0 items-center justify-between gap-1 rounded-md border px-2 py-1.5 text-left text-xs font-semibold md:block md:rounded-none md:border-0 md:border-b-2 md:px-4 md:py-3 md:text-sm', wideMobile && 'col-span-2 md:col-auto', active ? 'border-blue-300 bg-blue-50 text-blue-700 md:border-blue-600 md:bg-transparent' : 'border-line bg-white text-slate-600 hover:text-slate-800 md:border-transparent')} onClick={onClick}>
      <span className="min-w-0 leading-4">{label}</span> <span className="shrink-0 rounded-md bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600 md:ml-1 md:px-2">{count}</span>
    </button>
  );
}

function ScopeCountChip({ count, active }: { count: number; active: boolean }) {
  if (count <= 0) return null;
  return (
    <span
      className={clsx(
        'inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full px-1.5 text-[11px] font-bold tabular-nums ring-1 ring-inset',
        active ? 'bg-blue-600 text-white ring-blue-600' : 'bg-slate-100 text-slate-600 ring-slate-200',
      )}
      aria-label={`${count} việc làm ngay`}
    >
      {count}
    </span>
  );
}

function ScopeRail({ value, counts, loading, onChange, unavailable = false }: {
  unavailable?: boolean;
  value: WorkScopeKey;
  counts?: MyWorkScopeSummary;
  loading: boolean;
  onChange: (value: WorkScopeKey) => void;
}) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const countValue = (scope: WorkScopeKey) => counts?.[scope]?.total ?? 0;
  const selectedLabel = scopes.find((item) => item.key === value)?.label ?? 'Tôi phụ trách';
  return (
    <aside className="min-w-0 rounded-lg border border-line bg-white p-2 md:p-3 xl:sticky xl:top-4 xl:self-start" aria-label="Phạm vi công việc" aria-busy={loading}>
      {unavailable ? <p className="mb-2 text-xs text-amber-700">Số lượng chưa khả dụng. Bấm Làm mới để thử lại.</p> : null}
      <div className="flex min-h-11 items-center justify-between gap-2 md:hidden">
        <span className="text-xs font-semibold text-slate-500">Phạm vi</span>
        <span className="min-w-0 truncate text-sm font-semibold text-ink">{selectedLabel}{!loading && !unavailable && countValue(value) > 0 ? ` · ${countValue(value)}` : ''}</span>
        <button type="button" className="shrink-0 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700" onClick={() => setMobileOpen(true)}>Đổi</button>
      </div>
      <Dialog open={mobileOpen} title="Phạm vi công việc" size="sm" mobileFullscreen manageFocus onClose={() => setMobileOpen(false)}>
        <div className="grid gap-2">
          {scopes.map((item) => <button key={item.key} type="button" aria-pressed={item.key === value} className={clsx('flex min-h-11 items-center justify-between rounded-md border px-3 text-left text-sm font-semibold', item.key === value ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line text-slate-700')} onClick={() => { onChange(item.key); setMobileOpen(false); }}>
            <span>{item.label}</span><ScopeCountChip count={countValue(item.key)} active={item.key === value} />
          </button>)}
        </div>
      </Dialog>
      <div className="mb-2 hidden items-center justify-between gap-3 md:flex">
        <h2 className="text-sm font-semibold text-ink">Phạm vi</h2>
        <span className="flex shrink-0 items-center gap-2 text-xs font-semibold text-blue-700 xl:hidden">
          {scopes.find((item) => item.key === value)?.label}
          <ScopeCountChip count={countValue(value)} active />
        </span>
      </div>
      <div className="hidden gap-2 overflow-x-auto pb-1 md:flex xl:flex-col xl:overflow-visible xl:pb-0">
        {scopes.map((item) => {
          const active = item.key === value;
          return (
            <button
              key={item.key}
              type="button"
              aria-pressed={active}
              className={clsx(
                'relative min-w-fit rounded-md border px-3 py-2 text-left transition xl:w-full',
                active ? 'border-blue-200 bg-blue-50 text-blue-800' : 'border-transparent text-slate-700 hover:border-line hover:bg-slate-50',
              )}
              onClick={() => onChange(item.key)}
            >
              {active ? <span className="absolute inset-y-2 left-0 w-1 rounded-r bg-blue-600" aria-hidden="true" /> : null}
              <span className="flex items-center gap-2 text-sm font-semibold">
                {active ? <CheckCircle2 className="h-4 w-4 shrink-0 text-blue-600" /> : <span className="h-4 w-4 shrink-0" aria-hidden="true" />}
                <span>{item.label}</span>
                <span className="ml-auto">
                  <ScopeCountChip count={countValue(item.key)} active={active} />
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </aside>
  );
}

function FilterBar({ searchDraft, project, status, deadline, workType, priority, sort, facets, hasFilters, advancedOpen, onSearchDraft, onSearch, onChange, onReset, onToggleAdvanced, accounting = false, entryKind = '' }: {
  accounting?: boolean; entryKind?: WorkQueueKind;
  searchDraft: string; project: string; status: string; deadline: string; workType: string; priority: string; sort: string; facets: MyWorkFacets; hasFilters: boolean; advancedOpen: boolean;
  onSearchDraft: (value: string) => void; onSearch: () => void; onChange: (key: string, value: string) => void; onReset: () => void; onToggleAdvanced: () => void;
}) {
  const searchId = useId();
  const advancedId = useId();
  const projects = normalizeFacetOptions(facets.projects, 'Công trình');
  const statuses = normalizeFacetOptions(facets.statuses, 'Trạng thái');
  const workTypes = normalizeFacetOptions(facets.work_types, 'Loại việc');
  const priorities = normalizeFacetOptions(facets.priorities, 'Ưu tiên');
  return (
    <div className="my-work-filters relative z-10 hidden space-y-4 rounded-lg border border-line bg-white p-4 text-sm md:block">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2"><SlidersHorizontal className="h-4 w-4 text-blue-700" /><h2 className="text-sm font-semibold text-ink">Tìm công việc</h2></div>
        {hasFilters ? <button type="button" className="focus-ring inline-flex items-center gap-1 rounded text-xs font-semibold text-slate-600 hover:text-blue-700" onClick={onReset}>Về mặc định<X className="h-3.5 w-3.5" /></button> : null}
      </div>

      <form className="min-w-0 space-y-1" onSubmit={(event) => { event.preventDefault(); onSearch(); }}>
        <label htmlFor={searchId} className="block text-xs font-semibold leading-4 text-slate-600">Từ khóa</label>
        <div className="flex items-center gap-3">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input id={searchId} className="block h-10 w-full rounded-md border border-line bg-white pl-9 pr-3 text-sm outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100" value={searchDraft} onChange={(event) => onSearchDraft(event.target.value)} placeholder="Tên việc, công trình..." />
          </div>
          <button className="focus-ring inline-flex h-10 w-[88px] shrink-0 items-center justify-center rounded-md bg-blue-600 px-3 text-sm font-semibold text-white hover:bg-blue-700" type="submit">Tìm</button>
        </div>
      </form>

      <div className={clsx('my-work-filter-fields grid items-start gap-3', accounting && 'my-work-filter-fields--accounting')}>
        {accounting ? (
          <div className="min-w-0 leading-4">
            <AsyncProjectPicker value={project} onChange={(value) => onChange('project', value)} endpoint="/project-finance/projects" queryKey="my-accounting-work-project-options" emptyLabel="Tất cả công trình" />
          </div>
        ) : <FilterSelect fieldLabel="Công trình" value={project} label="Tất cả công trình" options={projects} onChange={(value) => onChange('project', value)} />}
        {accounting ? <FilterSelect fieldLabel="Loại hồ sơ" value={entryKind} label="Tất cả loại hồ sơ" options={[{ value: 'work_item', label: 'Công việc quy trình' }, { value: 'accounting_handoff', label: 'Hồ sơ kế toán' }]} onChange={(value) => onChange('entry_kind', value)} /> : null}
        <button
          type="button"
          aria-expanded={advancedOpen}
          aria-controls={advancedId}
          className={clsx('my-work-filter-toggle focus-ring inline-flex h-10 items-center justify-center gap-2 whitespace-nowrap rounded-md border px-3 text-sm font-semibold', advancedOpen ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-blue-300 text-blue-700 hover:bg-blue-50')}
          onClick={onToggleAdvanced}
        ><SlidersHorizontal className="h-4 w-4 shrink-0" />Bộ lọc thêm</button>
      </div>

      {advancedOpen ? (
        <div id={advancedId} className="my-work-filter-advanced grid gap-3 border-t border-line pt-4">
          <FilterSelect fieldLabel="Trạng thái" value={status} label="Tất cả trạng thái" options={statuses.length ? statuses.map((item) => ({ ...item, label: statusLabel(item.value) })) : statusOptions} onChange={(value) => onChange('status', value)} />
          {entryKind !== 'accounting_handoff' ? <>
            <FilterSelect fieldLabel="Hạn việc" value={deadline} label="Tất cả hạn việc" options={deadlineOptions} onChange={(value) => onChange('deadline', value)} />
            <FilterSelect fieldLabel="Loại việc" value={workType} label="Tất cả loại việc" options={workTypes.map((item) => ({ ...item, label: workTypeLabel(item.value) }))} onChange={(value) => onChange('type', value)} />
            <FilterSelect fieldLabel="Ưu tiên" value={priority} label="Tất cả ưu tiên" options={priorities.map((item) => ({ ...item, label: priorityLabel(item.value) }))} onChange={(value) => onChange('priority', value)} />
          </> : null}
          <div className="col-span-full min-w-0">
            <FilterSelect fieldLabel="Sắp xếp" value={sort} label="Sắp xếp" options={sortOptions} onChange={(value) => onChange('sort', value)} />
          </div>
        </div>
      ) : null}
    </div>
  );
}

type MobileFilterValues = {
  project: string;
  status: string;
  deadline: string;
  workType: string;
  priority: string;
  sort: string;
  entryKind: WorkQueueKind;
};

function MobileFilterBar({ searchDraft, project, status, deadline, workType, priority, sort, entryKind, facets, accounting, onSearchDraft, onSearch, onChange, onApply }: MobileFilterValues & {
  searchDraft: string;
  facets: MyWorkFacets;
  accounting: boolean;
  onSearchDraft: (value: string) => void;
  onSearch: () => void;
  onChange: (key: string, value: string) => void;
  onApply: (values: MobileFilterValues) => void;
}) {
  const searchId = useId();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<MobileFilterValues>({ project, status, deadline, workType, priority, sort, entryKind });
  const projects = normalizeFacetOptions(facets.projects, 'Công trình');
  const statuses = normalizeFacetOptions(facets.statuses, 'Trạng thái');
  const workTypes = normalizeFacetOptions(facets.work_types, 'Loại việc');
  const priorities = normalizeFacetOptions(facets.priorities, 'Ưu tiên');
  const applied = [
    project && { key: 'project', label: `Công trình: ${projects.find((item) => item.value === project)?.label ?? 'Đã chọn'}` },
    entryKind && { key: 'entry_kind', label: entryKind === 'accounting_handoff' ? 'Hồ sơ kế toán' : 'Công việc quy trình' },
    status && { key: 'status', label: `Trạng thái: ${statusLabel(status)}` },
    deadline && { key: 'deadline', label: `Hạn việc: ${deadlineOptions.find((item) => item.value === deadline)?.label ?? deadline}` },
    workType && { key: 'type', label: `Loại việc: ${workTypeLabel(workType)}` },
    priority && { key: 'priority', label: `Ưu tiên: ${priorityLabel(priority)}` },
    sort !== 'priority' && { key: 'sort', label: `Sắp xếp: ${sortOptions.find((item) => item.value === sort)?.label ?? sort}` },
  ].filter((item): item is { key: string; label: string } => Boolean(item));
  const setField = (key: keyof MobileFilterValues, value: string) => setDraft((current) => ({
    ...current,
    [key]: value,
    ...(key === 'entryKind' ? { status: '', deadline: '', workType: '', priority: '' } : {}),
  }));

  return (
    <div className="mb-3 space-y-2 md:hidden">
      <div className="flex min-w-0 gap-2">
        <form className="min-w-0 flex-1" onSubmit={(event) => { event.preventDefault(); onSearch(); }}>
          <label className="sr-only" htmlFor={searchId}>Tìm công việc</label>
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input id={searchId} type="search" className="h-11 w-full min-w-0 rounded-md border border-line bg-white pl-9 pr-11 text-sm outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100" value={searchDraft} onChange={(event) => onSearchDraft(event.target.value)} placeholder="Tìm tên việc, công trình…" />
            <button type="submit" className="absolute inset-y-0 right-0 grid w-11 place-items-center rounded-r-md text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500" aria-label="Tìm công việc"><Search className="h-4 w-4" /></button>
          </div>
        </form>
        <button type="button" className="h-11 shrink-0 rounded-md border border-blue-300 bg-white px-3 text-sm font-semibold text-blue-700" onClick={() => { setDraft({ project, status, deadline, workType, priority, sort, entryKind }); setOpen(true); }}>Lọc {applied.length}</button>
      </div>
      {applied.length ? <div className="flex flex-wrap gap-1.5" aria-label="Bộ lọc đang áp dụng">
        {applied.map((item) => <button key={item.key} type="button" className="inline-flex max-w-full items-center gap-1 rounded-md bg-blue-50 px-2 py-1 text-xs font-medium text-blue-800" aria-label={`Bỏ lọc ${item.label}`} onClick={() => onChange(item.key, item.key === 'sort' ? 'priority' : '')}><span className="truncate">{item.label}</span><X className="h-3.5 w-3.5 shrink-0" /></button>)}
      </div> : null}
      <Dialog open={open} title="Lọc công việc" size="sm" mobileFullscreen manageFocus onClose={() => setOpen(false)} footer={<div className="flex items-center justify-between gap-3"><button type="button" className="min-h-11 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700" onClick={() => setDraft({ project: '', status: '', deadline: '', workType: '', priority: '', sort: 'priority', entryKind: '' })}>Xóa lọc</button><button type="button" className="min-h-11 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white" onClick={() => { onApply(draft); setOpen(false); }}>Áp dụng</button></div>}>
        <div className="grid gap-3 text-sm">
          {accounting ? <div><span className="mb-1 block text-xs font-semibold text-slate-600">Công trình</span><AsyncProjectPicker value={draft.project} onChange={(value) => setField('project', value)} endpoint="/project-finance/projects" queryKey="my-accounting-work-project-options" emptyLabel="Tất cả công trình" /></div>
            : <FilterSelect fieldLabel="Công trình" value={draft.project} label="Tất cả công trình" options={projects} onChange={(value) => setField('project', value)} />}
          {accounting ? <FilterSelect fieldLabel="Loại hồ sơ" value={draft.entryKind} label="Tất cả loại hồ sơ" options={[{ value: 'work_item', label: 'Công việc quy trình' }, { value: 'accounting_handoff', label: 'Hồ sơ kế toán' }]} onChange={(value) => setField('entryKind', value)} /> : null}
          <FilterSelect fieldLabel="Trạng thái" value={draft.status} label="Tất cả trạng thái" options={statuses.length ? statuses.map((item) => ({ ...item, label: statusLabel(item.value) })) : statusOptions} onChange={(value) => setField('status', value)} />
          {draft.entryKind !== 'accounting_handoff' ? <>
            <FilterSelect fieldLabel="Hạn việc" value={draft.deadline} label="Tất cả hạn việc" options={deadlineOptions} onChange={(value) => setField('deadline', value)} />
            <FilterSelect fieldLabel="Loại việc" value={draft.workType} label="Tất cả loại việc" options={workTypes.map((item) => ({ ...item, label: workTypeLabel(item.value) }))} onChange={(value) => setField('workType', value)} />
            <FilterSelect fieldLabel="Ưu tiên" value={draft.priority} label="Tất cả ưu tiên" options={priorities.map((item) => ({ ...item, label: priorityLabel(item.value) }))} onChange={(value) => setField('priority', value)} />
          </> : null}
          <FilterSelect fieldLabel="Sắp xếp" value={draft.sort} label="Sắp xếp" options={sortOptions} onChange={(value) => setField('sort', value)} />
        </div>
      </Dialog>
    </div>
  );
}

function normalizeFacetOptions(values: FacetOption[] | undefined, fallback: string) {
  return asArray<FacetOption>(values).map((item) => {
    if (typeof item === 'string') return { value: item, label: item };
    const value = item.value ?? item.id ?? item.project_id ?? '';
    return { value, label: item.label ?? ([item.code ?? item.project_code, item.name ?? item.project_name].filter(Boolean).join(' · ') || fallback) };
  }).filter((item) => item.value);
}

function mergeApprovalProjectOptions(base: Array<{ value: string; label: string }>, nodeRows: ApprovalRequest[], reportRows: WorkReportListItem[]) {
  const options = new Map(base.map((option) => [option.value, option]));
  nodeRows.forEach((request) => {
    if (!request.project_id || options.has(request.project_id)) return;
    options.set(request.project_id, { value: request.project_id, label: request.project_name || 'Công trình' });
  });
  reportRows.forEach((report) => {
    if (!report.project_id || options.has(report.project_id)) return;
    options.set(report.project_id, { value: report.project_id, label: [report.project_code, report.project_name].filter(Boolean).join(' · ') || 'Công trình' });
  });
  return Array.from(options.values());
}

function FilterSelect({ value, label, fieldLabel, options, onChange }: { value: string; label: string; fieldLabel?: string; options: Array<{ value: string; label: string }>; onChange: (value: string) => void }) {
  const control = <select className={clsx('min-w-0 rounded-md border border-line bg-white px-3 text-sm font-medium text-slate-700', fieldLabel ? 'block h-11 w-full outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100 md:h-10' : 'py-2')} value={value} onChange={(event) => onChange(event.target.value)}><option value="">{label}</option>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>;
  return fieldLabel ? (
    <label className="block min-w-0">
      <span className="mb-1 block text-xs font-semibold leading-4 text-slate-600">{fieldLabel}</span>
      {control}
    </label>
  ) : control;
}

function WorkList({ rows, entries, prioritizeAccounting = false, selectedAccountingId, onSelectAccounting, onAcknowledgeAccounting, selectedId, startupActions = [], selectedStartupActionId, startupActionsLoading = false, startupActionsError, workflowActions = [], selectedWorkflowActionId, workflowActionsLoading = false, workflowActionsError, pendingWorkflowActionId, loading, fetchingMore, error, total, hasMore, filtered, emptyTitle, onSelect, onSelectStartupAction, onOpenStartupAction, onSelectWorkflowAction, onRunWorkflowAction, onOpen, onLoadMore, onReset, onRetry, onRetryStartupActions, onRetryWorkflowActions }: {
  entries?: WorkQueueEntry[]; selectedAccountingId?: string;
  prioritizeAccounting?: boolean;
  onSelectAccounting?: (item: AccountingWorkItem) => void; onAcknowledgeAccounting?: (item: AccountingWorkItem) => void;
  rows: WorkItem[]; selectedId?: string; loading: boolean; fetchingMore: boolean; error: unknown; total: number; hasMore: boolean; filtered: boolean;
  emptyTitle?: string;
  workflowActions?: MyWorkflowAction[]; selectedWorkflowActionId?: string; workflowActionsLoading?: boolean; workflowActionsError?: unknown; pendingWorkflowActionId?: string;
  startupActions?: ProjectStartupAction[]; selectedStartupActionId?: string; startupActionsLoading?: boolean; startupActionsError?: unknown;
  onSelect: (id: string) => void; onOpen: (item: WorkItem) => void; onLoadMore: () => void; onReset: () => void; onRetry: () => void;
  onSelectWorkflowAction?: (id: string) => void; onRunWorkflowAction?: (action: MyWorkflowAction) => void; onRetryWorkflowActions?: () => void;
  onSelectStartupAction?: (id: string) => void; onOpenStartupAction?: (action: ProjectStartupAction) => void; onRetryStartupActions?: () => void;
}) {
  const visibleEntries: WorkQueueEntry[] = entries ?? rows.map((item) => ({ kind: 'work_item', item }));
  const orderedEntries = orderedWorkListEntries(visibleEntries, startupActions, workflowActions, prioritizeAccounting);
  const displayedCount = orderedEntries.length;
  const hasEntries = displayedCount > 0;
  if ((loading || workflowActionsLoading || startupActionsLoading) && !hasEntries) return <WorkListSkeleton />;
  if (!hasEntries && error && workflowActionsError) return <div className="space-y-2"><SourceError title="Không tải được danh sách công việc" onRetry={onRetry} /><SourceError title="Không tải được hành động quy trình" onRetry={onRetryWorkflowActions} /></div>;
  if (!hasEntries && error) return <SourceError title="Không tải được danh sách công việc" onRetry={onRetry} />;
  if (!hasEntries && workflowActionsError) return <SourceError title="Không tải được hành động quy trình" onRetry={onRetryWorkflowActions} />;
  if (!hasEntries && startupActionsError) return <SourceError title="Không tải được điều phối khởi tạo công trình" onRetry={onRetryStartupActions} />;
  if (!hasEntries) return <EmptyState title={emptyTitle ?? (filtered ? 'Không có công việc khớp bộ lọc' : 'Không có việc cần xử lý trong phạm vi này')} action={filtered ? <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700" onClick={onReset}>Xóa bộ lọc</button> : undefined} />;
  return (
    <div className="min-w-0">
      <div className="mb-2 flex items-center justify-between gap-3"><p className="text-sm font-semibold text-ink">{total} mục</p></div>
      {error ? <div className="mb-2 flex items-center justify-between gap-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800"><span className="font-semibold">Chưa cập nhật được dữ liệu mới; đang giữ danh sách gần nhất.</span><button className="shrink-0 font-semibold text-amber-900 underline underline-offset-2" onClick={onRetry}>Thử lại</button></div> : null}
      {workflowActionsError ? <div className="mb-2 flex items-center justify-between gap-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800"><span className="font-semibold">Không tải được hành động chuyển giai đoạn.</span>{onRetryWorkflowActions ? <button className="shrink-0 font-semibold text-amber-900 underline underline-offset-2" onClick={onRetryWorkflowActions}>Thử lại</button> : null}</div> : null}
      {startupActionsError ? <div className="mb-2 flex items-center justify-between gap-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800"><span className="font-semibold">Không tải được điều phối khởi tạo công trình.</span>{onRetryStartupActions ? <button className="shrink-0 font-semibold text-amber-900 underline underline-offset-2" onClick={onRetryStartupActions}>Thử lại</button> : null}</div> : null}
      <div className="rounded-lg border border-line bg-white md:max-h-[720px] md:overflow-y-auto">
        {orderedEntries.map((entry) => {
          const key = workQueueEntryKey(entry);
          if (entry.kind === 'project_startup') return <ProjectStartupActionCard key={key} action={entry.item} selected={selectedStartupActionId === entry.item.id} onSelect={() => onSelectStartupAction?.(entry.item.id)} onOpen={() => onOpenStartupAction?.(entry.item)} />;
          if (entry.kind === 'workflow_action') return <WorkflowActionCard key={key} action={entry.item} selected={selectedWorkflowActionId === entry.item.id} pending={pendingWorkflowActionId === entry.item.id} onSelect={() => onSelectWorkflowAction?.(entry.item.id)} onRun={() => onRunWorkflowAction?.(entry.item)} />;
          if (entry.kind === 'accounting_handoff') return <AccountingWorkCard key={key} item={entry.item} selected={selectedAccountingId === entry.item.handoff_id} onSelect={() => onSelectAccounting?.(entry.item)} onAcknowledge={entry.item.status === 'handed_off' && onAcknowledgeAccounting ? () => onAcknowledgeAccounting(entry.item) : undefined} />;
          return <WorkItemCard key={key} item={entry.item} selected={selectedId === entry.item.id} showMobileActionLabel onSelect={() => {
            if (window.matchMedia('(max-width: 767px)').matches) onOpen(entry.item);
            else onSelect(entry.item.id);
          }} onOpen={() => onOpen(entry.item)} />;
        })}
        {startupActionsLoading ? <WorkflowActionSkeleton /> : null}
        {workflowActionsLoading ? <WorkflowActionSkeleton /> : null}
      </div>
      {hasMore ? <button className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" disabled={fetchingMore} onClick={onLoadMore}>{fetchingMore ? <RefreshCw className="h-4 w-4 animate-spin" /> : null}Tải thêm · Đã hiển thị {displayedCount}/{total}</button> : <p className="mt-3 text-center text-xs text-slate-500">Đã hiển thị {displayedCount}/{total}</p>}
    </div>
  );
}

function ProjectStartupActionCard({ action, selected, onSelect, onOpen }: { action: ProjectStartupAction; selected: boolean; onSelect: () => void; onOpen: () => void }) {
  const upcoming = action.action_kind === 'upcoming';
  return (
    <article className={clsx('border-b border-line last:border-b-0 sm:flex sm:items-stretch', selected ? 'bg-amber-50/70 shadow-[inset_3px_0_0_0_#d97706]' : 'bg-white hover:bg-slate-50')}>
      <button type="button" className="min-w-0 flex-1 px-4 py-3 text-left" aria-pressed={selected} onClick={onSelect}>
        <div className="flex flex-wrap items-center gap-2">
          <span className={clsx('rounded-md px-2 py-0.5 text-xs font-semibold', upcoming ? 'bg-slate-100 text-slate-700' : 'bg-amber-100 text-amber-800')}>{upcoming ? 'Sắp tới · chỉ xem' : 'Cần thiết lập công trình'}</span>
          <span className="text-sm font-semibold text-ink">{action.project_code} · {action.project_name}</span>
        </div>
        <p className="mt-1 text-xs font-medium text-slate-600">Phụ trách: {action.owner_name || 'Chưa xác định'}</p>
        <p className="mt-1 line-clamp-2 text-xs text-slate-500">{upcoming ? 'Anh/chị thuộc nhóm vai trò của bước đầu; theo dõi trước khi quy trình bắt đầu.' : action.ready_to_start ? 'Đã đủ điều kiện để bắt đầu quy trình.' : `Còn ${action.blockers.length} điều kiện cần hoàn tất.`}</p>
      </button>
      <button type="button" className={clsx('mx-4 mb-3 inline-flex shrink-0 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold sm:my-3 sm:ml-0 sm:h-9 sm:self-center sm:py-0', upcoming ? 'border border-line bg-white text-blue-700 hover:bg-blue-50' : 'bg-blue-600 text-white hover:bg-blue-700')} onClick={() => { onSelect(); onOpen(); }}>
        <UserCog className="h-4 w-4" />{upcoming ? 'Xem thiết lập' : 'Tiếp tục thiết lập'}
      </button>
    </article>
  );
}

function WorkflowActionCard({ action, selected, pending, onSelect, onRun }: { action: MyWorkflowAction; selected: boolean; pending: boolean; onSelect: () => void; onRun: () => void }) {
  return (
    <article className={clsx('border-b border-line last:border-b-0 sm:flex sm:items-stretch', selected ? 'bg-blue-50/70 shadow-[inset_3px_0_0_0_#2563eb]' : 'bg-white hover:bg-slate-50')}>
      <button type="button" className="min-w-0 flex-1 px-4 py-3 text-left" aria-pressed={selected} onClick={onSelect}>
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-md bg-blue-100 px-2 py-0.5 text-xs font-semibold text-blue-800">Cần chốt giai đoạn</span>
          <span className="text-sm font-semibold text-ink">{action.title}</span>
        </div>
        <p className="mt-1 truncate text-xs font-medium text-slate-600" title={`${action.project_code} · ${action.project_name} › ${action.run_node_name}`}>{action.project_code} · {action.project_name} › {action.run_node_name}</p>
        <p className="mt-1 line-clamp-2 text-xs text-slate-500">Đã đủ điều kiện để tiếp tục quy trình.</p>
      </button>
      <button
        type="button"
        className="mx-4 mb-3 inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 sm:my-3 sm:ml-0 sm:h-9 sm:self-center sm:py-0"
        disabled={pending}
        onClick={() => { onSelect(); onRun(); }}
      >
        {pending ? <RefreshCw className="h-4 w-4 animate-spin" /> : <PlayCircle className="h-4 w-4" />}
        {pending ? 'Đang chuyển…' : action.action_label}
      </button>
    </article>
  );
}

function WorkflowActionSkeleton() {
  return <div className="animate-pulse border-b border-line bg-blue-50/40 px-4 py-3"><div className="h-5 w-44 rounded bg-blue-100" /><div className="mt-2 h-3 w-4/5 rounded bg-slate-100" /><div className="mt-2 h-8 w-40 rounded bg-blue-100 sm:ml-auto" /></div>;
}

function WorkListSkeleton() {
  return (
    <div className="min-w-0">
      <div className="mb-2 h-5 w-16 animate-pulse rounded bg-slate-200" />
      <div className="overflow-hidden rounded-lg border border-line bg-white" aria-label="Đang tải danh sách công việc">
        {[0, 1, 2].map((row) => <div key={row} className="animate-pulse border-b border-line px-4 py-3 last:border-b-0"><div className="flex gap-3"><div className="h-5 w-20 rounded bg-slate-200" /><div className="h-5 w-1/2 rounded bg-slate-200" /></div><div className="mt-2 h-3 w-4/5 rounded bg-slate-100" /><div className="mt-2 h-3 w-2/3 rounded bg-slate-100" /></div>)}
      </div>
    </div>
  );
}

function NextWorkPanel({ item, onOpen }: { item: WorkItem | null; onOpen: (item: WorkItem) => void }) {
  if (!item) return <aside className="hidden rounded-lg border border-dashed border-slate-300 bg-slate-50 p-4 xl:block"><p className="text-sm font-semibold text-ink">Việc tiếp theo</p><p className="mt-2 text-sm text-slate-600">Chọn một công việc để xem hành động cần thực hiện.</p></aside>;
  const context = workItemWorkflowContext(item);
  const upcoming = workItemIsUpcoming(item);
  const exceptionalPriority = item.priority && !['normal', 'low'].includes(item.priority);
  return (
    <aside className="hidden rounded-lg border border-line bg-white p-4 xl:sticky xl:top-4 xl:block xl:self-start">
      <div className="flex flex-wrap items-center gap-2"><StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />{upcoming ? <span className="text-xs font-semibold text-amber-700">Sắp tới · chỉ xem</span> : null}{exceptionalPriority ? <span className="text-xs font-semibold text-rose-700">{priorityLabel(item.priority)}</span> : null}</div>
      <p className="mt-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Việc tiếp theo</p>
      <h3 className="mt-1 text-base font-semibold text-ink">{item.title}</h3>
      <p className="mt-2 line-clamp-2 text-xs font-medium leading-5 text-slate-600" title={context.path}>{context.path}</p>
      <p className="mt-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Thực hiện tại</p>
      <p className="mt-1 text-sm font-semibold text-blue-800">{executionTargetWorkspaceLabel(item)}</p>
      {item.description ? <p className="mt-3 text-sm leading-6 text-slate-600">{item.description}</p> : null}
      {upcoming ? <p className="mt-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">Bước này chưa đến lượt. Có thể xem trước hồ sơ nhưng chưa thể bắt đầu hoặc cập nhật.</p> : item.blocked_reason ? <p className="mt-3 rounded-md bg-rose-50 p-3 text-sm text-rose-700">Điểm chặn: {item.blocked_reason}</p> : <p className="mt-3 text-sm leading-6 text-slate-600">{nextActionCopy(item)}</p>}
      <button className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => onOpen(item)}>{upcoming ? 'Xem trước công việc' : executionTargetActionLabel(item)} <ChevronRight className="h-4 w-4" /></button>
      {item.project_id ? <Link className="mt-2 inline-flex w-full justify-center rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to={`/projects/${item.project_id}`}>Mở công trình</Link> : null}
    </aside>
  );
}

function NextQueuePanel({ item, workflowAction, startupAction, pending, onOpen, onRunWorkflowAction, onOpenStartupAction }: { item: WorkItem | null; workflowAction: MyWorkflowAction | null; startupAction: ProjectStartupAction | null; pending: boolean; onOpen: (item: WorkItem) => void; onRunWorkflowAction: (action: MyWorkflowAction) => void; onOpenStartupAction: (action: ProjectStartupAction) => void }) {
  if (startupAction) return <ProjectStartupNextPanel action={startupAction} onOpen={() => onOpenStartupAction(startupAction)} />;
  if (!workflowAction) return <NextWorkPanel item={item} onOpen={onOpen} />;
  return (
    <aside className="hidden rounded-lg border border-blue-200 bg-white p-4 xl:sticky xl:top-4 xl:block xl:self-start">
      <span className="rounded-md bg-blue-100 px-2 py-1 text-xs font-semibold text-blue-800">Cần chốt giai đoạn</span>
      <p className="mt-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Việc tiếp theo</p>
      <h3 className="mt-1 text-base font-semibold text-ink">{workflowAction.title}</h3>
      <p className="mt-2 text-xs font-medium leading-5 text-slate-600">{workflowAction.project_code} · {workflowAction.project_name} › {workflowAction.run_node_name}</p>
      <p className="mt-3 text-sm leading-6 text-slate-600">{workflowAction.description}</p>
      <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800">Các công việc và điều kiện bắt buộc của giai đoạn hiện tại đã hoàn tất.</div>
      <button className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={pending} onClick={() => onRunWorkflowAction(workflowAction)}>
        {pending ? <RefreshCw className="h-4 w-4 animate-spin" /> : <PlayCircle className="h-4 w-4" />}
        {pending ? 'Đang chuyển…' : workflowAction.action_label}
      </button>
      <Link className="mt-2 inline-flex w-full justify-center rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to={`/projects/${workflowAction.project_id}`}>Mở công trình</Link>
    </aside>
  );
}

function ProjectStartupNextPanel({ action, onOpen }: { action: ProjectStartupAction; onOpen: () => void }) {
  const upcoming = action.action_kind === 'upcoming';
  return (
    <aside className="hidden rounded-lg border border-amber-200 bg-white p-4 xl:sticky xl:top-4 xl:block xl:self-start">
      <span className={clsx('rounded-md px-2 py-1 text-xs font-semibold', upcoming ? 'bg-slate-100 text-slate-700' : 'bg-amber-100 text-amber-800')}>{upcoming ? 'Sắp tới · chỉ xem' : 'Cần thiết lập'}</span>
      <p className="mt-3 text-xs font-semibold uppercase tracking-wide text-slate-500">Việc tiếp theo</p>
      <h3 className="mt-1 text-base font-semibold text-ink">{action.project_code} · {action.project_name}</h3>
      <p className="mt-2 text-sm leading-6 text-slate-600">{upcoming ? `Người phụ trách thiết lập hiện tại: ${action.owner_name || 'Chưa xác định'}.` : action.ready_to_start ? 'Thiết lập đã đầy đủ; hãy kiểm tra lần cuối và bắt đầu quy trình.' : `Hoàn tất ${action.blockers.length} điều kiện còn thiếu trước khi bắt đầu quy trình.`}</p>
      <button className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={onOpen}><UserCog className="h-4 w-4" />{upcoming ? 'Xem thiết lập' : 'Tiếp tục thiết lập'}</button>
      <Link className="mt-2 inline-flex w-full justify-center rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to={`/projects/${action.project_id}`}>Mở công trình</Link>
    </aside>
  );
}

function ActivityAttentionPanel({ rows, loading, error, selectedQueue, streamStatus, returnTo, markingId, onSelectQueue, onMarkRead, onRetry }: {
  rows: ActivityAttentionItem[]; loading: boolean; error: unknown; selectedQueue: AttentionQueueKey; streamStatus: StreamStatus; returnTo: string; markingId?: string;
  onSelectQueue: (queue: AttentionQueueKey) => void; onMarkRead: (activityId: string) => void; onRetry: () => void;
}) {
  const counts = useMemo(() => attentionQueues.reduce<Record<AttentionQueueKey, number>>((acc, item) => { acc[item.key] = rows.filter((row) => inAttentionQueue(row, item.key)).length; return acc; }, { all: 0, mentions: 0, questions: 0, blockers: 0, reminders: 0, unread: 0 }), [rows]);
  const visibleRows = rows.filter((item) => inAttentionQueue(item, selectedQueue));
  return (
    <section className="mt-4 rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-col gap-3 border-b border-line pb-4 md:flex-row md:items-center md:justify-between"><div><h2 className="font-semibold text-ink">Trao đổi cần phản hồi</h2><p className="mt-1 text-sm text-slate-500">Nhắc đến, câu hỏi, điểm chặn và nhắc hạn</p></div><LiveStatusBadge status={streamStatus} /></div>
      <div className="my-3 flex gap-2 overflow-x-auto">{attentionQueues.map((item) => { const Icon = item.icon; return <button key={item.key} className={clsx('inline-flex min-w-fit items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold', selectedQueue === item.key ? 'border-blue-200 bg-blue-50 text-blue-800' : 'border-line text-slate-700 hover:bg-slate-50')} onClick={() => onSelectQueue(item.key)}><Icon className="h-4 w-4" />{item.label}<span className="rounded-md bg-white px-2 py-0.5 text-xs text-slate-600">{counts[item.key]}</span></button>; })}</div>
      {loading ? <AttentionListSkeleton /> : null}
      {error ? <SourceError title="Không tải được trao đổi cần phản hồi" onRetry={onRetry} /> : null}
      {!loading && !error && !visibleRows.length ? <EmptyState title="Không có trao đổi cần phản hồi trong nhóm này" /> : null}
      {visibleRows.length ? <ul aria-label="Danh sách trao đổi cần phản hồi" className="divide-y divide-line overflow-hidden rounded-md border border-line bg-white">{visibleRows.map((item) => <AttentionListItem key={item.id} item={item} returnTo={returnTo} marking={markingId === item.id} onMarkRead={onMarkRead} />)}</ul> : null}
    </section>
  );
}

function AttentionListSkeleton() {
  return (
    <div className="divide-y divide-line overflow-hidden rounded-md border border-line bg-white" role="status" aria-label="Đang tải trao đổi cần phản hồi">
      {[0, 1, 2].map((item) => (
        <div key={item} className="animate-pulse px-3 py-4 sm:px-4">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 flex-1">
              <div className="h-5 w-32 rounded bg-slate-200" />
              <div className="mt-3 h-4 w-48 max-w-full rounded bg-slate-200" />
              <div className="mt-2 h-3 w-80 max-w-full rounded bg-slate-100" />
              <div className="mt-3 h-4 w-full max-w-xl rounded bg-slate-100" />
            </div>
            <div className="h-7 w-16 shrink-0 rounded bg-slate-100" />
          </div>
        </div>
      ))}
      <span className="sr-only">Đang tải trao đổi cần phản hồi...</span>
    </div>
  );
}

function AttentionListItem({ item, returnTo, marking, onMarkRead }: { item: ActivityAttentionItem; returnTo: string; marking: boolean; onMarkRead: (id: string) => void }) {
  const target = item.work_item_id ? workItemDetailPath(item.work_item_id, returnTo, item.project_id) : item.project_id ? `/projects/${item.project_id}` : '';
  return (
    <li className={clsx('px-3 py-4 transition-colors sm:px-4', item.is_unread ? 'bg-blue-50/70' : 'bg-white hover:bg-slate-50/70')}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><div className="min-w-0 flex-1"><div className="flex flex-wrap gap-1">{item.is_unread ? <span className="rounded-md bg-blue-600 px-2 py-0.5 text-xs font-semibold text-white">Chưa đọc</span> : null}{item.mentioned_me ? <span className="rounded-md bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-700">@tôi</span> : null}{asArray<string>(item.queue_types).slice(0, 3).map((queue) => <span key={queue} className="rounded-md bg-white px-2 py-0.5 text-xs font-semibold text-slate-600">{attentionQueueLabel(queue)}</span>)}</div><p className="mt-2 text-sm font-semibold text-ink">{activityLabel(item.activity_type)}</p><p className="mt-1 text-xs text-slate-500">{identityReferenceText(item.actor_name, item.actor_account_deleted, 'Hệ thống')} · {item.target_label ?? item.work_item_title ?? (item.scope_type ? scopeTypeLabel(item.scope_type) : 'Công trình')} · {compactDateTime(item.created_at)}</p></div>{item.is_unread ? <button className="self-start shrink-0 rounded-md border border-blue-200 bg-white px-2 py-1 text-xs font-semibold text-blue-700 disabled:opacity-60" disabled={marking} onClick={() => onMarkRead(item.id)}>Đã đọc</button> : null}</div>
      <ActivityReplyQuote replyTo={item.reply_to} />
      {item.comment ? <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-700">{item.comment}</p> : null}
      <ActivityAttachments attachments={item.attachments} compact />
      {item.reactions?.length ? <div className="mt-2"><ActivityReactionBar reactions={item.reactions} /></div> : null}
      {target ? <Link to={target} className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-blue-700 hover:text-blue-800">{item.work_item_id ? 'Mở đúng công việc' : 'Mở công trình'} <ChevronRight className="h-3.5 w-3.5" /></Link> : null}
    </li>
  );
}

function attentionQueueLabel(queue: string) { return attentionQueues.find((item) => item.key === queue)?.label ?? queue; }
function inAttentionQueue(item: ActivityAttentionItem, queue: AttentionQueueKey) { if (queue === 'all') return true; if (queue === 'unread') return Boolean(item.is_unread); return asArray<string>(item.queue_types).includes(queue); }

function LiveStatusBadge({ status }: { status: StreamStatus }) {
  const label = status === 'connected' ? 'Đang trực tuyến' : status === 'reconnecting' ? 'Đang kết nối lại' : 'Mất kết nối · Làm mới thủ công';
  return <span aria-label={label} title={label} className={clsx('inline-flex h-10 items-center gap-2 rounded-md px-2.5 text-xs font-semibold md:h-auto md:py-1.5', status === 'connected' ? 'bg-emerald-50 text-emerald-700' : status === 'reconnecting' ? 'bg-amber-50 text-amber-700' : 'bg-slate-100 text-slate-600')}><span className={clsx('h-2 w-2 rounded-full', status === 'connected' ? 'bg-emerald-500' : status === 'reconnecting' ? 'bg-amber-500' : 'bg-slate-400')} /><span className="hidden md:inline">{label}</span></span>;
}

function SourceError({ title, onRetry }: { title: string; onRetry?: () => void }) { return <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700"><p className="font-semibold">{title}</p>{onRetry ? <button className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 font-semibold" onClick={onRetry}>Thử lại</button> : null}</div>; }
