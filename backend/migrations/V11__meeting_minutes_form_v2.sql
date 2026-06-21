ALTER TABLE workflow_form_fields
  DROP CONSTRAINT IF EXISTS workflow_form_fields_type_check;

ALTER TABLE workflow_form_fields
  ADD CONSTRAINT workflow_form_fields_type_check CHECK (field_type IN (
    'text',
    'number',
    'date',
    'textarea',
    'select',
    'multi_select',
    'checkbox',
    'evidence_reference',
    'structured_list'
  ));

ALTER TABLE work_items
  DROP CONSTRAINT IF EXISTS work_items_source_type_check;

ALTER TABLE work_items
  ADD CONSTRAINT work_items_source_type_check CHECK (source_type IN (
    'pipeline_run_step',
    'boq_item',
    'procurement_request',
    'stock_movement',
    'daily_report',
    'issue',
    'acceptance',
    'rework_request',
    'replanning_request',
    'handover',
    'accounting_budget',
    'payable',
    'receivable',
    'payment',
    'workflow_form_action_item'
  ));

WITH meeting_template AS (
  SELECT id
  FROM workflow_form_templates
  WHERE code='builtin_meeting_minutes'
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  id,
  2,
  'published',
  '{"source":"builtin","code":"builtin_meeting_minutes","variant":"structured_v2"}'::jsonb,
  '{"creates_work_items_from":"action_items"}'::jsonb,
  now()
FROM meeting_template
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH meeting_version AS (
  SELECT v.id AS version_id
  FROM workflow_form_versions v
  JOIN workflow_form_templates t ON t.id=v.template_id
  WHERE t.code='builtin_meeting_minutes' AND v.version_no=2
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('meeting_title', 'Tiêu đề cuộc họp', 'text', true, '[]'::jsonb, '{"min_length":3,"max_length":160}'::jsonb, NULL, 10),
    ('meeting_date', 'Ngày họp', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 20),
    ('location', 'Địa điểm', 'text', false, '[]'::jsonb, '{"max_length":160}'::jsonb, NULL, 30),
    ('chair', 'Chủ trì', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":120}'::jsonb, NULL, 40),
    (
      'attendees',
      'Thành phần tham dự',
      'structured_list',
      true,
      '[]'::jsonb,
      '{
        "min_items": 1,
        "max_items": 80,
        "item_fields": [
          {"key":"name","label":"Tên","type":"text","required":true,"min_length":2,"max_length":120},
          {"key":"role","label":"Role/Đơn vị","type":"text","required":false,"max_length":120},
          {"key":"attendance","label":"Trạng thái","type":"select","required":true,"options":[{"value":"present","label":"Có mặt"},{"value":"absent","label":"Vắng"},{"value":"remote","label":"Online"}]}
        ]
      }'::jsonb,
      NULL,
      50
    ),
    ('agenda', 'Nội dung chính', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":6000}'::jsonb, NULL, 60),
    (
      'decisions',
      'Quyết định',
      'structured_list',
      false,
      '[]'::jsonb,
      '{
        "max_items": 80,
        "item_fields": [
          {"key":"content","label":"Nội dung quyết định","type":"textarea","required":true,"min_length":3,"max_length":1000},
          {"key":"owner","label":"Owner","type":"text","required":false,"max_length":120},
          {"key":"target","label":"Target","type":"text","required":false,"max_length":160}
        ]
      }'::jsonb,
      NULL,
      70
    ),
    (
      'action_items',
      'Action items',
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
      'Submit sẽ tạo work item theo từng action item.',
      80
    ),
    ('confirmed', 'Xác nhận biên bản', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, NULL, 90)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT mv.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM meeting_version mv
CROSS JOIN field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;
