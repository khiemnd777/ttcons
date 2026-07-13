ALTER TABLE project_members
  ADD COLUMN IF NOT EXISTS notes TEXT,
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

WITH ranked AS (
  SELECT id,
         row_number() OVER (
           PARTITION BY project_id, staff_id
           ORDER BY active DESC, created_at DESC, id DESC
         ) AS membership_rank
  FROM project_members
  WHERE active
)
UPDATE project_members pm
SET active=false, updated_at=now()
FROM ranked
WHERE ranked.id=pm.id AND ranked.membership_rank > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_project_members_active_staff
  ON project_members(project_id, staff_id)
  WHERE active;

INSERT INTO permissions (id, key, name, description, module)
VALUES (
  gen_random_uuid(),
  'project.member.manage',
  'Quản lý thành viên công trình',
  'Thêm, cập nhật vai trò/quyền và ngừng tham gia thành viên trong phạm vi công trình',
  'project'
)
ON CONFLICT (key) DO UPDATE SET
  name=EXCLUDED.name,
  description=EXCLUDED.description,
  module=EXCLUDED.module;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key='project.member.manage'
WHERE r.name IN ('admin','director','pm')
ON CONFLICT DO NOTHING;
