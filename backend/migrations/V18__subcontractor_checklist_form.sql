WITH template_seed(code, name, description, form_type) AS (
  VALUES
    (
      'builtin_subcontractor_checklist',
      'Checklist thầu phụ',
      'Checklist thi công dành cho đối soát hạng mục, khối lượng, evidence và phản hồi thầu phụ.',
      'checklist'
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

WITH subcontractor_template AS (
  SELECT id
  FROM workflow_form_templates
  WHERE code='builtin_subcontractor_checklist'
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  id,
  1,
  'published',
  '{"source":"builtin","code":"builtin_subcontractor_checklist","variant":"construction_subcontractor_v1"}'::jsonb,
  '{"phase":"construction","submit_requires_all_items_clear":true,"evidence_scope":"work_item"}'::jsonb,
  now()
FROM subcontractor_template
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH subcontractor_version AS (
  SELECT v.id AS version_id
  FROM workflow_form_versions v
  JOIN workflow_form_templates t ON t.id=v.template_id
  WHERE t.code='builtin_subcontractor_checklist' AND v.version_no=1
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('check_date', 'Ngày kiểm tra', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('subcontractor_name', 'Đội / thầu phụ', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 20),
    ('subcontractor_representative', 'Đại diện thầu phụ', 'text', false, '[]'::jsonb, '{"max_length":120}'::jsonb, NULL, 30),
    (
      'work_package',
      'Gói / phạm vi thi công',
      'select',
      true,
      '[
        {"value":"demolition","label":"Tháo dỡ"},
        {"value":"masonry","label":"Xây tô"},
        {"value":"ceiling","label":"Trần"},
        {"value":"flooring","label":"Sàn"},
        {"value":"painting","label":"Sơn"},
        {"value":"mep","label":"MEP"},
        {"value":"carpentry","label":"Mộc / nội thất"},
        {"value":"aluminum_glass","label":"Nhôm kính"},
        {"value":"cleaning","label":"Vệ sinh"},
        {"value":"other","label":"Khác"}
      ]'::jsonb,
      '{}'::jsonb,
      NULL,
      40
    ),
    ('construction_area', 'Khu vực / tầng / phòng', 'text', false, '[]'::jsonb, '{"max_length":200}'::jsonb, NULL, 50),
    (
      'checklist_items',
      'Checklist items',
      'structured_list',
      true,
      '[]'::jsonb,
      '{
        "min_items": 1,
        "max_items": 250,
        "submit_clear_statuses": ["done", "not_applicable"],
        "item_fields": [
          {"key":"title","label":"Hạng mục","type":"text","required":true,"min_length":3,"max_length":180},
          {"key":"boq_item_ref","label":"BOQ / mã việc","type":"text","required":false,"max_length":120},
          {"key":"claimed_quantity","label":"KL claim","type":"number","required":false,"min":0},
          {"key":"confirmed_quantity","label":"KL xác nhận","type":"number","required":false,"min":0},
          {"key":"unit","label":"ĐVT","type":"text","required":false,"max_length":30},
          {"key":"status","label":"Status","type":"select","required":true,"options":[{"value":"done","label":"Done"},{"value":"not_applicable","label":"N/A"},{"value":"needs_follow_up","label":"Needs follow-up"},{"value":"blocked","label":"Blocked"}]},
          {"key":"evidence_id","label":"Evidence","type":"evidence_reference","required":false},
          {"key":"owner_response","label":"Phản hồi","type":"select","required":false,"options":[{"value":"accepted","label":"Chấp nhận"},{"value":"adjust_quantity","label":"Điều chỉnh KL"},{"value":"rework_required","label":"Yêu cầu làm lại"},{"value":"waiting_evidence","label":"Chờ evidence"}]},
          {"key":"due_date","label":"Due date","type":"date","required":false},
          {"key":"comment","label":"Comment","type":"textarea","required":false,"max_length":1000}
        ]
      }'::jsonb,
      'Submit chỉ được phép khi mọi item ở trạng thái Done hoặc N/A. Evidence nếu chọn phải thuộc đúng work item.',
      60
    ),
    ('overall_comment', 'Ghi chú tổng quan', 'textarea', false, '[]'::jsonb, '{"max_length":2000}'::jsonb, NULL, 70),
    ('confirmed', 'Xác nhận checklist thầu phụ', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, NULL, 80)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT sv.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM subcontractor_version sv
CROSS JOIN field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;
