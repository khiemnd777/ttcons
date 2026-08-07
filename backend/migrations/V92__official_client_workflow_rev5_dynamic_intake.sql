-- Official immutable Revision 5: project intake is a workflow form and all
-- branch routing is evaluated from connector configuration.

INSERT INTO workflow_form_templates (id, code, name, description, form_type, status)
VALUES (
  '45700000-0000-0000-0000-000000000001',
  'client_project_intake_routing',
  'Tiếp nhận dự án và chọn luồng đầu vào',
  'Xác nhận điểm bắt đầu; hồ sơ BOQ chỉ xuất hiện và bắt buộc khi người dùng chọn nhánh BOQ có sẵn.',
  'generic_submission',
  'active'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO workflow_form_versions (id, template_id, version_no, status, schema_json, validation_json, published_at)
VALUES (
  '45700000-0000-0000-0000-000000000002',
  '45700000-0000-0000-0000-000000000001',
  1,
  'published',
  '{"layout":"single_column","purpose":"workflow_routing"}'::jsonb,
  '{}'::jsonb,
  now()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO workflow_form_fields (
  id, form_version_id, field_key, label, field_type, required, options_json,
  validation_json, help_text, order_index, visibility_condition_json
)
VALUES
  (
    '45700000-0000-0000-0000-000000000003',
    '45700000-0000-0000-0000-000000000002',
    'entry_route',
    'Điểm bắt đầu quy trình',
    'select',
    true,
    '[
      {"value":"survey_required","label":"Khảo sát trước báo giá"},
      {"value":"client_boq","label":"Chủ đầu tư có sẵn BOQ · vào thẳng Báo giá"}
    ]'::jsonb,
    '{}'::jsonb,
    'Lựa chọn này được lưu trong hồ sơ workflow và Connector dùng trực tiếp để xác định node kế tiếp.',
    10,
    '{}'::jsonb
  ),
  (
    '45700000-0000-0000-0000-000000000004',
    '45700000-0000-0000-0000-000000000002',
    'client_boq_evidence',
    'Hồ sơ BOQ của chủ đầu tư',
    'evidence_reference',
    false,
    '[]'::jsonb,
    '{"required_when":{"operator":"equals","field_key":"entry_route","value":"client_boq"}}'::jsonb,
    'Tải minh chứng trong sổ Minh chứng của đầu việc, sau đó chọn tệp tại đây.',
    20,
    '{"operator":"equals","field_key":"entry_route","value":"client_boq"}'::jsonb
  )
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by,report_config_json
)
SELECT
  '40000000-0000-0000-0000-000000000024',
  source.name,
  'Revision 5: chọn nhánh trong biểu mẫu Tiếp nhận dự án; Connector đánh giá câu trả lời động và tự chuyển khi đủ điều kiện.',
  source.project_type,
  5,
  'published',
  source.created_by,
  source.template_family_id,
  5,
  source.id,
  4,
  encode(digest('official-client-workflow-rev5-dynamic-intake','sha256'),'hex'),
  now(),
  NULL,
  source.report_config_json
FROM pipeline_templates source
WHERE source.id='40000000-0000-0000-0000-000000000023'
  AND source.status='published'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  replace(source.id::text,'44400000','45400000')::uuid,
  '40000000-0000-0000-0000-000000000024',
  source.business_code,source.name,
  CASE WHEN source.business_code='project_intake'
    THEN 'Xác nhận lựa chọn Khảo sát hoặc Chủ đầu tư có BOQ ngay trong biểu mẫu; Connector quyết định node kế tiếp.'
    ELSE source.description
  END,
  source.type,source.position_x,source.position_y,source.order_index,
  source.responsible_role_ids,source.required_permission,source.permissions,source.entry_conditions,
  CASE WHEN source.business_code='project_intake'
    THEN COALESCE(source.exit_conditions,'{}'::jsonb) || '{"auto_advance_when_ready":true}'::jsonb
    ELSE source.exit_conditions
  END,
  source.approval_rule,source.approval_policy_json,source.rework_enabled,source.replanning_enabled,
  source.fail_path,source.status
FROM pipeline_template_nodes source
WHERE source.template_id='40000000-0000-0000-0000-000000000023'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  replace(source.id::text,'44500000','45500000')::uuid,
  replace(source.node_id::text,'44400000','45400000')::uuid,
  source.business_code,source.name,
  CASE WHEN source.business_code='project_intake.confirm'
    THEN 'Chọn điểm bắt đầu, bổ sung BOQ khi có sẵn và xác nhận để Connector chuyển đúng nhánh.'
    ELSE source.description
  END,
  source.step_type,source.required_role_ids,source.required_permission,
  CASE WHEN source.business_code='project_intake.confirm' THEN false ELSE source.checklist_required END,
  CASE WHEN source.business_code='project_intake.confirm' THEN false ELSE source.document_required END,
  CASE WHEN source.business_code='project_intake.confirm' THEN false ELSE source.photo_required END,
  CASE WHEN source.business_code='project_intake.confirm'
    THEN '45700000-0000-0000-0000-000000000002'::uuid
    ELSE source.form_version_id
  END,
  CASE WHEN source.business_code='project_intake.confirm' THEN true ELSE source.form_required END,
  CASE WHEN source.business_code='project_intake.confirm' THEN 'workflow_form' ELSE source.surface_kind END,
  source.domain_action_key,source.order_index
FROM pipeline_template_steps source
JOIN pipeline_template_nodes node ON node.id=source.node_id
WHERE node.template_id='40000000-0000-0000-0000-000000000023'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_edges (
  id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority
)
SELECT
  replace(source.id::text,'44600000','45600000')::uuid,
  '40000000-0000-0000-0000-000000000024',
  replace(source.from_node_id::text,'44400000','45400000')::uuid,
  replace(source.to_node_id::text,'44400000','45400000')::uuid,
  CASE WHEN from_node.business_code='project_intake'
    THEN 'workflow_form_answer_equals'
    ELSE source.condition_type
  END,
  CASE
    WHEN from_node.business_code='project_intake' AND to_node.business_code='survey'
      THEN '{"field_key":"entry_route","value":"survey_required"}'::jsonb
    WHEN from_node.business_code='project_intake' AND to_node.business_code='boq_without_design'
      THEN '{"field_key":"entry_route","value":"client_boq"}'::jsonb
    ELSE source.condition_config
  END,
  CASE
    WHEN from_node.business_code='project_intake' AND to_node.business_code='survey' THEN 'Khảo sát trước báo giá'
    WHEN from_node.business_code='project_intake' AND to_node.business_code='boq_without_design' THEN 'Chủ đầu tư có BOQ'
    ELSE source.label
  END,
  source.priority
FROM pipeline_template_edges source
JOIN pipeline_template_nodes from_node ON from_node.id=source.from_node_id
JOIN pipeline_template_nodes to_node ON to_node.id=source.to_node_id
WHERE source.template_id='40000000-0000-0000-0000-000000000023'
ON CONFLICT (id) DO NOTHING;

-- Projects that have not started may adopt Rev5. Existing runtime snapshots remain unchanged.
UPDATE projects project
SET pipeline_template_id='40000000-0000-0000-0000-000000000024',updated_at=now()
WHERE project.pipeline_template_id='40000000-0000-0000-0000-000000000023'
  AND NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=project.id);
