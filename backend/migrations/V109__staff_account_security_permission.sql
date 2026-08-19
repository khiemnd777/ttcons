INSERT INTO permissions (id, key, name, description, module)
VALUES (
  gen_random_uuid(),
  'staff.account_security.manage',
  'Quản trị bảo mật tài khoản nhân sự',
  'Đặt mật khẩu và email khôi phục đã xác minh cho tài khoản liên kết với nhân sự',
  'staff'
)
ON CONFLICT (key) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    module = EXCLUDED.module;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key = 'staff.account_security.manage'
WHERE role.name = 'admin'
ON CONFLICT DO NOTHING;
