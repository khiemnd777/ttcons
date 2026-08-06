export type AnyRecord = Record<string, any>;

export type AuditLogRecord = AnyRecord & {
  id: string;
  project_id?: string;
  project_code?: string;
  project_name?: string;
  actor_id?: string;
  actor_name?: string;
  event_type: string;
  target_type?: string;
  target_id?: string;
  description: string;
  metadata_json?: Record<string, unknown>;
  created_at: string;
};

export type AuditLogPage = {
  items: AuditLogRecord[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
  options: {
    event_types: string[];
    target_types: string[];
    actors: string[];
    projects: Array<{ id: string; code: string; name: string }>;
  };
};

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
  partner_id?: string;
  client_name: string;
  partner_name?: string;
  partner_tax_code?: string;
  pm_name?: string;
  status: string;
  progress_percent: number;
	workflow_entry_mode: 'survey_required' | 'client_boq';
  planned_start_date?: string | null;
  planned_end_date?: string | null;
  open_issues?: number;
  pending_approvals?: number;
};

export type ProjectProgressStageKey =
  | 'not_started'
  | 'survey'
  | 'design'
  | 'commercial'
  | 'planning'
  | 'construction'
  | 'acceptance'
  | 'handover_finance'
  | 'completed'
  | 'other';

export type ProjectProgressItem = {
  id: string;
  code: string;
  name: string;
  client_name?: string;
  project_status: string;
  progress_percent: number;
  planned_start_date?: string | null;
  planned_end_date?: string | null;
  pm_name?: string;
  pipeline_run_id?: string;
  pipeline_run_status?: string;
  current_node_id?: string;
  current_node_name?: string;
  current_node_type?: string;
  current_node_status?: string;
  current_node_order?: number;
  current_node_block_reason?: string;
  open_issues: number;
  pending_approvals: number;
  blocked_items: number;
  overdue_items: number;
  stage_key: ProjectProgressStageKey;
};

export type ProjectProgressBoardResponse = {
  items: ProjectProgressItem[];
  generated_at: string;
};

export type ProjectMemberAccessRole = {
  id: string;
  name: string;
  display_name: string;
  scope: 'global' | 'project';
  description?: string;
  grants_project_view?: boolean;
};

export type ProjectMember = {
  id: string;
  project_id: string;
  staff_id: string;
  user_id?: string;
  full_name: string;
  email?: string;
  phone?: string;
  department?: string;
  position?: string;
  role_ids: string[];
  global_roles: ProjectMemberAccessRole[];
  project_roles: ProjectMemberAccessRole[];
  notes?: string;
  active: boolean;
  is_project_manager: boolean;
  created_at?: string;
  updated_at?: string;
};

export type ProjectMemberStaffOption = {
  id: string;
  user_id: string;
  full_name: string;
  email?: string;
  department?: string;
  position?: string;
  member_id?: string;
  member_active?: boolean;
  global_roles: ProjectMemberAccessRole[];
};

export type ProjectMembersResponse = {
  items: ProjectMember[];
  summary: {
    active: number;
    inactive: number;
    departments: number;
    roles: number;
  };
  options: {
    staff: ProjectMemberStaffOption[];
    access_roles: ProjectMemberAccessRole[];
    role_labels: string[];
  };
  capabilities: {
    can_manage: boolean;
    can_view_inactive: boolean;
  };
};

