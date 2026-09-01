import type { EvidenceRecord, WorkItem } from '../../lib/types/domain';
import type { ActivityAttachmentOption } from './ActivityComposer';
import type { WorkItemPrimarySurfaceKind } from './WorkItemPrimarySurface';
import {
  completionRequirementPresentation,
  missingCompletionRequirementLabels,
  permissionRequirementPresentation,
  type WorkItemRequirementNavigationTarget,
} from './workItemRequirements';
import { isDomainExecutionTarget } from './workItemExecutionTarget';
import {
  actionLabels,
  asArray,
  availableActions,
  hasWorkflowFormSummary,
  workItemIsUpcoming,
  workflowFormNextAction,
  type WorkItemActionKey,
} from './workItemPresentation';

export type RequirementState = 'done' | 'warning' | 'blocked' | 'neutral';

export type RequirementRow = {
  id: string;
  label: string;
  description: string;
  guidance: string;
  actionLabel?: string;
  target?: WorkItemRequirementNavigationTarget;
  state: RequirementState;
  kind?: 'permission';
};

const decisionActions = new Set<WorkItemActionKey>(['approve', 'reject', 'request-change']);
export const constructionHandoffCompletionLabel = 'Kiểm tra điều kiện & chốt bước';

export function constructionHandoffNeedsActivation(item?: WorkItem | null, workflowFormSubmitted?: boolean) {
  return Boolean(
    item
    && item.domain_action_key === 'construction.activate_handoff'
    && (workflowFormSubmitted ?? item.workflow_form_status === 'submitted')
    && item.business_source
    && !item.business_source?.ready,
  );
}

export function workItemActionLabel(item: WorkItem, action: WorkItemActionKey) {
  if (action === 'complete' && item.domain_action_key === 'construction.activate_handoff' && item.business_source?.ready) {
    return constructionHandoffCompletionLabel;
  }
  return actionLabels[action];
}

export function permittedActions(item?: WorkItem | null, workflowFormSubmitted?: boolean): WorkItemActionKey[] {
  if (!item || ['completed', 'cancelled'].includes(item.status)) return [];

  let statusActions = availableActions(item.status);
  if (isDomainExecutionTarget(item.execution_target) || hasWorkflowFormSummary(item)) {
    statusActions = statusActions.filter((action) => action !== 'start');
  }
  const serverActions = asArray<string>(item.available_actions);
  const submitted = workflowFormSubmitted ?? item.workflow_form_status === 'submitted';
  if (submitted) {
    statusActions = statusActions.filter((action) => action !== 'start');
    if (item.status === 'ready' && serverActions.includes('complete') && !statusActions.includes('complete')) {
      statusActions = ['complete', ...statusActions];
    }
  }
  if (constructionHandoffNeedsActivation(item, submitted)) {
    statusActions = statusActions.filter((action) => action !== 'complete');
  }
  if (Array.isArray(item.available_actions)) {
    const allowed = new Set(serverActions);
    return statusActions.filter((action) => allowed.has(action));
  }

  return statusActions.filter((action) => {
    if (decisionActions.has(action)) return (item.capabilities?.can_decide ?? item.can_decide) === true;
    return (item.capabilities?.can_act ?? item.can_act) === true;
  });
}


export function buildRequirementRows(
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
      label: 'Quyền thao tác',
      description: permissionPresentation.description,
      guidance: permissionPresentation.guidance,
      kind: 'permission',
      state: 'neutral',
    });
  }

  return rows;
}

export function resolveNextAction(
  item: WorkItem,
  actions: WorkItemActionKey[],
  primarySurfaceKind?: WorkItemPrimarySurfaceKind,
) {
  if (workItemIsUpcoming(item)) return actions.includes('start') ? actionLabels.start : 'Chờ workflow chuyển đến node này';
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

export function firstRoleLabel(item: WorkItem) {
  return asArray<string>(item.role_ids).filter(Boolean)[0];
}

export function buildActivityAttachmentOptions(evidence: EvidenceRecord[]): ActivityAttachmentOption[] {
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
