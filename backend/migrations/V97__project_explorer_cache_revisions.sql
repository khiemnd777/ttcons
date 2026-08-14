CREATE TABLE IF NOT EXISTS project_explorer_cache_state (
  id SMALLINT PRIMARY KEY,
  catalog_version BIGINT NOT NULL DEFAULT 1 CHECK (catalog_version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT project_explorer_cache_state_singleton CHECK (id = 1)
);

INSERT INTO project_explorer_cache_state (id, catalog_version)
VALUES (1, 1)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS project_explorer_actor_versions (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO project_explorer_actor_versions (user_id, version)
SELECT id, 1 FROM users
ON CONFLICT (user_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS project_explorer_project_versions (
  project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO project_explorer_project_versions (project_id, version)
SELECT id, 1 FROM projects
ON CONFLICT (project_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS project_explorer_invalidation_outbox (
  id BIGSERIAL PRIMARY KEY,
  event_kind TEXT NOT NULL CHECK (event_kind IN ('catalog', 'actor', 'project')),
  project_id UUID,
  actor_id UUID,
  revision BIGINT NOT NULL CHECK (revision > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at TIMESTAMPTZ,
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  last_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_project_explorer_outbox_pending
  ON project_explorer_invalidation_outbox(next_attempt_at, id)
  WHERE published_at IS NULL;

CREATE OR REPLACE FUNCTION project_explorer_bump_catalog(p_project_id UUID DEFAULT NULL)
RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
  next_version BIGINT;
BEGIN
  UPDATE project_explorer_cache_state
  SET catalog_version = catalog_version + 1, updated_at = now()
  WHERE id = 1
  RETURNING catalog_version INTO next_version;

  INSERT INTO project_explorer_invalidation_outbox(event_kind, project_id, revision)
  VALUES ('catalog', p_project_id, next_version);
  RETURN next_version;
END;
$$;

CREATE OR REPLACE FUNCTION project_explorer_bump_actor(p_user_id UUID)
RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
  next_version BIGINT;
BEGIN
  IF p_user_id IS NULL THEN
    RETURN NULL;
  END IF;
  INSERT INTO project_explorer_actor_versions(user_id, version, updated_at)
  VALUES (p_user_id, 1, now())
  ON CONFLICT (user_id) DO UPDATE
  SET version = project_explorer_actor_versions.version + 1,
      updated_at = now()
  RETURNING version INTO next_version;

  INSERT INTO project_explorer_invalidation_outbox(event_kind, actor_id, revision)
  VALUES ('actor', p_user_id, next_version);
  RETURN next_version;
END;
$$;

CREATE OR REPLACE FUNCTION project_explorer_bump_project(p_project_id UUID)
RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
  next_version BIGINT;
BEGIN
  IF p_project_id IS NULL THEN
    RETURN NULL;
  END IF;
  INSERT INTO project_explorer_project_versions(project_id, version, updated_at)
  VALUES (p_project_id, 1, now())
  ON CONFLICT (project_id) DO UPDATE
  SET version = project_explorer_project_versions.version + 1,
      updated_at = now()
  RETURNING version INTO next_version;

  INSERT INTO project_explorer_invalidation_outbox(event_kind, project_id, revision)
  VALUES ('project', p_project_id, next_version);
  RETURN next_version;
END;
$$;

CREATE OR REPLACE FUNCTION project_explorer_projects_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    PERFORM project_explorer_bump_catalog(OLD.id);
    RETURN OLD;
  END IF;

  PERFORM project_explorer_bump_project(NEW.id);
  IF TG_OP = 'INSERT'
     OR OLD.name IS DISTINCT FROM NEW.name
     OR OLD.code IS DISTINCT FROM NEW.code
     OR OLD.client_name IS DISTINCT FROM NEW.client_name
     OR OLD.status IS DISTINCT FROM NEW.status
     OR OLD.pm_staff_id IS DISTINCT FROM NEW.pm_staff_id
     OR OLD.created_at IS DISTINCT FROM NEW.created_at THEN
    PERFORM project_explorer_bump_catalog(NEW.id);
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_projects_revision ON projects;
CREATE TRIGGER project_explorer_projects_revision
AFTER INSERT OR UPDATE OR DELETE ON projects
FOR EACH ROW EXECUTE FUNCTION project_explorer_projects_revision_trigger();

CREATE OR REPLACE FUNCTION project_explorer_project_child_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM project_explorer_bump_project(NEW.project_id);
    RETURN NEW;
  ELSIF TG_OP = 'DELETE' THEN
    PERFORM project_explorer_bump_project(OLD.project_id);
    RETURN OLD;
  END IF;

  IF OLD.project_id IS DISTINCT FROM NEW.project_id THEN
    PERFORM project_explorer_bump_project(project_id)
    FROM (
      SELECT OLD.project_id AS project_id
      UNION
      SELECT NEW.project_id
    ) affected
    WHERE project_id IS NOT NULL
    ORDER BY project_id;
  ELSIF OLD.status IS DISTINCT FROM NEW.status THEN
    PERFORM project_explorer_bump_project(NEW.project_id);
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_issues_revision ON issues;
CREATE TRIGGER project_explorer_issues_revision
AFTER INSERT OR DELETE OR UPDATE OF project_id, status ON issues
FOR EACH ROW EXECUTE FUNCTION project_explorer_project_child_revision_trigger();

DROP TRIGGER IF EXISTS project_explorer_approval_requests_revision ON approval_requests;
CREATE TRIGGER project_explorer_approval_requests_revision
AFTER INSERT OR DELETE OR UPDATE OF project_id, status ON approval_requests
FOR EACH ROW EXECUTE FUNCTION project_explorer_project_child_revision_trigger();

CREATE OR REPLACE FUNCTION project_explorer_staff_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.full_name IS DISTINCT FROM NEW.full_name THEN
    PERFORM project_explorer_bump_project(p.id)
    FROM projects p
    WHERE p.pm_staff_id = NEW.id
    ORDER BY p.id;
    IF EXISTS (SELECT 1 FROM projects p WHERE p.pm_staff_id = NEW.id AND p.status <> 'cancelled') THEN
      PERFORM project_explorer_bump_catalog(NULL);
    END IF;
  END IF;

  IF OLD.user_id IS DISTINCT FROM NEW.user_id OR OLD.status IS DISTINCT FROM NEW.status THEN
    PERFORM project_explorer_bump_actor(user_id)
    FROM (
      SELECT OLD.user_id AS user_id
      UNION
      SELECT NEW.user_id
    ) affected
    WHERE user_id IS NOT NULL
    ORDER BY user_id;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_staff_revision ON staff;
CREATE TRIGGER project_explorer_staff_revision
AFTER UPDATE OF full_name, user_id, status ON staff
FOR EACH ROW EXECUTE FUNCTION project_explorer_staff_revision_trigger();

CREATE OR REPLACE FUNCTION project_explorer_project_members_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE'
     AND OLD.project_id IS NOT DISTINCT FROM NEW.project_id
     AND OLD.staff_id IS NOT DISTINCT FROM NEW.staff_id
     AND OLD.active IS NOT DISTINCT FROM NEW.active THEN
    RETURN NEW;
  END IF;

  PERFORM project_explorer_bump_actor(s.user_id)
  FROM staff s
  WHERE s.user_id IS NOT NULL
    AND s.id IN (
      CASE WHEN TG_OP = 'INSERT' THEN NEW.staff_id ELSE OLD.staff_id END,
      CASE WHEN TG_OP = 'DELETE' THEN OLD.staff_id ELSE NEW.staff_id END
    )
  GROUP BY s.user_id
  ORDER BY s.user_id;
  RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_project_members_revision ON project_members;
CREATE TRIGGER project_explorer_project_members_revision
AFTER INSERT OR UPDATE OR DELETE ON project_members
FOR EACH ROW EXECUTE FUNCTION project_explorer_project_members_revision_trigger();

CREATE OR REPLACE FUNCTION project_explorer_user_roles_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM project_explorer_bump_actor(user_id)
  FROM (
    SELECT CASE WHEN TG_OP = 'INSERT' THEN NEW.user_id ELSE OLD.user_id END AS user_id
    UNION
    SELECT CASE WHEN TG_OP = 'DELETE' THEN OLD.user_id ELSE NEW.user_id END
  ) affected
  WHERE user_id IS NOT NULL
  ORDER BY user_id;
  RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_user_roles_revision ON user_roles;
CREATE TRIGGER project_explorer_user_roles_revision
AFTER INSERT OR UPDATE OR DELETE ON user_roles
FOR EACH ROW EXECUTE FUNCTION project_explorer_user_roles_revision_trigger();

CREATE OR REPLACE FUNCTION project_explorer_roles_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.name IS DISTINCT FROM NEW.name AND (OLD.name IN ('admin', 'director') OR NEW.name IN ('admin', 'director')) THEN
    PERFORM project_explorer_bump_actor(ur.user_id)
    FROM user_roles ur
    WHERE ur.role_id = NEW.id
    GROUP BY ur.user_id
    ORDER BY ur.user_id;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_roles_revision ON roles;
CREATE TRIGGER project_explorer_roles_revision
AFTER UPDATE OF name ON roles
FOR EACH ROW EXECUTE FUNCTION project_explorer_roles_revision_trigger();

CREATE OR REPLACE FUNCTION project_explorer_partner_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.name IS DISTINCT FROM NEW.name OR OLD.tax_code IS DISTINCT FROM NEW.tax_code THEN
    PERFORM project_explorer_bump_project(p.id)
    FROM projects p
    WHERE p.partner_id = NEW.id
    ORDER BY p.id;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_partner_revision ON partners;
CREATE TRIGGER project_explorer_partner_revision
AFTER UPDATE OF name, tax_code ON partners
FOR EACH ROW EXECUTE FUNCTION project_explorer_partner_revision_trigger();

CREATE OR REPLACE FUNCTION project_explorer_template_revision_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.name IS DISTINCT FROM NEW.name THEN
    PERFORM project_explorer_bump_project(p.id)
    FROM projects p
    WHERE p.pipeline_template_id = NEW.id
    ORDER BY p.id;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_template_revision ON pipeline_templates;
CREATE TRIGGER project_explorer_template_revision
AFTER UPDATE OF name ON pipeline_templates
FOR EACH ROW EXECUTE FUNCTION project_explorer_template_revision_trigger();