export type Partner = AnyRecord & {
  id: string;
  partner_code: string;
  name: string;
  legal_name?: string;
  partner_types: string[];
  tax_code?: string;
  phone?: string;
  email?: string;
  address?: string;
  contact_name?: string;
  payment_terms_days?: number;
  notes?: string;
  active: boolean;
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
  approval_policy_json?: AnyRecord;
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
  revision_no?: number;
  template_family_id?: string;
  source_template_id?: string;
  engine_version?: number;
  definition_hash?: string;
  published_at?: string;
  published_by?: string;
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
  latest_published?: boolean;
  step_usage_count?: number;
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

export type WorkflowFormTemplateLibraryItem = AnyRecord & {
  template_id: string;
  template_code: string;
  template_name: string;
  template_description?: string;
  form_type: 'checklist' | 'daily_report' | 'meeting_minutes' | 'generic_submission' | string;
  template_status: 'active' | 'archived' | string;
  latest_version_id?: string;
  latest_version_no?: number;
  latest_version_status?: string;
  version_count?: number;
  published_version_count?: number;
  draft_version_count?: number;
  field_count?: number;
  step_usage_count?: number;
  instance_count?: number;
};

export type WorkflowFormManagedVersion = AnyRecord & {
  version_id: string;
  template_id: string;
  template_code?: string;
  template_name?: string;
  form_type?: string;
  template_status?: string;
  version_no: number;
  version_status: 'draft' | 'published' | 'archived' | string;
  schema_json?: AnyRecord;
  validation_json?: AnyRecord;
  published_at?: string;
  latest_published?: boolean;
  step_usage_count?: number;
  instance_count?: number;
  field_count?: number;
  fields?: WorkflowFormField[];
};

export type WorkflowFormManagedTemplate = WorkflowFormTemplateLibraryItem & {
  created_by?: string;
  created_at?: string;
  updated_at?: string;
  versions?: WorkflowFormManagedVersion[];
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
  price_event_count?: number;
  price_events_truncated?: boolean;
  selected_quotation_item_id?: string;
  selected_quotation_code?: string;
  selected_supplier_partner_id?: string;
  selected_supplier_name?: string;
  quoted_unit_price?: number;
  price_waiver_reason?: string;
  procurement_request_id?: string;
  procurement_request_code?: string;
  procurement_request_status?: string;
  has_active_procurement?: boolean;
  active_procurement_count?: number;
};

export type BOQProcurementResult = {
  id?: string;
  request_code?: string;
  status?: string;
  requests?: Array<{ id: string; request_code?: string; status?: string; supplier_name?: string }>;
  created_requests?: Array<{ id: string; request_code?: string; status?: string; supplier_name?: string }>;
};

export type SupplierQuotationItem = ProjectOperationsRecord & {
  quotation_id: string;
  boq_item_id: string;
  material_id?: string;
  description: string;
  unit: string;
  quantity: number;
  unit_price: number;
  discount_amount: number;
  tax_rate: number;
  amount_before_tax: number;
  total_amount: number;
  lead_time_days: number;
  compliance_status: string;
  deviation_note?: string;
  work_item_name?: string;
  variance_to_norm?: number;
};

export type SupplierQuotation = ProjectOperationsRecord & {
  quotation_code: string;
  supplier_partner_id: string;
  supplier_name: string;
  supplier_tax_code?: string;
  quotation_date: string;
  valid_until?: string;
  currency: string;
  payment_terms_days: number;
  lead_time_days: number;
  delivery_terms?: string;
  warranty_terms?: string;
  status: string;
  technical_status: string;
  commercial_status: string;
  notes?: string;
  item_count?: number;
  amount_before_tax?: number;
  total_amount?: number;
  items?: SupplierQuotationItem[];
};

export type SupplierQuoteComparison = {
  boq_item_id: string;
  status: string;
  category: string;
  work_item_name: string;
  unit: string;
  quantity: number;
  expected_unit_price: number;
  norm_unit_price?: number;
  final_unit_price?: number;
  norm_price_locked: boolean;
  final_price_locked: boolean;
  selected_quotation_item_id?: string;
  price_waiver_reason?: string;
  quotations: Array<{
    quotation_item_id: string;
    quotation_id: string;
    quotation_code: string;
    supplier_partner_id: string;
    supplier_name: string;
    quotation_status: string;
    valid_until?: string;
    unit_price: number;
    discount_amount: number;
    tax_rate: number;
    total_amount: number;
    lead_time_days: number;
    compliance_status: string;
    variance_to_norm: number;
    is_selected: boolean;
  }>;
};

export type ContractVersion = ProjectOperationsRecord & {
  contract_id: string;
  version_no: number;
  version_label: string;
  source_variation_id?: string;
  amount: number;
  scope_summary: string;
  commercial_terms_json?: Record<string, any>;
  payment_terms_json?: Record<string, any>;
  planned_start_date?: string;
  planned_end_date?: string;
  document_id?: string;
  definition_hash: string;
  status: string;
  review_comment?: string;
  signature_comment?: string;
};

export type ContractMilestone = ProjectOperationsRecord & {
  contract_id: string;
  contract_version_id: string;
  milestone_code: string;
  milestone_name: string;
  sequence_no: number;
  milestone_type: string;
  amount_basis: string;
  percentage?: number;
  gross_amount: number;
  retention_percent: number;
  retention_amount: number;
  net_amount: number;
  planned_date?: string;
  due_days: number;
  acceptance_required: boolean;
  acceptance_id?: string;
  status: string;
  obligation_type?: string;
  obligation_id?: string;
};

export type ContractVariation = ProjectOperationsRecord & {
  contract_id: string;
  variation_code: string;
  title: string;
  variation_type: string;
  reason: string;
  amount_delta: number;
  days_delta: number;
  scope_delta?: string;
  status: string;
  resulting_version_id?: string;
  priority: 'normal' | 'urgent' | string;
  source_work_item_id?: string;
	  source_work_item_title?: string;
	  source_construction_action_id?: string;
	  source_construction_action_title?: string;
  procurement_request_id?: string;
  qs_comment?: string;
  purchasing_comment?: string;
  events?: AnyRecord[];
};

export type ContractWorkflowSignature = AnyRecord & {
  workflow_form_signature_id: string;
  contract_version_id: string;
  form_instance_id: string;
  signer_name: string;
  signed_at: string;
  payload_hash: string;
};

export type ContractSigningForm = AnyRecord & {
  id: string;
  title: string;
  status: string;
  current_revision?: number;
  work_item_id: string;
  signatures?: Array<AnyRecord & { id: string; purpose: string; signer_user_id?: string; linked_contract_version_id?: string }>;
};

export type ContractDispatch = AnyRecord & {
  id: string;
  contract_id: string;
  contract_version_id: string;
  dispatch_no: number;
  recipient_name: string;
  delivery_method: string;
  tracking_reference?: string;
  status: 'sent' | 'received' | string;
  sent_at: string;
  received_at?: string;
  returned_scan_document_id?: string;
  returned_scan_document_name?: string;
};

export type ContractRetention = ProjectOperationsRecord & {
  contract_id: string;
  source_milestone_id: string;
  milestone_code: string;
  milestone_name: string;
  direction: string;
  held_amount: number;
  released_amount: number;
  outstanding_amount: number;
  warranty_end_date?: string;
  status: string;
  releases?: Array<Record<string, any>>;
};

export type Contract = ProjectOperationsRecord & {
  contract_code: string;
  title: string;
  contract_type: string;
  direction: string;
  counterparty_partner_id?: string;
  counterparty_name: string;
  currency: string;
  original_amount: number;
  current_amount: number;
  advance_percent: number;
  retention_percent: number;
  warranty_days: number;
  signed_date?: string;
  effective_date?: string;
  expiry_date?: string;
  status: string;
  current_version_id?: string;
  current_version_no?: number;
  current_version_label?: string;
  current_version_status?: string;
  milestone_count?: number;
  variation_count?: number;
  signed_scan_received?: boolean;
  workflow_signatures?: ContractWorkflowSignature[];
	signing_forms?: ContractSigningForm[];
	dispatches?: ContractDispatch[];
	execution_readiness?: {
		app_signed: boolean;
		paper_dispatched: boolean;
		returned_scan_received: boolean;
		contract_active: boolean;
	};
  retention_outstanding?: number;
  versions?: ContractVersion[];
  milestones?: ContractMilestone[];
  variations?: ContractVariation[];
  retentions?: ContractRetention[];
};

export type QualityITPItem = ProjectOperationsRecord & {
  itp_plan_id: string;
  sequence_no: number;
  activity_name: string;
  inspection_stage: string;
  acceptance_criteria: string;
  inspection_method: string;
  control_point: string;
  frequency_type: string;
  frequency_value: number;
  responsible_role: string;
  evidence_required: boolean;
  client_witness_required: boolean;
};

export type QualityITPPlan = ProjectOperationsRecord & {
  family_id: string;
  itp_code: string;
  title: string;
  discipline: string;
  work_package: string;
  revision_no: number;
  definition_hash: string;
  status: string;
  item_count: number;
  items: QualityITPItem[];
};

export type QualityInspectionRequestRecord = ProjectOperationsRecord & {
  rfi_code: string;
  itp_plan_id: string;
  itp_item_id: string;
  itp_code: string;
  activity_name: string;
  control_point: string;
  work_item_id?: string;
  work_item_title?: string;
  location: string;
  lot_reference?: string;
  requested_inspection_at: string;
  status: string;
  result?: string;
  evidence_required: boolean;
};

export type QualityNCRRecord = ProjectOperationsRecord & {
  ncr_code: string;
  source_type: string;
  source_id: string;
  rfi_id?: string;
  rfi_code?: string;
  work_item_id?: string;
  rework_request_id?: string;
  rework_status?: string;
  title: string;
  severity: string;
  nonconformity: string;
  root_cause?: string;
  corrective_action?: string;
  due_date?: string;
  status: string;
};

export type RecurringPlan = ProjectOperationsRecord & {
  plan_code: string;
  title: string;
  recurrence_type: string;
  interval_value: number;
  start_date: string;
  end_date?: string;
  next_due_date: string;
  responsible_role: string;
  assigned_user_id?: string;
  assigned_user_name?: string;
  evidence_required: boolean;
  status: string;
  pending_count: number;
  overdue_count: number;
  completed_count: number;
};

export type RecurringOccurrence = ProjectOperationsRecord & {
  plan_id: string;
  plan_code: string;
  plan_title: string;
  occurrence_date: string;
  due_at?: string;
  assigned_user_id?: string;
  assigned_user_name?: string;
  evidence_required: boolean;
  status: string;
  effective_status: string;
};

export type QualityWorkspace = {
  itp_plans: QualityITPPlan[];
  inspection_requests: QualityInspectionRequestRecord[];
  ncrs: QualityNCRRecord[];
  recurring_plans: RecurringPlan[];
  recurring_occurrences: RecurringOccurrence[];
  evidence_options: Array<ProjectOperationsRecord & { work_item_id?: string; name: string; evidence_type: string; status: string; work_item_title?: string }>;
  work_items: Array<ProjectOperationsRecord & { title: string; status: string; work_type: string }>;
  participants: Array<{ id: string; full_name: string; email: string }>;
};

export type ConstructionCheckCriterion = {
  title: string;
  criteria?: string;
  evidence_required?: boolean;
};

export type ConstructionHandoffCandidate = {
  source_field_key: string;
  source_item_index: number;
  source_form_code?: string;
  title: string;
  location?: string;
  scheduled_at?: string;
  responsible_role: string;
  assigned_user_id?: string;
  evidence_required?: boolean;
  criteria: ConstructionCheckCriterion[];
};

export type ConstructionHandoffItem = ProjectOperationsRecord & {
  source_field_key: string;
  source_item_index: number;
  title: string;
  location?: string;
  scheduled_at?: string;
  responsible_role_ids: string[];
  assigned_user_id?: string;
  assigned_user_name?: string;
  evidence_required: boolean;
  criteria_jsonb: ConstructionCheckCriterion[];
  generated_work_item_id: string;
  generated_check_id: string;
  work_item_status?: string;
  check_status?: string;
  check_result?: 'pass' | 'fail';
  status: 'activated' | 'completed' | 'cancelled';
};

export type ConstructionHandoff = ProjectOperationsRecord & {
  pipeline_run_id: string;
  source_run_node_id?: string;
  source_node_name?: string;
  target_run_node_id: string;
  target_node_name?: string;
  source_form_instance_id?: string;
  source_form_revision_no?: number;
  revision_no: number;
  title: string;
  notes?: string;
  status: 'draft' | 'activated' | 'cancelled';
  activated_by_name?: string;
  activated_at?: string;
  items: ConstructionHandoffItem[];
  idempotent_replay?: boolean;
};

export type ConstructionHandoffWorkspace = {
  available: boolean;
  can_activate: boolean;
  reason?: string;
  pipeline_run_id?: string;
  source_run_node_id?: string;
  source_node_name?: string;
  source_node_type?: string;
  source_node_status?: string;
  target_run_node_id?: string;
  target_node_name?: string;
  target_node_status?: string;
  source_form_instance_id?: string;
  source_form_revision_no?: number;
  suggested_title?: string;
  candidates: ConstructionHandoffCandidate[];
  handoffs: ConstructionHandoff[];
  participants: Array<{ id: string; full_name: string; email: string }>;
};

export type ConstructionCheckTemplate = ProjectOperationsRecord & {
  name: string;
  description?: string;
  criteria_jsonb: ConstructionCheckCriterion[];
  status: 'active' | 'archived';
};

export type ConstructionCheck = ProjectOperationsRecord & {
  work_item_id: string;
  work_item_title?: string;
  source_daily_report_id?: string;
  source_report_date?: string;
  source_report_weather?: string;
  template_id?: string;
  template_name?: string;
  title: string;
  location?: string;
  criteria_jsonb: ConstructionCheckCriterion[];
  scheduled_at?: string;
  status: 'pending' | 'completed' | 'cancelled';
  result?: 'pass' | 'fail';
  note?: string;
  evidence_id?: string;
  evidence_name?: string;
  context_evidence: Array<EvidenceRecord & { evidence_role?: 'context'; sort_order?: number }>;
  result_evidence: Array<EvidenceRecord & { evidence_role?: 'result'; sort_order?: number }>;
  requested_by_name?: string;
  checked_by_name?: string;
  checked_at?: string;
  technical_rfi_id?: string;
  source_handoff_item_id?: string;
  handoff_id?: string;
  handoff_title?: string;
  handoff_revision_no?: number;
  handoff_source_field_key?: string;
  handoff_source_item_index?: number;
  handoff_source_node_name?: string;
};

export type ConstructionActionEvent = ProjectOperationsRecord & {
  event_type: 'created' | 'resolution_submitted' | 'verified_pass' | 'verified_fail' | 'impact_approved';
  comment?: string;
  evidence_id?: string;
  evidence_name?: string;
  actor_name?: string;
};

export type ConstructionActionItem = ProjectOperationsRecord & {
  work_item_id: string;
  work_item_title?: string;
  source_daily_report_id?: string;
  source_report_date?: string;
  source_report_weather?: string;
  source_report_link_kind?: 'direct' | 'check';
  source_check_id?: string;
  source_check_title?: string;
  title: string;
  description?: string;
  priority: 'normal' | 'urgent';
  assigned_user_id: string;
  assigned_user_name?: string;
  due_at?: string;
  status: 'open' | 'waiting_confirmation' | 'done' | 'cancelled';
  impact_scope: 'none' | 'timeline' | 'budget' | 'design_scope';
  impact_status?: string;
  before_evidence_id?: string;
  before_evidence_name?: string;
  resolution_evidence_id?: string;
  resolution_evidence_name?: string;
  resolution_note?: string;
  retry_count: number;
  technical_issue_id?: string;
  technical_ncr_id?: string;
  technical_rework_id?: string;
  technical_replanning_id?: string;
	  linked_variation_id?: string;
	  linked_variation_code?: string;
	  linked_variation_status?: string;
	  linked_variation_priority?: string;
  events: ConstructionActionEvent[];
};

export type ConstructionDailyReportSource = {
  id: string;
  report_date: string;
  weather?: string;
  work_item_id?: string;
  primary_evidence_id?: string;
  evidence: EvidenceRecord[];
  suggested_title?: string;
};

export type ConstructionWorkspace = {
	  runtime: {
	    has_pipeline_run: boolean;
	    pipeline_run_id?: string;
	    pipeline_run_status?: string;
	    engine_version?: number;
	    current_node_id?: string;
	    current_node_name?: string;
	    current_node_type?: string;
	    current_node_business_code?: string;
	    current_node_status?: string;
	    construction_active: boolean;
	  };
  templates: ConstructionCheckTemplate[];
  checks: ConstructionCheck[];
  actions: ConstructionActionItem[];
	  work_items: Array<ProjectOperationsRecord & {
	    title: string;
	    status: string;
	    work_type: string;
	    source_type?: string;
	    source_id?: string;
	    run_node_id?: string;
	    run_step_id?: string;
	    pipeline_engine_version?: number;
	    run_node_type?: string;
	    run_node_business_code?: string;
	    run_step_business_code?: string;
	    report_eligible: boolean;
	  }>;
  participants: Array<{ id: string; full_name: string; email: string }>;
};

export type ConstructionVariationContext = {
	  available: boolean;
	  can_propose: boolean;
	  contract?: Pick<Contract, 'id' | 'contract_code' | 'title' | 'contract_type' | 'direction' | 'counterparty_name' | 'current_amount' | 'status'> | null;
	  variations: ContractVariation[];
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
  parent_activity_id?: string;
  reply_to?: ActivityReplyPreview;
  reply_count?: number;
  reactions?: ActivityReactionSummary[];
	edited_at?: string;
	edited_by?: string;
	withdrawn_at?: string;
	withdrawn_by?: string;
	message_revision_no?: number;
	can_edit?: boolean;
	can_withdraw?: boolean;
};

export type ActivityReplyPreview = {
  id: string;
  actor_id?: string;
  actor_name?: string;
  activity_type: string;
  comment?: string;
  created_at?: string;
	withdrawn?: boolean;
};

export type ActivityReactionSummary = {
  reaction: '👍' | '👀' | '🙏' | '🎉';
  count: number;
  reacted_by_me: boolean;
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
  parent_activity_id?: string;
  mentions?: ActivityMentionPayload[];
  attachments?: ActivityAttachmentPayload[];
  metadata?: AnyRecord;
};

export type ProjectActivitySummary = {
  total: number;
  discussion: number;
  system: number;
  decisions: number;
  unread: number;
  mentioned: number;
  questions: number;
  blockers: number;
  reminders: number;
  attention: number;
};

export type ProjectActivityFeed = {
  items: WorkItemActivity[];
  summary: ProjectActivitySummary;
  has_more: boolean;
  next_cursor?: string;
};

export type ProjectActivityMemberOption = {
  id: string;
  user_id?: string;
  label: string;
  department?: string;
  position?: string;
};

export type ProjectActivityRoleOption = {
  id: string;
  label: string;
};

export type ProjectActivityTargetOption = {
  scope_type: string;
  scope_id: string;
  label: string;
  work_item_id?: string;
  status?: string;
};

export type ProjectActivityAttachmentOption = {
  type: ActivityAttachmentPayload['type'];
  id: string;
  label: string;
  status?: string;
  visibility?: string;
  work_item_id?: string;
};

export type ProjectActivityOptions = {
  members: ProjectActivityMemberOption[];
  roles: ProjectActivityRoleOption[];
  targets: ProjectActivityTargetOption[];
  attachments: ProjectActivityAttachmentOption[];
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
  work_item_title?: string;
  work_item_status?: string;
  run_node_name?: string;
  can_review?: boolean;
  payload_json?: AnyRecord;
};

export type ProjectEvidenceSummary = {
  total: number;
  waiting: number;
  accepted: number;
  rejected: number;
  photos: number;
  documents: number;
};

export type ProjectEvidenceOption = {
  id: string;
  label: string;
};

export type ProjectEvidencePage = {
  items: EvidenceRecord[];
  summary: ProjectEvidenceSummary;
  options: {
    work_items: ProjectEvidenceOption[];
    submitters: ProjectEvidenceOption[];
  };
  filtered_total: number;
  has_more: boolean;
  next_cursor?: string;
};

export type PayableRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_id?: string;
  cost_account_id?: string;
  project_name?: string;
  payable_code: string;
  vendor_type: string;
  vendor_partner_id?: string;
  vendor_name: string;
  source_type?: string;
  source_id?: string;
  gross_amount: number;
  advance_amount?: number;
  retention_amount?: number;
  deduction_amount?: number;
  paid_amount?: number;
  remaining_amount?: number;
  due_date?: string;
  status: 'draft' | 'recorded' | 'approved' | 'partially_paid' | 'paid' | 'overdue' | 'cancelled' | string;
  invoice_count?: number;
  invoice_total?: number;
  verified_invoice_total?: number;
  schedule_count?: number;
  scheduled_total?: number;
  scheduled_remaining?: number;
  advance_request_count?: number;
  advance_requested_total?: number;
  advance_approved_total?: number;
  advance_paid_total?: number;
  advance_settled_total?: number;
  approved_payment_total?: number;
  latest_invoice_status?: string;
  latest_invoice_number?: string;
  latest_invoice_date?: string;
  effective_due_date?: string;
  overdue_days?: number;
  aging_bucket?: 'settled' | 'no_due_date' | 'not_due' | '1_30' | '31_60' | '61_90' | '90_plus' | string;
  reconciliation_issue_count?: number;
  reconciliation_status?: 'ok' | 'needs_review' | string;
};

