import type { EvidenceRecord } from './activityEvidence';

export type AnyRecord = Record<string, any>;

export type AuditLogRecord = AnyRecord & {
  id: string;
  project_id?: string;
  project_code?: string;
  project_name?: string;
  actor_id?: string;
  actor_name?: string;
  actor_account_deleted?: boolean;
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

export type EventLogRecord = AnyRecord & {
  id: string;
  category: 'unexpected_5xx' | 'panic' | 'controlled_503';
  http_status: number;
  error_code: string;
  method: string;
  route_template: string;
  request_class: string;
  request_id: string;
  actor_id?: string;
  actor_email?: string;
  project_id?: string;
  project_code?: string;
  project_name?: string;
  duration_ms: number;
  error_type?: string;
  error_fingerprint: string;
  message_preview?: string;
  internal_message?: string;
  stack_trace?: string;
  created_at: string;
};

export type EventLogPage = {
  items: EventLogRecord[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
  summary: {
    unexpected_5xx: number;
    panic: number;
    controlled_503: number;
  };
  options: {
    categories: string[];
    statuses: number[];
    methods: string[];
    routes: string[];
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
  phone?: string;
  department?: string;
  position?: string;
  status?: string;
  has_avatar?: boolean;
  avatar_updated_at?: string;
  must_change_password?: boolean;
  recovery_email_available?: boolean;
  recovery_email_verified?: boolean;
  recovery_email_masked?: string;
  pending_recovery_email_masked?: string;
};

export type UserNotification = AnyRecord & {
  id: string;
  event_key: string;
  notification_type: string;
  category_key: string;
  category_label: string;
  intent: 'action_required' | 'awareness' | 'outcome' | 'security';
  severity: 'info' | 'success' | 'warning' | 'critical';
  icon_key: string;
  tone_key: string;
  source_kind: string;
  source_id: string;
  source_event_id: string;
  project_id: string;
  project_code: string;
  project_name: string;
  actor_user_id?: string;
  actor_name?: string;
  actor_account_deleted?: boolean;
  title_snapshot: string;
  message_snapshot: string;
  action_key: string;
  action_payload_json?: AnyRecord;
  action_href?: string | null;
  action_label: string;
  read_at?: string | null;
  seen_at?: string | null;
  archived_at?: string | null;
  resolved_at?: string | null;
  created_at: string;
};

export type NotificationPage = {
  items: UserNotification[];
  has_more: boolean;
  next_cursor?: string;
};

export type NotificationSummary = {
  unread_total: number;
  action_required: number;
  by_category: Record<string, number>;
  actor_revision: number;
};

export type NotificationCatalogItem = {
  event_key: string;
  domain_key: string;
  category_key: string;
  category_label: string;
  intent: UserNotification['intent'];
  severity: UserNotification['severity'];
  icon_key: string;
  tone_key: string;
  enabled: boolean;
  required_delivery: boolean;
  permissions_any: string[];
};

export type NotificationPreference = {
  category_key: string;
  event_key: string;
  in_app_enabled: boolean;
  email_enabled: boolean;
  push_enabled: boolean;
  digest_mode: 'immediate' | 'daily' | 'weekly' | 'off';
  quiet_hours_json: AnyRecord;
};

export type WorkReportSummary = {
  scopes: Partial<Record<'mine' | 'source' | 'review' | 'all' | 'supervision', number>>;
  total_visible: number;
};

export type WorkReportStatus = 'draft' | 'recorded' | 'submitted' | 'needs_changes' | 'acknowledged';

export type WorkReportListItem = AnyRecord & {
  source_id: string;
  work_report_id?: string;
  project_id: string;
  project_code: string;
  project_name: string;
  report_date: string;
  author_id: string;
  author_name: string;
  author_account_deleted?: boolean;
  recipient_id?: string;
  recipient_name?: string;
  recipient_account_deleted?: boolean;
  status: WorkReportStatus;
  current_revision_no: number;
  submitted_at?: string;
  is_supervisor_report?: boolean;
  viewer_acknowledged?: boolean;
};

export type WorkReportRevision = AnyRecord & {
  id: string;
  revision_no: number;
  snapshot_json: AnyRecord;
  snapshot_sha256: string;
  submitted_by?: string;
  submitted_by_name?: string;
  submitted_by_account_deleted?: boolean;
  submitted_at: string;
  comment?: string;
  evidence_ids: string[];
  evidence_records: EvidenceRecord[];
};

export type WorkReportEvent = AnyRecord & {
  id: string;
  event_type: string;
  revision_no: number;
  actor_id?: string;
  actor_name?: string;
  actor_account_deleted?: boolean;
  comment?: string;
  created_at: string;
};

export type WorkReportDetail = WorkReportListItem & {
  report_delivery_eligible: boolean;
  work_item_id?: string;
  work_item_title?: string;
  weather?: string;
  progress_percent?: number;
  structured_json?: AnyRecord;
  work_summary?: string;
  manpower_summary?: string;
  material_usage_summary?: string;
  issues_summary?: string;
  review_comment?: string;
  revisions: WorkReportRevision[];
  events: WorkReportEvent[];
  capabilities?: { can_acknowledge: boolean; can_request_changes: boolean; can_discuss: boolean };
  read_receipts?: Array<{ revision_no: number; user_id: string; user_name: string; acknowledged_at: string; comment?: string }>;
};
