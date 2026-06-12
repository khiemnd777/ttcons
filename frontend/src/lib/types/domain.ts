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

export type ProjectOperationsRecord = AnyRecord & {
  id: string;
  project_id?: string;
  created_at?: string;
  updated_at?: string;
};

export type WorkItemActivity = ProjectOperationsRecord & {
  work_item_id: string;
  scope_type?: string;
  scope_id?: string;
  actor_id?: string;
  actor_name?: string;
  activity_type: string;
  comment?: string;
  from_status?: string;
  to_status?: string;
  metadata_json?: AnyRecord;
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

export type WorkItem = ProjectOperationsRecord & {
  title: string;
  description?: string;
  status: string;
  work_type?: string;
  source_type?: string;
  source_id?: string;
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
