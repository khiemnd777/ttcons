-- V97 treats a missing actor row as logical version 1. Invalidate every actor
-- once and ensure the first access mutation for a newly-created user advances
-- to version 2 rather than reusing that logical default.
INSERT INTO project_explorer_actor_versions(user_id, version, updated_at)
SELECT id, 2, now()
FROM users
ON CONFLICT (user_id) DO UPDATE
SET version = project_explorer_actor_versions.version + 1,
    updated_at = now();

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
  VALUES (p_user_id, 2, now())
  ON CONFLICT (user_id) DO UPDATE
  SET version = project_explorer_actor_versions.version + 1,
      updated_at = now()
  RETURNING version INTO next_version;

  INSERT INTO project_explorer_invalidation_outbox(event_kind, actor_id, revision)
  VALUES ('actor', p_user_id, next_version);
  RETURN next_version;
END;
$$;

CREATE OR REPLACE FUNCTION project_explorer_user_bootstrap_trigger()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO project_explorer_actor_versions(user_id, version, updated_at)
  VALUES (NEW.id, 1, now())
  ON CONFLICT (user_id) DO NOTHING;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS project_explorer_user_bootstrap ON users;
CREATE TRIGGER project_explorer_user_bootstrap
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION project_explorer_user_bootstrap_trigger();
