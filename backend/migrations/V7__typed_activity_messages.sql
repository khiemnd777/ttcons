ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_work_item_project_fk;

ALTER TABLE work_item_activities
  ALTER COLUMN work_item_id DROP NOT NULL;

ALTER TABLE work_item_activities
  ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'project';

ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_visibility_check;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_visibility_check CHECK (visibility IN (
    'internal',
    'project',
    'client'
  ));

ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_type_check;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_type_check CHECK (activity_type IN (
    'created',
    'started',
    'completed',
    'blocked',
    'blocker_resolved',
    'status_changed',
    'comment',
    'status',
    'blocker',
    'question',
    'decision',
    'reminder',
    'file_note',
    'system_event',
    'evidence_added',
    'evidence_accepted',
    'evidence_rejected',
    'review_requested',
    'approval_requested',
    'approved',
    'rejected',
    'changes_requested'
  ));

ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_scope_integrity_check;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_scope_integrity_check CHECK (
    (scope_type = 'project' AND scope_id = project_id AND work_item_id IS NULL)
    OR
    (scope_type <> 'project' AND work_item_id IS NOT NULL)
  );

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_work_item_activities_scope_created
  ON work_item_activities(project_id, scope_type, created_at DESC);