export type PayableReconciliationWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  details?: AnyRecord;
};

export type PayableLedgerEntry = AnyRecord & {
  occurred_at: string;
  entry_type: string;
  source_id: string;
  source_code: string;
  status: string;
  description: string;
  debit_amount: number;
  credit_amount: number;
  document_amount: number;
  cash_amount: number;
  running_balance: number;
  actor_name?: string;
};

export type PayableReconciliationSnapshot = AnyRecord & {
  summary: PayableRecord & {
    verified_invoice_count?: number;
    rejected_invoice_count?: number;
    overdue_schedule_count?: number;
    scheduled_paid_total?: number;
    next_schedule_due_date?: string;
    approved_payable_payment_total?: number;
    approved_advance_payment_total?: number;
    payment_allocation_total?: number;
    pending_payment_count?: number;
    computed_remaining?: number;
    balance_difference?: number;
    unverified_invoice_gap?: number;
    advance_unpaid_total?: number;
    advance_unsettled_total?: number;
    warning_count?: number;
  };
  warnings: PayableReconciliationWarning[];
  ledger: PayableLedgerEntry[];
};

export type PayablePaymentScheduleRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  vendor_name?: string;
  schedule_code: string;
  milestone_name: string;
  schedule_type: 'advance' | 'progress' | 'final' | 'retention' | 'other' | string;
  planned_amount: number;
  paid_amount: number;
  remaining_amount: number;
  due_date?: string;
  status: 'draft' | 'approved' | 'partially_paid' | 'paid' | 'overdue' | 'cancelled' | 'deleted' | string;
  notes?: string;
  created_by_name?: string;
  approved_by_name?: string;
};

