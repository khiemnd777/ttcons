ALTER TABLE pipeline_run_steps
  ADD COLUMN IF NOT EXISTS form_required BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS form_version_id UUID;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_run_steps_form_version_fk'
  ) THEN
    ALTER TABLE pipeline_run_steps
      ADD CONSTRAINT pipeline_run_steps_form_version_fk
      FOREIGN KEY (form_version_id) REFERENCES workflow_form_versions(id);
  END IF;
END $$;

UPDATE pipeline_run_steps rs
SET form_required = COALESCE(ts.form_required, false),
    form_version_id = ts.form_version_id
FROM pipeline_template_steps ts
WHERE ts.id = rs.template_step_id
  AND (rs.form_version_id IS NULL OR rs.form_required IS DISTINCT FROM COALESCE(ts.form_required, false));

CREATE INDEX IF NOT EXISTS idx_pipeline_run_steps_required_form
  ON pipeline_run_steps(run_node_id, form_version_id)
  WHERE form_required;

-- Existing runs keep the engine version snapshotted by V43. Future runs from the
-- official client workflow use the strict completion gate while preserving old history.
UPDATE pipeline_templates
SET engine_version = 2,
    updated_at = now()
WHERE id = '40000000-0000-0000-0000-000000000020'
  AND status = 'published';
