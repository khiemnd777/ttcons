type LabelMap = Record<string, string>;

export type EnumLabelGroup =
  | 'status'
  | 'result'
  | 'budget_status'
  | 'severity'
  | 'acceptance_type'
  | 'payment_type'
  | 'payment_method'
  | 'vendor_type'
  | 'impact_scope'
  | 'movement_type'
  | 'surplus_status'
  | 'node_type'
  | 'fail_path'
  | 'condition_type'
  | 'step_type'
  | 'target_type'
  | 'event_type';

export const statusLabels: LabelMap = {
  active: 'Kích hoạt',
  inactive: 'Ngừng hoạt động',
  draft: 'Nháp',
  submitted: 'Đã gửi',
  waiting_approval: 'Chờ duyệt',
  approved: 'Đã duyệt',
  rejected: 'Từ chối',
  in_progress: 'Đang xử lý',
  pending: 'Chờ xử lý',
  ready: 'Sẵn sàng',
  blocked: 'Bị chặn',
  completed: 'Hoàn tất',
  cancelled: 'Đã hủy',
  deleted: 'Đã xóa',
  overdue: 'Quá hạn',
  recorded: 'Đã ghi nhận',
  ordered: 'Đã đặt hàng',
  partially_paid: 'Thanh toán một phần',
  paid: 'Đã thanh toán',
  partially_received: 'Thu một phần',
  received: 'Đã nhận',
  revised: 'Đã chỉnh sửa',
  published: 'Đã phát hành',
  archived: 'Lưu trữ',
  open: 'Mở',
  resolved: 'Đã xử lý',
  closed: 'Đã đóng',
  planning: 'Đang lập kế hoạch',
  acceptance: 'Đang nghiệm thu',
  handover: 'Đang bàn giao',
  financial_closure: 'Đóng tài chính',
  fail: 'Không đạt',
  pass: 'Đạt',
  conditional_pass: 'Đạt có điều kiện',
  handed_over: 'Đã bàn giao',
  checking: 'Đang kiểm tra',
};