export type PayableAdvanceRequestRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  vendor_name?: string;
  payable_vendor_name?: string;
  request_code: string;
  requested_by?: string;
  requested_by_name?: string;
  amount: number;
  paid_amount?: number;
  settled_amount: number;
  reason?: string;
  due_date?: string;
  paid_at?: string;
  settled_at?: string;
  status: 'draft' | 'submitted' | 'approved' | 'partially_settled' | 'settled' | 'cancelled' | 'deleted' | string;
  payment_id?: string;
  payment_code?: string;
  notes?: string;
  approved_by_name?: string;
};

export type PayablePaymentRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  payable_vendor_name?: string;
  payment_code: string;
  payment_type: string;
  payment_method: string;
  cash_account_id?: string;
  target_type?: string;
  target_id?: string;
  amount: number;
  allocation_id?: string;
  allocation_type?: 'payable_payment' | 'advance_payment' | 'advance_settlement' | string;
  allocation_amount?: number;
  allocation_notes?: string;
  schedule_id?: string;
  schedule_code?: string;
  milestone_name?: string;
  advance_request_id?: string;
  advance_request_code?: string;
  input_invoice_id?: string;
  input_invoice_number?: string;
  status: 'draft' | 'submitted' | 'pending' | 'approved' | 'paid' | 'cancelled' | string;
  paid_at?: string;
  approved_by_name?: string;
  created_by_name?: string;
};

