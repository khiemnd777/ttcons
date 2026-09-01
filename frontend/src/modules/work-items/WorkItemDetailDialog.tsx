import { ReactNode, useCallback, useEffect, useRef, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Circle,
  Clock3,
  FileCheck2,
  GitBranch,
  Loader2,
  MessageSquare,
  PencilLine,
  Save,
  Shield,
  ShieldCheck,
  Users,
} from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { get, post } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { statusLabel } from '../../lib/utils/status';
import { useToast } from '../../components/ToastProvider';
import type { ActivityPayload, EvidenceRecord, ProjectActivityOptions, WorkflowFormDetail, WorkItem, WorkItemActivity, WorkItemStartRequirement } from '../../lib/types/domain';
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
  WorkflowFormEventsPanel,
  type WorkflowFormActionHandle,
  type WorkflowFormActionState,
} from './WorkflowFormPanel';
import {
  completionRequirementNavigationFromError,
  completionRequirementPresentation,
  missingCompletionRequirementLabels,
  permissionRequirementPresentation,
  type WorkItemRequirementNavigationRequest,
  type WorkItemRequirementNavigationTarget,
} from './workItemRequirements';
import { firstFocusableRequirementControl, revealWorkItemRequirementTarget } from './workItemRequirementNavigation';
import { executionTargetPath, isDomainExecutionTarget } from './workItemExecutionTarget';
import { constructionHandoffNavigationState } from './workItemNavigation';
import type { WorkItemWorkflowNavigation } from './workItemWorkflowNavigation';
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
import {
  buildActivityAttachmentOptions,
  buildRequirementRows,
  constructionHandoffCompletionLabel,
  constructionHandoffNeedsActivation,
  firstRoleLabel,
  permittedActions,
  resolveNextAction,
  workItemActionLabel,
  type RequirementRow,
  type RequirementState,
} from './workItemDetailDomain';

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
  workflowNavigation,
  onClose,
  onChanged,
}: {
  item?: WorkItem | null;
  workItemId?: string;
  workflowNavigation?: WorkItemWorkflowNavigation;
  onClose?: () => void;
  onChanged?: (item: WorkItem) => void;
}) {
  const [actionText, setActionText] = useState('');
  const [pendingAction, setPendingAction] = useState<WorkItemActionKey | null>(null);
  const [startConfirmationOpen, setStartConfirmationOpen] = useState(false);
  const [startReason, setStartReason] = useState('');
  const [startReasonAttempted, setStartReasonAttempted] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const location = useLocation();
  const navigate = useNavigate();
  const selectedWorkId = workItemId ?? item?.id ?? '';
  const navigationSequence = useRef(0);
  const workflowFormActionRef = useRef<WorkflowFormActionHandle | null>(null);
  const amendmentDialogContentRef = useRef<HTMLDivElement | null>(null);
  const earlyStartDialogContentRef = useRef<HTMLDivElement | null>(null);
  const navigateAmendmentValidation = useValidationNavigation(amendmentDialogContentRef);
  const navigateEarlyStartValidation = useValidationNavigation(earlyStartDialogContentRef);
  const [navigationRequest, setNavigationRequest] = useState<WorkItemRequirementNavigationRequest | null>(null);
  const [workflowFormActionState, setWorkflowFormActionState] = useState<WorkflowFormActionState | null>(null);
  const [amendmentDialogOpen, setAmendmentDialogOpen] = useState(false);
  const [amendmentReason, setAmendmentReason] = useState('');
  const [amendmentReasonAttempted, setAmendmentReasonAttempted] = useState(false);
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
    mutationFn: ({ workItemId, action, comment, bypassStartRequirements, startRequirementsToken }: {
      workItemId: string;
      action: WorkItemActionKey;
      comment: string;
      bypassStartRequirements?: boolean;
      startRequirementsToken?: string;
    }) => post<WorkItem>(`/work-items/${workItemId}/${action}`, {
      comment,
      reason: comment,
      bypass_start_requirements: bypassStartRequirements,
      start_requirements_token: startRequirementsToken,
    }),
    onSuccess: (updated, variables) => {
      setActionText('');
      setPendingAction(null);
      setStartConfirmationOpen(false);
      setStartReason('');
      setStartReasonAttempted(false);
      queryClient.invalidateQueries({ queryKey: ['work-item', variables.workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      if (updated.project_id) {
        queryClient.invalidateQueries({ queryKey: ['project-operations', updated.project_id] });
      }
      onChanged?.(updated);
      toast.success(`Đã ${actionLabels[variables.action].toLowerCase()}`);
    },
    onError: (error, variables) => {
      if (variables.bypassStartRequirements) {
        queryClient.invalidateQueries({ queryKey: ['work-item', variables.workItemId] });
      }
      toast.error('Không thao tác được đầu việc', error);
      if (variables.action === 'complete' && detailItem) {
        const target = completionRequirementNavigationFromError(detailItem, error);
        if (target) navigateToRequirement(target);
      }
    },
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
    if (!detailItem?.id) return;
    if (workItemIsUpcoming(detailItem)) {
      if (action !== 'start' || !detailItem.start_readiness?.can_bypass) return;
      setStartReason('');
      setStartReasonAttempted(false);
      setStartConfirmationOpen(true);
      return;
    }
    const comment = actionText.trim();
    if (actionRequiresReason(action) && !comment) {
      setPendingAction(action);
      toast.warn({ title: 'Cần nhập lý do', description: actionDescriptions[action] });
      return;
    }
    workAction.mutate({ workItemId: detailItem.id, action, comment });
  }

  function confirmEarlyStart() {
    if (!detailItem?.id || !detailItem.start_readiness?.can_bypass) return;
    const reason = startReason.trim();
    if (!reason) {
      setStartReasonAttempted(true);
      toast.warn({ title: 'Cần nhập lý do bắt đầu sớm', description: 'Lý do được lưu cùng lịch sử và audit của đầu việc.' });
      navigateEarlyStartValidation('start_reason');
      return;
    }
    workAction.mutate({
      workItemId: detailItem.id,
      action: 'start',
      comment: reason,
      bypassStartRequirements: true,
      startRequirementsToken: detailItem.start_readiness.snapshot_token,
    });
  }

  function closeDialog() {
    setActionText('');
    setPendingAction(null);
    setStartConfirmationOpen(false);
    setStartReason('');
    setStartReasonAttempted(false);
    onClose?.();
  }

  function openConstructionHandoff() {
    if (!detailItem?.project_id) return;
    const planningPath = `/projects/${encodeURIComponent(detailItem.project_id)}/planning`;
    closeDialog();
    navigate(planningPath, {
      state: constructionHandoffNavigationState(detailItem.project_id, location.state),
    });
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
    setStartConfirmationOpen(false);
    setStartReason('');
    setStartReasonAttempted(false);
    setAmendmentDialogOpen(false);
    setAmendmentReason('');
    setAmendmentReasonAttempted(false);
  }, [selectedWorkId]);

  const actions = permittedActions(detailItem, workflowFormActionState?.submitted);
  const evidence = asArray<EvidenceRecord>(detailItem?.evidence);
  const activities = asArray<WorkItemActivity>(detailItem?.activities);
  const attachmentOptions = buildActivityAttachmentOptions(evidence);
  const futureReadOnly = detailItem ? workItemIsUpcoming(detailItem) : false;
  const resolvedPrimarySurface = detailItem ? resolveWorkItemPrimarySurface(detailItem) : null;
  const primarySurface = futureReadOnly && resolvedPrimarySurface?.kind !== 'workflow_form'
    ? null
    : resolvedPrimarySurface;
  const requirementRows = detailItem ? buildRequirementRows(detailItem, evidence, actions, futureReadOnly) : [];
  const incompleteRequirementCount = futureReadOnly
    ? numberValue(detailItem?.start_readiness?.unmet_count)
    : requirementRows.filter((row) => row.state === 'blocked' || row.state === 'warning').length;
  const nextAction = detailItem ? resolveNextAction(detailItem, actions, primarySurface?.kind) : '';
  const latestActivity = activities[0];
  const skippedHistory = detailItem?.status === 'cancelled';
  const showMobileWorkflowFormActions = Boolean(
    workflowFormActionState?.available
    && !workflowFormActionState.submitted
    && !workflowFormActionState.readOnly,
  );
  return (
    <>
      {workDetail.isLoading && !detailItem ? <LoadingSpinner /> : null}
      {workDetail.error && !detailItem ? <EmptyState title="Không tải được chi tiết đầu việc" /> : null}
      {detailItem ? (
        <div className={clsx('space-y-4', showMobileWorkflowFormActions && 'pb-20 xl:pb-0')}>
          <WorkItemWorkspaceHeader item={detailItem} evidence={evidence} workflowNavigation={workflowNavigation} />

          {futureReadOnly ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              <p className="font-semibold">Công việc chưa đến lượt · Chỉ xem</p>
              <p className="mt-1">
                {detailItem.start_readiness?.can_bypass
                  ? 'Có thể xem trước ngữ cảnh và hồ sơ. Chọn Bắt đầu để xác nhận các điều kiện còn thiếu và thực hiện công việc này sớm.'
                  : 'Có thể xem trước ngữ cảnh và hồ sơ. Cần chốt nhánh workflow hoặc xử lý điều kiện chặn trước khi bắt đầu công việc này sớm.'}
              </p>
            </div>
          ) : null}

          {detailItem.is_early_execution ? (
            <div className="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900">
              <p className="font-semibold">Đang thực hiện sớm · Workflow chưa chuyển đến node này</p>
              <p className="mt-1">Các điều kiện hoàn tất và quyền nghiệp vụ vẫn được áp dụng bình thường. Tiến độ workflow chính không bị dịch chuyển.</p>
            </div>
          ) : null}

          <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_18rem]">
            <main className="min-w-0 space-y-4">
              <WorkItemStartRequirementPanel item={detailItem} />
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
              onRequestWorkflowFormAmendment={() => {
                setAmendmentReason('');
                setAmendmentReasonAttempted(false);
                setAmendmentDialogOpen(true);
              }}
              onSaveWorkflowFormAmendment={() => workflowFormActionRef.current?.saveAmendment()}
              onCancelWorkflowFormAmendment={() => workflowFormActionRef.current?.cancelAmendment()}
              onOpenConstructionHandoff={openConstructionHandoff}
              onNavigateRequirement={navigateToRequirement}
              onClose={closeDialog}
              navigationRequest={navigationRequest}
            />
          </div>
        </div>
      ) : null}
      <Dialog
        open={amendmentDialogOpen}
        title="Chỉnh sửa hồ sơ đã chốt?"
        description={`Thao tác này sẽ tạo phiên bản ${Number(workflowFormActionState?.currentRevision ?? 0) + 1}. Trạng thái đầu việc và quy trình được giữ nguyên.`}
        onClose={() => {
          setAmendmentDialogOpen(false);
          setAmendmentReasonAttempted(false);
        }}
        size="md"
        footer={(
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setAmendmentDialogOpen(false)}>
              Huỷ
            </button>
            <button
              type="button"
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700"
              onClick={() => {
                const reason = amendmentReason.trim();
                if (!reason) {
                  setAmendmentReasonAttempted(true);
                  toast.warn({ title: 'Thiếu lý do chỉnh sửa', description: 'Nhập lý do để lịch sử thay đổi có đủ bối cảnh.' });
                  navigateAmendmentValidation('amendment_reason');
                  return;
                }
                workflowFormActionRef.current?.beginAmendment(reason);
                setAmendmentDialogOpen(false);
                setAmendmentReason('');
                setAmendmentReasonAttempted(false);
              }}
            >
              Bắt đầu chỉnh sửa
            </button>
          </div>
        )}
      >
        <div ref={amendmentDialogContentRef}>
        <label
          className="block text-sm font-semibold text-slate-700"
          data-validation-key="amendment_reason"
          data-validation-error={amendmentReasonAttempted && !amendmentReason.trim() ? 'true' : undefined}
        >
          Lý do chỉnh sửa <span className="text-rose-600">*</span>
          <textarea
            className={clsx(
              'mt-2 min-h-28 w-full rounded-md border bg-white px-3 py-2 text-sm font-normal outline-none focus:ring-2 focus:ring-blue-500',
              amendmentReasonAttempted && !amendmentReason.trim() ? 'border-rose-400' : 'border-line',
            )}
            value={amendmentReason}
            name="amendment_reason"
            maxLength={1000}
            onChange={(event) => {
              setAmendmentReason(event.target.value);
              if (event.target.value.trim()) setAmendmentReasonAttempted(false);
            }}
            placeholder="Ví dụ: Bổ sung phạm vi theo biên bản khảo sát ngày 17-08"
            aria-invalid={amendmentReasonAttempted && !amendmentReason.trim()}
            aria-describedby={amendmentReasonAttempted && !amendmentReason.trim() ? 'amendment-reason-error' : undefined}
          />
        </label>
        {amendmentReasonAttempted && !amendmentReason.trim() ? <p id="amendment-reason-error" role="alert" className="mt-1 text-xs font-medium text-rose-600">Lý do chỉnh sửa là bắt buộc.</p> : null}
        </div>
      </Dialog>
      <Dialog
        open={startConfirmationOpen}
        title="Bắt đầu công việc trước lượt?"
        description="Workflow chính vẫn giữ nguyên node hiện tại. Thao tác này và lý do sẽ được lưu trong lịch sử audit."
        onClose={() => {
          if (workAction.isPending) return;
          setStartConfirmationOpen(false);
          setStartReasonAttempted(false);
        }}
        size="md"
        footer={(
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
              disabled={workAction.isPending}
              onClick={() => setStartConfirmationOpen(false)}
            >
              Quay lại
            </button>
            <button
              type="button"
              className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={workAction.isPending || !detailItem?.start_readiness?.can_bypass}
              onClick={confirmEarlyStart}
            >
              {workAction.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              Bắt đầu ngay
            </button>
          </div>
        )}
      >
        <div ref={earlyStartDialogContentRef}>
          <EarlyStartConfirmationContent
            requirements={detailItem?.start_readiness?.requirements ?? []}
            reason={startReason}
            reasonInvalid={startReasonAttempted && !startReason.trim()}
            onReasonChange={(value) => {
              setStartReason(value);
              if (value.trim()) setStartReasonAttempted(false);
            }}
          />
        </div>
      </Dialog>
    </>
  );
}

