export type AnyRecord = Record<string, any>;

export type User = {
  id: string;
  email: string;
  full_name: string;
  roles: string[];
  permissions?: string[];
};

export type Project = AnyRecord & {
  id: string;
  name: string;
  code: string;
  client_name: string;
  status: string;
  progress_percent: number;
};

export type PipelineNode = AnyRecord & {
  id: string;
  name: string;
  description?: string;
  type: string;
  order_index: number;
  position_x?: number;
  position_y?: number;
  responsible_role_ids?: string[];
  approval_rule?: string;
  required_permission?: string;
  permissions?: string[];
  exit_conditions?: AnyRecord;
  fail_path?: string;
  rework_enabled?: boolean;
  replanning_enabled?: boolean;
  status?: string;
  steps?: AnyRecord[];
};

export type PipelineTemplateEdge = AnyRecord & {
  id: string;
  template_id?: string;
  from_node_id: string;
  to_node_id: string;
  condition_type?: string;
  condition_config?: AnyRecord;
  label?: string;
  priority?: number;
};

export type PipelineEdgeConditionDefinition = {
  type: string;
  label: string;
  description: string;
  default_label: string;
  category: string;
};

export type PipelineTemplate = AnyRecord & {
  id: string;
  name: string;
  description?: string;
  project_type?: string;
  status: string;
  version: number;
  node_count?: number;
  nodes?: PipelineNode[];
  edges?: PipelineTemplateEdge[];
};

export type WorkflowFormCatalogItem = AnyRecord & {
  template_id: string;
  template_code: string;
  template_name: string;
  template_description?: string;
  template_status: 'active' | 'archived' | string;
  form_type: 'checklist' | 'daily_report' | 'meeting_minutes' | 'generic_submission' | string;
  version_id: string;
  version_no: number;
  version_status: 'published' | string;
  published_at?: string;
  field_count?: number;
  required_field_keys?: string[];
  fields?: Array<{
    field_key: string;
    label: string;
    field_type: string;
    required: boolean;
    order_index?: number;
  }>;
};

export type ProjectOperationsRecord = AnyRecord & {
  id: string;
  project_id?: string;
  created_at?: string;
  updated_at?: string;
};

export type BOQUnitPriceEvent = ProjectOperationsRecord & {
  boq_item_id: string;
  event_type: 'expected_updated' | 'norm_locked' | 'final_locked' | string;
  price_source: 'expected' | 'norm' | 'final' | string;
  from_unit_price?: number;
  to_unit_price: number;
  revision_no: number;
  comment?: string;
  actor_name?: string;
};

export type BOQItem = ProjectOperationsRecord & {
  category: string;
  work_item_name: string;
  description?: string;
  unit: string;
  quantity: number;
  unit_price: number;
  expected_unit_price: number;
  norm_unit_price?: number;
  final_unit_price?: number;
  effective_unit_price?: number;
  total_amount?: number;
  effective_total_amount?: number;
  material_id?: string;
  material_name?: string;
  status: string;
  unit_price_source?: 'expected' | 'norm' | 'final' | string;
  price_lifecycle_status?: 'expected' | 'norm_locked' | 'final_locked' | string;
  norm_price_locked?: boolean;
  norm_price_locked_by?: string;
  norm_price_locked_at?: string;
  norm_price_comment?: string;
  final_price_locked?: boolean;
  final_price_locked_by?: string;
  final_price_locked_at?: string;
  final_price_comment?: string;
  price_revision_no?: number;
  price_events?: BOQUnitPriceEvent[];
};

export type WorkItemActivity = ProjectOperationsRecord & {
  work_item_id?: string;
  scope_type?: string;
  scope_id?: string;
  actor_id?: string;
  actor_name?: string;
  activity_type: string;
  visibility?: 'internal' | 'project' | 'client' | string;
  comment?: string;
  from_status?: string;
  to_status?: string;
  metadata_json?: AnyRecord;
  work_item_title?: string;
  target_label?: string;
  mentions?: AnyRecord[];
  mentioned_me?: boolean;
  read_at?: string;
  is_unread?: boolean;
  queue_types?: string[];
  attachments?: ActivityAttachmentSummary[];
};

