import type { EvidenceRecord, ProjectEvidenceSummary, WorkItemActivity } from './activityEvidence';
import type { ProjectOperationsRecord } from './commercial';
import type { AnyRecord } from './common';
import type { Project } from './projectsWorkflowDefinitions';
import type { WorkflowFormDetail } from './workflowForms';

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
	is_early_execution?: boolean;
	start_readiness?: WorkItemStartReadiness;
  available_actions?: string[];
  readiness?: 'actionable' | 'decision' | 'upcoming' | 'read_only' | 'history' | string;
  priority?: string;
  department?: string;
  role_ids?: string[];
  assignee_staff_id?: string;
  assignee_name?: string;
  assignee_account_deleted?: boolean;
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
  business_source?: {
    action_key?: string;
    label?: string;
    ready?: boolean;
    status?: string;
    message?: string;
    route?: string;
    target_type?: string;
    target_id?: string;
    checks?: Array<{
      key: string;
      label: string;
      ready: boolean;
      status?: string;
      message?: string;
      route?: string;
    }>;
    facts?: AnyRecord;
  };
  execution_target?: WorkItemExecutionTarget;
  required_permission_name?: string;
  required_permission_description?: string;
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
	completion_requirements?: WorkItemCompletionRequirement[];
  activities?: WorkItemActivity[];
  evidence?: EvidenceRecord[];
  assignments?: ProjectOperationsRecord[];
};

export type MyWorkflowAction = {
  id: string;
  action_type: 'advance_run_node' | string;
  title: string;
  action_label: string;
  description: string;
  project_id: string;
  project_name: string;
  project_code: string;
  pipeline_run_id: string;
  run_node_id: string;
  run_node_name: string;
  run_node_type?: string;
  run_node_business_code?: string;
  next_run_node_id?: string;
  next_run_node_name?: string;
  next_run_node_type?: string;
  terminal?: boolean;
  readiness: 'actionable' | string;
};

export type WorkItemExecutionTarget = {
  kind: 'domain' | 'capture' | 'evidence' | 'approval' | 'mixed' | 'work_item' | string;
  workspace: 'preparation' | 'planning' | 'boq' | 'contracts' | 'procurement' | 'warehouse' | 'operations' | 'acceptance' | 'handover' | 'project_finance' | 'work_item' | string;
  route: string;
  fallback_route?: string;
  action_label: string;
  completion_mode: 'domain_gate' | 'workflow_form' | 'evidence' | 'approval' | 'domain_then_form' | 'work_item' | string;
  source_status?: string;
  source_ready?: boolean;
  source_message?: string;
  action_key?: string;
  target_type?: string;
  target_id?: string;
};

export type WorkItemCompletionRequirement = {
  key: 'workflow_form' | 'checklist' | 'document' | 'photo' | string;
  label: string;
  required: boolean;
  status: 'satisfied' | 'missing' | string;
  satisfied_by?: 'workflow_form' | 'evidence' | 'completion_payload' | string;
};

export type WorkItemStartRequirement = {
	id: string;
	type: 'work_item' | 'transition' | 'planned_start' | 'node_approval' | 'automation' | 'blocker' | 'route' | string;
	node_id?: string;
	node_label?: string;
	step_id?: string;
	step_label?: string;
	work_item_id?: string;
	label: string;
	status: string;
	satisfied: boolean;
	can_open: boolean;
	description?: string;
	workflow_form_status?: string;
	planned_start_at?: string;
};

export type WorkItemStartReadiness = {
	state: 'ready' | 'requirements_unmet' | 'route_unresolved' | 'not_allowed' | string;
	can_start: boolean;
	can_bypass: boolean;
	unmet_count: number;
	requirements: WorkItemStartRequirement[];
	snapshot_token?: string;
	current_node_id?: string;
	current_node_name?: string;
	target_node_id?: string;
	target_node_name?: string;
};

export type ApprovalDecisionContext = {
  current_stage: number;
  required_roles: string[];
  quorum: number;
  comment_required: boolean;
  requester_cannot_approve: boolean;
};

export type ApprovalRequest = ProjectOperationsRecord & {
  project_name?: string;
  target_id?: string;
  target_type?: string;
  target_name?: string;
  requested_by?: string;
  requested_by_name?: string;
  reason?: string;
  status: string;
  current_stage?: number;
  capabilities?: { can_decide?: boolean };
  available_actions?: string[];
  decision_context?: ApprovalDecisionContext;
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
