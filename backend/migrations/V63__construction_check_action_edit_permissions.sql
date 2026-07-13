INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,'construction'
FROM (VALUES
  ('construction.check.edit','Chỉnh sửa lịch kiểm tra','Chỉnh sửa kiểm tra thủ công khi chưa ghi nhận kết quả'),
  ('construction.action.edit','Chỉnh sửa việc cần xử lý','Chỉnh sửa nội dung và phân công của việc đang mở')
) p(key,name,description)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r
JOIN permissions p ON p.module='construction' AND p.key IN ('construction.check.edit','construction.action.edit')
WHERE r.name='admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r
JOIN permissions p ON p.module='construction'
WHERE r.name IN ('pm','qa_qc','supervisor','construction_team')
  AND p.key IN ('construction.check.edit','construction.action.edit')
ON CONFLICT DO NOTHING;