export type ActivityMentionPayload = {
  type: 'user' | 'staff' | 'role';
  id: string;
  label?: string;
};

export type ActivityAttachmentPayload = {
  type: 'evidence' | 'document';
  id: string;
  label?: string;
};

export type ActivityAttachmentSummary = ActivityAttachmentPayload & {
  status?: string;
  visibility?: string;
  file_url?: string;
  evidence_type?: string;
  document_type?: string;
  document_id?: string;
};

export type ActivityPayload = {
  activity_type: 'comment' | 'status' | 'blocker' | 'question' | 'decision' | 'reminder' | 'file_note';
  comment: string;
  visibility: 'internal' | 'project' | 'client';
  scope_type?: string;
  scope_id?: string;
  work_item_id?: string;
  mentions?: ActivityMentionPayload[];
  attachments?: ActivityAttachmentPayload[];
  metadata?: AnyRecord;
};

export type ActivityAttentionItem = WorkItemActivity & {
  queue_types?: Array<'mentions' | 'questions' | 'blockers' | 'reminders' | 'unread' | string>;
};

export type EvidenceRecord = ProjectOperationsRecord & {
  work_item_id: string;
  target_type?: string;
  target_id?: string;
  evidence_type: 'document' | 'photo' | 'checklist' | 'link' | 'note' | 'other' | string;
  document_type?: string;
  name: string;
  document_id?: string;
  document_name?: string;
  title?: string;
  description?: string;
  file_url?: string;
  mime_type?: string;
  file_size?: number;
  storage_provider?: 'external' | 'local' | string;
  storage_key?: string;
  checksum_sha256?: string;
  caption?: string;
  revision_no?: string;
  visibility?: 'internal' | 'project' | 'client' | string;
  status: 'draft' | 'submitted' | 'accepted' | 'rejected' | string;
  uploaded_by?: string;
  submitted_by?: string;
  submitted_by_name?: string;
  reviewed_by?: string;
  reviewed_by_name?: string;
  reviewed_at?: string;
  payload_json?: AnyRecord;
};

export type PayableRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_code: string;
  vendor_type: string;
  vendor_name: string;
  source_type?: string;
  source_id?: string;
  gross_amount: number;
  advance_amount?: number;
  retention_amount?: number;
  deduction_amount?: number;
  paid_amount?: number;
  remaining_amount?: number;
  status: 'draft' | 'recorded' | 'approved' | 'partially_paid' | 'paid' | 'overdue' | 'cancelled' | string;
  invoice_count?: number;
  invoice_total?: number;
  verified_invoice_total?: number;
  latest_invoice_status?: string;
  latest_invoice_number?: string;
  latest_invoice_date?: string;
};

export type ReceivableRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_code: string;
  client_name: string;
  contract_amount?: number;
  milestone_name?: string;
  gross_amount: number;
  received_amount?: number;
  remaining_amount?: number;
  status: 'draft' | 'recorded' | 'approved' | 'partially_received' | 'received' | 'overdue' | 'cancelled' | string;
  invoice_count?: number;
  invoice_total?: number;
  issued_invoice_total?: number;
  latest_invoice_status?: string;
  latest_invoice_number?: string;
  latest_invoice_date?: string;
};

export type InputInvoiceRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  procurement_request_id?: string;
  procurement_request_code?: string;
  document_id?: string;
  document_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  invoice_date: string;
  vendor_name: string;
  vendor_tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  currency: 'VND' | string;
  status: 'draft' | 'verified' | 'rejected' | 'cancelled' | 'deleted' | string;
  notes?: string;
  storage_provider?: 'local' | string;
  storage_key?: string;
  mime_type?: string;
  file_name?: string;
  file_size?: number;
  checksum_sha256?: string;
  uploaded_by?: string;
  uploaded_by_name?: string;
  verified_by?: string;
  verified_by_name?: string;
  verified_at?: string;
  rejected_by?: string;
  rejected_by_name?: string;
  rejected_at?: string;
};

