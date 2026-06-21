ALTER TABLE workflow_form_events
  DROP CONSTRAINT IF EXISTS workflow_form_events_type_check;

ALTER TABLE workflow_form_events
  ADD CONSTRAINT workflow_form_events_type_check CHECK (event_type IN (
    'created',
    'draft_saved',
    'submitted',
    'reopened',
    'cancelled',
    'signed',
    'pdf_exported'
  ));

ALTER TABLE workflow_form_signatures
  ADD COLUMN IF NOT EXISTS visual_storage_provider TEXT NOT NULL DEFAULT 'none',
  ADD COLUMN IF NOT EXISTS visual_storage_key TEXT,
  ADD COLUMN IF NOT EXISTS visual_mime_type TEXT,
  ADD COLUMN IF NOT EXISTS visual_file_size BIGINT,
  ADD COLUMN IF NOT EXISTS visual_checksum_sha256 TEXT;

ALTER TABLE workflow_form_signatures
  DROP CONSTRAINT IF EXISTS workflow_form_signatures_visual_storage_provider_check,
  ADD CONSTRAINT workflow_form_signatures_visual_storage_provider_check CHECK (visual_storage_provider IN ('none', 'local'));

ALTER TABLE workflow_form_signatures
  DROP CONSTRAINT IF EXISTS workflow_form_signatures_visual_storage_key_check,
  ADD CONSTRAINT workflow_form_signatures_visual_storage_key_check CHECK (
    (visual_storage_provider = 'none' AND visual_storage_key IS NULL)
    OR
    (visual_storage_provider = 'local' AND visual_storage_key IS NOT NULL)
  );

ALTER TABLE workflow_form_signatures
  DROP CONSTRAINT IF EXISTS workflow_form_signatures_visual_checksum_check,
  ADD CONSTRAINT workflow_form_signatures_visual_checksum_check CHECK (
    visual_checksum_sha256 IS NULL OR visual_checksum_sha256 ~ '^[a-f0-9]{64}$'
  );

CREATE INDEX IF NOT EXISTS idx_workflow_form_signatures_visual_storage
  ON workflow_form_signatures(visual_storage_key)
  WHERE visual_storage_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS workflow_form_pdf_exports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  form_version_id UUID NOT NULL REFERENCES workflow_form_versions(id),
  form_revision_no INTEGER NOT NULL,
  title TEXT NOT NULL,
  file_name TEXT NOT NULL,
  storage_provider TEXT NOT NULL DEFAULT 'local',
  storage_key TEXT NOT NULL,
  mime_type TEXT NOT NULL DEFAULT 'application/pdf',
  file_size BIGINT NOT NULL,
  checksum_sha256 TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  metadata_json JSONB NOT NULL DEFAULT '{}',
  generated_by UUID REFERENCES users(id),
  generated_by_name TEXT,
  generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_pdf_exports_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT workflow_form_pdf_exports_revision_nonnegative CHECK (form_revision_no >= 0),
  CONSTRAINT workflow_form_pdf_exports_provider_check CHECK (storage_provider = 'local'),
  CONSTRAINT workflow_form_pdf_exports_checksum_check CHECK (checksum_sha256 ~ '^[a-f0-9]{64}$'),
  CONSTRAINT workflow_form_pdf_exports_payload_hash_check CHECK (payload_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT workflow_form_pdf_exports_file_size_positive CHECK (file_size > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_workflow_form_pdf_exports_storage_key
  ON workflow_form_pdf_exports(storage_provider, storage_key);

CREATE INDEX IF NOT EXISTS idx_workflow_form_pdf_exports_instance
  ON workflow_form_pdf_exports(form_instance_id, generated_at DESC);

CREATE INDEX IF NOT EXISTS idx_workflow_form_pdf_exports_project
  ON workflow_form_pdf_exports(project_id, generated_at DESC);

CREATE OR REPLACE FUNCTION prevent_workflow_form_pdf_export_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'workflow_form_pdf_exports are immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_workflow_form_pdf_exports_immutable ON workflow_form_pdf_exports;

CREATE TRIGGER trg_workflow_form_pdf_exports_immutable
BEFORE UPDATE OR DELETE ON workflow_form_pdf_exports
FOR EACH ROW
EXECUTE FUNCTION prevent_workflow_form_pdf_export_mutation();
