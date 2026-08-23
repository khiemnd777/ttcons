CREATE OR REPLACE FUNCTION enforce_sensitive_excel_role_name()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.name NOT IN ('admin','developer') AND EXISTS (
    SELECT 1
    FROM role_permissions assignment
    JOIN permissions permission ON permission.id=assignment.permission_id
    WHERE assignment.role_id=OLD.id
      AND permission.key IN ('project.import','workflow_definition.import')
  ) THEN
    RAISE EXCEPTION 'import permissions are reserved for admin and developer';
  END IF;

  IF NEW.name NOT IN ('admin','developer','pm') AND EXISTS (
    SELECT 1
    FROM role_permissions assignment
    JOIN permissions permission ON permission.id=assignment.permission_id
    WHERE assignment.role_id=OLD.id
      AND permission.key IN ('project.export','workflow_definition.export')
  ) THEN
    RAISE EXCEPTION 'export permissions are reserved for admin, developer and pm';
  END IF;

  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_sensitive_excel_role_name ON roles;
CREATE TRIGGER trg_sensitive_excel_role_name
BEFORE UPDATE OF name ON roles
FOR EACH ROW
WHEN (OLD.name IS DISTINCT FROM NEW.name)
EXECUTE FUNCTION enforce_sensitive_excel_role_name();
