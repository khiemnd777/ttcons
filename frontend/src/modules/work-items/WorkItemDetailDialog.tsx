import { ReactNode, useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  Circle,
  Clock3,
  FileCheck2,
  Loader2,
  MessageSquare,
  Save,
  ShieldCheck,
  Users,
} from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { get, post } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import { useToast } from '../../components/ToastProvider';
import type { ActivityPayload, EvidenceRecord, ProjectActivityOptions, WorkItem, WorkItemActivity } from '../../lib/types/domain';
import { ActivityAttachments } from './ActivityAttachments';
import { ActivityReactionBar, ActivityReplyQuote } from './ActivityConversationMeta';
import { ActivityComposer, type ActivityAttachmentOption, type ActivityMentionOption } from './ActivityComposer';
import { EvidencePanel } from './EvidencePanel';
import {
  WorkItemPrimarySurface,
  primarySurfaceUsesEvidence,
  primarySurfaceUsesWorkflowForm,
  resolveWorkItemPrimarySurface,
  type WorkItemPrimarySurfaceKind,
} from './WorkItemPrimarySurface';
import {
  WorkflowFormPanel,
  type WorkflowFormActionHandle,
  type WorkflowFormActionState,
} from './WorkflowFormPanel';
import {
  completionRequirementPresentation,
  missingCompletionRequirementLabels,
  permissionRequirementPresentation,
  type WorkItemRequirementNavigationRequest,
  type WorkItemRequirementNavigationTarget,
} from './workItemRequirements';
import { firstFocusableRequirementControl, revealWorkItemRequirementTarget } from './workItemRequirementNavigation';
import {
  actionDescriptions,
  actionLabels,
  actionRequiresReason,
  activityLabel,
  asArray,
  availableActions,
  compactDateTime,
  departmentLabel,
  hasWorkflowFormSummary,
  isWorkOverdue,
  numberValue,
  priorityLabel,
  visibilityLabel,
  workItemIsUpcoming,
  workItemWorkflowContext,
  workTypeLabel,
  workflowFormChipText,
  workflowFormActivityComment,
  workflowFormNextAction,
  workflowFormStatusLabel,
  type WorkItemActionKey,
} from './workItemPresentation';

export function WorkItemDetailDialog({
  open,
  item,
  onClose,
  onChanged,
}: {
  open: boolean;
  item: WorkItem | null;
  onClose: () => void;
  onChanged?: (item: WorkItem) => void;
}) {
  const workContextLabel = item ? workItemWorkflowContext(item).path : undefined;

  return (
    <Dialog
      open={open}
      title={item?.title ?? 'Chi tiết đầu việc'}
      description={item ? `${workContextLabel} · ${departmentLabel(item.department)}` : undefined}
      onClose={onClose}
      size="workspace"
    >
      <WorkItemDetailWorkspace item={item} onClose={onClose} onChanged={onChanged} />
    </Dialog>
  );
}

