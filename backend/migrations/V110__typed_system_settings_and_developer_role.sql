ALTER TABLE system_settings
  ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'business',
  ADD COLUMN IF NOT EXISTS value_type TEXT NOT NULL DEFAULT 'string',
  ADD COLUMN IF NOT EXISTS is_secret BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS requires_restart BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS value_ciphertext BYTEA,
  ADD COLUMN IF NOT EXISTS value_nonce BYTEA,
  ADD COLUMN IF NOT EXISTS key_version SMALLINT,
  ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'database',
  ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname='system_settings_value_type_check'
  ) THEN
    ALTER TABLE system_settings
      ADD CONSTRAINT system_settings_value_type_check
      CHECK (value_type IN ('boolean','integer','number','duration','string','enum','multi_select','secret'));
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname='system_settings_version_check'
  ) THEN
    ALTER TABLE system_settings
      ADD CONSTRAINT system_settings_version_check CHECK (version > 0);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname='system_settings_secret_payload_check'
  ) THEN
    ALTER TABLE system_settings
      ADD CONSTRAINT system_settings_secret_payload_check CHECK (
        NOT is_secret
        OR (value_ciphertext IS NULL AND value_nonce IS NULL)
        OR (value_ciphertext IS NOT NULL AND value_nonce IS NOT NULL AND key_version IS NOT NULL)
      );
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_system_settings_category_key
  ON system_settings(category, key);

UPDATE system_settings
SET value_json = CASE
      WHEN jsonb_typeof(value_json)='object' THEN COALESCE(value_json->'construction', '30'::jsonb)
      WHEN jsonb_typeof(value_json)='number' THEN value_json
      ELSE '30'::jsonb
    END,
    category='business',
    value_type='number',
    is_secret=false,
    requires_restart=false,
    source='database',
    updated_at=now()
WHERE key='client_payment_min_percent';

UPDATE system_settings
SET value_json='["return_to_warehouse","return_to_subcontractor"]'::jsonb,
    category='business',
    value_type='multi_select',
    is_secret=false,
    requires_restart=false,
    source='database',
    updated_at=now()
WHERE key='surplus_handling_options';

INSERT INTO roles (id, name, display_name, description, system_role, external_only)
VALUES (
  '20000000-0000-0000-0000-000000000018',
  'developer',
  'Nhà phát triển',
  'Quản trị kỹ thuật và thiết lập hệ thống',
  true,
  false
)
ON CONFLICT (name) DO UPDATE
SET display_name=EXCLUDED.display_name,
    description=EXCLUDED.description,
    system_role=true,
    external_only=false;

DELETE FROM role_permissions role_permission
USING roles role, permissions permission
WHERE role_permission.role_id=role.id
  AND role_permission.permission_id=permission.id
  AND (
    (permission.key='settings.view' AND role.name NOT IN ('admin','developer'))
    OR (permission.key='settings.update' AND role.name <> 'developer')
    OR (permission.key='seal.manage' AND role.name NOT IN ('admin','developer'))
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
CROSS JOIN permissions permission
WHERE role.name='developer'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key IN ('settings.view','seal.use','seal.manage')
WHERE role.name='admin'
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_restricted_settings_permission_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_role TEXT;
  target_permission TEXT;
BEGIN
  SELECT name INTO target_role FROM roles WHERE id=NEW.role_id;
  SELECT key INTO target_permission FROM permissions WHERE id=NEW.permission_id;

  IF target_permission='settings.view' AND target_role NOT IN ('admin','developer') THEN
    RAISE EXCEPTION 'settings.view is reserved for admin and developer';
  END IF;
  IF target_permission='settings.update' AND target_role <> 'developer' THEN
    RAISE EXCEPTION 'settings.update is reserved for developer';
  END IF;
  IF target_permission='seal.manage' AND target_role NOT IN ('admin','developer') THEN
    RAISE EXCEPTION 'seal.manage is reserved for admin and developer';
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_restricted_settings_permission_assignment ON role_permissions;
CREATE TRIGGER trg_restricted_settings_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_restricted_settings_permission_assignment();
