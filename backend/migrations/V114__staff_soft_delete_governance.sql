ALTER TABLE users
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deleted_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE staff
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deleted_by UUID REFERENCES users(id) ON DELETE SET NULL;

UPDATE staff
SET deleted_at=COALESCE(updated_at,created_at,now())
WHERE status='deleted' AND deleted_at IS NULL;

UPDATE users account
SET status='inactive',
    deleted_at=COALESCE(account.updated_at,account.created_at,now()),
    recovery_email=NULL,
    recovery_email_verified_at=NULL,
    must_change_password=false,
    updated_at=now()
WHERE account.deleted_at IS NULL
  AND EXISTS (
    SELECT 1
    FROM staff person
    WHERE person.user_id=account.id AND person.status='deleted'
  )
  AND NOT EXISTS (
    SELECT 1
    FROM staff person
    WHERE person.user_id=account.id AND person.status<>'deleted'
  );

UPDATE project_members member
SET active=false,
    updated_at=now()
FROM staff person
WHERE person.id=member.staff_id
  AND person.status='deleted'
  AND member.active;

UPDATE auth_sessions session
SET revoked_at=COALESCE(session.revoked_at,now())
FROM users account
WHERE account.id=session.user_id
  AND account.deleted_at IS NOT NULL
  AND session.revoked_at IS NULL;

UPDATE auth_email_tokens token
SET invalidated_at=now()
FROM users account
WHERE account.id=token.user_id
  AND account.deleted_at IS NOT NULL
  AND token.consumed_at IS NULL
  AND token.invalidated_at IS NULL;

DELETE FROM user_roles assignment
USING users account
WHERE account.id=assignment.user_id
  AND account.deleted_at IS NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname='users_deleted_state_check'
  ) THEN
    ALTER TABLE users
      ADD CONSTRAINT users_deleted_state_check
      CHECK (deleted_at IS NULL OR status='inactive');
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname='staff_deleted_state_check'
  ) THEN
    ALTER TABLE staff
      ADD CONSTRAINT staff_deleted_state_check
      CHECK ((status='deleted')=(deleted_at IS NOT NULL));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_deleted_at
  ON users(deleted_at)
  WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_staff_deleted_at
  ON staff(deleted_at)
  WHERE deleted_at IS NOT NULL;

INSERT INTO permissions (id,key,name,description,module)
VALUES (
  gen_random_uuid(),
  'staff.delete',
  'Xóa nhân sự',
  'Soft-delete tài khoản nhân sự, thu hồi quyền truy cập và giữ nguyên liên kết nghiệp vụ',
  'staff'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    module=EXCLUDED.module;

DELETE FROM role_permissions assignment
USING roles role,permissions permission
WHERE assignment.role_id=role.id
  AND assignment.permission_id=permission.id
  AND permission.key='staff.delete'
  AND role.name NOT IN ('admin','developer');

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON permission.key='staff.delete'
WHERE role.name IN ('admin','developer')
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_restricted_staff_delete_permission_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_role TEXT;
  target_permission TEXT;
BEGIN
  SELECT name INTO target_role FROM roles WHERE id=NEW.role_id;
  SELECT key INTO target_permission FROM permissions WHERE id=NEW.permission_id;

  IF target_permission='staff.delete' AND target_role NOT IN ('admin','developer') THEN
    RAISE EXCEPTION 'staff.delete is reserved for admin and developer';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_restricted_staff_delete_permission_assignment ON role_permissions;
CREATE TRIGGER trg_restricted_staff_delete_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_restricted_staff_delete_permission_assignment();

CREATE TABLE IF NOT EXISTS staff_deletion_batches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key UUID NOT NULL UNIQUE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  preview_hash TEXT NOT NULL,
  staff_count INTEGER NOT NULL,
  deleted_count INTEGER NOT NULL DEFAULT 0,
  result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT staff_deletion_batches_preview_hash_check CHECK (preview_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT staff_deletion_batches_staff_count_check CHECK (staff_count BETWEEN 1 AND 100),
  CONSTRAINT staff_deletion_batches_deleted_count_check CHECK (deleted_count BETWEEN 0 AND staff_count),
  CONSTRAINT staff_deletion_batches_result_object_check CHECK (jsonb_typeof(result_json)='object')
);

CREATE INDEX IF NOT EXISTS idx_staff_deletion_batches_actor_created
  ON staff_deletion_batches(actor_id,created_at DESC)
  WHERE actor_id IS NOT NULL;
