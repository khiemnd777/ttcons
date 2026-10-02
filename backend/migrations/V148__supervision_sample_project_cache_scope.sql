-- Real company supervision excludes sample projects. Changing that boundary
-- must invalidate cached indexes, including an index that is currently empty.
CREATE FUNCTION project_explorer_sample_scope_trigger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.is_sample IS DISTINCT FROM NEW.is_sample THEN
    PERFORM project_explorer_bump_catalog(NEW.id);
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER project_explorer_sample_scope AFTER UPDATE OF is_sample ON projects
FOR EACH ROW EXECUTE FUNCTION project_explorer_sample_scope_trigger();
