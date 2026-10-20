CREATE TABLE account_recovery_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','issued','completed','rejected')),
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  issued_at TIMESTAMPTZ,
  issued_by UUID REFERENCES users(id) ON DELETE SET NULL,
  verification_method TEXT CHECK (verification_method IN ('in_person','voice_call','known_channel')),
  completed_at TIMESTAMPTZ,
  rejected_at TIMESTAMPTZ,
  rejected_by UUID REFERENCES users(id) ON DELETE SET NULL,
  CHECK (status NOT IN ('issued','completed') OR issued_at IS NOT NULL),
  CHECK (status NOT IN ('issued','completed') OR verification_method IS NOT NULL),
  CHECK ((status='completed') = (completed_at IS NOT NULL)),
  CHECK ((status='rejected') = (rejected_at IS NOT NULL))
);

CREATE UNIQUE INDEX uq_account_recovery_open_user
  ON account_recovery_requests(user_id) WHERE status IN ('pending','issued');
CREATE INDEX idx_account_recovery_status_requested
  ON account_recovery_requests(status,requested_at DESC,id DESC);

ALTER TABLE auth_email_tokens DROP CONSTRAINT auth_email_tokens_purpose_check;
ALTER TABLE auth_email_tokens ADD CONSTRAINT auth_email_tokens_purpose_check
  CHECK (purpose IN ('verify_recovery_email','reset_password','developer_reset_password'));
ALTER TABLE auth_email_tokens
  ADD COLUMN recovery_request_id UUID REFERENCES account_recovery_requests(id) ON DELETE CASCADE;
ALTER TABLE auth_email_tokens ADD CONSTRAINT auth_email_tokens_developer_request_check
  CHECK ((purpose='developer_reset_password') = (recovery_request_id IS NOT NULL));

INSERT INTO permissions (id,key,name,description,module)
VALUES (gen_random_uuid(),'account_recovery.manage','Xử lý yêu cầu khôi phục mật khẩu',
        'Xem yêu cầu và cấp liên kết đặt lại mật khẩu sau khi xác minh danh tính','auth')
ON CONFLICT (key) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id FROM roles role
JOIN permissions permission ON permission.key='account_recovery.manage'
WHERE role.name='developer' AND role.system_role AND NOT role.external_only
ON CONFLICT DO NOTHING;

CREATE FUNCTION enforce_developer_recovery_permission_assignment()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF (SELECT key FROM permissions WHERE id=NEW.permission_id)='account_recovery.manage'
     AND NOT EXISTS (SELECT 1 FROM roles WHERE id=NEW.role_id AND name='developer' AND system_role AND NOT external_only) THEN
    RAISE EXCEPTION 'account_recovery.manage is reserved for the system developer role';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER trg_developer_recovery_permission_assignment
BEFORE INSERT OR UPDATE OF role_id,permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION enforce_developer_recovery_permission_assignment();

INSERT INTO notification_event_types (
  event_key,domain_key,category_key,category_label,intent,severity,icon_key,tone_key,
  audience_policy_key,visibility_policy_key,permissions_any,action_route_template,
  action_label,required_delivery,enabled
) VALUES (
  'account.recovery_requested','admin','security','Bảo mật','action_required','warning','shield','warning',
  'explicit_users','project_permissions_any',ARRAY['account_recovery.manage']::text[],
  '/recovery-requests?request={{request_id}}','Xem yêu cầu',true,true
);
INSERT INTO notification_templates (
  event_key,locale,version,title_template,body_template,required_payload_keys,active
) VALUES (
  'account.recovery_requested','vi-VN',1,'Yêu cầu khôi phục mật khẩu',
  'Một tài khoản cần xác minh danh tính trước khi cấp liên kết đặt lại mật khẩu.',
  ARRAY['request_id']::text[],true
);
