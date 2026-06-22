WITH template_seed(code, name, description, form_type) AS (
  VALUES
    (
      'builtin_owner_client_checklist',
      'Checklist chủ đầu tư',
      'Checklist đầu ra dành cho chủ đầu tư/client xác nhận hạng mục, evidence, quyết định và yêu cầu bổ sung.',
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

WITH owner_template AS (
  SELECT id
  FROM workflow_form_templates
  WHERE code='builtin_owner_client_checklist'
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  id,
  1,
  'published',
  '{"source":"builtin","code":"builtin_owner_client_checklist","variant":"construction_owner_client_v1"}'::jsonb,
  '{"phase":"construction","audience":"client","submit_requires_all_items_clear":true,"evidence_scope":"work_item"}'::jsonb,
  now()
FROM owner_template
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH owner_version AS (
  SELECT v.id AS version_id
  FROM workflow_form_versions v
  JOIN workflow_form_templates t ON t.id=v.template_id
  WHERE t.code='builtin_owner_client_checklist' AND v.version_no=1
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('check_date', 'Ngày kiểm tra', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_representative', 'Đại diện chủ đầu tư', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 20),
    ('company_representative', 'Đại diện công ty', 'text', false, '[]'::jsonb, '{"max_length":160}'::jsonb, NULL, 30),
    (
      'handover_scope',
      'Phạm vi kiểm tra',
      'select',
      true,
      '[
        {"value":"area","label":"Khu vực / phòng"},
        {"value":"work_package","label":"Gói thi công"},
        {"value":"milestone","label":"Mốc nghiệm thu"},
        {"value":"handover_item","label":"Hạng mục bàn giao"},
        {"value":"defect_closure","label":"Đóng tồn tại"},
        {"value":"other","label":"Khác"}
      ]'::jsonb,
      '{}'::jsonb,
      NULL,
      40
    ),
    ('scope_detail', 'Chi tiết phạm vi', 'text', false, '[]'::jsonb, '{"max_length":240}'::jsonb, NULL, 50),
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
          {"key":"output_ref","label":"Output / mã việc","type":"text","required":false,"max_length":120},
          {"key":"status","label":"Status","type":"select","required":true,"options":[{"value":"done","label":"Done"},{"value":"not_applicable","label":"N/A"},{"value":"needs_follow_up","label":"Needs follow-up"},{"value":"blocked","label":"Blocked"}]},
          {"key":"client_decision","label":"Quyết định CĐT","type":"select","required":true,"options":[{"value":"pass","label":"Đạt"},{"value":"conditional_pass","label":"Đạt có điều kiện"},{"value":"fail","label":"Không đạt"},{"value":"not_applicable","label":"N/A"}]},
          {"key":"evidence_id","label":"Evidence","type":"evidence_reference","required":false},
          {"key":"severity","label":"Severity","type":"select","required":false,"options":[{"value":"low","label":"Low"},{"value":"medium","label":"Medium"},{"value":"high","label":"High"},{"value":"critical","label":"Critical"}]},
          {"key":"required_action","label":"Yêu cầu bổ sung","type":"textarea","required":false,"max_length":1000},
          {"key":"due_date","label":"Due date","type":"date","required":false},
          {"key":"comment","label":"Comment","type":"textarea","required":false,"max_length":1000}
        ]
      }'::jsonb,
      'Submit chỉ được phép khi mọi item ở trạng thái Done hoặc N/A. Chữ ký/PDF dùng flow workflow form sau khi submit.',
      60
    ),
    ('final_client_note', 'Ghi chú cuối của chủ đầu tư', 'textarea', false, '[]'::jsonb, '{"max_length":2000}'::jsonb, NULL, 70),
    ('confirmed', 'Xác nhận checklist chủ đầu tư', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, NULL, 80)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT ov.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM owner_version ov
CROSS JOIN field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;