function WorkItemStartRequirementPanel({ item }: { item: WorkItem }) {
  const readiness = item.start_readiness;
  const requirements = readiness?.requirements ?? [];
  if (!requirements.length) return null;

  const unmetCount = numberValue(readiness?.unmet_count);
  return (
    <details
      className={clsx('group rounded-md border bg-white shadow-panel', unmetCount ? 'border-amber-200' : 'border-line')}
      open={unmetCount > 0}
    >
      <summary className={clsx('flex cursor-pointer list-none items-center justify-between gap-3 rounded-md px-3 py-3', unmetCount ? 'bg-amber-50 hover:bg-amber-100' : 'bg-slate-50 hover:bg-slate-100')}>
        <div className="flex min-w-0 items-center gap-2">
          {unmetCount ? <AlertTriangle className="h-4 w-4 shrink-0 text-amber-700" /> : <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />}
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Điều kiện để bắt đầu</p>
            <h3 className="mt-0.5 text-sm font-semibold text-ink">
              {unmetCount ? `Còn ${unmetCount} điều kiện chưa hoàn tất` : 'Đã đủ điều kiện bắt đầu theo quy trình'}
            </h3>
          </div>
        </div>
        <span className="inline-flex shrink-0 items-center gap-2 text-xs font-semibold text-slate-600">
          {requirements.length} mục
          <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
        </span>
      </summary>
      <div className="divide-y divide-line border-t border-line bg-white">
        {requirements.map((requirement) => (
          <StartRequirementRow key={requirement.id} item={item} requirement={requirement} />
        ))}
      </div>
      {readiness?.state === 'route_unresolved' ? (
        <p className="border-t border-rose-100 bg-rose-50 px-3 py-2 text-xs leading-5 text-rose-700">
          Phải chốt nhánh workflow trước khi có thể bắt đầu công việc này sớm.
        </p>
      ) : readiness?.can_bypass ? (
        <p className="border-t border-amber-100 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
          Có thể chọn <span className="font-semibold">Bắt đầu</span> để xác nhận và thực hiện sớm. Các điều kiện hoàn tất của chính công việc vẫn giữ nguyên.
        </p>
      ) : null}
    </details>
  );
}

