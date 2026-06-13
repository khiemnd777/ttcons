ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_scope_integrity_check;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_scope_integrity_check CHECK (
    (scope_type = 'project' AND scope_id = project_id AND work_item_id IS NULL)
    OR
    (scope_type = 'work_item' AND scope_id = work_item_id AND work_item_id IS NOT NULL)
    OR
    (scope_type NOT IN ('project', 'work_item') AND scope_id IS NOT NULL)
  );

CREATE INDEX IF NOT EXISTS idx_work_item_activities_scope_id_created
  ON work_item_activities(scope_type, scope_id, created_at DESC);
