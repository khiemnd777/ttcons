type LabelMap = Record<string, string>;

export type EnumLabelGroup =
  | 'status'
  | 'result'
  | 'budget_status'
  | 'severity'
  | 'acceptance_type'
  | 'payment_type'
  | 'payment_method'
  | 'payment_schedule_type'
  | 'payment_allocation_type'
  | 'vendor_type'
  | 'impact_scope'
  | 'movement_type'
  | 'surplus_status'
  | 'node_type'
  | 'fail_path'
  | 'condition_type'
  | 'step_type'
  | 'work_type'
  | 'priority'
  | 'visibility'
  | 'source_type'
  | 'scope_type'
  | 'target_type'
  | 'activity_type'
  | 'evidence_type'
  | 'attachment_type'
  | 'mention_type'
  | 'event_type';

export const statusLabels: LabelMap = {
  active: 'Kích hoạt',
  inactive: 'Ngừng hoạt động',
  draft: 'Nháp',
  submitted: 'Đã gửi',
  waiting_approval: 'Chờ duyệt',
  waiting_review: 'Chờ review',
  waiting_evidence: 'Chờ evidence',
  needs_changes: 'Cần chỉnh sửa',
  changes_requested: 'Yêu cầu chỉnh sửa',
  approved: 'Đã duyệt',
  verified: 'Đã xác thực',
  issued: 'Đã phát hành',
  voided: 'Đã huỷ',
  accepted: 'Đã chấp nhận',
  rejected: 'Từ chối',
  in_progress: 'Đang xử lý',
  pending: 'Chờ xử lý',
  ready: 'Sẵn sàng',
  blocked: 'Bị chặn',
  completed: 'Hoàn tất',
  done: 'Đã xong',
  cancelled: 'Đã hủy',
  deleted: 'Đã xóa',
  overdue: 'Quá hạn',
  recorded: 'Đã ghi nhận',
  ordered: 'Đã đặt hàng',
  partially_paid: 'Thanh toán một phần',
  paid: 'Đã thanh toán',
  partially_settled: 'Quyết toán một phần',
  settled: 'Đã quyết toán',
  partially_received: 'Thu một phần',
  received: 'Đã nhận',
  revised: 'Đã chỉnh sửa',
  reopened: 'Mở lại',
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

const workTypeLabels: LabelMap = {
  task: 'Công việc',
  checklist: 'Checklist',
  approval: 'Phê duyệt',
  document_upload: 'Tải tài liệu',
  photo_upload: 'Tải ảnh',
  design_review: 'Review thiết kế',
  budget_check: 'Kiểm tra ngân sách',
  material_check: 'Kiểm tra vật tư',
  daily_report: 'Nhật ký thi công',
  meeting_minutes: 'Biên bản họp',
  acceptance_check: 'Kiểm tra nghiệm thu',
  rework_task: 'Việc rework',
  handover_check: 'Kiểm tra bàn giao',
  accounting_check: 'Kiểm tra kế toán',
};

const recordTypeLabels: LabelMap = {
  project: 'Công trình',
  pipeline_template: 'Template pipeline',
  pipeline_run: 'Pipeline run',
  pipeline_run_node: 'Node pipeline',
  pipeline_run_step: 'Step pipeline',
  work_item: 'Work item',
  boq_item: 'BOQ item',
  procurement: 'Mua sắm',
  procurement_request: 'Yêu cầu mua',
  stock_movement: 'Phiếu kho',
  daily_report: 'Nhật ký thi công',
  issue: 'Issue',
  acceptance: 'Nghiệm thu',
  rework: 'Rework',
  rework_request: 'Yêu cầu rework',
  replanning: 'Replanning',
  replanning_request: 'Yêu cầu replanning',
  handover: 'Bàn giao',
  accounting_budget: 'Ngân sách',
  payable: 'Phải trả',
  receivable: 'Phải thu',
  payment: 'Thanh toán',
  input_invoice: 'Hoá đơn đầu vào',
  output_invoice: 'Hoá đơn đầu ra',
  evidence_record: 'Evidence',
  workflow_form_instance: 'Workflow form',
  workflow_form_action_item: 'Action item từ workflow form',
  document: 'Tài liệu',
  operation: 'Thi công',
  milestone: 'Mốc thanh toán',
  manual: 'Nhập thủ công',
};

const activityTypeLabels: LabelMap = {
  created: 'Tạo work item',
  started: 'Bắt đầu',
  completed: 'Hoàn tất',
  blocked: 'Báo blocker',
  blocker_resolved: 'Gỡ blocker',
  status_changed: 'Đổi trạng thái',
  comment: 'Ghi chú',
  status: 'Cập nhật trạng thái',
  blocker: 'Blocker',
  question: 'Câu hỏi',
  decision: 'Quyết định',
  reminder: 'Nhắc việc',
  file_note: 'Ghi chú file',
  system_event: 'Sự kiện hệ thống',
  evidence_added: 'Thêm evidence',
  evidence_accepted: 'Duyệt evidence',
  evidence_rejected: 'Từ chối evidence',
  review_requested: 'Gửi review',
  approval_requested: 'Gửi duyệt',
  approved: 'Đã duyệt',
  rejected: 'Từ chối',
  changes_requested: 'Yêu cầu sửa',
};

const evidenceTypeLabels: LabelMap = {
  document: 'Tài liệu',
  photo: 'Ảnh',
  checklist: 'Checklist',
  link: 'Link',
  note: 'Ghi chú',
  other: 'Khác',
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
    inbound: 'Phiếu thu',
    outbound: 'Phiếu chi',
    advance: 'Tạm ứng',
    advance_settlement: 'Quyết toán tạm ứng',
    other: 'Khác',
  },
  payment_method: {
    bank_transfer: 'Chuyển khoản',
    cash: 'Tiền mặt',
  },
  payment_schedule_type: {
    advance: 'Ứng trước',
    progress: 'Thanh toán kỳ',
    final: 'Kỳ cuối',
    retention: 'Giữ lại',
    other: 'Khác',
  },
  payment_allocation_type: {
    payable_payment: 'Thanh toán phải trả',
    advance_payment: 'Chi ứng trước',
    receivable_receipt: 'Thu phải thu',
    advance_receipt: 'Thu ứng trước',
    advance_settlement: 'Quyết toán ứng trước',
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
    loss: 'Hao hụt kho',
    return_to_warehouse: 'Lưu kho vật tư dư',
    return_to_subcontractor: 'Trả thầu phụ',
    'return-to-warehouse': 'Lưu kho vật tư dư',
    'return-to-subcontractor': 'Trả thầu phụ',
  },
  surplus_status: {
    pending: 'Chờ xử lý',
    store_pending: 'Chờ lưu kho',
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
  step_type: workTypeLabels,
  work_type: workTypeLabels,
  priority: {
    low: 'Thấp',
    normal: 'Bình thường',
    high: 'Cao',
    urgent: 'Khẩn cấp',
  },
  visibility: {
    internal: 'Nội bộ',
    project: 'Công trình',
    client: 'Chủ đầu tư',
  },
  source_type: recordTypeLabels,
  scope_type: recordTypeLabels,
  target_type: recordTypeLabels,
  activity_type: activityTypeLabels,
  evidence_type: evidenceTypeLabels,
  attachment_type: {
    evidence: 'Evidence',
    document: 'Tài liệu',
  },
  mention_type: {
    user: 'User',
    staff: 'Nhân sự',
    role: 'Role',
  },
  event_type: {
    review_requested: 'Yêu cầu review',
    pipeline_template_published: 'Đã phát hành template',
    material_requested: 'Yêu cầu mua vật tư',
    material_approved: 'Đã duyệt vật tư',
    material_ordered: 'Đã đặt vật tư',
    warehouse_received: 'Kho đã nhận',
    warehouse_issued: 'Đã xuất kho',
    warehouse_transferred: 'Đã điều chuyển kho',
    warehouse_loss_recorded: 'Ghi nhận hao hụt kho',
    surplus_stored: 'Đã lưu vật tư dư',
    surplus_returned_to_subcontractor: 'Đã trả vật tư dư cho thầu phụ',
    acceptance_passed: 'Nghiệm thu đạt',
    acceptance_failed: 'Nghiệm thu không đạt',
    rework_requested: 'Đã yêu cầu rework',
    replanning_triggered: 'Kích hoạt replanning',
    node_started: 'Bắt đầu node',
    node_blocked: 'Node bị chặn',
    node_completed: 'Hoàn tất node',
    step_completed: 'Hoàn tất step',
    handover_completed: 'Hoàn tất bàn giao',
    handover_blocked: 'Bàn giao bị chặn',
    approval_approved: 'Đã duyệt',
    approval_granted: 'Approval đã duyệt',
    approval_rejected: 'Đã từ chối',
    approval_changes_requested: 'Yêu cầu sửa approval',
    stock_movement_created: 'Đã tạo phiếu kho',
    payable_approved: 'Đã duyệt phải trả',
    payable_paid: 'Đã thanh toán phải trả',
    payable_payment_schedule_created: 'Tạo kỳ thanh toán đầu vào',
    payable_payment_schedule_approved: 'Duyệt kỳ thanh toán đầu vào',
    payable_advance_requested: 'Tạo ứng trước đầu vào',
    payable_advance_approved: 'Duyệt ứng trước đầu vào',
    payable_advance_payment_recorded: 'Ghi nhận chi ứng trước',
    payable_advance_settled: 'Quyết toán ứng trước',
    payable_payment_created: 'Tạo phiếu thanh toán đầu vào',
    payable_payment_allocated: 'Ghi nhận thanh toán đầu vào',
    receivable_approved: 'Đã duyệt phải thu',
    receivable_received: 'Đã thu phải thu',
    payment_approved: 'Đã duyệt thanh toán',
    work_item_started: 'Bắt đầu work item',
    work_item_completed: 'Hoàn tất work item',
    work_item_blocked: 'Work item bị chặn',
    work_item_blocker_resolved: 'Đã gỡ blocker work item',
    work_item_review_requested: 'Yêu cầu review work item',
    work_item_approval_requested: 'Yêu cầu duyệt work item',
    work_item_approved: 'Đã duyệt work item',
    work_item_rejected: 'Từ chối work item',
    work_item_changes_requested: 'Yêu cầu sửa work item',
    work_item_evidence_added: 'Đã thêm evidence',
    work_item_evidence_accepted: 'Đã chấp nhận evidence',
    workflow_form_draft_saved: 'Đã lưu nháp workflow form',
    workflow_form_submitted: 'Đã submit workflow form',
    workflow_form_signed: 'Đã ký workflow form',
    signed: 'Đã ký',
    work_item_evidence_rejected: 'Từ chối evidence',
    work_item_comment: 'Ghi chú work item',
    work_item_activity_status: 'Activity trạng thái work item',
    work_item_activity_blocker: 'Activity blocker work item',
    work_item_activity_question: 'Activity câu hỏi work item',
    work_item_activity_decision: 'Activity quyết định work item',
    work_item_activity_reminder: 'Activity nhắc việc work item',
    work_item_activity_file_note: 'Activity ghi chú file work item',
    project_activity_comment: 'Ghi chú công trình',
    project_activity_status: 'Activity trạng thái công trình',
    project_activity_blocker: 'Activity blocker công trình',
    project_activity_question: 'Activity câu hỏi công trình',
    project_activity_decision: 'Activity quyết định công trình',
    project_activity_reminder: 'Activity nhắc việc công trình',
    project_activity_file_note: 'Activity ghi chú file công trình',
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
