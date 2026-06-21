WITH daily_template AS (
  SELECT id
  FROM workflow_form_templates
  WHERE code='builtin_daily_report'
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  id,
  2,
  'published',
  '{"source":"builtin","code":"builtin_daily_report","variant":"planning_structured_v2"}'::jsonb,
  '{"phase":"planning","creates_work_items_from":"next_plan_items"}'::jsonb,
  now()
FROM daily_template
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH daily_version AS (
  SELECT v.id AS version_id
  FROM workflow_form_versions v
  JOIN workflow_form_templates t ON t.id=v.template_id
  WHERE t.code='builtin_daily_report' AND v.version_no=2
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('report_date', 'Ngày report', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('reported_by', 'Người report', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":120}'::jsonb, NULL, 20),
    (
      'planning_scope',
      'Phạm vi kế hoạch',
      'select',
      true,
      '[
        {"value":"survey","label":"Khảo sát"},
        {"value":"design","label":"Thiết kế"},
        {"value":"boq","label":"BOQ / Dự toán"},
        {"value":"budget","label":"Ngân sách"},
        {"value":"procurement","label":"Mua sắm"},
        {"value":"site_preparation","label":"Chuẩn bị thi công"},
        {"value":"other","label":"Khác"}
      ]'::jsonb,
      '{}'::jsonb,
      NULL,
      30
    ),
    ('progress_percent', 'Tiến độ %', 'number', false, '[]'::jsonb, '{"min":0,"max":100}'::jsonb, NULL, 40),
    (
      'completed_items',
      'Đã làm',
      'structured_list',
      true,
      '[]'::jsonb,
      '{
        "min_items": 1,
        "max_items": 120,
        "item_fields": [
          {"key":"title","label":"Hạng mục","type":"text","required":true,"min_length":3,"max_length":180},
          {"key":"owner_role","label":"Owner role","type":"text","required":false,"max_length":80},
          {"key":"result","label":"Kết quả","type":"select","required":true,"options":[{"value":"done","label":"Done"},{"value":"partial","label":"Partial"},{"value":"blocked","label":"Blocked"}]},
          {"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}
        ]
      }'::jsonb,
      NULL,
      50
    ),
    (
      'next_plan_items',
      'Sẽ làm',
      'structured_list',
      false,
      '[]'::jsonb,
      '{
        "max_items": 120,
        "item_fields": [
          {"key":"title","label":"Việc cần làm","type":"text","required":true,"min_length":3,"max_length":180},
          {"key":"owner_role","label":"Owner role","type":"text","required":true,"min_length":2,"max_length":80},
          {"key":"owner_name","label":"Owner","type":"text","required":false,"max_length":120},
          {"key":"due_date","label":"Due date","type":"date","required":true},
          {"key":"priority","label":"Priority","type":"select","required":true,"options":[{"value":"low","label":"Low"},{"value":"normal","label":"Normal"},{"value":"high","label":"High"},{"value":"urgent","label":"Urgent"}]},
          {"key":"description","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}
        ]
      }'::jsonb,
      'Submit sẽ tạo work item theo từng việc cần làm.',
      60
    ),
    (
      'blockers',
      'Blocker / Risk',
      'structured_list',
      false,
      '[]'::jsonb,
      '{
        "max_items": 80,
        "item_fields": [
          {"key":"content","label":"Nội dung","type":"textarea","required":true,"min_length":3,"max_length":1000},
          {"key":"impact","label":"Impact","type":"select","required":true,"options":[{"value":"schedule","label":"Tiến độ"},{"value":"budget","label":"Ngân sách"},{"value":"material","label":"Vật tư"},{"value":"design","label":"Thiết kế"},{"value":"approval","label":"Phê duyệt"},{"value":"other","label":"Khác"}]},
          {"key":"owner_role","label":"Owner role","type":"text","required":false,"max_length":80},
          {"key":"target_date","label":"Target date","type":"date","required":false},
          {"key":"severity","label":"Severity","type":"select","required":true,"options":[{"value":"low","label":"Low"},{"value":"medium","label":"Medium"},{"value":"high","label":"High"},{"value":"critical","label":"Critical"}]}
        ]
      }'::jsonb,
      NULL,
      70
    ),
    ('confirmed', 'Xác nhận report', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, NULL, 80)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT dv.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM daily_version dv
CROSS JOIN field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;
