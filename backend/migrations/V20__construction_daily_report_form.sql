WITH template_seed(code, name, description, form_type) AS (
  VALUES
    (
      'builtin_construction_daily_report',
      'Daily report thi công',
      'Nhật ký thi công theo ngày với việc đã làm, nhân công, khối lượng thực tế, vật tư sử dụng, ảnh hiện trường và kế hoạch tiếp theo.',
      'daily_report'
    )
)
INSERT INTO workflow_form_templates (code, name, description, form_type, status)
SELECT code, name, description, form_type, 'active'
FROM template_seed
ON CONFLICT (code) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    form_type=EXCLUDED.form_type,
    status='active',
    updated_at=now();

WITH construction_template AS (
  SELECT id
  FROM workflow_form_templates
  WHERE code='builtin_construction_daily_report'
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  id,
  1,
  'published',
  '{"source":"builtin","code":"builtin_construction_daily_report","variant":"construction_daily_report_v1"}'::jsonb,
  '{"phase":"construction","creates_work_items_from":"next_plan_items","evidence_scope":"work_item","material_usage_mode":"report_only"}'::jsonb,
  now()
FROM construction_template
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH construction_version AS (
  SELECT v.id AS version_id
  FROM workflow_form_versions v
  JOIN workflow_form_templates t ON t.id=v.template_id
  WHERE t.code='builtin_construction_daily_report' AND v.version_no=1
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('report_date', 'Ngày report', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('reported_by', 'Người report', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":120}'::jsonb, NULL, 20),
    (
      'construction_scope',
      'Phạm vi thi công',
      'select',
      true,
      '[
        {"value":"site_preparation","label":"Chuẩn bị mặt bằng"},
        {"value":"civil","label":"Xây tô / phần thô"},
        {"value":"ceiling","label":"Trần"},
        {"value":"flooring","label":"Sàn"},
        {"value":"painting","label":"Sơn"},
        {"value":"mep","label":"MEP"},
        {"value":"carpentry","label":"Mộc / nội thất"},
        {"value":"aluminum_glass","label":"Nhôm kính"},
        {"value":"handover_prep","label":"Chuẩn bị bàn giao"},
        {"value":"other","label":"Khác"}
      ]'::jsonb,
      '{}'::jsonb,
      NULL,
      30
    ),
    (
      'weather',
      'Thời tiết / điều kiện thi công',
      'select',
      true,
      '[
        {"value":"normal","label":"Bình thường"},
        {"value":"hot","label":"Nắng nóng"},
        {"value":"rain","label":"Mưa"},
        {"value":"wind","label":"Gió mạnh"},
        {"value":"restricted_access","label":"Hạn chế mặt bằng"},
        {"value":"other","label":"Khác"}
      ]'::jsonb,
      '{}'::jsonb,
      NULL,
      40
    ),
    ('progress_percent', 'Tiến độ %', 'number', false, '[]'::jsonb, '{"min":0,"max":100}'::jsonb, NULL, 50),
    ('site_photo_evidence_id', 'Ảnh hiện trường tổng quan', 'evidence_reference', false, '[]'::jsonb, '{}'::jsonb, 'Chọn evidence ảnh đã upload trong work item này.', 60),
    (
      'completed_items',
      'Đã làm',
      'structured_list',
      true,
      '[]'::jsonb,
      '{
        "min_items": 1,
        "max_items": 250,
        "item_fields": [
          {"key":"title","label":"Hạng mục","type":"text","required":true,"min_length":3,"max_length":180},
          {"key":"work_area","label":"Khu vực","type":"text","required":false,"max_length":160},
          {"key":"crew_name","label":"Tổ / đội","type":"text","required":false,"max_length":120},
          {"key":"actual_quantity","label":"KL thực tế","type":"number","required":false,"min":0},
          {"key":"unit","label":"ĐVT","type":"text","required":false,"max_length":30},
          {"key":"material_used","label":"Vật tư dùng","type":"text","required":false,"max_length":180},
          {"key":"result","label":"Kết quả","type":"select","required":true,"options":[{"value":"done","label":"Done"},{"value":"partial","label":"Partial"},{"value":"blocked","label":"Blocked"}]},
          {"key":"evidence_id","label":"Ảnh / evidence","type":"evidence_reference","required":false},
          {"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}
        ]
      }'::jsonb,
      'Ghi theo từng hạng mục đã làm trong ngày. Evidence nếu chọn phải thuộc đúng work item.',
      70
    ),
    (
      'crew_items',
      'Nhân công / tổ đội',
      'structured_list',
      false,
      '[]'::jsonb,
      '{
        "max_items": 120,
        "item_fields": [
          {"key":"crew_name","label":"Tổ / đội","type":"text","required":true,"min_length":2,"max_length":120},
          {"key":"trade","label":"Trade","type":"text","required":false,"max_length":80},
          {"key":"worker_count","label":"Số người","type":"number","required":true,"min":1},
          {"key":"work_hours","label":"Giờ công","type":"number","required":false,"min":0},
          {"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":800}
        ]
      }'::jsonb,
      NULL,
      80
    ),
    (
      'material_usage_items',
      'Vật tư sử dụng',
      'structured_list',
      false,
      '[]'::jsonb,
      '{
        "max_items": 200,
        "item_fields": [
          {"key":"material_name","label":"Vật tư","type":"text","required":true,"min_length":2,"max_length":160},
          {"key":"material_code","label":"Mã vật tư","type":"text","required":false,"max_length":80},
          {"key":"quantity","label":"Số lượng","type":"number","required":true,"min":0},
          {"key":"unit","label":"ĐVT","type":"text","required":true,"min_length":1,"max_length":30},
          {"key":"source","label":"Nguồn","type":"select","required":true,"options":[{"value":"warehouse_issue","label":"Đã xuất kho"},{"value":"site_balance","label":"Tồn tại công trường"},{"value":"subcontractor","label":"Thầu phụ mang vào"},{"value":"other","label":"Khác"}]},
          {"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":800}
        ]
      }'::jsonb,
      'Chỉ ghi nhận báo cáo; không tự tạo stock movement.',
      90
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
      100
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
          {"key":"impact","label":"Impact","type":"select","required":true,"options":[{"value":"schedule","label":"Tiến độ"},{"value":"budget","label":"Ngân sách"},{"value":"material","label":"Vật tư"},{"value":"quality","label":"Chất lượng"},{"value":"access","label":"Mặt bằng"},{"value":"other","label":"Khác"}]},
          {"key":"owner_role","label":"Owner role","type":"text","required":false,"max_length":80},
          {"key":"target_date","label":"Target date","type":"date","required":false},
          {"key":"severity","label":"Severity","type":"select","required":true,"options":[{"value":"low","label":"Low"},{"value":"medium","label":"Medium"},{"value":"high","label":"High"},{"value":"critical","label":"Critical"}]}
        ]
      }'::jsonb,
      NULL,
      110
    ),
    ('confirmed', 'Xác nhận report thi công', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, NULL, 120)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT cv.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM construction_version cv
CROSS JOIN field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;
