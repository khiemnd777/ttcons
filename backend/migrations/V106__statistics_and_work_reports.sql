INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,module
FROM (VALUES
  ('statistics.view','Xem thống kê','Xem số liệu tổng hợp trong phạm vi được phân quyền','statistics'),
  ('statistics.export','Xuất dữ liệu thống kê','Xuất số liệu thống kê ra PDF, Excel hoặc CSV','statistics'),
  ('work_report.view','Xem báo cáo công việc','Xem báo cáo công việc trong phạm vi được phân quyền','work_report'),
  ('work_report.create','Lập báo cáo công việc','Lập báo cáo công việc từ nguồn nghiệp vụ','work_report'),
  ('work_report.submit','Gửi báo cáo công việc','Gửi hoặc gửi lại báo cáo công việc','work_report'),
  ('work_report.review','Xem xét báo cáo công việc','Yêu cầu bổ sung hoặc xác nhận đã xem báo cáo','work_report'),
  ('work_report.export','Xuất báo cáo công việc','Xuất báo cáo công việc ra PDF, Excel hoặc CSV','work_report'),
  ('work_report.manage','Quản lý báo cáo công việc','Xem và quản lý báo cáo thuộc công trình được phân quyền','work_report')
) permission_seed(key,name,description,module)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    module=EXCLUDED.module;

-- Preserve the audience of the former aggregate Reports workspace without
-- repurposing report.view as access to authored employee reports.
INSERT INTO role_permissions (role_id,permission_id)
SELECT DISTINCT existing.role_id,target.id
FROM role_permissions existing
JOIN permissions legacy ON legacy.id=existing.permission_id AND legacy.key='report.view'
JOIN permissions target ON target.key IN ('statistics.view','statistics.export')
ON CONFLICT DO NOTHING;

-- Field-report authors retain their existing construction responsibility.
INSERT INTO role_permissions (role_id,permission_id)
SELECT DISTINCT existing.role_id,target.id
FROM role_permissions existing
JOIN permissions source_permission ON source_permission.id=existing.permission_id AND source_permission.key='operation.report'
JOIN permissions target ON target.key IN (
  'work_report.view','work_report.create','work_report.submit','work_report.export'
)
ON CONFLICT DO NOTHING;

-- Management roles receive report inbox/review capabilities. Project scope and
-- recipient identity remain enforced by the API for every record.
INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON
  (role.name='admin' AND permission.key IN (
    'statistics.view','statistics.export','work_report.view','work_report.create',
    'work_report.submit','work_report.review','work_report.export','work_report.manage'
  )) OR
  (role.name='director' AND permission.key IN (
    'work_report.view','work_report.review','work_report.export','work_report.manage'
  )) OR
  (role.name='pm' AND permission.key IN (
    'work_report.view','work_report.create','work_report.submit','work_report.review',
    'work_report.export','work_report.manage'
  ))
ON CONFLICT DO NOTHING;

ALTER TABLE operation_daily_reports
  ADD COLUMN IF NOT EXISTS report_delivery_eligible BOOLEAN NOT NULL DEFAULT true;

-- Historical rows remain readable but are not represented as if they had been
-- formally sent or acknowledged before this lifecycle existed.
UPDATE operation_daily_reports
SET report_delivery_eligible=false
WHERE report_delivery_eligible=true;

CREATE TABLE IF NOT EXISTS work_reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_kind TEXT NOT NULL,
  source_id UUID NOT NULL,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  report_type TEXT NOT NULL,
  report_date DATE NOT NULL,
  author_id UUID NOT NULL REFERENCES users(id),
  recipient_id UUID NOT NULL REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'submitted',
  current_revision_no INTEGER NOT NULL DEFAULT 1,
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  reviewed_by UUID REFERENCES users(id),
  reviewed_at TIMESTAMPTZ,
  review_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT work_reports_source_kind_check CHECK (source_kind IN ('operation_daily_report','workflow_form_instance')),
  CONSTRAINT work_reports_type_nonblank CHECK (NULLIF(btrim(report_type),'') IS NOT NULL),
  CONSTRAINT work_reports_status_check CHECK (status IN ('submitted','needs_changes','acknowledged')),
  CONSTRAINT work_reports_revision_positive CHECK (current_revision_no > 0),
  CONSTRAINT work_reports_review_state_check CHECK (
    (status='submitted' AND reviewed_by IS NULL AND reviewed_at IS NULL) OR
    (status IN ('needs_changes','acknowledged') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
  ),
  UNIQUE (source_kind,source_id)
);

CREATE INDEX IF NOT EXISTS idx_work_reports_project_date
  ON work_reports(project_id,report_date DESC,id);

CREATE INDEX IF NOT EXISTS idx_work_reports_author_status
  ON work_reports(author_id,status,report_date DESC);

CREATE INDEX IF NOT EXISTS idx_work_reports_recipient_status
  ON work_reports(recipient_id,status,report_date DESC);

CREATE TABLE IF NOT EXISTS work_report_revisions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id UUID NOT NULL REFERENCES work_reports(id) ON DELETE CASCADE,
  revision_no INTEGER NOT NULL,
  snapshot_json JSONB NOT NULL,
  snapshot_sha256 TEXT NOT NULL,
  submitted_by UUID NOT NULL REFERENCES users(id),
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT work_report_revisions_revision_positive CHECK (revision_no > 0),
  CONSTRAINT work_report_revisions_hash_check CHECK (snapshot_sha256 ~ '^[a-f0-9]{64}$'),
  UNIQUE (report_id,revision_no)
);

CREATE INDEX IF NOT EXISTS idx_work_report_revisions_report
  ON work_report_revisions(report_id,revision_no DESC);

CREATE TABLE IF NOT EXISTS work_report_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id UUID NOT NULL REFERENCES work_reports(id) ON DELETE CASCADE,
  revision_no INTEGER NOT NULL,
  event_type TEXT NOT NULL,
  actor_id UUID NOT NULL REFERENCES users(id),
  comment TEXT,
  metadata_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT work_report_events_revision_positive CHECK (revision_no > 0),
  CONSTRAINT work_report_events_type_check CHECK (event_type IN ('submitted','resubmitted','needs_changes','acknowledged'))
);

CREATE INDEX IF NOT EXISTS idx_work_report_events_report
  ON work_report_events(report_id,created_at,id);

CREATE OR REPLACE FUNCTION prevent_work_report_history_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  -- Keep history immutable while the report exists. PostgreSQL executes the
  -- child cascade after deleting the parent row, so this exception allows a
  -- deliberate parent/project teardown without leaving undeletable records.
  IF TG_OP = 'DELETE'
     AND NOT EXISTS (SELECT 1 FROM work_reports WHERE id = OLD.report_id) THEN
    RETURN OLD;
  END IF;

  RAISE EXCEPTION 'work report history is immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_work_report_revisions_immutable ON work_report_revisions;
CREATE TRIGGER trg_work_report_revisions_immutable
BEFORE UPDATE OR DELETE ON work_report_revisions
FOR EACH ROW EXECUTE FUNCTION prevent_work_report_history_mutation();

DROP TRIGGER IF EXISTS trg_work_report_events_immutable ON work_report_events;
CREATE TRIGGER trg_work_report_events_immutable
BEFORE UPDATE OR DELETE ON work_report_events
FOR EACH ROW EXECUTE FUNCTION prevent_work_report_history_mutation();