export type OutputInvoiceRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  payment_id?: string;
  payment_code?: string;
  payment_type?: string;
  document_id?: string;
  document_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  invoice_date: string;
  client_name: string;
  client_tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  currency: 'VND' | string;
  status: 'draft' | 'issued' | 'voided' | 'cancelled' | 'deleted' | string;
  notes?: string;
  storage_provider?: 'local' | string;
  storage_key?: string;
  mime_type?: string;
  file_name?: string;
  file_size?: number;
  checksum_sha256?: string;
  uploaded_by?: string;
  uploaded_by_name?: string;
  issued_by?: string;
  issued_by_name?: string;
  issued_at?: string;
  voided_by?: string;
  voided_by_name?: string;
  voided_at?: string;
};

export type WorkflowFormField = ProjectOperationsRecord & {
  form_version_id: string;
  field_key: string;
  label: string;
  field_type: 'text' | 'number' | 'date' | 'textarea' | 'select' | 'multi_select' | 'checkbox' | 'evidence_reference' | 'structured_list' | string;
  required: boolean;
  options_json?: Array<{ value: string; label?: string }> | string[];
  validation_json?: AnyRecord;
  help_text?: string;
  order_index?: number;
  visibility_condition_json?: AnyRecord;
};

export type WorkflowFormDetail = {
  available: boolean;
  instance?: AnyRecord & {
    id: string;
    project_id: string;
    work_item_id?: string;
    form_type: string;
    title: string;
    status: 'draft' | 'submitted' | 'reopened' | 'cancelled' | string;
    current_revision?: number;
    submitted_at?: string;
    updated_at?: string;
  };
  template?: AnyRecord & {
    id: string;
    code: string;
    name: string;
    description?: string;
    form_type: string;
  };
  version?: AnyRecord & {
    id: string;
    version_no: number;
    status: string;
  };
  fields?: WorkflowFormField[];
  answers?: AnyRecord;
  events?: AnyRecord[];
  signatures?: WorkflowFormSignature[];
  pdf_exports?: WorkflowFormPDFExport[];
};

export type WorkflowFormSignature = ProjectOperationsRecord & {
  project_id: string;
  work_item_id: string;
  form_instance_id: string;
  form_version_id: string;
  form_revision_no: number;
  purpose: 'internal_ack' | 'prepared_by' | 'reviewed_by' | 'approved_by' | 'received_by' | string;
  signer_user_id: string;
  signer_name: string;
  signer_roles_json?: string[] | AnyRecord[];
  comment?: string;
  payload_json?: AnyRecord;
  payload_hash: string;
  visual_storage_provider?: 'none' | 'local' | string;
  visual_storage_key?: string;
  visual_mime_type?: string;
  visual_file_size?: number;
  visual_checksum_sha256?: string;
  signed_at: string;
};

export type WorkflowFormPDFExport = ProjectOperationsRecord & {
  work_item_id: string;
  form_instance_id: string;
  form_version_id: string;
  form_revision_no: number;
  title: string;
  file_name: string;
  storage_provider: 'local' | string;
  storage_key: string;
  mime_type: string;
  file_size: number;
  checksum_sha256: string;
  payload_hash: string;
  metadata_json?: AnyRecord;
  generated_by?: string;
  generated_by_name?: string;
  generated_at: string;
};

export type SubcontractorReconciliationItem = ProjectOperationsRecord & {
  reconciliation_id?: string;
  work_item_id?: string;
  form_instance_id?: string;
  item_index: number;
  checklist_title: string;
  boq_item_id?: string;
  boq_item_ref?: string;
  boq_work_item_name?: string;
  current_boq_work_item_name?: string;
  boq_quantity?: number;
  current_boq_quantity?: number;
  boq_unit?: string;
  current_boq_unit?: string;
  claimed_quantity?: number;
  confirmed_quantity?: number;
  variance_quantity?: number;
  unit?: string;
  status?: string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  owner_response?: string;
  decision: string;
  due_date?: string;
  comment?: string;
  payload_json?: AnyRecord;
};

export type SubcontractorReconciliationRecord = ProjectOperationsRecord & {
  project_id: string;
  work_item_id: string;
  form_instance_id: string;
  form_revision_no: number;
  status: 'draft' | 'submitted' | 'reopened' | string;
  subcontractor_name?: string;
  work_package?: string;
  construction_area?: string;
  summary_json?: AnyRecord;
  comment?: string;
  submitted_by_name?: string;
  updated_by_name?: string;
  submitted_at?: string;
  items?: SubcontractorReconciliationItem[];
};

