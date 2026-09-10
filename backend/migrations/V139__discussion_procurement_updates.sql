-- Source snapshots remain immutable; expand the existing source allowlist.
ALTER TABLE activity_work_updates DROP CONSTRAINT activity_work_updates_source_type_check;
ALTER TABLE activity_work_updates ADD CONSTRAINT activity_work_updates_source_type_check
  CHECK (source_type IN ('workflow_form','design_release','evidence','procurement_request'));

-- Quiet header saves and formal actions both advance the source edit version.
ALTER TABLE procurement_requests ADD COLUMN edit_version INTEGER NOT NULL DEFAULT 1
  CHECK (edit_version > 0);