function StartRequirementRow({
  item,
  requirement,
  compact = false,
}: {
  item?: WorkItem;
  requirement: WorkItemStartRequirement;
  compact?: boolean;
}) {
  const content = (
    <>
      {requirement.satisfied
        ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
        : <Circle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />}
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold text-ink">{requirement.label}</p>
        <p className="mt-0.5 text-xs leading-5 text-slate-600">
          {[requirement.node_label, requirement.step_label, statusLabel(requirement.status)].filter(Boolean).join(' · ')}
        </p>
        {requirement.description ? <p className="mt-0.5 text-xs leading-5 text-slate-600">{requirement.description}</p> : null}
      </div>
      {requirement.can_open && requirement.work_item_id && !compact ? (
        <span className="inline-flex shrink-0 items-center gap-1 self-center text-xs font-semibold text-blue-700">
          Mở công việc
          <ArrowRight className="h-3.5 w-3.5" />
        </span>
      ) : null}
    </>
  );

  if (item && requirement.can_open && requirement.work_item_id && !compact) {
    const returnTo = item.project_id ? `/projects/${item.project_id}` : '/my-work';
    return (
      <Link
        className="flex items-start gap-3 px-3 py-2.5 transition hover:bg-blue-50 focus-visible:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500"
        to={`/projects/${item.project_id}/work-items/${requirement.work_item_id}?returnTo=${encodeURIComponent(returnTo)}`}
      >
        {content}
      </Link>
    );
  }
  return <div className={clsx('flex items-start gap-3', compact ? 'py-2' : 'px-3 py-2.5')}>{content}</div>;
}