export type SubcontractorReconciliationResponse = {
  available: boolean;
  reason?: string;
  work_item_id?: string;
  project_id?: string;
  form?: WorkflowFormDetail;
  reconciliation?: SubcontractorReconciliationRecord;
  boq_items?: BOQItem[];
};

export type OwnerClientReconciliationItem = ProjectOperationsRecord & {
  reconciliation_id?: string;
  work_item_id?: string;
  form_instance_id?: string;
  item_index: number;
  checklist_title: string;
  output_ref?: string;
  status?: string;
  client_decision?: string;
  severity?: string;
  required_action?: string;
  due_date?: string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  acceptance_id?: string;
  acceptance_title?: string;
  current_acceptance_title?: string;
  acceptance_type?: string;
  current_acceptance_type?: string;
  acceptance_status?: string;
  current_acceptance_status?: string;
  acceptance_result?: string;
  current_acceptance_result?: string;
  decision: string;
  comment?: string;
  payload_json?: AnyRecord;
};

export type OwnerClientReconciliationRecord = ProjectOperationsRecord & {
  project_id: string;
  work_item_id: string;
  form_instance_id: string;
  form_revision_no: number;
  status: 'draft' | 'submitted' | 'reopened' | string;
  client_representative?: string;
  company_representative?: string;
  handover_scope?: string;
  scope_detail?: string;
  summary_json?: AnyRecord;
  comment?: string;
  submitted_by_name?: string;
  updated_by_name?: string;
  submitted_at?: string;
  items?: OwnerClientReconciliationItem[];
};

export type OwnerClientReconciliationResponse = {
  available: boolean;
  reason?: string;
  work_item_id?: string;
  project_id?: string;
  form?: WorkflowFormDetail;
  reconciliation?: OwnerClientReconciliationRecord;
  acceptances?: ProjectOperationsRecord[];
};

export type MaterialCatalogItem = ProjectOperationsRecord & {
  code: string;
  name: string;
  unit: string;
  category?: string;
  default_supplier?: string;
  standard_price?: number;
  active?: boolean;
};

export type ProjectMaterialBalance = ProjectOperationsRecord & {
  material_id: string;
  code?: string;
  material_name?: string;
  unit?: string;
  category?: string;
  planned_quantity?: number;
  issued_quantity?: number;
  used_quantity?: number;
  surplus_quantity?: number;
  surplus_status?: string;
};

export type MaterialSnapshotItem = ProjectOperationsRecord & {
  snapshot_id?: string;
  work_item_id?: string;
  item_index: number;
  material_id?: string;
  material_code?: string;
  material_name: string;
  unit: string;
  source: 'site_balance' | 'warehouse_issue' | 'subcontractor' | 'other' | string;
  before_quantity: number;
  added_quantity: number;
  after_quantity: number;
  waste_quantity: number;
  used_quantity: number;
  affects_site_balance: boolean;
  site_balance_id?: string;
  before_evidence_id?: string;
  before_evidence_name?: string;
  before_evidence_status?: string;
  after_evidence_id?: string;
  after_evidence_name?: string;
  after_evidence_status?: string;
  construction_area?: string;
  condition: 'usable' | 'damaged' | 'mixed' | 'unknown' | string;
  comment?: string;
  payload_json?: AnyRecord;
};

export type MaterialSnapshotRecord = ProjectOperationsRecord & {
  project_id: string;
  work_item_id: string;
  status: 'draft' | 'submitted' | 'reopened' | string;
  snapshot_date?: string;
  construction_area?: string;
  summary_json?: AnyRecord;
  comment?: string;
  applied_to_site_balance?: boolean;
  submitted_by_name?: string;
  updated_by_name?: string;
  submitted_at?: string;
  items?: MaterialSnapshotItem[];
};

export type MaterialSnapshotResponse = {
  available: boolean;
  reason?: string;
  work_item_id?: string;
  project_id?: string;
  snapshot?: MaterialSnapshotRecord;
  materials?: MaterialCatalogItem[];
  material_balances?: ProjectMaterialBalance[];
};

