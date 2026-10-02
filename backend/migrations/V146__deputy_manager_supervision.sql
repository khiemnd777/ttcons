-- Add a distinct company supervision role without changing PM or assignments.
INSERT INTO roles (id,name,display_name,description,system_role,external_only)
VALUES ('20000000-0000-0000-0000-000000000019','deputy_manager','Phó Quản Lý',
  'Theo dõi tiến độ toàn công ty, công việc và báo cáo Giám sát',true,false)
ON CONFLICT (name) DO NOTHING;

INSERT INTO permissions (id,key,name,module,description)
VALUES (gen_random_uuid(),'project.supervision.view','Theo dõi Giám sát toàn công ty','project',
  'Phạm vi công ty cho Phó Quản Lý; không cấp quyền điều phối dự án, phê duyệt hay tài chính')
ON CONFLICT (key) DO NOTHING;

CREATE FUNCTION enforce_supervision_permission_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM permissions WHERE id=NEW.permission_id AND key='project.supervision.view')
     AND NOT EXISTS (SELECT 1 FROM roles WHERE id=NEW.role_id AND name='deputy_manager' AND system_role AND NOT external_only) THEN
    RAISE EXCEPTION 'project.supervision.view is reserved for the internal system deputy_manager role';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER trg_supervision_permission_assignment BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_supervision_permission_assignment();

CREATE FUNCTION preserve_supervision_role_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.name='deputy_manager' AND (NEW.name <> OLD.name OR NOT NEW.system_role OR NEW.external_only) THEN
    RAISE EXCEPTION 'the internal system deputy_manager identity is immutable';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER trg_supervision_role_identity BEFORE UPDATE OF name,system_role,external_only ON roles
FOR EACH ROW EXECUTE FUNCTION preserve_supervision_role_identity();

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id FROM roles role JOIN permissions permission ON permission.key IN (
  'project.supervision.view','project.view','pipeline.view','planning.view','operation.view',
  'work_report.view','work_report.review','work_report.export'
) WHERE role.name='deputy_manager' AND role.system_role AND NOT role.external_only
ON CONFLICT DO NOTHING;

ALTER TABLE work_reports ADD COLUMN is_supervisor_report BOOLEAN NOT NULL DEFAULT false;
-- An additive classification projection; immutable report snapshots/events stay
-- unchanged. Historical classification uses the canonical roles available now.
UPDATE work_reports report SET is_supervisor_report=true
WHERE EXISTS (SELECT 1 FROM users author WHERE author.id=report.author_id AND NOT author.is_sample AND (
  EXISTS (SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
    WHERE assignment.user_id=author.id AND (assignment.project_id IS NULL OR assignment.project_id=report.project_id) AND role.name='supervisor' AND NOT role.external_only)
  OR EXISTS (SELECT 1 FROM staff person JOIN project_members member ON member.staff_id=person.id
    WHERE person.user_id=author.id AND person.status <> 'deleted' AND member.project_id=report.project_id AND member.active
    AND (member.role_ids ?| ARRAY['supervisor','Giám sát'] OR EXISTS (SELECT 1 FROM roles role WHERE role.name='supervisor' AND member.role_ids ? role.id::text)))
));
CREATE INDEX idx_work_reports_supervision_date ON work_reports(report_date DESC,id) WHERE is_supervisor_report;

CREATE TABLE work_report_read_receipts (
  report_id UUID NOT NULL,
  revision_no INTEGER NOT NULL,
  user_id UUID NOT NULL REFERENCES users(id),
  acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  comment TEXT,
  PRIMARY KEY (report_id,revision_no,user_id),
  FOREIGN KEY (report_id,revision_no) REFERENCES work_report_revisions(report_id,revision_no) ON DELETE CASCADE
);
-- Derive receipts only from actual acknowledgement events, never from shared
-- status or the fact that a page was opened.
INSERT INTO work_report_read_receipts(report_id,revision_no,user_id,acknowledged_at,comment)
SELECT DISTINCT ON (report_id,revision_no,actor_id) report_id,revision_no,actor_id,created_at,comment
FROM work_report_events WHERE event_type='acknowledged' AND actor_id IS NOT NULL
ORDER BY report_id,revision_no,actor_id,created_at,id;
