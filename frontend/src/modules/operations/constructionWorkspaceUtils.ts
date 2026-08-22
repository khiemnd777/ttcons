import type { ConstructionActionItem, ConstructionCheck, ConstructionWorkspace } from '../../lib/types/domain';
import { identityReferenceText } from '../../components/identityReference.ts';

export const constructionImpactHelperText = 'Chỉ phân loại ảnh hưởng kỹ thuật. Muốn mở luồng thương mại, dùng “Đề xuất phát sinh”.';

export function dateTimeText(value?: string) {
  if (!value) return 'Chưa đặt';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('vi-VN', { dateStyle: 'short', timeStyle: 'short' }).format(date);
}

export function toDateTimeInput(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

export function optionalISO(value?: string) {
  return value ? new Date(value).toISOString() : '';
}

export function impactLabel(value: ConstructionActionItem['impact_scope']) {
  return ({ none: 'Không ảnh hưởng', timeline: 'Ảnh hưởng tiến độ', budget: 'Ảnh hưởng chi phí', design_scope: 'Ảnh hưởng thiết kế / phạm vi' } as const)[value] ?? value;
}

export function eventLabel(value: string) {
  return ({ created: 'Đã tạo', resolution_submitted: 'Đã gửi khắc phục', verified_pass: 'Xác nhận đạt', verified_fail: 'Trả lại xử lý', impact_approved: 'Đã xác nhận ảnh hưởng' } as Record<string, string>)[value] ?? value;
}

export function checkStatusLabel(check: ConstructionCheck) {
  if (check.result === 'pass') return 'Đạt';
  if (check.result === 'fail') return 'Không đạt';
  return check.status === 'pending' ? 'Chờ kiểm tra' : check.status === 'cancelled' ? 'Đã hủy' : 'Hoàn tất';
}

export function actionStatusLabel(status: ConstructionActionItem['status']) {
  return ({ open: 'Cần xử lý', waiting_confirmation: 'Chờ xác nhận', done: 'Đã xong', cancelled: 'Đã hủy' } as const)[status];
}

export function constructionActionAssigneeLabel(action: Pick<ConstructionActionItem, 'assigned_user_name' | 'assigned_user_account_deleted'>, fallback = 'Chưa xác định') {
  const label = identityReferenceText(action.assigned_user_name, action.assigned_user_account_deleted, fallback);
  return action.assigned_user_account_deleted ? `${label} · Cần phân công lại` : label;
}

export function constructionCheckActorLabel(check: ConstructionCheck) {
  if (check.checked_by_name) return identityReferenceText(check.checked_by_name, check.checked_by_account_deleted);
  return identityReferenceText(check.requested_by_name, check.requested_by_account_deleted, 'Chưa xác định người thực hiện');
}

export function constructionCheckFilterWorkItems(workspace: Pick<ConstructionWorkspace, 'checks' | 'work_items'>) {
  const referencedWorkItemIDs = new Set(workspace.checks.map((check) => check.work_item_id).filter(Boolean));
  return workspace.work_items.filter((item) => item.report_eligible || referencedWorkItemIDs.has(item.id));
}

export function isConstructionCheckEditable(check: ConstructionCheck) {
  return check.status === 'pending' && !check.technical_rfi_id && !check.source_handoff_item_id;
}

export function isConstructionActionEditable(action: ConstructionActionItem) {
  if (action.status !== 'open' || action.technical_ncr_id) return false;
  return !action.technical_issue_id || action.id !== action.technical_issue_id;
}

export function isOverdue(value?: string, status?: string) {
  if (!value || status === 'done' || status === 'completed' || status === 'cancelled') return false;
  const time = new Date(value).getTime();
  return Number.isFinite(time) && time < Date.now();
}

export function normalizedSearch(value: unknown) {
  return String(value ?? '').trim().toLocaleLowerCase('vi-VN');
}