export type Warehouse = ProjectOperationsRecord & {
  name: string;
  location?: string;
  type?: string;
  active?: boolean;
};

export type SurplusHandlingRecord = ProjectOperationsRecord & {
  handling_code: string;
  work_item_id: string;
  site_balance_id: string;
  material_id: string;
  material_code?: string;
  material_name?: string;
  material_unit?: string;
  material_snapshot_id?: string;
  material_snapshot_item_id?: string;
  stock_movement_id?: string;
  stock_movement_code?: string;
  stock_movement_type?: string;
  status: 'draft' | 'submitted' | 'cancelled' | string;
  handling_type: 'return_to_warehouse' | 'return_to_subcontractor' | string;
  quantity: number;
  unit: string;
  warehouse_id?: string;
  warehouse_name?: string;
  custody_location?: string;
  condition: 'usable' | 'damaged' | 'mixed' | 'unknown' | string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  comment?: string;
  submitted_by?: string;
  submitted_at?: string;
};

export type SurplusHandlingResponse = {
  available: boolean;
  work_item_id?: string;
  project_id?: string;
  material_balances?: ProjectMaterialBalance[];
  warehouses?: Warehouse[];
  handlings?: SurplusHandlingRecord[];
};

export type IncomingAcceptanceItem = ProjectOperationsRecord & {
  incoming_acceptance_id?: string;
  project_id?: string;
  work_item_id?: string;
  item_index: number;
  material_id?: string;
  material_code?: string;
  material_name: string;
  procurement_request_item_id?: string;
  delivered_quantity: number;
  accepted_quantity: number;
  rejected_quantity: number;
  unit: string;
  condition: 'usable' | 'damaged' | 'mixed' | 'unknown' | string;
  defect_note?: string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  decision: 'pending' | 'accepted' | 'conditional_acceptance' | 'rejected' | string;
  comment?: string;
  procurement_quantity_requested?: number;
  procurement_delivery_status?: string;
  procurement_approval_status?: string;
};

export type IncomingAcceptanceRecord = ProjectOperationsRecord & {
  acceptance_code: string;
  project_id: string;
  work_item_id?: string;
  procurement_request_id?: string;
  procurement_request_code?: string;
  procurement_request_title?: string;
  warehouse_id?: string;
  warehouse_name?: string;
  source_type: 'supplier' | 'subcontractor' | 'procurement' | 'other' | string;
  supplier_name?: string;
  subcontractor_name?: string;
  delivery_reference?: string;
  status: 'draft' | 'submitted' | 'cancelled' | string;
  result?: 'pass' | 'conditional_pass' | 'fail' | string;
  received_at?: string;
  summary_json?: AnyRecord;
  comment?: string;
  item_count?: number;
  submitted_by_name?: string;
  submitted_at?: string;
  items?: IncomingAcceptanceItem[];
};

export type ProcurementRequestOption = ProjectOperationsRecord & {
  project_id: string;
  request_code?: string;
  title?: string;
  status?: string;
  item_count?: number;
};

export type ProcurementRequestItemOption = ProjectOperationsRecord & {
  request_id: string;
  request_code?: string;
  procurement_request_title?: string;
  material_id?: string;
  material_code?: string;
  material_name?: string;
  work_item_name?: string;
  quantity_requested?: number;
  unit?: string;
  delivery_status?: string;
  approval_status?: string;
};

export type IncomingAcceptanceResponse = {
  available: boolean;
  work_item_id?: string;
  project_id?: string;
  records?: IncomingAcceptanceRecord[];
  materials?: MaterialCatalogItem[];
  warehouses?: Warehouse[];
  procurement_requests?: ProcurementRequestOption[];
  procurement_request_items?: ProcurementRequestItemOption[];
};

export type AcceptanceChecklistItem = ProjectOperationsRecord & {
  acceptance_id?: string;
  item_index: number;
  title: string;
  description?: string;
  is_passed?: boolean;
  note?: string;
  photo_required?: boolean;
  photo_url?: string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  decision: 'pending' | 'pass' | 'conditional_pass' | 'fail' | string;
  severity?: 'low' | 'medium' | 'high' | 'critical' | string;
  required_action?: string;
  due_date?: string;
  payload_json?: AnyRecord;
};

