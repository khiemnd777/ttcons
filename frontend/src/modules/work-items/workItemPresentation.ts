import type { WorkItem } from '../../lib/types/domain';
import { enumLabel, statusLabel } from '../../lib/utils/status';
export { dateTimeText as compactDateTime } from '../../lib/utils/format.ts';
export { workItemIsUpcoming, workItemWorkflowContext } from './workItemContext';

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
  block: 'Báo điểm chặn',
  'resolve-blocker': 'Gỡ điểm chặn',
  'request-review': 'Gửi rà soát',
  'request-approval': 'Gửi duyệt',
  approve: 'Duyệt',
  reject: 'Từ chối',
  'request-change': 'Yêu cầu sửa',
};

export const actionDescriptions: Record<WorkItemActionKey, string> = {
  start: 'Chuyển đầu việc sang đang xử lý.',
  complete: 'Hoàn tất đầu việc khi đủ minh chứng bắt buộc.',
  block: 'Ghi nhận điểm chặn và lý do dừng việc.',
  'resolve-blocker': 'Gỡ điểm chặn và đưa đầu việc về trạng thái làm tiếp.',
  'request-review': 'Gửi đầu việc sang hàng chờ rà soát.',
  'request-approval': 'Tạo hoặc đồng bộ yêu cầu phê duyệt cho đầu việc.',
  approve: 'Duyệt và hoàn tất đầu việc.',
  reject: 'Từ chối và trả đầu việc về cần chỉnh sửa.',
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
  client: 'Chủ đầu tư',
  unassigned: 'Chưa phân công',
};

const workflowFormTypeLabels: Record<string, string> = {
  checklist: 'Checklist',
  daily_report: 'Nhật ký',
  meeting_minutes: 'Biên bản họp',
  generic_submission: 'Hồ sơ nghiệp vụ',
};

const genericWorkflowFormTemplateCodes = new Set([
  'builtin_generic_submission',
  'generic_submission',
]);

const genericWorkflowFormTemplateNames = new Set([
  'generic submission',
  'workflow form',
  'biểu mẫu quy trình',
  'bieu mau quy trinh',
  'biểu mẫu gửi thông tin',
  'bieu mau gui thong tin',
  'hồ sơ nghiệp vụ',
  'ho so nghiep vu',
]);

export function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? value as T[] : [];
}

export function numberValue(value: unknown) {
  const number = Number(value ?? 0);
  return Number.isFinite(number) ? number : 0;
}

export function departmentLabel(value?: string) {
  const key = String(value || 'unassigned').toLowerCase();
  return departmentLabels[key] ?? value ?? 'Chưa phân công';
}

export function activityLabel(value?: string) {
  if (value === 'work_update') return 'Cập nhật công việc';
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
  return workflowFormTypeLabels[key] ?? (key ? key : 'Hồ sơ nghiệp vụ');
}

export function isGenericWorkflowFormTemplate(code?: unknown, name?: unknown) {
  const templateCode = String(code ?? '').trim().toLowerCase();
  const templateName = String(name ?? '').trim().toLowerCase();
  return genericWorkflowFormTemplateCodes.has(templateCode) || genericWorkflowFormTemplateNames.has(templateName);
}

export function hasWorkflowFormSummary(value?: Record<string, unknown> | null) {
  if (!value) return false;
  return Boolean(value.workflow_form_instance_id || value.workflow_form_version_id || value.workflow_form_template_code || value.workflow_form_template_name);
}

export function workflowFormBusinessLabel(value?: Record<string, unknown> | null) {
  if (!value) return 'Hồ sơ nghiệp vụ';
  const explicitTitle = firstNonEmpty(value.workflow_form_title, value.workflow_form_instance_title);
  if (explicitTitle && !isGenericWorkflowFormTemplate(undefined, explicitTitle)) return explicitTitle;
  const templateName = String(value.workflow_form_template_name ?? '').trim();
  if (templateName && !isGenericWorkflowFormTemplate(value.workflow_form_template_code, templateName)) return templateName;
  const runtimeTitle = firstNonEmpty(value.run_step_name, value.title, value.work_item_title);
  if (runtimeTitle && !isGenericWorkflowFormTemplate(undefined, runtimeTitle)) return runtimeTitle;
  return workflowFormTypeLabel(String(value.workflow_form_type ?? ''));
}

export function workflowFormChipText(value?: Record<string, unknown> | null) {
  const label = workflowFormBusinessLabel(value);
  const version = numberValue(value?.workflow_form_version_no);
  return version > 0 ? `${label} v${version}` : label;
}

export function workflowFormStatusLabel(value?: unknown) {
  const status = String(value ?? '').trim();
  if (status === 'submitted') return 'Đã chốt';
  return statusLabel(status || 'draft');
}

export function workflowFormEventLabel(value?: unknown) {
  const eventType = String(value ?? '').trim();
  const labels: Record<string, string> = {
    created: 'Đã tạo hồ sơ',
    draft_saved: 'Đã lưu nháp hồ sơ',
    submitted: 'Đã chốt hồ sơ',
    reopened: 'Đã mở lại hồ sơ',
    amended: 'Đã chỉnh sửa hồ sơ',
    cancelled: 'Đã hủy hồ sơ',
    signed: 'Đã ký hồ sơ',
    seal_applied: 'Đã đóng dấu hồ sơ',
    pdf_exported: 'Đã tạo PDF',
    pdf_export_deleted: 'Đã xoá PDF',
  };
  return labels[eventType] ?? enumLabel('event_type', eventType);
}

export function workflowFormActivityComment(value?: { metadata_json?: Record<string, unknown>; activity_type?: string; comment?: string }) {
  const comment = String(value?.comment ?? '');
  const workflowFormVersionID = String(value?.metadata_json?.form_version_id ?? '').trim();
  if (value?.activity_type === 'system_event' && workflowFormVersionID && comment === 'Đã gửi biểu mẫu quy trình') {
    return 'Đã chốt hồ sơ nghiệp vụ';
  }
  return comment;
}

export function workflowFormNextAction(value?: Record<string, unknown> | null) {
  if (!hasWorkflowFormSummary(value)) return 'Theo dõi hoạt động';
  if (value?.workflow_form_status !== 'submitted') return 'Chốt hồ sơ nghiệp vụ';
  return 'Xác nhận / PDF / hoàn tất bước';
}

function firstNonEmpty(...values: unknown[]) {
  for (const value of values) {
    const text = String(value ?? '').trim();
    if (text) return text;
  }
  return '';
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

function sortIndex(value: unknown) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : Number.MAX_SAFE_INTEGER;
}

function sortDate(value: unknown) {
  if (!value) return Number.MAX_SAFE_INTEGER;
  const date = new Date(String(value));
  return Number.isNaN(date.getTime()) ? Number.MAX_SAFE_INTEGER : date.getTime();
}

export function orderWorkItemsByWorkflow(items: WorkItem[]) {
  return [...items].sort((a, b) => {
    const nodeDiff = sortIndex(a.run_node_order_index) - sortIndex(b.run_node_order_index);
    if (nodeDiff !== 0) return nodeDiff;
    const stepDiff = sortIndex(a.run_step_order_index) - sortIndex(b.run_step_order_index);
    if (stepDiff !== 0) return stepDiff;
    const dueDiff = sortDate(a.due_at) - sortDate(b.due_at);
    if (dueDiff !== 0) return dueDiff;
    const createdDiff = sortDate(a.created_at) - sortDate(b.created_at);
    if (createdDiff !== 0) return createdDiff;
    return String(a.id).localeCompare(String(b.id));
  });
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
