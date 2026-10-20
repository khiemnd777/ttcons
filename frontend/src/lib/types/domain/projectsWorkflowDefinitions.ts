import type { AnyRecord } from './common';
import type { WorkflowFormField } from './workflowForms';

export type MyWorkNavigationSummary = {
  work_items_need_action: number;
  workflow_actions_ready: number;
  project_startups_pending: number;
  work_total: number;
  work_items_waiting_decision: number;
  node_approvals_pending: number;
  work_reports_review_pending: number;
  approval_total: number;
  accounting_handoffs_pending: number;
  accounting_total: number;
  total: number;
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
  pm_account_deleted?: boolean;
  status: string;
  progress_percent: number;
	is_sample?: boolean;
	sample_profile?: 'draft' | 'planning' | 'completed';
	sample_blueprint_version?: number;
	workflow_entry_mode: 'survey_required' | 'client_boq';
  planned_start_date?: string | null;
  planned_end_date?: string | null;
  created_at?: string;
  updated_at?: string;
  open_issues?: number;
  pending_approvals?: number;
};

export type ProjectExplorerPage = {
  items: Project[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
  options: {
    statuses: string[];
  };
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
  pm_account_deleted?: boolean;
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

export type ProjectProgressPMOption = {
  id: string;
  name: string;
  account_deleted?: boolean;
  is_sample?: boolean;
};

export type ProjectProgressSummaryResponse = {
  filtered_total: number;
  stage_counts: Record<ProjectProgressStageKey, number>;
  pm_options: ProjectProgressPMOption[];
  status_options: string[];
  generated_at: string;
};

export type ProjectProgressStagePage = {
  items: ProjectProgressItem[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
  generated_at: string;
};

export type ProjectMemberAccessRole = {
  id: string;
  name: string;
  display_name: string;
  scope: 'global' | 'project';
  description?: string;
  grants_project_view?: boolean;
  grants_project_member_manage?: boolean;
  grants_pipeline_run?: boolean;
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
  account_active?: boolean;
  account_deleted?: boolean;
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
  is_sample?: boolean;
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

export type ProjectStartupEligibleUser = {
  user_id: string;
  staff_id: string;
  full_name: string;
};

export type ProjectStartupStep = {
  id: string;
  name: string;
  role_ids: string[];
  required_permission?: string;
  eligible_users: ProjectStartupEligibleUser[];
  covered: boolean;
};

export type ProjectStartupRoleCoverage = {
  role_label: string;
  node_count: number;
  step_count: number;
  members: ProjectStartupEligibleUser[];
  covered: boolean;
};

export type ProjectStartupBlocker = {
  key: 'pipeline_template' | 'project_manager' | 'first_step_coverage' | string;
  label: string;
  step_id?: string;
};

export type ProjectStartupAction = {
  id: string;
  action_kind: 'setup_required' | 'upcoming';
  project_id: string;
  project_code: string;
  project_name: string;
  owner_user_id: string;
  owner_name: string;
  ready_to_start: boolean;
  blockers: ProjectStartupBlocker[];
  first_steps: ProjectStartupStep[];
  created_at?: string;
};

export type ProjectStartupActionsResponse = {
  items: ProjectStartupAction[];
  total: number;
  summary: { need_action: number; upcoming: number; total: number };
};

export type ProjectStartupReadiness = {
  project_id: string;
  project_code: string;
  project_name: string;
  created_by: string;
  owner_user_id: string;
  owner_name: string;
  pm_staff_id?: string;
  pm_user_id?: string;
  pm_name?: string;
  template_id?: string;
  template_name?: string;
  template_status?: string;
  role_coverage?: ProjectStartupRoleCoverage[];
  first_steps: ProjectStartupStep[];
  blockers: ProjectStartupBlocker[];
  ready_to_start: boolean;
  capabilities: {
    can_manage_project_manager: boolean;
    can_start_pipeline: boolean;
    can_update_project: boolean;
    is_startup_owner: boolean;
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
  config_schema?: Array<{
    key: string;
    label: string;
    control: 'source_form_field' | 'source_field_option' | 'text' | string;
    required: boolean;
    depends_on?: string;
    help_text?: string;
  }>;
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
  pinned_workflow_forms?: WorkflowFormCatalogItem[];
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
  default_eligible?: boolean;
  selectable?: boolean;
  step_usage_count?: number;
  field_count?: number;
  required_field_keys?: string[];
  fields?: Array<{
    field_key: string;
    label: string;
    field_type: string;
    required: boolean;
    options_json?: Array<{ value: string; label?: string }> | string[];
    validation_json?: AnyRecord;
    visibility_condition_json?: AnyRecord;
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
  definition_hash?: string;
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