function EarlyStartConfirmationContent({
  requirements,
  reason,
  reasonInvalid,
  onReasonChange,
}: {
  requirements: WorkItemStartRequirement[];
  reason: string;
  reasonInvalid: boolean;
  onReasonChange: (value: string) => void;
}) {
  const unmetRequirements = requirements.filter((requirement) => !requirement.satisfied);
  return (
    <div className="space-y-4">
      <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-3">
        <p className="text-sm font-semibold text-amber-900">Cần hoàn thành các điều kiện sau theo workflow:</p>
        <div className="mt-2 divide-y divide-amber-100">
          {unmetRequirements.map((requirement) => (
            <StartRequirementRow key={requirement.id} requirement={requirement} compact />
          ))}
        </div>
      </div>
      <p className="text-sm leading-6 text-slate-700">
        Anh có thể tiếp tục và bắt đầu công việc ngay mà không đi qua lần lượt các bước trên. Việc này không bỏ qua quyền thao tác, điểm chặn nghiệp vụ hoặc điều kiện hoàn tất của công việc đang mở.
      </p>
      <label className="block text-sm font-semibold text-ink" htmlFor="early-start-reason">
        Lý do bắt đầu sớm <span className="text-rose-600">*</span>
      </label>
      <div data-validation-key="start_reason" data-validation-error={reasonInvalid ? 'true' : undefined}>
        <textarea
          id="early-start-reason"
          name="start_reason"
          className={clsx(
            'min-h-24 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
            reasonInvalid ? 'border-rose-300 bg-rose-50' : 'border-line bg-white',
          )}
          value={reason}
          onChange={(event) => onReasonChange(event.target.value)}
          placeholder="Ví dụ: cần triển khai song song để kịp tiến độ công trình"
          aria-invalid={reasonInvalid}
          aria-describedby={reasonInvalid ? 'early-start-reason-error' : undefined}
        />
        {reasonInvalid ? <p id="early-start-reason-error" role="alert" className="mt-1 text-xs font-medium text-rose-700">Vui lòng nhập lý do để lưu cùng lịch sử audit.</p> : null}
      </div>
    </div>
  );
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
                <span className="font-semibold text-slate-700">{identityReferenceText(activity.actor_name, activity.actor_account_deleted, 'Hệ thống')}</span>
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
type ActivityHistoryFilter = 'all' | 'discussion' | 'form';
function WorkItemWorkspaceHeader({
  item,
  evidence,
  workflowNavigation,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  workflowNavigation?: WorkItemWorkflowNavigation;
}) {
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const context = workItemWorkflowContext(item);

  return (
    <section className="rounded-lg border border-line bg-white px-4 py-3 shadow-panel">
      {workflowNavigation ? <WorkItemWorkflowToolbar navigation={workflowNavigation} /> : null}
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          {!workflowNavigation ? <p className="truncate text-xs font-semibold text-blue-700">{context.node} · {context.step}</p> : null}
          <h3 className={clsx('text-base font-semibold text-ink', !workflowNavigation && 'mt-1')}>{item.title}</h3>
          {item.description ? <p className="mt-1 line-clamp-2 text-sm leading-5 text-slate-600">{item.description}</p> : null}
        </div>
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />
          {item.project_code ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{item.project_code}</span> : null}
          {item.priority ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{priorityLabel(item.priority)}</span> : null}
        </div>
      </div>
      <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-3 text-xs font-semibold text-slate-600">
        <MetaPill icon={<Users className="h-3.5 w-3.5" />} label={item.assignee_name ? identityReferenceText(item.assignee_name, item.assignee_account_deleted) : firstRoleLabel(item) ?? departmentLabel(item.department)} />
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

function WorkItemWorkflowToolbar({ navigation }: { navigation: WorkItemWorkflowNavigation }) {
  const previousPath = navigation.previous ? executionTargetPath(navigation.previous, navigation.returnTo) : '';
  const nextPath = navigation.next ? executionTargetPath(navigation.next, navigation.returnTo) : '';

  return (
    <nav
      className="-mx-4 -mt-3 mb-3 flex min-h-11 flex-col justify-center gap-2 border-b border-line px-4 py-2 text-xs font-semibold sm:flex-row sm:items-center sm:gap-3 sm:py-0"
      aria-label="Vị trí trong giai đoạn"
      data-work-item-workflow-context
    >
      <div className="flex min-w-0 shrink-0 items-center gap-2 whitespace-nowrap">
        <span className="inline-flex min-w-0 items-center gap-1 text-blue-700">
          <span className="uppercase tracking-wide text-slate-500">Giai đoạn</span>
          <span className="max-w-44 truncate text-sm">{navigation.stageName}</span>
        </span>
        <span className="h-4 w-px bg-line" aria-hidden="true" />
        <span className="text-sm tabular-nums text-ink">Bước {navigation.position}/{navigation.total}</span>
      </div>

      <div className="flex min-w-0 items-center gap-1 overflow-hidden sm:ml-auto">
        {navigation.previous ? (
          <Link
            className="inline-flex min-w-0 items-center gap-1 rounded-md px-2 py-1.5 text-blue-700 hover:bg-blue-50"
            to={previousPath}
            aria-label={`Mở bước trước: ${workflowNeighborTitle(navigation.previous)}`}
          >
            <ChevronLeft className="h-4 w-4 shrink-0" />
            <span className="shrink-0">Bước {workflowNeighborNumber(navigation.previous)}</span>
            <span className="hidden max-w-40 truncate lg:inline">· {workflowNeighborTitle(navigation.previous)}</span>
          </Link>
        ) : null}
        {navigation.next ? (
          <Link
            className="inline-flex min-w-0 items-center gap-1 rounded-md px-2 py-1.5 text-blue-700 hover:bg-blue-50"
            to={nextPath}
            aria-label={`Mở bước sau: ${workflowNeighborTitle(navigation.next)}`}
          >
            <span className="shrink-0">Bước {workflowNeighborNumber(navigation.next)}</span>
            <span className="hidden max-w-40 truncate lg:inline">· {workflowNeighborTitle(navigation.next)}</span>
            <ChevronRight className="h-4 w-4 shrink-0" />
          </Link>
        ) : null}
        {navigation.progressPath ? (
          <Link
            className="ml-auto inline-flex shrink-0 items-center gap-1 rounded-md border border-line bg-white px-2 py-1.5 text-slate-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 sm:ml-1"
            to={navigation.progressPath}
          >
            <GitBranch className="h-3.5 w-3.5" />
            Tiến độ
          </Link>
        ) : null}
      </div>
    </nav>
  );
}

function workflowNeighborNumber(item: WorkItem) {
  const value = Number(item.run_step_order_index);
  return Number.isFinite(value) && value > 0 ? value : '-';
}

function workflowNeighborTitle(item: WorkItem) {
  return item.run_step_name?.trim() || item.title;
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
              <RequirementIcon state={row.state} kind={row.kind} />
              <div className="min-w-0 flex-1 text-left">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-semibold text-ink">{row.label}</p>
                  {row.kind === 'permission' ? (
                    <span className="rounded-md bg-blue-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-blue-700">
                      Thông tin truy cập
                    </span>
                  ) : null}
                </div>
                <p className="mt-0.5 text-xs leading-5 text-slate-600">{row.description}</p>
                <p className="mt-1 text-xs leading-5 text-slate-700">
                  <span className="font-semibold">{row.kind === 'permission' ? 'Khi không thể thao tác:' : 'Cách xử lý:'}</span> {row.guidance}
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
  onRequestWorkflowFormAmendment,
  onSaveWorkflowFormAmendment,
  onCancelWorkflowFormAmendment,
  onOpenConstructionHandoff,
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
  onRequestWorkflowFormAmendment: () => void;
  onSaveWorkflowFormAmendment: () => void;
  onCancelWorkflowFormAmendment: () => void;
  onOpenConstructionHandoff: () => void;
  onNavigateRequirement: (target: WorkItemRequirementNavigationTarget) => void;
  onClose: () => void;
  navigationRequest: WorkItemRequirementNavigationRequest | null;
}) {
  const workflowFormIsPrimary = Boolean(
    workflowFormActionState?.available
    && !workflowFormActionState.submitted
    && !workflowFormActionState.readOnly,
  );
  const workflowFormCanAmend = Boolean(workflowFormActionState?.available && workflowFormActionState.canAmend && !workflowFormActionState.amending);
  const workflowFormIsAmending = Boolean(workflowFormActionState?.available && workflowFormActionState.amending);
  const constructionHandoffActivationRequired = constructionHandoffNeedsActivation(item, workflowFormActionState?.submitted)
    && !workItemIsUpcoming(item)
    && (item.capabilities?.can_act ?? item.can_act) === true;
  const formIssueCount = workflowFormActionState?.validationIssueLabels.length ?? 0;
  const primaryAction = workflowFormIsPrimary || constructionHandoffActivationRequired ? undefined : actions[0];
  const secondaryActions = workflowFormIsPrimary || constructionHandoffActivationRequired ? actions : actions.slice(1);
  const panelNextAction = workflowFormIsAmending
    ? `Lưu phiên bản chỉnh sửa ${workflowFormActionState?.currentRevision ? workflowFormActionState.currentRevision + 1 : ''}`.trim()
    : constructionHandoffActivationRequired
      ? 'Kích hoạt bàn giao thi công'
      : actions[0] === 'complete' && item.domain_action_key === 'construction.activate_handoff'
        ? constructionHandoffCompletionLabel
        : workflowFormCanAmend
      ? 'Hồ sơ đã chốt · Có thể chỉnh sửa'
      : workflowFormIsPrimary
    ? formIssueCount
      ? `Hoàn tất ${formIssueCount} thông tin bắt buộc`
      : 'Chốt hồ sơ nghiệp vụ'
    : nextAction;
  const panelRef = useRef<HTMLElement | null>(null);
  const [noteOpen, setNoteOpen] = useState(false);
  const navigateActionValidation = useValidationNavigation(panelRef);

  useEffect(() => {
    if (!pendingAction || !actionRequiresReason(pendingAction) || actionText.trim()) return;
    navigateActionValidation({ key: 'action_reason', prepare: () => setNoteOpen(true) });
  }, [actionText, navigateActionValidation, pendingAction]);

  useEffect(() => {
    const shouldRevealActionPanel = navigationRequest?.target.kind === 'action_panel'
      || (
        navigationRequest?.target.kind === 'workflow_form'
        && workflowFormIsPrimary
        && formIssueCount === 0
    );
    if (!shouldRevealActionPanel) return undefined;
    const submitVariant = typeof window !== 'undefined' && window.matchMedia('(max-width: 1279px)').matches
      ? 'mobile'
      : 'desktop';
    const submitButton = panelRef.current?.querySelector<HTMLElement>(`[data-workflow-form-submit="${submitVariant}"]`) ?? null;
    const revealContainer = submitVariant === 'mobile' && submitButton ? submitButton : panelRef.current;
    return revealWorkItemRequirementTarget(revealContainer, submitButton ?? firstFocusableRequirementControl(panelRef.current));
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

      {workflowFormIsPrimary || constructionHandoffActivationRequired || actions.length ? (
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
            <div className="hidden xl:block">
              <WorkflowFormPrimaryActions
                state={workflowFormActionState}
                variant="desktop"
                onSave={onSaveWorkflowForm}
                onSubmit={onSubmitWorkflowForm}
              />
            </div>
          ) : constructionHandoffActivationRequired ? (
            <button
              type="button"
              className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
              onClick={onOpenConstructionHandoff}
            >
              Mở bàn giao thi công
              <ArrowRight className="h-4 w-4" />
            </button>
          ) : primaryAction ? (
            <ActionButton action={primaryAction} label={workItemActionLabel(item, primaryAction)} primary submitting={submitting} onRunAction={onRunAction} />
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
          <div
            data-validation-key="action_reason"
            data-validation-error={pendingAction && actionRequiresReason(pendingAction) && !actionText.trim() ? 'true' : undefined}
          >
            <textarea
              name="action_reason"
              className={clsx(
                'min-h-20 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
                pendingAction && actionRequiresReason(pendingAction) && !actionText.trim() ? 'border-amber-300 bg-amber-50' : 'border-line bg-white',
              )}
              value={actionText}
              onChange={(event) => onActionTextChange(event.target.value)}
              placeholder="Ghi chú / lý do cho thao tác"
              aria-invalid={Boolean(pendingAction && actionRequiresReason(pendingAction) && !actionText.trim())}
            />
          </div>
          ) : null}

          {secondaryActions.length ? (
            <details className="group rounded-md border border-line bg-slate-50">
              <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100">
                <span>Thao tác khác</span>
                <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
              </summary>
              <div className="space-y-2 border-t border-line bg-white p-2">
                {secondaryActions.map((action) => (
                  <ActionButton key={action} action={action} label={workItemActionLabel(item, action)} submitting={submitting} onRunAction={onRunAction} />
                ))}
              </div>
            </details>
          ) : null}
        </div>
      ) : workflowFormCanAmend || workflowFormIsAmending ? (
        <p className="mt-3 rounded-md bg-slate-50 p-3 text-sm text-slate-600">
          {workflowFormIsAmending ? 'Kiểm tra nội dung rồi lưu phiên bản mới hoặc huỷ để giữ nguyên hồ sơ.' : 'Có thể tạo phiên bản mới mà không mở lại tiến độ quy trình.'}
        </p>
      ) : (
        <p className="mt-3 rounded-md bg-slate-50 p-3 text-sm text-slate-600">Không có thao tác trực tiếp cho trạng thái hiện tại.</p>
      )}

      <div className="mt-4 hidden space-y-3 border-t border-line pt-3 xl:block">
        <ActionFact icon={<ShieldCheck className="h-4 w-4" />} label="Trạng thái" value={<StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />} />
        <ActionFact icon={<Users className="h-4 w-4" />} label="Phụ trách" value={item.assignee_name ? identityReferenceText(item.assignee_name, item.assignee_account_deleted) : firstRoleLabel(item) ?? departmentLabel(item.department)} />
        <ActionFact icon={<Clock3 className="h-4 w-4" />} label="Hạn xử lý" value={dateText(item.due_at)} />
        {latestActivity ? (
          <ActionFact
            icon={<MessageSquare className="h-4 w-4" />}
            label="Hoạt động gần nhất"
            value={`${identityReferenceText(latestActivity.actor_name, latestActivity.actor_account_deleted, 'Hệ thống')} · ${compactDateTime(latestActivity.created_at)}`}
          />
        ) : null}
      </div>

      <div className="mt-4 flex flex-col gap-2 border-t border-line pt-3">
        {workflowFormIsAmending ? (
          <>
            <button
              type="button"
              className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={workflowFormActionState?.amendmentSaving}
              onClick={onSaveWorkflowFormAmendment}
            >
              {workflowFormActionState?.amendmentSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              Lưu chỉnh sửa
            </button>
            <button
              type="button"
              className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={workflowFormActionState?.amendmentSaving}
              onClick={onCancelWorkflowFormAmendment}
            >
              Huỷ chỉnh sửa
            </button>
          </>
        ) : workflowFormCanAmend ? (
          <button
            type="button"
            className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-100"
            onClick={onRequestWorkflowFormAmendment}
          >
            <PencilLine className="h-4 w-4" />
            Chỉnh sửa
          </button>
        ) : null}
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

      {workflowFormIsPrimary ? (
        <div
          data-workflow-form-mobile-actions
          className="fixed inset-x-3 bottom-3 z-50 rounded-lg border border-line bg-white/95 p-2 shadow-2xl backdrop-blur xl:hidden"
          style={{ paddingBottom: 'max(0.5rem, env(safe-area-inset-bottom))' }}
        >
          <WorkflowFormPrimaryActions
            state={workflowFormActionState}
            variant="mobile"
            onSave={onSaveWorkflowForm}
            onSubmit={onSubmitWorkflowForm}
          />
        </div>
      ) : null}
    </aside>
  );
}

function WorkflowFormPrimaryActions({
  state,
  variant,
  onSave,
  onSubmit,
}: {
  state: WorkflowFormActionState | null;
  variant: 'desktop' | 'mobile';
  onSave: () => void;
  onSubmit: () => void;
}) {
  const disabled = Boolean(state?.saving || state?.submitting);
  const saveButton = (
    <button
      type="button"
      className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
      disabled={disabled}
      onClick={onSave}
    >
      {state?.saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
      {state?.saveLabel ?? 'Lưu nháp'}
    </button>
  );
  const submitButton = (
    <button
      type="button"
      data-workflow-form-submit={variant}
      className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
      disabled={disabled}
      onClick={onSubmit}
    >
      {state?.submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileCheck2 className="h-4 w-4" />}
      {state?.submitLabel ?? 'Chốt hồ sơ'}
    </button>
  );

  return variant === 'mobile' ? (
    <div className="grid grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)] gap-2">
      {saveButton}
      {submitButton}
    </div>
  ) : (
    <div className="space-y-2">
      {submitButton}
      {saveButton}
    </div>
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
  const hasWorkflowForm = hasWorkflowFormSummary(item);
  const workflowFormHistory = useQuery({
    queryKey: ['work-item-form', item.id],
    queryFn: () => get<WorkflowFormDetail>(`/work-items/${item.id}/form`),
    enabled: hasWorkflowForm,
    retry: false,
  });
  const workflowFormEvents = workflowFormHistory.data?.events ?? [];
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
  if (hasWorkflowForm && (!primarySurfaceKind || !primarySurfaceUsesWorkflowForm(primarySurfaceKind))) {
    tabs.push({
      id: 'form',
      label: 'Hồ sơ / PDF',
      status: item.workflow_form_status ?? 'draft',
      description: workflowFormChipText(item),
    });
  }
  const activeTab = tabs.some((tab) => tab.id === selectedTab) ? selectedTab : tabs[0]?.id ?? 'activity';

  useEffect(() => {
    setSelectedTab('evidence');
  }, [item.id]);

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
        {activeTab === 'evidence' ? <EvidencePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} readOnly={readOnly} navigationRequest={navigationRequest} variant="embedded" /> : null}
        {activeTab === 'activity' ? (
          <WorkItemActivityPanel
            key={item.id}
            activities={activities}
            attachmentOptions={attachmentOptions}
            mentionOptions={mentionOptions}
            submitting={activitySubmitting}
            readOnly={readOnly}
            workflowFormHistoryAvailable={hasWorkflowForm}
            workflowFormEvents={workflowFormEvents}
            workflowFormHistoryLoading={workflowFormHistory.isLoading}
            workflowFormHistoryError={workflowFormHistory.isError}
            onRetryWorkflowFormHistory={() => void workflowFormHistory.refetch()}
            onSubmit={onSubmitActivity}
          />
        ) : null}
        {activeTab === 'form' ? <WorkflowFormPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} workItemStatus={item.status} readOnly={readOnly} navigationRequest={navigationRequest} /> : null}
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
  workflowFormHistoryAvailable,
  workflowFormEvents,
  workflowFormHistoryLoading,
  workflowFormHistoryError,
  onRetryWorkflowFormHistory,
  onSubmit,
}: {
  activities: WorkItemActivity[];
  attachmentOptions: ActivityAttachmentOption[];
  mentionOptions: ActivityMentionOption[];
  submitting: boolean;
  readOnly: boolean;
  workflowFormHistoryAvailable: boolean;
  workflowFormEvents: Record<string, unknown>[];
  workflowFormHistoryLoading: boolean;
  workflowFormHistoryError: boolean;
  onRetryWorkflowFormHistory: () => void;
  onSubmit: (payload: ActivityPayload) => Promise<void>;
}) {
  const [composerOpen, setComposerOpen] = useState(false);
  const [historyFilter, setHistoryFilter] = useState<ActivityHistoryFilter>('all');
  const latestActivity = activities[0];
  const discussionActivities = activities.filter((activity) => ['comment', 'blocker', 'question', 'decision', 'reminder', 'file_note'].includes(activity.activity_type));
  const visibleActivities = historyFilter === 'discussion' ? discussionActivities : activities;
  const filters: Array<{ id: ActivityHistoryFilter; label: string; count: number }> = [
    { id: 'all', label: 'Tất cả', count: activities.length },
    { id: 'discussion', label: 'Trao đổi', count: discussionActivities.length },
  ];
  if (workflowFormHistoryAvailable) filters.push({ id: 'form', label: 'Hồ sơ', count: workflowFormEvents.length });
  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 px-3 py-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Dòng hoạt động</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">{activities.length} hoạt động</h3>
          <p className="mt-1 text-xs text-slate-500">
            {latestActivity ? `${identityReferenceText(latestActivity.actor_name, latestActivity.actor_account_deleted, 'Hệ thống')} · ${compactDateTime(latestActivity.created_at)}` : 'Chưa có trao đổi hoặc sự kiện cho đầu việc này.'}
          </p>
        </div>
        {!readOnly ? <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
          onClick={() => {
            if (!composerOpen) setHistoryFilter('discussion');
            setComposerOpen((current) => !current);
          }}
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

      <div className="flex flex-wrap gap-2" role="group" aria-label="Lọc trao đổi và lịch sử">
        {filters.map((filter) => (
          <button
            key={filter.id}
            type="button"
            aria-pressed={historyFilter === filter.id}
            className={clsx(
              'inline-flex items-center gap-2 rounded-md border px-3 py-1.5 text-xs font-semibold transition',
              historyFilter === filter.id
                ? 'border-blue-200 bg-blue-50 text-blue-700'
                : 'border-line bg-white text-slate-600 hover:bg-slate-50 hover:text-ink',
            )}
            onClick={() => {
              setHistoryFilter(filter.id);
              if (filter.id === 'form') setComposerOpen(false);
            }}
          >
            {filter.label}
            <span className="rounded-full bg-white px-1.5 py-0.5 text-[11px] text-slate-600 ring-1 ring-inset ring-line">{filter.count}</span>
          </button>
        ))}
      </div>

      {historyFilter === 'form' ? (
        workflowFormHistoryLoading ? <LoadingSpinner />
          : workflowFormHistoryError ? (
            <div className="rounded-md border border-rose-200 bg-rose-50 px-3 py-3 text-sm text-rose-800">
              <p className="font-semibold">Không tải được lịch sử hồ sơ</p>
              <button type="button" className="mt-2 rounded-md border border-rose-200 bg-white px-3 py-1.5 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={onRetryWorkflowFormHistory}>
                Thử lại
              </button>
            </div>
          ) : workflowFormEvents.length ? <WorkflowFormEventsPanel events={workflowFormEvents} />
            : <EmptyState title="Chưa có sự kiện hồ sơ" />
      ) : visibleActivities.length ? (
        <div className="space-y-2">
          {visibleActivities.map((activity) => (
            <ActivityTimelineItem key={activity.id} activity={activity} />
          ))}
        </div>
      ) : (
        <EmptyState title={historyFilter === 'discussion' ? 'Chưa có trao đổi cho đầu việc này' : 'Chưa có hoạt động cho đầu việc này'} />
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
            {identityReferenceText(activity.actor_name, activity.actor_account_deleted, 'Hệ thống')} · {compactDateTime(activity.created_at)}
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
  label,
  primary = false,
  submitting,
  onRunAction,
}: {
  action: WorkItemActionKey;
  label?: string;
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
      {label ?? actionLabels[action]}
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

function RequirementIcon({ state, kind }: { state: RequirementState; kind?: RequirementRow['kind'] }) {
  if (kind === 'permission') return <Shield className="mt-0.5 h-4 w-4 shrink-0 text-blue-600" />;
  if (state === 'done') return <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />;
  if (state === 'blocked') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />;
  if (state === 'warning') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />;
  return <Circle className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />;
}
