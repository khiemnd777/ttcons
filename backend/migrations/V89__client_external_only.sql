-- The client is a business counterpart, not an application actor.
-- Historical workflow/runtime role labels remain immutable; only active App access
-- and future assignment/publication paths are closed.

ALTER TABLE roles
  ADD COLUMN IF NOT EXISTS external_only BOOLEAN NOT NULL DEFAULT false;

UPDATE roles
SET external_only=true
WHERE name='client';

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM user_roles external_assignment
    JOIN roles external_role
      ON external_role.id=external_assignment.role_id
     AND external_role.external_only
    WHERE EXISTS (
      SELECT 1
      FROM user_roles internal_assignment
      JOIN roles internal_role ON internal_role.id=internal_assignment.role_id
      WHERE internal_assignment.user_id=external_assignment.user_id
        AND NOT internal_role.external_only
    )
  ) THEN
    RAISE EXCEPTION 'Cannot make client external-only: at least one user also has an internal role';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM work_items item
    WHERE item.status NOT IN ('completed','cancelled')
      AND jsonb_typeof(item.role_ids)='array'
      AND jsonb_array_length(item.role_ids)>0
      AND NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements_text(item.role_ids) required(role_label)
        LEFT JOIN roles role
          ON lower(btrim(role.display_name))=lower(btrim(required.role_label))
        WHERE role.id IS NULL OR NOT role.external_only
      )
  ) THEN
    RAISE EXCEPTION 'Cannot make client external-only: at least one open work item requires only external roles';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM approval_requests request
    WHERE request.status='pending'
      AND jsonb_typeof(request.required_roles)='array'
      AND jsonb_array_length(request.required_roles)>0
      AND NOT EXISTS (
        SELECT 1
        FROM jsonb_array_elements_text(request.required_roles) required(role_label)
        LEFT JOIN roles role
          ON lower(btrim(role.display_name))=lower(btrim(required.role_label))
        WHERE role.id IS NULL OR NOT role.external_only
      )
  ) THEN
    RAISE EXCEPTION 'Cannot make client external-only: at least one pending approval requires only external roles';
  END IF;
END $$;

DELETE FROM role_permissions permission
USING roles role
WHERE permission.role_id=role.id
  AND role.external_only;

UPDATE auth_sessions session
SET revoked_at=COALESCE(session.revoked_at,now())
WHERE EXISTS (
  SELECT 1
  FROM user_roles assignment
  JOIN roles role ON role.id=assignment.role_id
  WHERE assignment.user_id=session.user_id
    AND role.external_only
);

UPDATE project_members member
SET active=false,
    updated_at=now()
FROM staff person
WHERE person.id=member.staff_id
  AND member.active
  AND EXISTS (
    SELECT 1
    FROM user_roles assignment
    JOIN roles role ON role.id=assignment.role_id
    WHERE assignment.user_id=person.user_id
      AND role.external_only
  );

UPDATE staff person
SET status='inactive',
    updated_at=now()
WHERE person.status='active'
  AND EXISTS (
    SELECT 1
    FROM user_roles assignment
    JOIN roles role ON role.id=assignment.role_id
    WHERE assignment.user_id=person.user_id
      AND role.external_only
  );

UPDATE users account
SET status='inactive',
    updated_at=now()
WHERE account.status='active'
  AND EXISTS (
    SELECT 1
    FROM user_roles assignment
    JOIN roles role ON role.id=assignment.role_id
    WHERE assignment.user_id=account.id
      AND role.external_only
  );