export function WorkItemDetailWorkspace({
  item,
  workItemId,
  onClose,
  onChanged,
}: {
  item?: WorkItem | null;
  workItemId?: string;
  onClose?: () => void;
  onChanged?: (item: WorkItem) => void;
}) {
  const [actionText, setActionText] = useState('');
  const [pendingAction, setPendingAction] = useState<WorkItemActionKey | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const selectedWorkId = workItemId ?? item?.id ?? '';
  const navigationSequence = useRef(0);
  const workflowFormActionRef = useRef<WorkflowFormActionHandle | null>(null);
  const [navigationRequest, setNavigationRequest] = useState<WorkItemRequirementNavigationRequest | null>(null);
  const [workflowFormActionState, setWorkflowFormActionState] = useState<WorkflowFormActionState | null>(null);
  const workDetail = useQuery({
    queryKey: ['work-item', selectedWorkId],
    queryFn: () => get<WorkItem>(`/work-items/${selectedWorkId}`),
    enabled: Boolean(selectedWorkId),
    retry: false,
  });
  const detailItem = workDetail.data ?? item;
  const activityOptions = useQuery({
    queryKey: ['project-activity-options', detailItem?.project_id],
    queryFn: () => get<ProjectActivityOptions>(`/projects/${detailItem?.project_id}/activity/options`),
    enabled: Boolean(detailItem?.project_id),
    retry: false,
  });
  const activityMentionOptions: ActivityMentionOption[] = [
    ...(activityOptions.data?.members ?? []).map((member) => ({
      type: 'staff' as const,
      id: member.id,
      label: member.label,
      description: [member.position, member.department].filter(Boolean).join(' · '),
    })),
    ...(activityOptions.data?.roles ?? []).map((role) => ({ type: 'role' as const, id: role.id, label: role.label })),
  ];

  const workAction = useMutation({
    mutationFn: ({ workItemId, action, comment }: { workItemId: string; action: WorkItemActionKey; comment: string }) =>
      post<WorkItem>(`/work-items/${workItemId}/${action}`, { comment, reason: comment }),
    onSuccess: (updated, variables) => {
      setActionText('');
      setPendingAction(null);
      queryClient.invalidateQueries({ queryKey: ['work-item', variables.workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      if (updated.project_id) {
        queryClient.invalidateQueries({ queryKey: ['project-operations', updated.project_id] });
      }
      onChanged?.(updated);
      toast.success(`Đã ${actionLabels[variables.action].toLowerCase()}`);
    },
    onError: (error) => toast.error('Không thao tác được đầu việc', error),
  });
  const activityAction = useMutation({
    mutationFn: ({ workItemId, payload }: { workItemId: string; payload: ActivityPayload }) =>
      post<WorkItemActivity>(`/work-items/${workItemId}/activity`, payload),
    onSuccess: (_activity, variables) => {
      queryClient.invalidateQueries({ queryKey: ['work-item', variables.workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      if (detailItem?.project_id) {
        queryClient.invalidateQueries({ queryKey: ['project-operations', detailItem.project_id] });
      }
      toast.success('Đã gửi hoạt động');
    },
    onError: (error) => toast.error('Không gửi được hoạt động', error),
  });

  function runAction(action: WorkItemActionKey) {
    if (!detailItem?.id || workItemIsUpcoming(detailItem)) return;
    const comment = actionText.trim();
    if (actionRequiresReason(action) && !comment) {
      setPendingAction(action);
      toast.warn({ title: 'Cần nhập lý do', description: actionDescriptions[action] });
      return;
    }
    workAction.mutate({ workItemId: detailItem.id, action, comment });
  }

  function closeDialog() {
    setActionText('');
    setPendingAction(null);
    onClose?.();
  }

  function navigateToRequirement(target: WorkItemRequirementNavigationTarget) {
    navigationSequence.current += 1;
    setNavigationRequest({ requestId: navigationSequence.current, target });
  }

  const handleWorkflowFormActionStateChange = useCallback((state: WorkflowFormActionState | null) => {
    setWorkflowFormActionState(state);
  }, []);

  useEffect(() => {
    setNavigationRequest(null);
    setWorkflowFormActionState(null);
  }, [selectedWorkId]);

  const actions = permittedActions(detailItem);
  const evidence = asArray<EvidenceRecord>(detailItem?.evidence);
  const activities = asArray<WorkItemActivity>(detailItem?.activities);
  const attachmentOptions = buildActivityAttachmentOptions(evidence);
  const futureReadOnly = detailItem ? workItemIsUpcoming(detailItem) : false;
  const resolvedPrimarySurface = detailItem ? resolveWorkItemPrimarySurface(detailItem) : null;
  const primarySurface = futureReadOnly && resolvedPrimarySurface?.kind !== 'workflow_form'
    ? null
    : resolvedPrimarySurface;
  const requirementRows = detailItem ? buildRequirementRows(detailItem, evidence, actions, futureReadOnly) : [];
  const incompleteRequirementCount = requirementRows.filter((row) => row.state === 'blocked' || row.state === 'warning').length;
  const nextAction = detailItem ? resolveNextAction(detailItem, actions, primarySurface?.kind) : '';
  const latestActivity = activities[0];
  const skippedHistory = detailItem?.status === 'cancelled';
  return (
    <>
      {workDetail.isLoading && !detailItem ? <LoadingSpinner /> : null}
      {workDetail.error && !detailItem ? <EmptyState title="Không tải được chi tiết đầu việc" /> : null}
      {detailItem ? (
        <div className="space-y-4">
          <WorkItemWorkspaceHeader item={detailItem} evidence={evidence} />

          {futureReadOnly ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              <p className="font-semibold">Công việc chưa đến lượt · chỉ xem</p>
              <p className="mt-1">Có thể xem trước ngữ cảnh và hồ sơ. Các thao tác bắt đầu, cập nhật, tải minh chứng, ký và gửi hồ sơ chỉ mở khi đến thời điểm kế hoạch và workflow chuyển đến node này.</p>
            </div>
          ) : null}

          <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_18rem]">
            <main className="min-w-0 space-y-4">
              {skippedHistory ? (
                <div className="rounded-lg border border-slate-200 bg-white p-4 text-sm text-slate-700 shadow-panel">
                  <p className="font-semibold text-ink">Hồ sơ lịch sử chỉ đọc</p>
                  <p className="mt-1">Work item thuộc nhánh không được lựa chọn. Nội dung được giữ lại để truy vết và không thể thao tác nghiệp vụ.</p>
                </div>
              ) : futureReadOnly && !primarySurface ? (
                <div className="rounded-lg border border-slate-200 bg-white p-4 text-sm text-slate-700 shadow-panel">
                  <p className="font-semibold text-ink">Nội dung xem trước</p>
                  <p className="mt-1">Điều kiện và hồ sơ được hiển thị để chuẩn bị; chưa cho phép ghi nhận nghiệp vụ.</p>
                  <div className="mt-3"><WorkItemRequirementPanel rows={requirementRows} onNavigate={navigateToRequirement} /></div>
                </div>
              ) : (
                <>
                  <WorkItemRequirementPanel rows={requirementRows} onNavigate={navigateToRequirement} />
                  {primarySurface ? (
                    <WorkItemPrimarySurface
                      item={detailItem}
                      evidence={evidence}
                      surface={primarySurface}
                      readOnly={futureReadOnly}
                      navigationRequest={navigationRequest}
                      workflowFormActionRef={workflowFormActionRef}
                      onWorkflowFormActionStateChange={handleWorkflowFormActionStateChange}
                    />
                  ) : null}
                </>
              )}

              {skippedHistory ? (
                <SkippedWorkItemHistory activities={activities} evidence={evidence} />
              ) : primarySurface || futureReadOnly ? (
                <WorkItemSupportingTabs
                  item={detailItem}
                  evidence={evidence}
                  activities={activities}
                  attachmentOptions={attachmentOptions}
                  mentionOptions={activityMentionOptions}
                  activitySubmitting={activityAction.isPending}
                  primarySurfaceKind={primarySurface?.kind}
                  navigationRequest={navigationRequest}
                  readOnly={futureReadOnly}
                  onSubmitActivity={(payload) => {
                    if (!detailItem.id) return Promise.resolve();
                    return activityAction.mutateAsync({ workItemId: detailItem.id, payload }).then(() => undefined);
                  }}
                />
              ) : null}
            </main>

            <WorkItemActionPanel
              item={detailItem}
              actions={actions}
              actionText={actionText}
              pendingAction={pendingAction}
              submitting={workAction.isPending}
              nextAction={nextAction}
              workflowFormActionState={workflowFormActionState}
              incompleteRequirementCount={incompleteRequirementCount}
              latestActivity={latestActivity}
              onActionTextChange={setActionText}
              onRunAction={runAction}
              onSaveWorkflowForm={() => workflowFormActionRef.current?.save()}
              onSubmitWorkflowForm={() => workflowFormActionRef.current?.submit()}
              onNavigateRequirement={navigateToRequirement}
              onClose={closeDialog}
              navigationRequest={navigationRequest}
            />
          </div>
        </div>
      ) : null}
    </>
  );
}

const decisionActions = new Set<WorkItemActionKey>(['approve', 'reject', 'request-change']);

function permittedActions(item?: WorkItem | null): WorkItemActionKey[] {
  if (!item || ['completed', 'cancelled'].includes(item.status)) return [];

  const statusActions = availableActions(item.status);
  const serverActions = asArray<string>(item.available_actions);
  if (Array.isArray(item.available_actions)) {
    const allowed = new Set(serverActions);
    return statusActions.filter((action) => allowed.has(action));
  }

  return statusActions.filter((action) => {
    if (decisionActions.has(action)) return (item.capabilities?.can_decide ?? item.can_decide) === true;
    return (item.capabilities?.can_act ?? item.can_act) === true;
  });
}

function SkippedWorkItemHistory({ activities, evidence }: { activities: WorkItemActivity[]; evidence: EvidenceRecord[] }) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Lịch sử workflow</p>
          <h3 className="mt-1 text-sm font-semibold text-ink">Lý do và dấu vết của bước đã bỏ qua</h3>
        </div>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{activities.length} hoạt động · {evidence.length} minh chứng</span>
      </div>
      {activities.length ? (
        <div className="mt-3 divide-y divide-line rounded-md border border-line">
          {activities.map((activity) => (
            <div key={activity.id} className="px-3 py-2.5">
              <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
                <span className="font-semibold text-slate-700">{activity.actor_name ?? 'Hệ thống'}</span>
                <span>{compactDateTime(activity.created_at)}</span>
              </div>
              <p className="mt-1 text-sm text-slate-700">{workflowFormActivityComment(activity) || statusLabel(activity.to_status ?? activity.activity_type ?? 'cancelled')}</p>
            </div>
          ))}
        </div>
      ) : (
        <p className="mt-3 rounded-md bg-slate-50 px-3 py-2 text-sm text-slate-600">Chưa có hoạt động lịch sử.</p>
      )}
    </section>
  );
}

type SupportTabKey = 'evidence' | 'activity' | 'form';
type RequirementState = 'done' | 'warning' | 'blocked' | 'neutral';

type RequirementRow = {
  id: string;
  label: string;
  description: string;
  guidance: string;
  actionLabel?: string;
  target?: WorkItemRequirementNavigationTarget;
  state: RequirementState;
};

function WorkItemWorkspaceHeader({
  item,
  evidence,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
}) {
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const context = workItemWorkflowContext(item);

  return (
    <section className="rounded-lg border border-line bg-white px-4 py-3 shadow-panel">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <p className="truncate text-xs font-semibold text-blue-700">{context.node} · {context.step}</p>
          <h3 className="mt-1 text-base font-semibold text-ink">{item.title}</h3>
          {item.description ? <p className="mt-1 line-clamp-2 text-sm leading-5 text-slate-600">{item.description}</p> : null}
        </div>
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />
          {item.project_code ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{item.project_code}</span> : null}
          {item.priority ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{priorityLabel(item.priority)}</span> : null}
        </div>
      </div>
      <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-3 text-xs font-semibold text-slate-600">
        <MetaPill icon={<Users className="h-3.5 w-3.5" />} label={item.assignee_name ?? firstRoleLabel(item) ?? departmentLabel(item.department)} />
        <MetaPill icon={<Clock3 className="h-3.5 w-3.5" />} label={dateText(item.due_at)} />
        <span className="rounded-md bg-slate-100 px-2 py-1">{activeEvidence.length}/{numberValue(item.evidence_count)} minh chứng</span>
        {hasWorkflowFormSummary(item) ? (
          <span className="max-w-full truncate rounded-md bg-emerald-50 px-2 py-1 text-emerald-700">
            {workflowFormChipText(item)} · {workflowFormStatusLabel(item.workflow_form_status)}
          </span>
        ) : null}
      </div>
    </section>
  );
}

function WorkItemRequirementPanel({
  rows,
  onNavigate,
}: {
  rows: RequirementRow[];
  onNavigate: (target: WorkItemRequirementNavigationTarget) => void;
}) {
  if (!rows.length) return null;
  const incompleteCount = rows.filter((row) => row.state === 'blocked' || row.state === 'warning').length;
  return (
    <details className={clsx('group rounded-md border bg-white shadow-panel', incompleteCount ? 'border-amber-200' : 'border-line')}>
      <summary className={clsx('flex cursor-pointer list-none items-center justify-between gap-3 rounded-md px-3 py-3', incompleteCount ? 'bg-amber-50 hover:bg-amber-100' : 'bg-slate-50 hover:bg-slate-100')}>
        <div className="flex min-w-0 items-center gap-2">
          {incompleteCount ? <AlertTriangle className="h-4 w-4 shrink-0 text-amber-700" /> : <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />}
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Điều kiện bắt buộc</p>
            <h3 className="mt-0.5 text-sm font-semibold text-ink">
              {incompleteCount ? `Còn ${incompleteCount} điều kiện cần xử lý` : 'Không có điều kiện đang chặn'}
            </h3>
          </div>
        </div>
        <span className="inline-flex shrink-0 items-center gap-2 text-xs font-semibold text-slate-600">
          Xem chi tiết
          <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
        </span>
      </summary>
      <div className="divide-y divide-line border-t border-line bg-white">
        {rows.map((row) => {
          const navigationTarget = row.target;
          const content = (
            <>
              <RequirementIcon state={row.state} />
              <div className="min-w-0 flex-1 text-left">
                <p className="text-sm font-semibold text-ink">{row.label}</p>
                <p className="mt-0.5 text-xs leading-5 text-slate-600">{row.description}</p>
                <p className="mt-1 text-xs leading-5 text-slate-700">
                  <span className="font-semibold">Cách xử lý:</span> {row.guidance}
                </p>
              </div>
              {navigationTarget && row.actionLabel ? (
                <span className="inline-flex shrink-0 items-center gap-1 self-center text-xs font-semibold text-blue-700">
                  {row.actionLabel}
                  <ArrowRight className="h-3.5 w-3.5" />
                </span>
              ) : null}
            </>
          );

          return navigationTarget ? (
            <button
              key={row.id}
              type="button"
              className="flex w-full items-start gap-3 px-3 py-2.5 transition hover:bg-blue-50 focus-visible:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500"
              onClick={() => onNavigate(navigationTarget)}
              aria-label={`${row.label}: ${row.actionLabel ?? 'Mở nội dung liên quan'}`}
            >
              {content}
            </button>
          ) : (
            <div key={row.id} className="flex items-start gap-3 px-3 py-2.5">
              {content}
            </div>
          );
        })}
      </div>
    </details>
  );
}

function WorkItemActionPanel({
  item,
  actions,
  actionText,
  pendingAction,
  submitting,
  nextAction,
  workflowFormActionState,
  incompleteRequirementCount,
  latestActivity,
  onActionTextChange,
  onRunAction,
  onSaveWorkflowForm,
  onSubmitWorkflowForm,
  onNavigateRequirement,
  onClose,
  navigationRequest,
}: {
  item: WorkItem;
  actions: WorkItemActionKey[];
  actionText: string;
  pendingAction: WorkItemActionKey | null;
  submitting: boolean;
  nextAction: string;
  workflowFormActionState: WorkflowFormActionState | null;
  incompleteRequirementCount: number;
  latestActivity?: WorkItemActivity;
  onActionTextChange: (value: string) => void;
  onRunAction: (action: WorkItemActionKey) => void;
  onSaveWorkflowForm: () => void;
  onSubmitWorkflowForm: () => void;
  onNavigateRequirement: (target: WorkItemRequirementNavigationTarget) => void;
  onClose: () => void;
  navigationRequest: WorkItemRequirementNavigationRequest | null;
}) {
  const workflowFormIsPrimary = Boolean(
    workflowFormActionState?.available
    && !workflowFormActionState.submitted
    && !workflowFormActionState.readOnly,
  );
  const formIssueCount = workflowFormActionState?.validationIssueLabels.length ?? 0;
  const primaryAction = workflowFormIsPrimary ? undefined : actions[0];
  const secondaryActions = workflowFormIsPrimary ? actions : actions.slice(1);
  const panelNextAction = workflowFormIsPrimary
    ? formIssueCount
      ? `Hoàn tất ${formIssueCount} thông tin bắt buộc`
      : 'Chốt hồ sơ nghiệp vụ'
    : nextAction;
  const panelRef = useRef<HTMLElement | null>(null);
  const [noteOpen, setNoteOpen] = useState(false);

  useEffect(() => {
    if (pendingAction && actionRequiresReason(pendingAction)) setNoteOpen(true);
  }, [pendingAction]);

  useEffect(() => {
    const shouldRevealActionPanel = navigationRequest?.target.kind === 'action_panel'
      || (
        navigationRequest?.target.kind === 'workflow_form'
        && workflowFormIsPrimary
        && formIssueCount === 0
      );
    if (!shouldRevealActionPanel) return undefined;
    const submitButton = panelRef.current?.querySelector<HTMLElement>('[data-workflow-form-submit]') ?? null;
    return revealWorkItemRequirementTarget(panelRef.current, submitButton ?? firstFocusableRequirementControl(panelRef.current));
  }, [formIssueCount, navigationRequest?.requestId, navigationRequest?.target.kind, workflowFormIsPrimary]);

  return (
    <aside ref={panelRef} tabIndex={-1} className="order-first scroll-mt-24 rounded-lg border border-line bg-white p-3 shadow-panel outline-none xl:order-none xl:sticky xl:top-[5.5rem] xl:self-start">
      <div className="border-b border-line pb-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Việc cần làm</p>
        <h3 className="mt-1 text-sm font-semibold leading-5 text-ink">{panelNextAction}</h3>
        {workflowFormIsPrimary && formIssueCount ? (
          <p className="mt-1 text-xs leading-5 text-amber-700">
            Thiếu: {workflowFormActionState?.validationIssueLabels.slice(0, 3).join(', ')}
            {formIssueCount > 3 ? ` +${formIssueCount - 3}` : ''}
          </p>
        ) : incompleteRequirementCount ? (
          <p className="mt-1 text-xs text-slate-500">{incompleteRequirementCount} điều kiện chưa hoàn tất</p>
        ) : null}
      </div>

      {workflowFormIsPrimary || actions.length ? (
        <div className="mt-3 space-y-2">
          {workflowFormIsPrimary && formIssueCount ? (
            <button
              type="button"
              className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-semibold text-amber-800 hover:bg-amber-100"
              onClick={() => onNavigateRequirement({ kind: 'workflow_form', focus: 'first_issue' })}
            >
              <ArrowRight className="h-4 w-4" />
              Đến thông tin còn thiếu
            </button>
          ) : null}

          {workflowFormIsPrimary ? (
            <>
              <button
                type="button"
                data-workflow-form-submit
                className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={workflowFormActionState?.saving || workflowFormActionState?.submitting}
                onClick={onSubmitWorkflowForm}
              >
                {workflowFormActionState?.submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileCheck2 className="h-4 w-4" />}
                {workflowFormActionState?.submitLabel ?? 'Chốt hồ sơ'}
              </button>
              <button
                type="button"
                className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={workflowFormActionState?.saving || workflowFormActionState?.submitting}
                onClick={onSaveWorkflowForm}
              >
                {workflowFormActionState?.saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                Lưu nháp
              </button>
            </>
          ) : primaryAction ? (
            <ActionButton action={primaryAction} primary submitting={submitting} onRunAction={onRunAction} />
          ) : null}

          <button
            type="button"
            className="w-full px-2 py-1 text-xs font-semibold text-slate-500 hover:text-blue-700"
            aria-expanded={noteOpen}
            onClick={() => setNoteOpen((current) => !current)}
          >
            {noteOpen ? 'Ẩn ghi chú' : actionText.trim() ? 'Sửa ghi chú' : 'Thêm ghi chú / lý do'}
          </button>
          {noteOpen ? (
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

          {secondaryActions.length ? (
            <details className="group rounded-md border border-line bg-slate-50">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100">
                <span>Thao tác khác</span>
                <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
              </summary>
              <div className="space-y-2 border-t border-line bg-white p-2">
                {secondaryActions.map((action) => (
                  <ActionButton key={action} action={action} submitting={submitting} onRunAction={onRunAction} />
                ))}
              </div>
            </details>
          ) : null}
        </div>
      ) : (
        <p className="mt-3 rounded-md bg-slate-50 p-3 text-sm text-slate-600">Không có thao tác trực tiếp cho trạng thái hiện tại.</p>
      )}

      <div className="mt-4 hidden space-y-3 border-t border-line pt-3 xl:block">
        <ActionFact icon={<ShieldCheck className="h-4 w-4" />} label="Trạng thái" value={<StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />} />
        <ActionFact icon={<Users className="h-4 w-4" />} label="Phụ trách" value={item.assignee_name ?? firstRoleLabel(item) ?? departmentLabel(item.department)} />
        <ActionFact icon={<Clock3 className="h-4 w-4" />} label="Hạn xử lý" value={dateText(item.due_at)} />
        {latestActivity ? (
          <ActionFact
            icon={<MessageSquare className="h-4 w-4" />}
            label="Hoạt động gần nhất"
            value={`${latestActivity.actor_name ?? 'Hệ thống'} · ${compactDateTime(latestActivity.created_at)}`}
          />
        ) : null}
      </div>

      <div className="mt-4 flex flex-col gap-2 border-t border-line pt-3">
        {item.project_id ? (
          <Link
            className="inline-flex items-center justify-center rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
            to={`/projects/${item.project_id}`}
          >
            Mở công trình
          </Link>
        ) : null}
        <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>
          Đóng
        </button>
      </div>
    </aside>
  );
}

function WorkItemSupportingTabs({
  item,
  evidence,
  activities,
  attachmentOptions,
  mentionOptions,
  activitySubmitting,
  primarySurfaceKind,
  navigationRequest,
  readOnly,
  onSubmitActivity,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  activities: WorkItemActivity[];
  attachmentOptions: ActivityAttachmentOption[];
  mentionOptions: ActivityMentionOption[];
  activitySubmitting: boolean;
  primarySurfaceKind?: WorkItemPrimarySurfaceKind;
  navigationRequest: WorkItemRequirementNavigationRequest | null;
  readOnly: boolean;
  onSubmitActivity: (payload: ActivityPayload) => Promise<void>;
}) {
  const [selectedTab, setSelectedTab] = useState<SupportTabKey>('evidence');
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const pendingEvidence = evidence.filter((record) => ['draft', 'submitted', 'pending'].includes(String(record.status)));
  const tabs: Array<{ id: SupportTabKey; label: string; count?: number; status?: string; description: string }> = [];
  if (readOnly || !primarySurfaceKind || !primarySurfaceUsesEvidence(primarySurfaceKind)) {
    tabs.push({
      id: 'evidence',
      label: 'Minh chứng',
      count: activeEvidence.length,
      description: pendingEvidence.length ? `${pendingEvidence.length} đang chờ` : 'Sẵn sàng',
    });
  }
  tabs.push({
    id: 'activity',
    label: 'Trao đổi & lịch sử',
    count: activities.length,
    description: activities[0] ? `Mới nhất ${compactDateTime(activities[0].created_at)}` : 'Chưa có hoạt động',
  });
  if (hasWorkflowFormSummary(item) && (!primarySurfaceKind || !primarySurfaceUsesWorkflowForm(primarySurfaceKind))) {
    tabs.push({
      id: 'form',
      label: 'Hồ sơ / PDF',
      status: item.workflow_form_status ?? 'draft',
      description: workflowFormChipText(item),
    });
  }
  const activeTab = tabs.some((tab) => tab.id === selectedTab) ? selectedTab : tabs[0]?.id ?? 'activity';

  useEffect(() => {
    if (!navigationRequest) return;
    const requestedTab = navigationRequest.target.kind === 'evidence'
      ? 'evidence'
      : navigationRequest.target.kind === 'workflow_form'
        ? 'form'
        : null;
    if (requestedTab && tabs.some((tab) => tab.id === requestedTab)) setSelectedTab(requestedTab);
  }, [navigationRequest?.requestId]);

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="border-b border-line bg-white px-3 pt-1">
        <div className="flex overflow-x-auto" role="tablist" aria-label="Thông tin hỗ trợ">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              id={`work-item-support-tab-${tab.id}`}
              aria-selected={activeTab === tab.id}
              aria-controls={`work-item-support-panel-${tab.id}`}
              className={clsx(
                'inline-flex min-h-10 shrink-0 items-center gap-2 border-b-2 px-3 py-2 text-sm font-semibold transition',
                activeTab === tab.id ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-600 hover:border-slate-300 hover:text-ink',
              )}
              onClick={() => setSelectedTab(tab.id)}
            >
              <span>{tab.label}</span>
              {typeof tab.count === 'number' ? <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-700 ring-1 ring-inset ring-line">{tab.count}</span> : null}
              {tab.status ? <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-100">{tab.id === 'form' ? workflowFormStatusLabel(tab.status) : statusLabel(tab.status)}</span> : null}
              <span className="sr-only">{tab.description}</span>
            </button>
          ))}
        </div>
      </div>

      <div
        role="tabpanel"
        id={`work-item-support-panel-${activeTab}`}
        aria-labelledby={`work-item-support-tab-${activeTab}`}
        className="bg-white p-4"
      >
        {activeTab === 'evidence' ? <EvidencePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} readOnly={readOnly} navigationRequest={navigationRequest} /> : null}
        {activeTab === 'activity' ? (
          <WorkItemActivityPanel
            activities={activities}
            attachmentOptions={attachmentOptions}
            mentionOptions={mentionOptions}
            submitting={activitySubmitting}
            readOnly={readOnly}
            onSubmit={onSubmitActivity}
          />
        ) : null}
        {activeTab === 'form' ? <WorkflowFormPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} readOnly={readOnly} navigationRequest={navigationRequest} /> : null}
      </div>
    </section>
  );
}

function WorkItemActivityPanel({
  activities,
  attachmentOptions,
  mentionOptions,
  submitting,
  readOnly,
  onSubmit,
}: {
  activities: WorkItemActivity[];
  attachmentOptions: ActivityAttachmentOption[];
  mentionOptions: ActivityMentionOption[];
  submitting: boolean;
  readOnly: boolean;
  onSubmit: (payload: ActivityPayload) => Promise<void>;
}) {
  const [composerOpen, setComposerOpen] = useState(false);
  const latestActivity = activities[0];
  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 px-3 py-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Dòng hoạt động</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">{activities.length} hoạt động</h3>
          <p className="mt-1 text-xs text-slate-500">
            {latestActivity ? `${latestActivity.actor_name ?? 'Hệ thống'} · ${compactDateTime(latestActivity.created_at)}` : 'Chưa có trao đổi hoặc sự kiện cho đầu việc này.'}
          </p>
        </div>
        {!readOnly ? <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
          onClick={() => setComposerOpen((current) => !current)}
        >
          <MessageSquare className="h-4 w-4" />
          {composerOpen ? 'Ẩn ô nhập' : 'Ghi hoạt động'}
        </button> : <span className="rounded-md bg-amber-50 px-3 py-2 text-xs font-semibold text-amber-700">Chỉ xem</span>}
      </div>

      {!readOnly && composerOpen ? (
        <ActivityComposer
          mode="work_item"
          attachmentOptions={attachmentOptions}
          mentionOptions={mentionOptions}
          defaultExpanded
          submitting={submitting}
          onSubmit={(payload) => onSubmit(payload).then(() => setComposerOpen(false))}
        />
      ) : null}

      {activities.length ? (
        <div className="space-y-2">
          {activities.map((activity) => (
            <ActivityTimelineItem key={activity.id} activity={activity} />
          ))}
        </div>
      ) : (
        <EmptyState title="Chưa có hoạt động cho đầu việc này" />
      )}
    </section>
  );
}

