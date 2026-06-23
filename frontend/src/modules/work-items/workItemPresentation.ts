import type { WorkItem } from '../../lib/types/domain';
import { enumLabel } from '../../lib/utils/status';

export type WorkItemActionKey =
  | 'start'
  | 'complete'
  | 'block'
  | 'resolve-blocker'
  | 'request-review'
  | 'request-approval'
  | 'approve'
  | 'reject'
  | 'request-change';

export const actionLabels: Record<WorkItemActionKey, string> = {
  start: 'Bắt đầu',
  complete: 'Hoàn tất',
  block: 'Báo blocker',
  'resolve-blocker': 'Resolve blocker',
  'request-review': 'Gửi review',
  'request-approval': 'Gửi duyệt',
  approve: 'Duyệt',
  reject: 'Từ chối',
  'request-change': 'Yêu cầu sửa',
};

export const actionDescriptions: Record<WorkItemActionKey, string> = {
  start: 'Chuyển work item sang đang xử lý.',
  complete: 'Hoàn tất work item khi đủ evidence bắt buộc.',
  block: 'Ghi nhận blocker và lý do dừng việc.',
  'resolve-blocker': 'Gỡ blocker và đưa work item về trạng thái làm tiếp.',
  'request-review': 'Gửi work item sang hàng chờ review.',
  'request-approval': 'Tạo hoặc đồng bộ approval request cho work item.',
  approve: 'Duyệt và hoàn tất work item.',
  reject: 'Từ chối và trả work item về cần chỉnh sửa.',
  'request-change': 'Yêu cầu chỉnh sửa trước khi duyệt.',
};

const departmentLabels: Record<string, string> = {
  design: 'Thiết kế',
  qs: 'QS',
  procurement: 'Mua hàng',
  warehouse: 'Kho',
  construction: 'Thi công',
  operations: 'Thi công',
  qa: 'QA/QC',
  qaqc: 'QA/QC',
  accounting: 'Kế toán',
  client: 'Client',
  unassigned: 'Chưa phân công',
};

const workflowFormTypeLabels: Record<string, string> = {
  checklist: 'Checklist',
  daily_report: 'Daily report',
  meeting_minutes: 'Biên bản họp',
  generic_submission: 'Submission',
};

export function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? value as T[] : [];
}

export function numberValue(value: unknown) {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? number : 0;
}

export function compactDateTime(value: unknown) {
  if (!value) return '-';
  const date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat('vi-VN', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

export function departmentLabel(value?: string) {
  const key = String(value || 'unassigned').toLowerCase();
  return departmentLabels[key] ?? value ?? 'Chưa phân công';
}

export function activityLabel(value?: string) {
  const key = String(value ?? '');
  return enumLabel('activity_type', key);
}

export function evidenceLabel(value?: string) {
  const key = String(value ?? '');
  return enumLabel('evidence_type', key);
}

export function workTypeLabel(value?: string) {
  return enumLabel('work_type', value);
}

export function workflowFormTypeLabel(value?: string) {
  const key = String(value ?? '').trim();
  return workflowFormTypeLabels[key] ?? (key ? key : 'Biểu mẫu nghiệp vụ');
}

export function hasWorkflowFormSummary(value?: Record<string, unknown> | null) {
  if (!value) return false;
  return Boolean(value.workflow_form_instance_id || value.workflow_form_version_id || value.workflow_form_template_code || value.workflow_form_template_name);
}

export function workflowFormBusinessLabel(value?: Record<string, unknown> | null) {
  if (!value) return 'Biểu mẫu nghiệp vụ';
  const templateName = String(value.workflow_form_template_name ?? '').trim();
  if (templateName) return templateName;
  return workflowFormTypeLabel(String(value.workflow_form_type ?? ''));
}

export function workflowFormChipText(value?: Record<string, unknown> | null) {
  const label = workflowFormBusinessLabel(value);
  const version = numberValue(value?.workflow_form_version_no);
  return version > 0 ? `${label} v${version}` : label;
}

export function workflowFormNextAction(value?: Record<string, unknown> | null) {
  if (!hasWorkflowFormSummary(value)) return 'Theo dõi activity';
  if (value?.workflow_form_status !== 'submitted') return 'Submit biểu mẫu nghiệp vụ';
  return 'Ký / PDF / hoàn tất step';
}

export function priorityLabel(value?: string) {
  return enumLabel('priority', value);
}

export function visibilityLabel(value?: string) {
  return enumLabel('visibility', value);
}

export function scopeTypeLabel(value?: string) {
  return enumLabel('scope_type', value);
}

export function attachmentTypeLabel(value?: string) {
  return enumLabel('attachment_type', value);
}

export function isWorkOverdue(item: WorkItem) {
  if (!item.due_at || ['completed', 'cancelled'].includes(item.status)) return false;
  const due = new Date(item.due_at);
  return !Number.isNaN(due.getTime()) && due.getTime() < Date.now();
}

function workItemRank(item: WorkItem) {
  if (item.status === 'blocked') return 0;
  if (isWorkOverdue(item)) return 1;
  if (item.status === 'waiting_approval' || item.status === 'waiting_review') return 2;
  if (item.status === 'waiting_evidence') return 3;
  if (item.due_at) return 4;
  return 5;
}

export function prioritizeWorkItems(items: WorkItem[]) {
  return [...items].sort((a, b) => {
    const rankDiff = workItemRank(a) - workItemRank(b);
    if (rankDiff !== 0) return rankDiff;
    const aDue = a.due_at ? new Date(a.due_at).getTime() : Number.MAX_SAFE_INTEGER;
    const bDue = b.due_at ? new Date(b.due_at).getTime() : Number.MAX_SAFE_INTEGER;
    return aDue - bDue;
  });
}

export function availableActions(status?: string): WorkItemActionKey[] {
  switch (status) {
    case 'pending':
      return ['start', 'block'];
    case 'ready':
    case 'needs_changes':
      return ['start', 'request-review', 'request-approval', 'block'];
    case 'in_progress':
    case 'waiting_evidence':
      return ['complete', 'request-review', 'request-approval', 'block'];
    case 'waiting_review':
    case 'waiting_approval':
      return ['approve', 'request-change', 'reject', 'block'];
    case 'blocked':
      return ['resolve-blocker'];
    default:
      return [];
  }
}

export function actionRequiresReason(action: WorkItemActionKey) {
  return action === 'block' || action === 'reject' || action === 'request-change';
}
