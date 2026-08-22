ALTER TABLE users
  ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;

INSERT INTO permissions (id, key, name, description, module)
VALUES (
  gen_random_uuid(),
  'staff.import',
  'Import tài khoản nhân sự',
  'Xem trước và tạo nguyên tử tài khoản nhân sự từ Excel hoặc CSV',
  'staff'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    module=EXCLUDED.module;

DELETE FROM role_permissions role_permission
USING roles role, permissions permission
WHERE role_permission.role_id=role.id
  AND role_permission.permission_id=permission.id
  AND permission.key='staff.import'
  AND role.name <> 'developer';

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key='staff.import'
WHERE role.name='developer'
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_restricted_staff_import_permission_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_role TEXT;
  target_permission TEXT;
BEGIN
  SELECT name INTO target_role FROM roles WHERE id=NEW.role_id;
  SELECT key INTO target_permission FROM permissions WHERE id=NEW.permission_id;

  IF target_permission='staff.import' AND target_role <> 'developer' THEN
    RAISE EXCEPTION 'staff.import is reserved for developer';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_restricted_staff_import_permission_assignment ON role_permissions;
CREATE TRIGGER trg_restricted_staff_import_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_restricted_staff_import_permission_assignment();

CREATE TABLE IF NOT EXISTS staff_account_imports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key UUID NOT NULL UNIQUE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  source_filename TEXT NOT NULL,
  preview_hash TEXT NOT NULL,
  row_count INTEGER NOT NULL,
  created_count INTEGER NOT NULL DEFAULT 0,
  role_assignment_count INTEGER NOT NULL DEFAULT 0,
  result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT staff_account_imports_filename_check CHECK (length(btrim(source_filename)) BETWEEN 1 AND 255),
  CONSTRAINT staff_account_imports_preview_hash_check CHECK (preview_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT staff_account_imports_row_count_check CHECK (row_count BETWEEN 1 AND 100),
  CONSTRAINT staff_account_imports_created_count_check CHECK (created_count BETWEEN 0 AND row_count),
  CONSTRAINT staff_account_imports_role_count_check CHECK (role_assignment_count BETWEEN 0 AND row_count),
  CONSTRAINT staff_account_imports_result_object_check CHECK (jsonb_typeof(result_json)='object')
);

CREATE INDEX IF NOT EXISTS idx_staff_account_imports_actor_created
  ON staff_account_imports(actor_id, created_at DESC)
  WHERE actor_id IS NOT NULL;
