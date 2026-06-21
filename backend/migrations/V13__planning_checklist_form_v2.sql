WITH checklist_template AS (
  SELECT id
  FROM workflow_form_templates
  WHERE code='builtin_checklist'
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  id,
  2,
  'published',
  '{"source":"builtin","code":"builtin_checklist","variant":"planning_structured_v2"}'::jsonb,
  '{"phase":"planning","submit_requires_all_items_clear":true}'::jsonb,
  now()
FROM checklist_template
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH checklist_version AS (
  SELECT v.id AS version_id
  FROM workflow_form_versions v
  JOIN workflow_form_templates t ON t.id=v.template_id
  WHERE t.code='builtin_checklist' AND v.version_no=2
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('check_date', 'Ngày kiểm tra', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('checked_by', 'Người kiểm tra', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":120}'::jsonb, NULL, 20),
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
    (
      'checklist_items',
      'Checklist items',
      'structured_list',
      true,
      '[]'::jsonb,
      '{
        "min_items": 1,
        "max_items": 200,
        "submit_clear_statuses": ["done", "not_applicable"],
        "item_fields": [
          {"key":"title","label":"Hạng mục","type":"text","required":true,"min_length":3,"max_length":180},
          {"key":"assignee_role","label":"Assignee role","type":"text","required":true,"min_length":2,"max_length":80},
          {"key":"assignee_name","label":"Assignee","type":"text","required":false,"max_length":120},
          {"key":"status","label":"Status","type":"select","required":true,"options":[{"value":"done","label":"Done"},{"value":"not_applicable","label":"N/A"},{"value":"needs_follow_up","label":"Needs follow-up"},{"value":"blocked","label":"Blocked"}]},
          {"key":"due_date","label":"Due date","type":"date","required":false},
          {"key":"evidence_id","label":"Evidence","type":"evidence_reference","required":false},
          {"key":"comment","label":"Comment","type":"textarea","required":false,"max_length":1000}
        ]
      }'::jsonb,
      'Submit chỉ được phép khi mọi item ở trạng thái Done hoặc N/A.',
      40
    ),
    ('overall_comment', 'Ghi chú tổng quan', 'textarea', false, '[]'::jsonb, '{"max_length":2000}'::jsonb, NULL, 50),
    ('confirmed', 'Xác nhận checklist', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, NULL, 60)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT cv.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM checklist_version cv
CROSS JOIN field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;
