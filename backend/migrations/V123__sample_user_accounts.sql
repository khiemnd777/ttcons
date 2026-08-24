ALTER TABLE users
  ADD COLUMN IF NOT EXISTS is_sample BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS sample_role_id UUID REFERENCES roles(id) ON DELETE RESTRICT;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_sample_role_identity_check;

ALTER TABLE users
  ADD CONSTRAINT users_sample_role_identity_check CHECK (
    (NOT is_sample AND sample_role_id IS NULL)
    OR (is_sample AND sample_role_id IS NOT NULL)
  );

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_sample_role
  ON users(sample_role_id)
  WHERE is_sample;

CREATE INDEX IF NOT EXISTS idx_users_sample_active
  ON users(is_sample,status)
  WHERE is_sample;

CREATE OR REPLACE FUNCTION enforce_sample_user_role_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_is_sample BOOLEAN;
  target_sample_role UUID;
  target_project_is_sample BOOLEAN;
BEGIN
  SELECT is_sample,sample_role_id
  INTO target_is_sample,target_sample_role
  FROM users
  WHERE id=NEW.user_id;

  IF target_is_sample THEN
    IF NEW.project_id IS NULL THEN
      RAISE EXCEPTION 'sample users cannot receive global roles';
    END IF;
    IF NEW.role_id IS DISTINCT FROM target_sample_role THEN
      RAISE EXCEPTION 'sample users can receive only their sample role';
    END IF;
    SELECT is_sample INTO target_project_is_sample FROM projects WHERE id=NEW.project_id;
    IF target_project_is_sample IS DISTINCT FROM true THEN
      RAISE EXCEPTION 'sample users can receive roles only in sample projects';
    END IF;
  ELSIF NEW.project_id IS NOT NULL THEN
    SELECT is_sample INTO target_project_is_sample FROM projects WHERE id=NEW.project_id;
    IF target_project_is_sample THEN
      RAISE EXCEPTION 'real users cannot receive roles in sample projects';
    END IF;
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_sample_user_role_scope ON user_roles;
CREATE TRIGGER trg_sample_user_role_scope
BEFORE INSERT OR UPDATE OF user_id,role_id,project_id ON user_roles
FOR EACH ROW EXECUTE FUNCTION enforce_sample_user_role_scope();

CREATE OR REPLACE FUNCTION enforce_project_member_sample_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_project_is_sample BOOLEAN;
  target_user_is_sample BOOLEAN;
BEGIN
  SELECT is_sample INTO target_project_is_sample FROM projects WHERE id=NEW.project_id;
  SELECT COALESCE(account.is_sample,false)
  INTO target_user_is_sample
  FROM staff profile
  LEFT JOIN users account ON account.id=profile.user_id
  WHERE profile.id=NEW.staff_id;

  IF target_project_is_sample IS DISTINCT FROM target_user_is_sample THEN
    RAISE EXCEPTION 'project member sample scope does not match project sample scope';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_project_member_sample_scope ON project_members;
CREATE TRIGGER trg_project_member_sample_scope
BEFORE INSERT OR UPDATE OF project_id,staff_id ON project_members
FOR EACH ROW EXECUTE FUNCTION enforce_project_member_sample_scope();

CREATE OR REPLACE FUNCTION enforce_project_manager_sample_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_user_is_sample BOOLEAN;
BEGIN
  IF NEW.pm_staff_id IS NULL THEN
    RETURN NEW;
  END IF;
  SELECT COALESCE(account.is_sample,false)
  INTO target_user_is_sample
  FROM staff profile
  LEFT JOIN users account ON account.id=profile.user_id
  WHERE profile.id=NEW.pm_staff_id;

  IF target_user_is_sample IS DISTINCT FROM NEW.is_sample THEN
    RAISE EXCEPTION 'project manager sample scope does not match project sample scope';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_project_manager_sample_scope ON projects;
CREATE TRIGGER trg_project_manager_sample_scope
BEFORE INSERT OR UPDATE OF pm_staff_id,is_sample ON projects
FOR EACH ROW EXECUTE FUNCTION enforce_project_manager_sample_scope();
