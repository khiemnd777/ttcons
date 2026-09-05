import type { ProjectOperationsRecord } from './commercial';
import type { AnyRecord } from './common';

export type WorkItemActivity = ProjectOperationsRecord & {
  work_item_id?: string;
  scope_type?: string;
  scope_id?: string;
  actor_id?: string;
  actor_name?: string;
  actor_account_deleted?: boolean;
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
  is_discussion_unread?: boolean;
  is_system_unread?: boolean;
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
  actor_account_deleted?: boolean;
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
  type: 'user' | 'staff' | 'role' | 'all';
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
  discussion_unread: number;
  mentioned: number;
  questions: number;
  blockers: number;
  reminders: number;
  attention: number;
};

export type ProjectDiscussionProject = {
  project_id: string;
  project_code: string;
  project_name: string;
  unread_count: number;
  latest_activity_id?: string;
  latest_activity_at?: string;
  latest_comment?: string;
  latest_actor_name?: string;
  can_post: boolean;
};

export type ProjectDiscussionPage = {
  items: ProjectDiscussionProject[];
  unread_total: number;
  total: number;
  has_more: boolean;
  next_cursor?: string;
};

export type ProjectUpdateKind = 'notification' | 'discussion' | 'system';
export type ProjectUpdateLatest = Pick<WorkItemActivity, 'comment' | 'actor_name' | 'from_status' | 'to_status' | 'target_label'> & {
  kind: ProjectUpdateKind;
  created_at: string;
  activity_type?: string;
};
export type ProjectUpdateSummary = {
  project_id: string;
  notification_unread: number;
  discussion_unread: number;
  system_unread: number;
  latest: ProjectUpdateLatest | null;
};
export type ProjectUpdatesPage = { items: ProjectUpdateSummary[] };

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

export type ActivityAttentionSummary = {
  count: number;
  has_more: boolean;
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
