-- Marker revocation/restoration changes the company-wide Explorer index even
-- when no user_role row changes. Invalidate both warm and cached-empty indexes.
CREATE FUNCTION project_explorer_supervision_grant_trigger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE old_role UUID; new_role UUID; old_permission UUID; new_permission UUID;
BEGIN
  IF TG_OP <> 'INSERT' THEN old_role=OLD.role_id; old_permission=OLD.permission_id; END IF;
  IF TG_OP <> 'DELETE' THEN new_role=NEW.role_id; new_permission=NEW.permission_id; END IF;
  IF EXISTS (SELECT 1 FROM permissions WHERE id IN (old_permission,new_permission) AND key='project.supervision.view') THEN
    PERFORM project_explorer_bump_actor(assignment.user_id)
    FROM (SELECT DISTINCT user_id FROM user_roles WHERE role_id IN (old_role,new_role) ORDER BY user_id) assignment;
  END IF;
  IF TG_OP='DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER project_explorer_supervision_grant AFTER INSERT OR DELETE OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION project_explorer_supervision_grant_trigger();
