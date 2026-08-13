import type { ApprovalRequest } from '../lib/types/domain';

export type ApprovalDecisionAction = 'approve' | 'request-change' | 'reject';

export function approvalCanDecide(request: ApprovalRequest) {
  return Boolean(request.capabilities?.can_decide) && (request.available_actions?.length ?? 0) > 0;
}

export function approvalRequiresComment(request: ApprovalRequest) {
  return Boolean(request.decision_context?.comment_required);
}

export function approvalRequesterLabel(request: ApprovalRequest) {
  return request.requested_by_name?.trim() || 'Chưa xác định người gửi';
}

export function approvalRequiredRolesLabel(request: ApprovalRequest) {
  const roles = request.decision_context?.required_roles
    ?.map((role) => role.trim())
    .filter(Boolean);
  return roles?.length ? roles.join(', ') : 'Chưa xác định vai trò duyệt';
}

export function approvalDecisionLabel(request: ApprovalRequest) {
  const context = request.decision_context;
  if (!context) return 'Chờ phê duyệt';
  const roles = context.required_roles?.filter(Boolean).join(', ') || 'Vai trò được phân quyền';
  return `Stage ${Number(context.current_stage ?? 0) + 1} · ${roles} · cần ${Math.max(1, Number(context.quorum ?? 1))} phê duyệt`;
}

export function approvalActionLabel(action: ApprovalDecisionAction) {
  if (action === 'approve') return 'Duyệt';
  if (action === 'request-change') return 'Yêu cầu chỉnh sửa';
  return 'Từ chối';
}