function ActivityTimelineItem({ activity }: { activity: WorkItemActivity }) {
  const comment = workflowFormActivityComment(activity);
  return (
    <article className="rounded-md border border-line bg-white px-3 py-2.5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-ink">{activityLabel(activity.activity_type)}</p>
          <p className="mt-1 text-xs text-slate-500">
            {activity.actor_name ?? 'Hệ thống'} · {compactDateTime(activity.created_at)}
          </p>
        </div>
        {activity.visibility ? <span className="w-fit rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{visibilityLabel(activity.visibility)}</span> : null}
      </div>
      <ActivityReplyQuote replyTo={activity.reply_to} />
      {activity.withdrawn_at ? <p className="mt-2 text-sm italic text-slate-500">Tin nhắn đã được thu hồi</p> : comment ? <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-5 text-slate-700">{comment}</p> : null}
      <ActivityAttachments attachments={activity.attachments} />
      {!activity.withdrawn_at && activity.reactions?.length ? <div className="mt-2"><ActivityReactionBar reactions={activity.reactions} /></div> : null}
    </article>
  );
}

function ActionButton({
  action,
  primary = false,
  submitting,
  onRunAction,
}: {
  action: WorkItemActionKey;
  primary?: boolean;
  submitting: boolean;
  onRunAction: (action: WorkItemActionKey) => void;
}) {
  return (
    <button
      type="button"
      className={clsx(
        'inline-flex w-full items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60',
        primary
          ? 'bg-blue-600 text-white hover:bg-blue-700'
          : action === 'reject' || action === 'block'
            ? 'border border-rose-200 text-rose-700 hover:bg-rose-50'
            : 'border border-line text-slate-700 hover:bg-slate-100',
      )}
      disabled={submitting}
      onClick={() => onRunAction(action)}
      title={actionDescriptions[action]}
    >
      {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
      {actionLabels[action]}
    </button>
  );
}

function ActionFact({ icon, label, value }: { icon: ReactNode; label: string; value: ReactNode }) {
  return (
    <div className="flex items-start gap-2">
      <span className="mt-0.5 shrink-0 text-slate-400">{icon}</span>
      <div className="min-w-0">
        <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
        <div className="mt-0.5 break-words text-sm font-semibold text-ink">{value}</div>
      </div>
    </div>
  );
}

function MetaPill({ icon, label }: { icon: ReactNode; label: string }) {
  return (
    <span className="inline-flex max-w-56 items-center gap-1.5 truncate rounded-md bg-slate-100 px-2 py-1 text-slate-700">
      <span className="shrink-0">{icon}</span>
      <span className="truncate">{label}</span>
    </span>
  );
}

function RequirementIcon({ state }: { state: RequirementState }) {
  if (state === 'done') return <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />;
  if (state === 'blocked') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />;
  if (state === 'warning') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />;
  return <Circle className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />;
}

function buildRequirementRows(
  item: WorkItem,
  evidence: EvidenceRecord[],
  actions: WorkItemActionKey[],
  readOnly: boolean,
): RequirementRow[] {
  const rows: RequirementRow[] = [];
  const requirement = item.evidence_requirement_json ?? {};
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');

  if (item.blocked_reason) {
    rows.push({
      id: 'blocker',
      label: 'Điểm chặn đang mở',
      description: item.blocked_reason,
      guidance: actions.includes('resolve-blocker')
        ? 'Ghi rõ cách đã xử lý, sau đó chọn Gỡ điểm chặn.'
        : 'Liên hệ người đang phụ trách hoặc người quản lý workflow để xử lý nguyên nhân chặn.',
      actionLabel: actions.includes('resolve-blocker') ? 'Đến thao tác gỡ chặn' : undefined,
      target: actions.includes('resolve-blocker') ? { kind: 'action_panel' } : undefined,
      state: 'blocked',
    });
  }

  if (item.domain_action_key) {
    const latestAutomation = (item.automation_events ?? []).find((event) => event.action_key === item.domain_action_key);
    const automationStatus = latestAutomation?.status;
    const reason = latestAutomation?.blocker_reason || latestAutomation?.error_message;
    rows.push({
      id: 'required-automation',
      label: 'Tự động hóa nghiệp vụ bắt buộc',
      description: automationStatus === 'succeeded'
        ? `${item.domain_action_key} đã hoàn tất thành công.`
        : automationStatus === 'blocked' || automationStatus === 'failed'
          ? `${item.domain_action_key}: ${reason || 'Đang bị chặn; dùng lại thao tác hoàn tất sau khi khắc phục.'}`
          : `${item.domain_action_key} sẽ được kiểm tra trước khi đầu việc hoàn tất.`,
      guidance: automationStatus === 'succeeded' || automationStatus === 'skipped'
        ? 'Không còn thao tác bổ sung cho điều kiện này.'
        : actions.includes('complete')
          ? 'Khắc phục lý do chặn nếu có, sau đó chọn Hoàn tất để hệ thống chạy lại kiểm tra.'
          : 'Tự động hóa sẽ được hệ thống kiểm tra khi đầu việc có thể Hoàn tất; liên hệ người quản lý workflow nếu trạng thái không thay đổi.',
      actionLabel: actions.includes('complete') && automationStatus !== 'succeeded' && automationStatus !== 'skipped'
        ? 'Đến thao tác Hoàn tất'
        : undefined,
      target: actions.includes('complete') && automationStatus !== 'succeeded' && automationStatus !== 'skipped'
        ? { kind: 'action_panel' }
        : undefined,
      state: automationStatus === 'succeeded' || automationStatus === 'skipped'
        ? 'done'
        : automationStatus === 'blocked' || automationStatus === 'failed'
          ? 'blocked'
          : 'neutral',
    });
  }

  if (item.completion_requirements?.length) {
    item.completion_requirements.forEach((completionRequirement) => {
      const satisfied = completionRequirement.status === 'satisfied';
      const presentation = completionRequirementPresentation(item, completionRequirement, readOnly);
      rows.push({
        id: `completion-${completionRequirement.key}`,
        label: completionRequirement.label,
        description: presentation.description,
        guidance: presentation.guidance,
        actionLabel: presentation.actionLabel,
        target: presentation.target,
        state: satisfied ? 'done' : 'blocked',
      });
    });
  } else {
    rows.push({
      id: 'evidence-optional',
      label: 'Minh chứng',
      description: activeEvidence.length ? `${activeEvidence.length} minh chứng đang hiệu lực.` : 'Không có yêu cầu minh chứng đặc biệt.',
      guidance: activeEvidence.length
        ? 'Mở Sổ minh chứng để xem các tệp đang hiệu lực.'
        : readOnly
          ? 'Có thể xem trước Sổ minh chứng; không bắt buộc tải tệp cho điều kiện hoàn tất.'
          : 'Có thể tải ảnh hoặc tài liệu nếu cần bổ sung bối cảnh cho đầu việc.',
      actionLabel: activeEvidence.length || readOnly ? 'Xem minh chứng' : 'Mở Sổ minh chứng',
      target: { kind: 'evidence', openUpload: false },
      state: activeEvidence.length ? 'done' : 'neutral',
    });
  }

  if (item.status === 'waiting_approval' || item.status === 'waiting_review' || item.approval_request_id) {
    const hasDecisionAction = actions.some((action) => decisionActions.has(action));
    rows.push({
      id: 'approval',
      label: item.status === 'waiting_review' ? 'Đang chờ rà soát' : 'Phê duyệt',
      description: item.approval_request_id ? 'Yêu cầu phê duyệt đã được liên kết với đầu việc.' : 'Đầu việc đang ở hàng chờ xử lý.',
      guidance: hasDecisionAction
        ? 'Kiểm tra hồ sơ liên quan, ghi chú nếu cần rồi chọn quyết định phù hợp tại Việc cần làm.'
        : 'Chờ người có quyền rà soát/phê duyệt xử lý; theo dõi diễn biến trong tab Hoạt động.',
      actionLabel: hasDecisionAction ? 'Đến điểm quyết định' : undefined,
      target: hasDecisionAction ? { kind: 'action_panel' } : undefined,
      state: item.status === 'waiting_approval' || item.status === 'waiting_review' ? 'warning' : 'done',
    });
  }

  if (requirement.required_permission) {
    const permissionPresentation = permissionRequirementPresentation(item);
    rows.push({
      id: 'permission',
      label: 'Quyền xử lý',
      description: permissionPresentation.description,
      guidance: permissionPresentation.guidance,
      state: 'neutral',
    });
  }

  return rows;
}

function resolveNextAction(
  item: WorkItem,
  actions: WorkItemActionKey[],
  primarySurfaceKind?: WorkItemPrimarySurfaceKind,
) {
  if (workItemIsUpcoming(item)) return 'Chờ workflow chuyển đến node này';
  const missingRequirements = missingCompletionRequirementLabels(item);
  const missingWorkflowForm = item.completion_requirements?.some((requirement) => requirement.key === 'workflow_form' && requirement.required && requirement.status === 'missing');

  if (item.status === 'blocked') return actionLabels['resolve-blocker'];
  if (missingWorkflowForm) return workflowFormNextAction(item);
  if (missingRequirements.length) return `Bổ sung: ${missingRequirements.join(', ')}`;
  if (primarySurfaceKind) {
    const surfaceAction = primarySurfaceNextAction(primarySurfaceKind, actions);
    if (surfaceAction) return surfaceAction;
  }
  if (actions.length) return actionLabels[actions[0]];
  return 'Theo dõi hoạt động';
}

function primarySurfaceNextAction(kind: WorkItemPrimarySurfaceKind, actions: WorkItemActionKey[]) {
  switch (kind) {
    case 'evidence':
      return 'Bổ sung minh chứng';
    case 'incoming_acceptance':
      return 'Ghi nhận nghiệm thu đầu vào';
    case 'material_snapshot':
      return 'Lưu ghi nhận vật tư';
    case 'surplus_handling':
      return 'Xử lý vật tư dư';
    case 'outgoing_acceptance':
      return 'Lập biên bản nghiệm thu';
    case 'owner_client_reconciliation':
      return 'Đối chiếu chủ đầu tư';
    case 'subcontractor_reconciliation':
      return 'Đối chiếu thầu phụ';
    case 'workflow_form':
      return 'Hoàn tất hồ sơ nghiệp vụ';
    case 'approval':
      return actions.length ? actionLabels[actions[0]] : 'Theo dõi phê duyệt';
    case 'generic':
      return actions.length ? actionLabels[actions[0]] : 'Theo dõi hoạt động';
    default:
      return null;
  }
}

function firstRoleLabel(item: WorkItem) {
  return asArray<string>(item.role_ids).filter(Boolean)[0];
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
