CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_projects_explorer_name_trgm
  ON projects USING gin (lower(name) gin_trgm_ops)
  WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_projects_explorer_code_trgm
  ON projects USING gin (lower(code) gin_trgm_ops)
  WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_projects_explorer_client_trgm
  ON projects USING gin (lower(client_name) gin_trgm_ops)
  WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_staff_explorer_name_trgm
  ON staff USING gin (lower(full_name) gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_projects_explorer_created
  ON projects(created_at DESC, id)
  WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_project_members_actor_scope
  ON project_members(staff_id, project_id)
  WHERE active;

CREATE INDEX IF NOT EXISTS idx_user_roles_actor_project_scope
  ON user_roles(user_id, project_id);

CREATE INDEX IF NOT EXISTS idx_issues_project_status
  ON issues(project_id, status);

CREATE INDEX IF NOT EXISTS idx_approval_requests_project_status
  ON approval_requests(project_id, status);
