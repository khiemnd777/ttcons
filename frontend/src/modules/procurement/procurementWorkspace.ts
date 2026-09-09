import { dateText } from '../../lib/utils/format';

export type ProcurementAction = 'submit' | 'approve' | 'order';
export type ProcurementRow = Record<string, any> & {
  id: string; project_id: string; status: string; edit_version: number;
  capabilities?: Partial<Record<'edit' | ProcurementAction, boolean>>;
};
export type ProcurementHeader = { title: string; supplier_partner_id: string; order_reference: string; expected_delivery_date: string };
export const procurementStatusLabels: Record<string, string> = {
  draft: 'Nháp', waiting_approval: 'Chờ duyệt', submitted: 'Chờ duyệt', approved: 'Chờ đặt hàng',
  ordered: 'Đang giao / nghiệm thu', partially_received: 'Đạt nghiệm thu một phần', received: 'Đã nghiệm thu đủ', changes_requested: 'Cần chỉnh sửa',
};
export const procurementActionLabels = { submit: 'Gửi duyệt', approve: 'Duyệt yêu cầu', order: 'Ghi nhận đặt hàng' };
export const procurementActionSuccess = { submit: 'Đã gửi yêu cầu mua để duyệt', approve: 'Đã duyệt yêu cầu mua', order: 'Đã ghi nhận đặt hàng' };

export function procurementHeader(row: ProcurementRow): ProcurementHeader {
  return { title: row.title ?? '', supplier_partner_id: row.supplier_partner_id ?? '', order_reference: row.order_reference ?? '', expected_delivery_date: String(row.expected_delivery_date ?? '').slice(0, 10) };
}
export function procurementActionFor(row: ProcurementRow, readOnly = false): ProcurementAction | null {
  if (readOnly) return null;
  const action = ['draft', 'changes_requested'].includes(row.status) ? 'submit'
    : ['waiting_approval', 'submitted'].includes(row.status) ? 'approve' : row.status === 'approved' ? 'order' : null;
  return action && row.capabilities?.[action] === true ? action : null;
}
export function validateProcurementHeader(values: ProcurementHeader, action?: ProcurementAction): Partial<Record<keyof ProcurementHeader, string>> {
  const errors: Partial<Record<keyof ProcurementHeader, string>> = {};
  if (!values.title.trim()) errors.title = 'Vui lòng nhập tiêu đề';
  if (action === 'order') {
    if (!values.supplier_partner_id) errors.supplier_partner_id = 'Vui lòng chọn nhà cung cấp';
    if (!values.order_reference.trim()) errors.order_reference = 'Vui lòng nhập số đơn đặt hàng';
    if (!values.expected_delivery_date) errors.expected_delivery_date = 'Vui lòng chọn ngày giao dự kiến';
  }
  if (values.expected_delivery_date && (!/^\d{4}-\d{2}-\d{2}$/.test(values.expected_delivery_date) || !Number.isFinite(Date.parse(values.expected_delivery_date)) || new Date(values.expected_delivery_date).toISOString().slice(0, 10) !== values.expected_delivery_date)) errors.expected_delivery_date = 'Ngày giao dự kiến không hợp lệ';
  return errors;
}
// Match only source action families supported by the canonical backend adapter.
// Generating requests from BOQ remains a separate mutation at the BOQ source.
export function procurementMutationWorkItemId(item?: { id: string; domain_action_key?: string }) {
  return item && ['source.input_contract_package_ready', 'source.procurement_contract_ready', 'procurement.require_approved'].includes(item.domain_action_key ?? '') ? item.id : '';
}
export function numeric(value: unknown) { return Number(value ?? 0); }
export function formatQuantity(value: unknown) { return numeric(value).toLocaleString('vi-VN', { maximumFractionDigits: 3 }); }
export function formatDate(value: unknown) { return dateText(String(value ?? '').slice(0, 10), 'Chưa chốt'); }
