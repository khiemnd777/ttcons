INSERT INTO permissions (id,key,name,description,module)
VALUES (
  gen_random_uuid(),
  'sample_data.create',
  'Tạo dữ liệu công trình mẫu',
  'Tạo công trình mẫu có trạng thái, quy trình, phân công và lịch sử nghiệp vụ phục vụ demo',
  'settings'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    module=EXCLUDED.module;

DELETE FROM role_permissions assignment
USING roles role,permissions permission
WHERE assignment.role_id=role.id
  AND assignment.permission_id=permission.id
  AND permission.key='sample_data.create'
  AND role.name <> 'developer';

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON permission.key='sample_data.create'
WHERE role.name='developer'
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_sample_data_permission_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_role TEXT;
  target_permission TEXT;
BEGIN
  SELECT name INTO target_role FROM roles WHERE id=NEW.role_id;
  SELECT key INTO target_permission FROM permissions WHERE id=NEW.permission_id;
  IF target_permission='sample_data.create' AND target_role <> 'developer' THEN
    RAISE EXCEPTION 'sample_data.create is reserved for developer';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_sample_data_permission_assignment ON role_permissions;
CREATE TRIGGER trg_sample_data_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_sample_data_permission_assignment();

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS is_sample BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS sample_profile TEXT,
  ADD COLUMN IF NOT EXISTS sample_blueprint_version INTEGER;

ALTER TABLE projects
  DROP CONSTRAINT IF EXISTS projects_sample_profile_check,
  DROP CONSTRAINT IF EXISTS projects_sample_blueprint_check;

ALTER TABLE projects
  ADD CONSTRAINT projects_sample_profile_check CHECK (
    (NOT is_sample AND sample_profile IS NULL AND sample_blueprint_version IS NULL)
    OR
    (is_sample AND sample_profile IN ('draft','planning','completed') AND sample_blueprint_version > 0)
  ),
  ADD CONSTRAINT projects_sample_blueprint_check CHECK (
    sample_blueprint_version IS NULL OR sample_blueprint_version > 0
  );

CREATE INDEX IF NOT EXISTS idx_projects_sample_created
  ON projects(is_sample,created_at DESC)
  WHERE is_sample;

CREATE TABLE IF NOT EXISTS sample_data_generations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key UUID NOT NULL UNIQUE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  profile TEXT NOT NULL,
  progress_percent INTEGER NOT NULL,
  blueprint_version INTEGER NOT NULL,
  template_id UUID REFERENCES pipeline_templates(id),
  template_definition_hash TEXT,
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT sample_data_generations_profile_check CHECK (profile IN ('draft','planning','completed')),
  CONSTRAINT sample_data_generations_progress_check CHECK (progress_percent IN (0,5,10,15,25,40,65,90,100)),
  CONSTRAINT sample_data_generations_blueprint_check CHECK (blueprint_version > 0),
  CONSTRAINT sample_data_generations_result_check CHECK (jsonb_typeof(result_json)='object'),
  CONSTRAINT sample_data_generations_profile_progress_check CHECK (
    (profile='draft' AND progress_percent=0)
    OR (profile='planning' AND progress_percent IN (5,10,15,25,40,65,90))
    OR (profile='completed' AND progress_percent=100)
  )
);

CREATE INDEX IF NOT EXISTS idx_sample_data_generations_actor_created
  ON sample_data_generations(actor_id,created_at DESC)
  WHERE actor_id IS NOT NULL;

CREATE OR REPLACE FUNCTION enforce_sample_data_role_name()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.name <> 'developer' AND EXISTS (
    SELECT 1
    FROM role_permissions assignment
    JOIN permissions permission ON permission.id=assignment.permission_id
    WHERE assignment.role_id=OLD.id
      AND permission.key='sample_data.create'
  ) THEN
    RAISE EXCEPTION 'sample_data.create is reserved for developer';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_sample_data_role_name ON roles;
CREATE TRIGGER trg_sample_data_role_name
BEFORE UPDATE OF name ON roles
FOR EACH ROW
WHEN (OLD.name IS DISTINCT FROM NEW.name)
EXECUTE FUNCTION enforce_sample_data_role_name();
