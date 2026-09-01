import type { EvidenceRecord } from './activityEvidence';
import type { Contract, ContractVariation, ProjectOperationsRecord } from './commercial';

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
  assigned_user_account_deleted?: boolean;
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
  activated_by_account_deleted?: boolean;
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
  requested_by_account_deleted?: boolean;
  checked_by_name?: string;
  checked_by_account_deleted?: boolean;
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
  assigned_user_account_deleted?: boolean;
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
