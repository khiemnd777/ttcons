ALTER TABLE work_item_activities
  ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS edited_by UUID REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS withdrawn_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS withdrawn_by UUID REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS message_revision_no INTEGER NOT NULL DEFAULT 0;

ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_message_revision_no_check;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_message_revision_no_check
    CHECK (message_revision_no >= 0);

CREATE TABLE IF NOT EXISTS activity_message_revisions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  activity_id UUID NOT NULL REFERENCES work_item_activities(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  revision_no INTEGER NOT NULL CHECK (revision_no > 0),
  action TEXT NOT NULL CHECK (action IN ('edited', 'withdrawn')),
  previous_comment TEXT,
  new_comment TEXT,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  actor_name TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (activity_id, revision_no)
);

CREATE INDEX IF NOT EXISTS idx_activity_message_revisions_project_created
  ON activity_message_revisions(project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_work_item_activities_withdrawn
  ON work_item_activities(project_id, withdrawn_at)
  WHERE withdrawn_at IS NOT NULL;
