ALTER TABLE pipeline_templates
  ADD COLUMN IF NOT EXISTS template_family_id UUID,
  ADD COLUMN IF NOT EXISTS revision_no INTEGER,
  ADD COLUMN IF NOT EXISTS source_template_id UUID,
  ADD COLUMN IF NOT EXISTS engine_version INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS definition_hash TEXT,
  ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS published_by UUID;

UPDATE pipeline_templates
SET template_family_id = id
WHERE template_family_id IS NULL;

UPDATE pipeline_templates
SET revision_no = GREATEST(COALESCE(version, 1), 1)
WHERE revision_no IS NULL;

UPDATE pipeline_templates
SET published_at = COALESCE(updated_at, created_at)
WHERE status = 'published' AND published_at IS NULL;

ALTER TABLE pipeline_templates
  ALTER COLUMN template_family_id SET NOT NULL,
  ALTER COLUMN revision_no SET NOT NULL,
  ALTER COLUMN revision_no SET DEFAULT 1;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_templates_family_fk'
  ) THEN
    ALTER TABLE pipeline_templates
      ADD CONSTRAINT pipeline_templates_family_fk
      FOREIGN KEY (template_family_id) REFERENCES pipeline_templates(id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_templates_source_fk'
  ) THEN
    ALTER TABLE pipeline_templates
      ADD CONSTRAINT pipeline_templates_source_fk
      FOREIGN KEY (source_template_id) REFERENCES pipeline_templates(id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_templates_published_by_fk'
  ) THEN
    ALTER TABLE pipeline_templates
      ADD CONSTRAINT pipeline_templates_published_by_fk
      FOREIGN KEY (published_by) REFERENCES users(id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_templates_revision_positive'
  ) THEN
    ALTER TABLE pipeline_templates
      ADD CONSTRAINT pipeline_templates_revision_positive CHECK (revision_no > 0);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_templates_engine_version_positive'
  ) THEN
    ALTER TABLE pipeline_templates
      ADD CONSTRAINT pipeline_templates_engine_version_positive CHECK (engine_version > 0);
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_templates_family_revision
  ON pipeline_templates(template_family_id, revision_no);

CREATE INDEX IF NOT EXISTS idx_pipeline_templates_source
  ON pipeline_templates(source_template_id)
  WHERE source_template_id IS NOT NULL;

ALTER TABLE pipeline_runs
  ADD COLUMN IF NOT EXISTS engine_version INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS template_definition_hash TEXT;

UPDATE pipeline_runs pr
SET engine_version = COALESCE(pt.engine_version, 1),
    template_definition_hash = pt.definition_hash
FROM pipeline_templates pt
WHERE pt.id = pr.template_id
  AND (pr.engine_version = 1 OR pr.template_definition_hash IS NULL);
