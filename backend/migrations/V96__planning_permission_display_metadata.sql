-- Keep the machine-readable key stable for authorization while ensuring every
-- environment has the approved Vietnamese metadata used by work-item detail.
INSERT INTO permissions (id, key, name, description, module)
VALUES (
  gen_random_uuid(),
  'planning.edit',
  'Cập nhật kế hoạch',
  'Cập nhật nội dung kế hoạch được phân công',
  'planning'
)
ON CONFLICT (key) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    module = EXCLUDED.module;
