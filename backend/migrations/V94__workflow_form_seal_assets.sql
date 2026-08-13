CREATE TABLE IF NOT EXISTS seal_assets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  original_name TEXT NOT NULL,
  source_mime_type TEXT NOT NULL,
  storage_provider TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  mime_type TEXT NOT NULL DEFAULT 'image/png',
  file_size BIGINT NOT NULL,
  width_px INTEGER NOT NULL,
  height_px INTEGER NOT NULL,
  checksum_sha256 TEXT NOT NULL,
  uploaded_by UUID NOT NULL REFERENCES users(id),
  uploaded_by_name TEXT NOT NULL,
  archived_by UUID REFERENCES users(id),
  archived_by_name TEXT,
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT seal_assets_name_nonblank_check CHECK (btrim(name) <> ''),
  CONSTRAINT seal_assets_status_check CHECK (status IN ('active', 'archived')),
  CONSTRAINT seal_assets_source_mime_type_check CHECK (source_mime_type IN ('image/png', 'image/jpeg', 'image/webp')),
  CONSTRAINT seal_assets_storage_provider_check CHECK (storage_provider IN ('local', 's3')),
  CONSTRAINT seal_assets_storage_key_canonical_check CHECK (
    btrim(storage_key) <> '' AND storage_key = btrim(storage_key)
  ),
  CONSTRAINT seal_assets_mime_type_check CHECK (mime_type = 'image/png'),
  CONSTRAINT seal_assets_file_size_positive_check CHECK (file_size > 0),
  CONSTRAINT seal_assets_dimensions_positive_check CHECK (width_px > 0 AND height_px > 0),
  CONSTRAINT seal_assets_checksum_check CHECK (checksum_sha256 ~ '^[a-f0-9]{64}$'),
  CONSTRAINT seal_assets_archive_consistency_check CHECK (
    (status = 'active' AND archived_by IS NULL AND archived_by_name IS NULL AND archived_at IS NULL)
    OR
    (status = 'archived' AND archived_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_seal_assets_status_created
  ON seal_assets(status, created_at DESC, id);

CREATE INDEX IF NOT EXISTS idx_seal_assets_storage
  ON seal_assets(storage_provider, storage_key);

CREATE OR REPLACE FUNCTION protect_seal_asset_content()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE'
    AND OLD.storage_provider = 'local'
    AND NEW.storage_provider = 's3'
    AND (to_jsonb(NEW) - 'storage_provider') = (to_jsonb(OLD) - 'storage_provider')
  THEN
    RETURN NEW;
  END IF;

  IF TG_OP = 'UPDATE'
    AND OLD.status = 'active'
    AND NEW.status = 'archived'
    AND NEW.archived_at IS NOT NULL
    AND (to_jsonb(NEW) - ARRAY['status', 'archived_by', 'archived_by_name', 'archived_at', 'updated_at']) =
        (to_jsonb(OLD) - ARRAY['status', 'archived_by', 'archived_by_name', 'archived_at', 'updated_at'])
  THEN
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'seal asset content is immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_seal_assets_protect_content ON seal_assets;

CREATE TRIGGER trg_seal_assets_protect_content
BEFORE UPDATE OR DELETE ON seal_assets
FOR EACH ROW
EXECUTE FUNCTION protect_seal_asset_content();

CREATE TABLE IF NOT EXISTS workflow_form_seal_applications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  form_version_id UUID NOT NULL REFERENCES workflow_form_versions(id),
  form_revision_no INTEGER NOT NULL,
  seal_asset_id UUID NOT NULL REFERENCES seal_assets(id) ON DELETE RESTRICT,
  purpose TEXT NOT NULL DEFAULT 'internal_ack',
  applied_by UUID NOT NULL REFERENCES users(id),
  applied_by_name TEXT NOT NULL,
  applied_by_roles_json JSONB NOT NULL DEFAULT '[]',
  comment TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_seal_applications_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT workflow_form_seal_applications_revision_nonnegative_check CHECK (form_revision_no >= 0),
  CONSTRAINT workflow_form_seal_applications_purpose_check CHECK (purpose IN (
    'internal_ack',
    'prepared_by',
    'reviewed_by',
    'approved_by',
    'received_by'
  )),
  CONSTRAINT workflow_form_seal_applications_roles_array_check
    CHECK (jsonb_typeof(applied_by_roles_json) = 'array'),
  CONSTRAINT workflow_form_seal_applications_payload_object_check
    CHECK (jsonb_typeof(payload_json) = 'object'),
  CONSTRAINT workflow_form_seal_applications_hash_check CHECK (payload_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT workflow_form_seal_applications_unique_form_purpose_asset
    UNIQUE (form_instance_id, purpose, seal_asset_id)
);

CREATE INDEX IF NOT EXISTS idx_workflow_form_seal_applications_instance
  ON workflow_form_seal_applications(form_instance_id, applied_at DESC, id);

CREATE INDEX IF NOT EXISTS idx_workflow_form_seal_applications_project
  ON workflow_form_seal_applications(project_id, applied_at DESC, id);

CREATE INDEX IF NOT EXISTS idx_workflow_form_seal_applications_asset
  ON workflow_form_seal_applications(seal_asset_id, applied_at DESC, id);

CREATE OR REPLACE FUNCTION prevent_workflow_form_seal_application_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'workflow_form_seal_applications are immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_workflow_form_seal_applications_immutable
  ON workflow_form_seal_applications;

CREATE TRIGGER trg_workflow_form_seal_applications_immutable
BEFORE UPDATE OR DELETE ON workflow_form_seal_applications
FOR EACH ROW
EXECUTE FUNCTION prevent_workflow_form_seal_application_mutation();

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
    'seal_applied',
    'pdf_exported',
    'pdf_export_deleted'
  ));

INSERT INTO permissions (id, key, name, description, module)
SELECT gen_random_uuid(), permission.key, permission.name, permission.description, 'settings'
FROM (VALUES
  ('seal.use', 'Sử dụng con dấu', 'Áp dụng con dấu công ty vào hồ sơ quy trình được phân công'),
  ('seal.manage', 'Quản lý con dấu', 'Tải lên và lưu trữ tài nguyên con dấu công ty')
) permission(key, name, description)
ON CONFLICT (key) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    module = EXCLUDED.module;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON
  (role.name = 'admin' AND permission.key IN ('seal.use', 'seal.manage'))
  OR
  (role.name IN ('director', 'contract_officer') AND permission.key = 'seal.use')
ON CONFLICT DO NOTHING;
