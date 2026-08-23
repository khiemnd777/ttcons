INSERT INTO permissions (id, key, name, description, module)
VALUES
  (
    gen_random_uuid(),
    'project.import',
    'Import công trình',
    'Xem trước và tạo nguyên tử công trình từ Excel có tham chiếu Pipeline, Form và Fields',
    'project'
  ),
  (
    gen_random_uuid(),
    'project.export',
    'Xuất công trình',
    'Xuất Excel công trình trong phạm vi được phép cùng các danh mục tham chiếu',
    'project'
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
    (permission.key='project.import' AND role.name NOT IN ('admin','developer'))
    OR
    (permission.key='project.export' AND role.name NOT IN ('admin','developer','pm'))
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON (
  (permission.key='project.import' AND role.name IN ('admin','developer'))
  OR
  (permission.key='project.export' AND role.name IN ('admin','developer','pm'))
)
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_project_excel_permission_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_role TEXT;
  target_permission TEXT;
BEGIN
  SELECT name INTO target_role FROM roles WHERE id=NEW.role_id;
  SELECT key INTO target_permission FROM permissions WHERE id=NEW.permission_id;

  IF target_permission='project.import' AND target_role NOT IN ('admin','developer') THEN
    RAISE EXCEPTION 'project.import is reserved for admin and developer';
  END IF;
  IF target_permission='project.export' AND target_role NOT IN ('admin','developer','pm') THEN
    RAISE EXCEPTION 'project.export is reserved for admin, developer and pm';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_project_excel_permission_assignment ON role_permissions;
CREATE TRIGGER trg_project_excel_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_project_excel_permission_assignment();

CREATE TABLE IF NOT EXISTS project_excel_imports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key UUID NOT NULL UNIQUE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  source_filename TEXT NOT NULL,
  source_size_bytes BIGINT NOT NULL,
  workbook_version TEXT NOT NULL,
  preview_hash TEXT NOT NULL,
  reference_fingerprint TEXT NOT NULL,
  row_count INTEGER NOT NULL,
  created_count INTEGER NOT NULL DEFAULT 0,
  result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT project_excel_imports_filename_check CHECK (length(btrim(source_filename)) BETWEEN 1 AND 255),
  CONSTRAINT project_excel_imports_source_size_check CHECK (source_size_bytes BETWEEN 1 AND 5242880),
  CONSTRAINT project_excel_imports_workbook_version_check CHECK (workbook_version='1'),
  CONSTRAINT project_excel_imports_preview_hash_check CHECK (preview_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT project_excel_imports_reference_fingerprint_check CHECK (reference_fingerprint ~ '^[0-9a-f]{64}$'),
  CONSTRAINT project_excel_imports_row_count_check CHECK (row_count BETWEEN 1 AND 100),
  CONSTRAINT project_excel_imports_created_count_check CHECK (created_count BETWEEN 0 AND row_count),
  CONSTRAINT project_excel_imports_result_object_check CHECK (jsonb_typeof(result_json)='object')
);

CREATE INDEX IF NOT EXISTS idx_project_excel_imports_actor_created
  ON project_excel_imports(actor_id, created_at DESC)
  WHERE actor_id IS NOT NULL;