export const enumLabels: Record<EnumLabelGroup, LabelMap> = {
  status: statusLabels,
  result: {
    pass: 'Đạt',
    fail: 'Không đạt',
    conditional_pass: 'Đạt có điều kiện',
  },
  budget_status: {
    within_budget: 'Trong ngân sách',
    budget_exceeded: 'Vượt ngân sách',
  },
  severity: {
    low: 'Thấp',
    medium: 'Trung bình',
    high: 'Cao',
    critical: 'Nghiêm trọng',
  },
  acceptance_type: {
    partial: 'Bán phần',
    final: 'Toàn phần',
  },
  payment_type: {
    supplier_payment: 'Thanh toán nhà cung cấp',
    subcontractor_payment: 'Thanh toán thầu phụ',
    client_receipt: 'Thu từ khách hàng',
    advance: 'Tạm ứng',
    advance_settlement: 'Quyết toán tạm ứng',
    other: 'Khác',
  },
  payment_method: {
    bank_transfer: 'Chuyển khoản',
    cash: 'Tiền mặt',
  },
  vendor_type: {
    supplier: 'Nhà cung cấp',
    subcontractor: 'Thầu phụ',
    labor_team: 'Đội nhân công',
    transport: 'Vận chuyển',
    equipment_rental: 'Thuê thiết bị',
    service_provider: 'Nhà cung cấp dịch vụ',
    other: 'Khác',
  },
  impact_scope: {
    design: 'Thiết kế',
    boq: 'BOQ / Dự toán',
    budget: 'Ngân sách',
    timeline: 'Tiến độ',
    material: 'Vật tư',
    subcontractor: 'Thầu phụ',
    client_requirement: 'Yêu cầu khách hàng',
  },
  movement_type: {
    receive: 'Nhập kho',
    issue: 'Xuất công trình',
    transfer: 'Điều chuyển',
    return_to_warehouse: 'Lưu kho vật tư dư',
    return_to_subcontractor: 'Trả thầu phụ',
    'return-to-warehouse': 'Lưu kho vật tư dư',
    'return-to-subcontractor': 'Trả thầu phụ',
  },
  surplus_status: {
    pending: 'Chờ xử lý',
    handled: 'Đã xử lý',
  },
  node_type: {
    survey: 'Khảo sát',
    design: 'Thiết kế',
    boq: 'BOQ / Dự toán',
    budget_approval: 'Duyệt ngân sách',
    procurement: 'Mua vật tư',
    warehouse_issue: 'Xuất kho',
    construction: 'Thi công',
    acceptance: 'Nghiệm thu',
    rework: 'Rework',
    replanning: 'Replanning',
    handover: 'Bàn giao',
    financial_closure: 'Đóng tài chính',
    custom: 'Tùy chỉnh',
  },
  fail_path: {
    none: 'Không chuyển hướng',
    previous: 'Quay lại node trước',
    rework: 'Tạo rework',
    replanning: 'Tạo replanning',
  },
  condition_type: {
    always: 'Luôn chuyển',
    order_index: 'Theo thứ tự node',
    approval_approved: 'Approval đã duyệt',
    approval_rejected: 'Approval bị từ chối',
    acceptance_passed: 'Nghiệm thu đạt',
    acceptance_failed: 'Nghiệm thu không đạt',
    rework_required: 'Cần rework',
    rework_completed: 'Rework hoàn tất',
    replanning_required: 'Cần replanning',
    materials_issued: 'Vật tư đã xuất đủ',
    boq_approved: 'BOQ đã duyệt',
    procurement_approved: 'Mua vật tư đã duyệt',
    handover_ready: 'Sẵn sàng bàn giao',
    financial_ready: 'Sẵn sàng đóng tài chính',
  },
  step_type: {
    task: 'Công việc',
    checklist: 'Checklist',
    approval: 'Phê duyệt',
    document_upload: 'Tải tài liệu',
    photo_upload: 'Tải ảnh',
    budget_check: 'Kiểm tra ngân sách',
    material_check: 'Kiểm tra vật tư',
    acceptance_check: 'Kiểm tra nghiệm thu',
    accounting_check: 'Kiểm tra kế toán',
  },
  target_type: {
    procurement_request: 'Yêu cầu mua',
    replanning_request: 'Yêu cầu replanning',
    pipeline_run_node: 'Node pipeline',
    stock_movement: 'Phiếu kho',
    issue: 'Issue',
    acceptance: 'Nghiệm thu',
    rework_request: 'Rework',
    handover: 'Bàn giao',
    payable: 'Phải trả',
    receivable: 'Phải thu',
    payment: 'Thanh toán',
  },
  event_type: {
    review_requested: 'Yêu cầu review',
    material_approved: 'Đã duyệt vật tư',
    material_ordered: 'Đã đặt vật tư',
    warehouse_received: 'Kho đã nhận',
    warehouse_issued: 'Đã xuất kho',
    warehouse_transferred: 'Đã điều chuyển kho',
    surplus_stored: 'Đã lưu vật tư dư',
    surplus_returned_to_subcontractor: 'Đã trả vật tư dư cho thầu phụ',
    acceptance_passed: 'Nghiệm thu đạt',
    acceptance_failed: 'Nghiệm thu không đạt',
    rework_requested: 'Đã yêu cầu rework',
    handover_completed: 'Hoàn tất bàn giao',
    approval_approved: 'Đã duyệt',
    approval_rejected: 'Đã từ chối',
    stock_movement_created: 'Đã tạo phiếu kho',
  },
};

export function enumLabel(group: EnumLabelGroup, value?: unknown) {
  if (value === null || value === undefined || value === '') return '-';
  const key = String(value);
  return enumLabels[group][key] ?? key;
}

export function enumOptions(group: EnumLabelGroup, values: string[]) {
  return values.map((value) => ({ label: enumLabel(group, value), value }));
}

export function statusLabel(status?: string) {
  return enumLabel('status', status);
}
