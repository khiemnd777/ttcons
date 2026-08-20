CREATE TABLE IF NOT EXISTS event_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  category TEXT NOT NULL,
  http_status INTEGER NOT NULL,
  error_code TEXT NOT NULL DEFAULT 'INTERNAL_ERROR',
  method TEXT NOT NULL,
  route_template TEXT NOT NULL,
  request_class TEXT NOT NULL DEFAULT 'general',
  request_id TEXT NOT NULL,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  actor_email TEXT,
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  error_type TEXT,
  error_fingerprint TEXT NOT NULL,
  internal_message TEXT,
  stack_trace TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT event_logs_category_check CHECK (category IN ('unexpected_5xx','panic','controlled_503')),
  CONSTRAINT event_logs_http_status_check CHECK (http_status BETWEEN 500 AND 599),
  CONSTRAINT event_logs_request_id_length_check CHECK (length(request_id) BETWEEN 1 AND 128),
  CONSTRAINT event_logs_duration_check CHECK (duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_event_logs_created
  ON event_logs(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_event_logs_category_created
  ON event_logs(category, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_logs_request_id
  ON event_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_event_logs_actor_created
  ON event_logs(actor_id, created_at DESC)
  WHERE actor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_event_logs_project_created
  ON event_logs(project_id, created_at DESC)
  WHERE project_id IS NOT NULL;

INSERT INTO permissions (id, key, name, description, module)
VALUES (
  gen_random_uuid(),
  'event_log.view',
  'Xem nhật ký sự kiện',
  'Tra cứu lỗi kỹ thuật 5xx đã lọc dữ liệu nhạy cảm',
  'event_log'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    module=EXCLUDED.module;

DELETE FROM role_permissions role_permission
USING roles role, permissions permission
WHERE role_permission.role_id=role.id
  AND role_permission.permission_id=permission.id
  AND permission.key='event_log.view'
  AND role.name NOT IN ('admin','developer');

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key='event_log.view'
WHERE role.name IN ('admin','developer')
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_restricted_event_log_permission_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_role TEXT;
  target_permission TEXT;
BEGIN
  SELECT name INTO target_role FROM roles WHERE id=NEW.role_id;
  SELECT key INTO target_permission FROM permissions WHERE id=NEW.permission_id;

  IF target_permission='event_log.view' AND target_role NOT IN ('admin','developer') THEN
    RAISE EXCEPTION 'event_log.view is reserved for admin and developer';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_restricted_event_log_permission_assignment ON role_permissions;
CREATE TRIGGER trg_restricted_event_log_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_restricted_event_log_permission_assignment();
