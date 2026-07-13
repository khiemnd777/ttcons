ALTER TABLE work_item_activities
  ADD COLUMN IF NOT EXISTS parent_activity_id UUID;

ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_parent_fk;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_parent_fk
    FOREIGN KEY (parent_activity_id) REFERENCES work_item_activities(id) ON DELETE SET NULL;

ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_parent_not_self_check;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_parent_not_self_check
    CHECK (parent_activity_id IS NULL OR parent_activity_id <> id);

CREATE INDEX IF NOT EXISTS idx_work_item_activities_parent_created
  ON work_item_activities(parent_activity_id, created_at)
  WHERE parent_activity_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS activity_reactions (
  activity_id UUID NOT NULL REFERENCES work_item_activities(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reaction TEXT NOT NULL CHECK (reaction IN ('👍', '👀', '🙏', '🎉')),
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (activity_id, user_id, reaction)
);

CREATE INDEX IF NOT EXISTS idx_activity_reactions_project_user_active
  ON activity_reactions(project_id, user_id, updated_at DESC)
  WHERE active;