export type ReceivableRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  revenue_account_id?: string;
  project_name?: string;
  receivable_code: string;
  client_partner_id?: string;
  client_name: string;
  contract_amount?: number;
  milestone_name?: string;
  gross_amount: number;
  received_amount?: number;
  remaining_amount?: number;
  due_date?: string;
  status: 'draft' | 'recorded' | 'approved' | 'partially_received' | 'received' | 'overdue' | 'cancelled' | string;
  invoice_count?: number;
  invoice_total?: number;
  issued_invoice_count?: number;
  voided_invoice_count?: number;
  issued_invoice_total?: number;
  schedule_count?: number;
  scheduled_total?: number;
  scheduled_received_total?: number;
  scheduled_remaining?: number;
  advance_receipt_count?: number;
  advance_requested_total?: number;
  advance_approved_total?: number;
  advance_received_total?: number;
  advance_settled_total?: number;
  approved_receipt_total?: number;
  approved_advance_receipt_total?: number;
  receipt_allocation_total?: number;
  pending_receipt_count?: number;
  latest_invoice_status?: string;
  latest_invoice_number?: string;
  latest_invoice_date?: string;
  effective_due_date?: string;
  overdue_days?: number;
  aging_bucket?: 'settled' | 'no_due_date' | 'not_due' | '1_30' | '31_60' | '61_90' | '90_plus' | string;
  reconciliation_issue_count?: number;
  reconciliation_status?: 'ok' | 'needs_review' | string;
};

export type ReceivableReconciliationWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  details?: AnyRecord;
};

export type ReceivableLedgerEntry = AnyRecord & {
  occurred_at: string;
  entry_type: string;
  source_id: string;
  source_code: string;
  status: string;
  description: string;
  debit_amount: number;
  credit_amount: number;
  document_amount: number;
  cash_amount: number;
  running_balance: number;
  actor_name?: string;
};

export type ReceivableReconciliationSnapshot = AnyRecord & {
  summary: ReceivableRecord & {
    overdue_schedule_count?: number;
    next_schedule_due_date?: string;
    computed_remaining?: number;
    balance_difference?: number;
    unissued_invoice_gap?: number;
    advance_unreceived_total?: number;
    advance_unsettled_total?: number;
    warning_count?: number;
  };
  warnings: ReceivableReconciliationWarning[];
  ledger: ReceivableLedgerEntry[];
};

export type ReceivablePaymentScheduleRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  client_name?: string;
  schedule_code: string;
  milestone_name: string;
  schedule_type: 'advance' | 'progress' | 'final' | 'retention' | 'other' | string;
  planned_amount: number;
  received_amount: number;
  remaining_amount: number;
  due_date?: string;
  status: 'draft' | 'approved' | 'partially_received' | 'received' | 'overdue' | 'cancelled' | 'deleted' | string;
  notes?: string;
  created_by_name?: string;
  approved_by_name?: string;
};

export type ReceivableAdvanceReceiptRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  client_name?: string;
  receivable_client_name?: string;
  receipt_code: string;
  requested_by?: string;
  requested_by_name?: string;
  amount: number;
  received_amount?: number;
  settled_amount: number;
  reason?: string;
  due_date?: string;
  received_at?: string;
  settled_at?: string;
  status: 'draft' | 'submitted' | 'approved' | 'partially_received' | 'received' | 'partially_settled' | 'settled' | 'cancelled' | 'deleted' | string;
  payment_id?: string;
  payment_code?: string;
  notes?: string;
  approved_by_name?: string;
};

export type ReceivablePaymentRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  receivable_client_name?: string;
  payment_code: string;
  payment_type: string;
  payment_method: string;
  cash_account_id?: string;
  target_type?: string;
  target_id?: string;
  amount: number;
  allocation_id?: string;
  allocation_type?: 'receivable_receipt' | 'advance_receipt' | 'advance_settlement' | string;
  allocation_amount?: number;
  allocation_notes?: string;
  schedule_id?: string;
  schedule_code?: string;
  milestone_name?: string;
  advance_receipt_id?: string;
  advance_receipt_code?: string;
  output_invoice_id?: string;
  output_invoice_number?: string;
  status: 'draft' | 'submitted' | 'pending' | 'approved' | 'paid' | 'cancelled' | string;
  paid_at?: string;
  approved_by_name?: string;
  created_by_name?: string;
};

export type InputInvoiceRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_id?: string;
  cost_account_id?: string;
  tax_code_id?: string;
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
  vendor_partner_id?: string;
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
  accounting_book_id?: string;
  revenue_account_id?: string;
  tax_code_id?: string;
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
  client_partner_id?: string;
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

export type AccountingBudgetRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_code: string;
  name: string;
  planned_amount: number;
  approved_amount: number;
  committed_amount: number;
  actual_amount: number;
  paid_amount: number;
  available_amount: number;
  variance_amount?: number;
  utilization_percent?: number;
  status: 'draft' | 'submitted' | 'approved' | 'rejected' | 'cancelled' | string;
};

