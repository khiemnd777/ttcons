import type { ApprovalRequest } from '../lib/types/domain';

export type ApprovalDecisionAction = 'approve' | 'request-change' | 'reject';

export function approvalCanDecide(request: ApprovalRequest) {
  return Boolean(request.capabilities?.can_decide) && (request.available_actions?.length ?? 0) > 0;
}

export function approvalRequiresComment(request: ApprovalRequest) {
  return Boolean(request.decision_context?.comment_required);
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
