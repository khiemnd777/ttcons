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
