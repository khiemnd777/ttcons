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

const activityLabels: Record<string, string> = {
  created: 'Tạo work item',
  started: 'Bắt đầu',
  completed: 'Hoàn tất',
  blocked: 'Báo blocker',
  blocker_resolved: 'Resolve blocker',
  review_requested: 'Gửi review',
  approval_requested: 'Gửi duyệt',
  approved: 'Đã duyệt',
  rejected: 'Từ chối',
  changes_requested: 'Yêu cầu sửa',
  evidence_added: 'Thêm evidence',
  evidence_accepted: 'Duyệt evidence',
  evidence_rejected: 'Từ chối evidence',
  comment: 'Ghi chú',
  status: 'Cập nhật trạng thái',
  blocker: 'Blocker',
  question: 'Câu hỏi',
  decision: 'Quyết định',
  reminder: 'Nhắc việc',
  file_note: 'Ghi chú file',
  system_event: 'Sự kiện hệ thống',
};

const evidenceLabels: Record<string, string> = {
  document: 'Tài liệu',
  photo: 'Ảnh',
  checklist: 'Checklist',
  link: 'Link',
  note: 'Ghi chú',
  other: 'Khác',
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
  return activityLabels[key] ?? enumLabel('event_type', key);
}

export function evidenceLabel(value?: string) {
  const key = String(value ?? '');
  return evidenceLabels[key] ?? (key || '-');
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