export type AccountingPaymentRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_id?: string;
  cash_account_id?: string;
  payment_code: string;
  payment_type: string;
  target_type?: string;
  target_id?: string;
  amount: number;
  payment_method: string;
  status: 'draft' | 'submitted' | 'pending' | 'waiting_approval' | 'approved' | 'cancelled' | string;
  paid_at?: string;
};

export type AccountingAccountRecord = AnyRecord & {
  id: string;
  book_id: string;
  account_code: string;
  name: string;
  account_type: string;
  normal_balance: 'debit' | 'credit' | string;
  is_postable: boolean;
};

export type AccountingPeriodRecord = AnyRecord & {
  id: string;
  book_id: string;
  fiscal_year: number;
  period_no: number;
  date_from: string;
  date_to: string;
  status: 'open' | 'closed' | string;
  closed_at?: string;
};

export type VATReturnSnapshotRecord = AnyRecord & {
  id: string;
  tax_period_id: string;
  book_id: string;
  amendment_no: number;
  input_invoice_count: number;
  output_invoice_count: number;
  input_vat_amount: number;
  deductible_input_vat_amount: number;
  output_vat_amount: number;
  input_adjustment_vat_amount: number;
  output_adjustment_vat_amount: number;
  credit_brought_forward: number;
  vat_payable: number;
  credit_carried_forward: number;
  prepared_at: string;
};

export type AccountingTaxPeriodRecord = AnyRecord & {
  id: string;
  book_id: string;
  fiscal_year: number;
  period_type: 'month' | 'quarter';
  period_no: number;
  date_from: string;
  date_to: string;
  status: 'open' | 'prepared' | 'filed' | 'locked' | 'amended' | string;
  amendment_no: number;
};

export type AccountingTaxPeriodDetail = AccountingTaxPeriodRecord & {
  summary?: AnyRecord;
  issues?: AnyRecord[];
  purchases?: AnyRecord[];
  sales?: AnyRecord[];
  adjustments?: AnyRecord[];
  returns?: VATReturnSnapshotRecord[];
  events?: AnyRecord[];
  payments?: AccountingPaymentRecord[];
  latest_return?: VATReturnSnapshotRecord;
};

export type JournalEntryRecord = ProjectOperationsRecord & {
  project_id: string;
  book_id: string;
  period_id: string;
  entry_no: string;
  entry_date: string;
  document_date?: string;
  description: string;
  source_type: string;
  source_event: string;
  debit_total: number;
  credit_total: number;
  status: 'posted' | string;
  reversed?: boolean;
  reversal_entry_id?: string;
};

export type AccountingReconciliationIssue = ProjectOperationsRecord & {
  project_id: string;
  source_type: string;
  source_id: string;
  source_code?: string;
  issue_code: string;
  description: string;
  status: 'open' | 'resolved' | 'accepted' | string;
  resolution_note?: string;
  resolution_journal_entry_id?: string;
};

export type CashflowSummary = AnyRecord & {
  from: string;
  to: string;
  group_by: 'day' | 'week' | 'month' | string;
  status: 'approved' | 'all' | string;
  project_id?: string;
  opening_balance: number;
  payment_count: number;
  unclassified_count: number;
  non_cash_count: number;
  total_inflow: number;
  total_outflow: number;
  net_flow: number;
  closing_balance: number;
  pending_payment_count: number;
  pending_payment_total: number;
};

export type CashflowSeriesRow = AnyRecord & {
  period_start: string;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
  running_balance: number;
  payment_count: number;
};

export type CashflowCategoryRow = AnyRecord & {
  category: string;
  direction: 'inflow' | 'outflow' | 'non_cash' | 'unclassified' | string;
  payment_count: number;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
};

export type CashflowLedgerRow = AnyRecord & {
  occurred_at: string;
  id: string;
  project_id: string;
  project_name?: string;
  payment_code: string;
  payment_type: string;
  payment_method: string;
  status: string;
  direction: 'inflow' | 'outflow' | 'non_cash' | 'unclassified' | string;
  category: string;
  amount: number;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
  target_type?: string;
  target_id?: string;
  created_by_name?: string;
  approved_by_name?: string;
};

export type CashflowReport = AnyRecord & {
  summary: CashflowSummary;
  series: CashflowSeriesRow[];
  categories: CashflowCategoryRow[];
  ledger: CashflowLedgerRow[];
};

export type ReportProjectOption = {
  id: string;
  code: string;
  name: string;
};

export type ReportProjectOptionsPage = {
  items: ReportProjectOption[];
  has_more: boolean;
  next_cursor?: string;
  selected: ReportProjectOption | null;
};

export type ReportOptions = {
  projects: ReportProjectOptionsPage;
};

export type ProjectReportRow = AnyRecord & {
  project_id: string;
  name: string;
  status: string;
  progress_percent: number;
  open_issues: number;
  pending_approvals: number;
};

export type WarehouseReportRow = AnyRecord & {
  material_id: string;
  name: string;
  category?: string;
  stock_quantity: number;
  surplus_quantity: number;
};

export type ProcurementReportRow = AnyRecord & {
  status: string;
  budget_status: string;
  total: number;
  amount: number;
};

export type AccountingReportRow = AnyRecord & {
  project_id: string;
  name: string;
  payable_remaining: number;
  receivable_remaining: number;
  used_budget: number;
};

export type StaffReportRow = AnyRecord & {
  staff_id: string;
  full_name: string;
  department?: string;
  position?: string;
  project_count: number;
};

export type TaxReportSummary = AnyRecord & {
  from: string;
  to: string;
  group_by: 'month' | 'quarter' | string;
  source: 'all' | 'input' | 'output' | string;
  status: 'posted' | 'all' | string;
  project_id?: string;
  invoice_count: number;
  input_invoice_count: number;
  output_invoice_count: number;
  input_taxable_amount: number;
  input_vat_amount: number;
  input_total_amount: number;
  output_taxable_amount: number;
  output_vat_amount: number;
  output_total_amount: number;
  net_vat_amount: number;
  net_vat_status: 'payable' | 'credit' | 'neutral' | string;
  missing_tax_code_count: number;
  vat_math_mismatch_count: number;
  excluded_unposted_count: number;
  excluded_unposted_total: number;
  warning_count: number;
};

export type TaxPeriodRow = AnyRecord & {
  period_start: string;
  invoice_count: number;
  input_invoice_count: number;
  output_invoice_count: number;
  input_taxable_amount: number;
  input_vat_amount: number;
  input_total_amount: number;
  output_taxable_amount: number;
  output_vat_amount: number;
  output_total_amount: number;
  net_vat_amount: number;
};