export type AcceptanceRecord = ProjectOperationsRecord & {
  acceptance_code?: string;
  project_id: string;
  node_id?: string;
  work_item_id?: string;
  work_item_title?: string;
  form_instance_id?: string;
  workflow_form_title?: string;
  pdf_export_id?: string;
  pdf_file_name?: string;
  type: 'partial' | 'final' | string;
  title: string;
  requested_by?: string;
  requested_by_name?: string;
  inspected_by?: string;
  approved_by?: string;
  approved_by_name?: string;
  submitted_by?: string;
  submitted_by_name?: string;
  status: 'draft' | 'submitted' | 'completed' | 'cancelled' | string;
  result?: 'pass' | 'conditional_pass' | 'fail' | string;
  acceptance_date?: string;
  location?: string;
  client_representative?: string;
  contractor_representative?: string;
  scope_summary?: string;
  signature_status?: 'pending' | 'signed' | 'waived' | string;
  document_status?: 'pending' | 'linked' | 'waived' | string;
  notes?: string;
  summary_json?: AnyRecord;
  checklist_count?: number;
  submitted_at?: string;
  checklist?: AcceptanceChecklistItem[];
};

export type OutgoingAcceptanceResponse = {
  available: boolean;
  work_item_id?: string;
  project_id?: string;
  records?: AcceptanceRecord[];
  form?: WorkflowFormDetail & { available?: boolean };
};

export type WorkItem = ProjectOperationsRecord & {
  title: string;
  description?: string;
  status: string;
  work_type?: string;
  source_type?: string;
  source_id?: string;
  project_name?: string;
  project_code?: string;
  queue_scopes?: string[];
  priority?: string;
  department?: string;
  role_ids?: string[];
  assignee_staff_id?: string;
  assignee_name?: string;
  watcher_staff_ids?: string[];
  planned_start_at?: string;
  due_at?: string;
  started_at?: string;
  completed_at?: string;
  blocked_at?: string;
  blocked_reason?: string;
  pipeline_run_id?: string;
  run_node_id?: string;
  run_node_name?: string;
  run_step_id?: string;
  run_step_name?: string;
  evidence_count?: number;
  approval_request_id?: string;
  activities?: WorkItemActivity[];
  evidence?: EvidenceRecord[];
  assignments?: ProjectOperationsRecord[];
};

export type ProjectOperationsVisibility = AnyRecord & {
  scope: 'admin' | 'director' | 'project_member' | string;
  scoped: boolean;
  can_view_all_project: boolean;
  department?: string;
  permissions?: string[];
  role_labels?: string[];
  hidden_sections?: string[];
};

export type ProjectOperationsHealthCounts = AnyRecord & {
  active: number;
  blocked: number;
  completed: number;
  overdue: number;
  waiting_approval: number;
  waiting_evidence: number;
  open_issues?: number;
  failed_acceptances?: number;
  pending_procurement?: number;
  material_risks?: number;
};

export type ProjectOperationsTimelineNode = AnyRecord & {
  id: string;
  name: string;
  status: string;
  order_index?: number;
  type?: string;
  step_count?: number;
  completed_steps?: number;
  started_at?: string;
  completed_at?: string;
  progress_percent?: number;
};

export type ProjectOperationsResponse = AnyRecord & {
  project: Project;
  project_summary?: AnyRecord;
  visibility: ProjectOperationsVisibility;
  health_counts: ProjectOperationsHealthCounts;
  workflow_timeline: ProjectOperationsTimelineNode[];
  active_work: WorkItem[];
  blockers: WorkItem[];
  approvals: ProjectOperationsRecord[];
  latest_evidence: EvidenceRecord[];
  activity_summary: AnyRecord;
  latest_activity: WorkItemActivity[];
  audit_summary: AnyRecord;
  department_surfaces: ProjectOperationsRecord[];
  pipeline_run?: AnyRecord | null;
  summary?: AnyRecord;
  work_items: WorkItem[];
};
