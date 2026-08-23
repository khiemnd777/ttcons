INSERT INTO permissions (id, key, name, description, module)
VALUES
  (
    gen_random_uuid(),
    'workflow_definition.import',
    'Import định nghĩa workflow',
    'Xem trước và tạo bản nháp Pipeline, Form và Fields từ gói Excel định nghĩa workflow',
    'pipeline'
  ),
  (
    gen_random_uuid(),
    'workflow_definition.export',
    'Xuất định nghĩa workflow',
    'Xuất gói Excel định nghĩa Pipeline, Form và Fields trong phạm vi được phép',
    'pipeline'
  )
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    module=EXCLUDED.module;

DELETE FROM role_permissions assignment
USING roles role, permissions permission
WHERE assignment.role_id=role.id
  AND assignment.permission_id=permission.id
  AND (
    (permission.key='workflow_definition.import' AND role.name NOT IN ('admin','developer'))
    OR
    (permission.key='workflow_definition.export' AND role.name NOT IN ('admin','developer','pm'))
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON (
  (permission.key='workflow_definition.import' AND role.name IN ('admin','developer'))
  OR
  (permission.key='workflow_definition.export' AND role.name IN ('admin','developer','pm'))
)
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_workflow_definition_permission_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_role TEXT;
  target_permission TEXT;
BEGIN
  SELECT name INTO target_role FROM roles WHERE id=NEW.role_id;
  SELECT key INTO target_permission FROM permissions WHERE id=NEW.permission_id;

  IF target_permission='workflow_definition.import' AND target_role NOT IN ('admin','developer') THEN
    RAISE EXCEPTION 'workflow_definition.import is reserved for admin and developer';
  END IF;
  IF target_permission='workflow_definition.export' AND target_role NOT IN ('admin','developer','pm') THEN
    RAISE EXCEPTION 'workflow_definition.export is reserved for admin, developer and pm';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_workflow_definition_permission_assignment ON role_permissions;
CREATE TRIGGER trg_workflow_definition_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_workflow_definition_permission_assignment();

ALTER TABLE workflow_form_versions
  ADD COLUMN IF NOT EXISTS definition_hash TEXT;

ALTER TABLE workflow_form_versions
  DROP CONSTRAINT IF EXISTS workflow_form_versions_definition_hash_check;
ALTER TABLE workflow_form_versions
  ADD CONSTRAINT workflow_form_versions_definition_hash_check
  CHECK (definition_hash IS NULL OR definition_hash ~ '^[0-9a-f]{64}$');

CREATE TABLE IF NOT EXISTS workflow_definition_imports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key UUID NOT NULL UNIQUE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  scope TEXT NOT NULL,
  source_filename TEXT NOT NULL,
  source_size_bytes BIGINT NOT NULL,
  workbook_version TEXT NOT NULL,
  preview_hash TEXT NOT NULL,
  reference_fingerprint TEXT NOT NULL,
  pipeline_count INTEGER NOT NULL DEFAULT 0,
  form_count INTEGER NOT NULL DEFAULT 0,
  result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_definition_imports_scope_check CHECK (scope IN ('form','pipeline','bundle')),
  CONSTRAINT workflow_definition_imports_filename_check CHECK (length(btrim(source_filename)) BETWEEN 1 AND 255),
  CONSTRAINT workflow_definition_imports_source_size_check CHECK (source_size_bytes BETWEEN 1 AND 10485760),
  CONSTRAINT workflow_definition_imports_workbook_version_check CHECK (workbook_version='1'),
  CONSTRAINT workflow_definition_imports_preview_hash_check CHECK (preview_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT workflow_definition_imports_reference_fingerprint_check CHECK (reference_fingerprint ~ '^[0-9a-f]{64}$'),
  CONSTRAINT workflow_definition_imports_pipeline_count_check CHECK (pipeline_count BETWEEN 0 AND 25),
  CONSTRAINT workflow_definition_imports_form_count_check CHECK (form_count BETWEEN 0 AND 100),
  CONSTRAINT workflow_definition_imports_nonempty_check CHECK (pipeline_count > 0 OR form_count > 0),
  CONSTRAINT workflow_definition_imports_result_object_check CHECK (jsonb_typeof(result_json)='object')
);

CREATE INDEX IF NOT EXISTS idx_workflow_definition_imports_actor_created
  ON workflow_definition_imports(actor_id, created_at DESC)
  WHERE actor_id IS NOT NULL;