export type TaxCounterpartyRow = AnyRecord & {
  direction: 'input' | 'output' | string;
  tax_code?: string;
  counterparty_name: string;
  invoice_count: number;
  taxable_amount: number;
  vat_amount: number;
  total_amount: number;
  warning_count: number;
};

export type TaxLedgerRow = AnyRecord & {
  invoice_date: string;
  id: string;
  direction: 'input' | 'output' | string;
  project_id: string;
  project_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  counterparty_name: string;
  tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  currency: 'VND' | string;
  status: string;
  source_type: string;
  source_id?: string;
  source_code?: string;
  secondary_source_type?: string;
  secondary_source_id?: string;
  secondary_source_code?: string;
  document_id?: string;
  document_name?: string;
  missing_tax_code?: boolean;
  vat_math_mismatch?: boolean;
  warning_flags?: string;
};

export type TaxReportWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  count: number;
  amount?: number;
};

export type TaxReport = AnyRecord & {
  summary: TaxReportSummary;
  periods: TaxPeriodRow[];
  counterparties: TaxCounterpartyRow[];
  ledger: TaxLedgerRow[];
  warnings: TaxReportWarning[];
};

export type ProfitReportSummary = AnyRecord & {
  from: string;
  to: string;
  group_by: 'month' | 'quarter' | string;
  status: 'posted' | 'all' | string;
  project_id?: string;
  ledger_count: number;
  revenue_invoice_count: number;
  cost_invoice_count: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  total_cost_amount: number;
  gross_profit: number;
  net_profit: number;
  gross_margin_percent: number;
  net_margin_percent: number;
  unclassified_cost_count: number;
  excluded_unposted_count: number;
  excluded_unposted_total: number;
  warning_count: number;
};

export type ProfitPeriodRow = AnyRecord & {
  period_start: string;
  ledger_count: number;
  revenue_invoice_count: number;
  cost_invoice_count: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  total_cost_amount: number;
  gross_profit: number;
  net_profit: number;
  gross_margin_percent: number;
  net_margin_percent: number;
};

export type ProfitProjectRow = AnyRecord & {
  project_id: string;
  project_name: string;
  ledger_count: number;
  revenue_invoice_count: number;
  cost_invoice_count: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  total_cost_amount: number;
  gross_profit: number;
  net_profit: number;
  gross_margin_percent: number;
  net_margin_percent: number;
  warning_count: number;
};

export type ProfitCostBreakdownRow = AnyRecord & {
  cost_type: 'direct' | 'indirect' | string;
  cost_group: string;
  classification_reason: string;
  invoice_count: number;
  cost_amount: number;
  vat_amount: number;
  total_amount: number;
  warning_count: number;
};

export type ProfitLedgerRow = AnyRecord & {
  invoice_date: string;
  id: string;
  direction: 'revenue' | 'cost' | string;
  cost_type: 'revenue' | 'direct' | 'indirect' | string;
  cost_group: string;
  classification_reason: string;
  project_id: string;
  project_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  counterparty_name: string;
  tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  currency: 'VND' | string;
  status: string;
  source_type: string;
  source_id?: string;
  source_code?: string;
  secondary_source_type?: string;
  secondary_source_id?: string;
  secondary_source_code?: string;
  document_id?: string;
  document_name?: string;
  unclassified_cost?: boolean;
  warning_flags?: string;
};

export type ProfitReportWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  count?: number;
  amount?: number;
};

export type ProfitReport = AnyRecord & {
  summary: ProfitReportSummary;
  periods: ProfitPeriodRow[];
  projects: ProfitProjectRow[];
  cost_breakdown: ProfitCostBreakdownRow[];
  ledger: ProfitLedgerRow[];
  warnings: ProfitReportWarning[];
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

export type PlanningDailyReportItem = AnyRecord & {
  id: string;
  form_instance_id: string;
  project_id: string;
  project_name: string;
  work_item_id?: string;
  work_item_title?: string;
  work_item_status?: string;
  run_node_id?: string;
  run_node_name?: string;
  run_step_id?: string;
  run_step_name?: string;
  form_type: 'daily_report' | string;
  title: string;
  status: 'draft' | 'submitted' | 'reopened' | string;
  current_revision?: number;
  template_code?: string;
  template_name?: string;
  version_no?: number;
  report_date?: string;
  reported_by?: string;
  planning_scope?: string;
  progress_percent?: number | null;
  completed_items?: AnyRecord[];
  next_plan_items?: AnyRecord[];
  blockers?: AnyRecord[];
  completed_count?: number;
  next_plan_count?: number;
  blocker_count?: number;
  generated_work_item_count?: number;
  evidence_count?: number;
  signature_count?: number;
  pdf_export_count?: number;
  submitted_by?: string;
  submitted_by_name?: string;
  submitted_at?: string;
  updated_at?: string;
  answers?: AnyRecord;
};

export type MeetingMinutesItem = AnyRecord & {
  id: string;
  form_instance_id: string;
  project_id: string;
  project_name: string;
  work_item_id?: string;
  work_item_title?: string;
  work_item_status?: string;
  run_node_id?: string;
  run_node_name?: string;
  run_step_id?: string;
  run_step_name?: string;
  form_type: 'meeting_minutes' | string;
  title: string;
  status: 'draft' | 'submitted' | 'reopened' | string;
  current_revision?: number;
  template_code?: string;
  template_name?: string;
  version_no?: number;
  meeting_title?: string;
  meeting_date?: string;
  location?: string;
  chair?: string;
  agenda?: string;
  attendees?: AnyRecord[];
  attendees_text?: string;
  decisions?: AnyRecord[];
  decisions_text?: string;
  action_items?: AnyRecord[];
  action_items_text?: string;
  confirmed?: boolean;
  attendee_count?: number;
  decision_count?: number;
  action_item_count?: number;
  generated_work_item_count?: number;
  evidence_count?: number;
  signature_count?: number;
  pdf_export_count?: number;
  submitted_by?: string;
  submitted_by_name?: string;
  submitted_at?: string;
  updated_at?: string;
  answers?: AnyRecord;
};

export type PlanningChecklistItem = AnyRecord & {
  id: string;
  form_instance_id: string;
  project_id: string;
  project_name: string;
  work_item_id?: string;
  work_item_title?: string;
  work_item_status?: string;
  run_node_id?: string;
  run_node_name?: string;
  run_step_id?: string;
  run_step_name?: string;
  form_type: 'checklist' | string;
  title: string;
  status: 'draft' | 'submitted' | 'reopened' | string;
  current_revision?: number;
  template_code?: string;
  template_name?: string;
  version_no?: number;
  check_date?: string;
  checked_by?: string;
  planning_scope?: string;
  checklist_items?: AnyRecord[];
  items_done_text?: string;
  exceptions_text?: string;
  overall_comment?: string;
  confirmed?: boolean;
  item_count?: number;
  done_count?: number;
  needs_action_count?: number;
  evidence_linked_count?: number;
  evidence_missing_count?: number;
  evidence_count?: number;
  signature_count?: number;
  pdf_export_count?: number;
  submitted_by?: string;
  submitted_by_name?: string;
  submitted_at?: string;
  updated_at?: string;
  answers?: AnyRecord;
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
  default_supplier_partner_id?: string;
  default_supplier_partner_name?: string;
  default_supplier_tax_code?: string;
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

export type StockPosition = ProjectOperationsRecord & {
  warehouse_id: string;
  warehouse_name: string;
  warehouse_type?: string;
  location?: string;
  material_id: string;
  code: string;
  name: string;
  unit: string;
  category?: string;
  stock_quantity: number;
};

export type StockDocumentItem = ProjectOperationsRecord & {
  document_id?: string;
  material_id: string;
  material_code?: string;
  material_name?: string;
  quantity: number;
  unit: string;
  reason?: string;
  source_line_type?: string;
  source_line_id?: string;
  movement_code?: string;
};

export type StockDocument = ProjectOperationsRecord & {
  document_code: string;
  project_id?: string;
  project_name?: string;
  movement_type: 'receive' | 'issue' | 'transfer' | 'return_to_warehouse' | 'return_to_subcontractor' | string;
  source_warehouse_id?: string;
  source_warehouse_name?: string;
  target_warehouse_id?: string;
  target_warehouse_name?: string;
  reference_type?: string;
  reference_id?: string;
  document_date: string;
  note?: string;
  status: string;
  item_count?: number;
  total_quantity?: number;
  created_by_name?: string;
  items?: StockDocumentItem[];
};

export type WarehouseReceiptOption = ProjectOperationsRecord & {
  incoming_acceptance_id: string;
  acceptance_code: string;
  procurement_request_id?: string;
  request_code?: string;
  supplier_name?: string;
  received_at?: string;
  result?: string;
  warehouse_id?: string;
  incoming_acceptance_item_id: string;
  material_id: string;
  material_code?: string;
  material_name: string;
  accepted_quantity: number;
  stock_received_quantity: number;
  remaining_quantity: number;
  unit: string;
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
  supplier_partner_id?: string;
  supplier_name?: string;
  subcontractor_partner_id?: string;
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
  partners?: Partner[];
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
  evidence_ids?: string[];
  evidence_records?: EvidenceRecord[];
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
  can_act?: boolean;
  can_decide?: boolean;
  capabilities?: {
    can_act?: boolean;
    can_decide?: boolean;
  };
  available_actions?: string[];
  readiness?: 'actionable' | 'decision' | 'upcoming' | 'read_only' | 'history' | string;
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
	  run_node_type?: string;
	  run_node_business_code?: string;
  run_node_order_index?: number;
  run_step_id?: string;
  run_step_name?: string;
	  run_step_business_code?: string;
  run_step_order_index?: number;
  evidence_count?: number;
  workflow_form_instance_id?: string;
  workflow_form_status?: 'draft' | 'submitted' | 'reopened' | 'cancelled' | string;
  workflow_form_type?: string;
  workflow_form_revision?: number;
  workflow_form_submitted_at?: string;
  workflow_form_template_id?: string;
  workflow_form_version_id?: string;
  workflow_form_version_no?: number;
  workflow_form_version_status?: string;
  workflow_form_template_code?: string;
  workflow_form_template_name?: string;
  workflow_form_required?: boolean;
  required_workflow_form_version_id?: string;
  pipeline_engine_version?: number;
  surface_kind?: string;
  domain_action_key?: string;
	  report_eligible?: boolean;
  automation_events?: Array<{
    id: string;
    action_key: string;
    status: 'succeeded' | 'blocked' | 'failed' | 'skipped' | string;
    blocker_reason?: string;
    error_message?: string;
    target_type?: string;
    target_id?: string;
    updated_at?: string;
  }>;
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
  skipped_steps?: number;
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
  evidence_summary?: ProjectEvidenceSummary;
  activity_summary: AnyRecord;
  latest_activity: WorkItemActivity[];
  audit_summary: AnyRecord;
  department_surfaces: ProjectOperationsRecord[];
  pipeline_run?: AnyRecord | null;
  summary?: AnyRecord;
  work_items: WorkItem[];
};

export type ProjectOperationsSidebarResponse = AnyRecord & {
  project?: Project;
  project_summary?: AnyRecord;
  visibility?: ProjectOperationsVisibility;
  health_counts?: ProjectOperationsHealthCounts;
  workflow_timeline?: ProjectOperationsTimelineNode[];
  pipeline_run?: AnyRecord | null;
  summary?: AnyRecord;
  work_items?: WorkItem[];
};

export type ProjectFinanceSummary = AnyRecord & {
  project_id: string;
  project_name: string;
  project_code: string;
  contract_value: number | null;
  eligible_revenue: number | null;
  committed_material_cost: number | null;
  confirmed_material_cost: number | null;
  provisional_profit: number | null;
  unconfirmed_cost_count: number;
  unconfirmed_cost_amount: number | null;
  open_cash_request_count: number;
  open_inbound_amount: number | null;
  open_outbound_amount: number | null;
  scope_warning: string;
};

export type ProjectMaterialCost = AnyRecord & {
  id: string;
  project_id: string;
  vendor_name: string;
  description: string;
  total_amount: number;
  recognized_cost_amount: number;
  status: 'draft' | 'submitted' | 'needs_changes' | 'confirmed' | 'cancelled' | string;
};

export type ProjectCashRequest = AnyRecord & {
  id: string;
  project_id: string;
  request_code: string;
  direction: 'inbound' | 'outbound';
  title: string;
  counterparty_name: string;
  amount: number;
  status: 'draft' | 'submitted' | 'approved' | 'handed_to_accounting' | 'needs_changes' | 'processed' | 'cancelled' | string;
};

export type ProjectFinanceWorkspace = {
  can_view_report: boolean;
  summary: ProjectFinanceSummary;
  tasks: AnyRecord[];
  material_costs: ProjectMaterialCost[];
  cash_requests: ProjectCashRequest[];
};

export type ProjectFinanceOptions = {
  procurements: AnyRecord[];
  incoming_acceptances: AnyRecord[];
  stock_documents: AnyRecord[];
  partners: AnyRecord[];
};
