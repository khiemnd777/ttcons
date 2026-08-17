-- Lean official workflow capture: keep only step-specific business inputs.
-- Published V1 form definitions and every existing runtime reference remain immutable.

WITH source_versions AS (
  SELECT version.*
  FROM workflow_form_versions version
  WHERE version.version_no=1
    AND version.schema_json->>'source'='official_client_workflow'
    AND version.validation_json->>'template_id'='40000000-0000-0000-0000-000000000020'
),
missing_lean_versions AS (
  SELECT
    source.*,
    COALESCE((
      SELECT max(existing.version_no)+1
      FROM workflow_form_versions existing
      WHERE existing.template_id=source.template_id
    ),2) AS target_version_no
  FROM source_versions source
  WHERE NOT EXISTS (
    SELECT 1
    FROM workflow_form_versions existing
    WHERE existing.template_id=source.template_id
      AND existing.schema_json->>'variant'='lean_capture_v2'
  )
)
INSERT INTO workflow_form_versions (
  template_id,version_no,status,schema_json,validation_json,published_at,created_by
)
SELECT
  source.template_id,
  source.target_version_no,
  'published',
  source.schema_json || jsonb_build_object(
    'variant','lean_capture_v2',
    'source_revision',6,
    'capture_policy','step_specific_only'
  ),
  source.validation_json || jsonb_build_object(
    'source_template_id','40000000-0000-0000-0000-000000000025',
    'universal_fields_removed',jsonb_build_array(
      'scope_summary','work_result','linked_evidence_id','next_action','confirmed'
    )
  ),
  now(),
  source.created_by
FROM missing_lean_versions source
ON CONFLICT (template_id,version_no) DO NOTHING;

WITH source_versions AS (
  SELECT version.*
  FROM workflow_form_versions version
  WHERE version.version_no=1
    AND version.schema_json->>'source'='official_client_workflow'
    AND version.validation_json->>'template_id'='40000000-0000-0000-0000-000000000020'
),
target_versions AS (
  SELECT version.*
  FROM workflow_form_versions version
  WHERE version.schema_json->>'variant'='lean_capture_v2'
)
INSERT INTO workflow_form_fields (
  form_version_id,field_key,label,field_type,required,options_json,
  validation_json,help_text,order_index,visibility_condition_json
)
SELECT
  target.id,
  field.field_key,
  field.label,
  CASE
    WHEN template.code IN ('client_output_invoice_issue','client_input_invoice_closure')
      AND field.field_key='invoice_amount'
    THEN 'currency'
    ELSE field.field_type
  END,
  CASE
    WHEN template.code IN ('client_output_invoice_issue','client_input_invoice_closure')
      AND field.field_key IN ('invoice_no','input_invoice_no','invoice_amount')
    THEN false
    ELSE field.required
  END,
  field.options_json,
  CASE
    WHEN template.code='client_output_invoice_issue'
      AND field.field_key IN ('invoice_no','invoice_amount')
    THEN field.validation_json
      || CASE WHEN field.field_key='invoice_amount' THEN '{"currency_code":"VND"}'::jsonb ELSE '{}'::jsonb END
      || '{"required_when":{"operator":"equals","field_key":"invoice_status","value":"issued"}}'::jsonb
    WHEN template.code='client_input_invoice_closure'
      AND field.field_key IN ('input_invoice_no','invoice_amount')
    THEN field.validation_json
      || CASE WHEN field.field_key='invoice_amount' THEN '{"currency_code":"VND"}'::jsonb ELSE '{}'::jsonb END
      || '{"required_when":{"operator":"equals","field_key":"invoice_status","value":"received"}}'::jsonb
    ELSE field.validation_json
  END,
  field.help_text,
  CASE
    WHEN template.code IN ('client_output_invoice_issue','client_input_invoice_closure')
    THEN field.order_index+10
    ELSE field.order_index
  END,
  CASE
    WHEN template.code='client_output_invoice_issue'
      AND field.field_key IN ('invoice_no','invoice_amount')
    THEN '{"operator":"equals","field_key":"invoice_status","value":"issued"}'::jsonb
    WHEN template.code='client_input_invoice_closure'
      AND field.field_key IN ('input_invoice_no','invoice_amount')
    THEN '{"operator":"equals","field_key":"invoice_status","value":"received"}'::jsonb
    ELSE field.visibility_condition_json
  END
FROM source_versions source
JOIN target_versions target ON target.template_id=source.template_id
JOIN workflow_form_templates template ON template.id=source.template_id
JOIN workflow_form_fields field ON field.form_version_id=source.id
WHERE field.field_key NOT IN (
  'scope_summary','work_result','linked_evidence_id','next_action','confirmed'
)
ON CONFLICT (form_version_id,field_key) DO NOTHING;

WITH invoice_versions AS (
  SELECT template.code,version.id AS version_id
  FROM workflow_form_templates template
  JOIN workflow_form_versions version ON version.template_id=template.id
  WHERE version.schema_json->>'variant'='lean_capture_v2'
    AND template.code IN ('client_output_invoice_issue','client_input_invoice_closure')
)
INSERT INTO workflow_form_fields (
  form_version_id,field_key,label,field_type,required,options_json,
  validation_json,help_text,order_index,visibility_condition_json
)
SELECT
  invoice.version_id,
  'invoice_status',
  CASE invoice.code
    WHEN 'client_output_invoice_issue' THEN 'Trạng thái xuất hóa đơn'
    ELSE 'Trạng thái nhận hóa đơn'
  END,
  'select',
  true,
  CASE invoice.code
    WHEN 'client_output_invoice_issue' THEN '[
      {"value":"issued","label":"Đã xuất"},
      {"value":"pending","label":"Chưa xuất"},
      {"value":"not_applicable","label":"Không áp dụng"}
    ]'::jsonb
    ELSE '[
      {"value":"received","label":"Đã nhận"},
      {"value":"pending","label":"Chưa nhận"},
      {"value":"not_applicable","label":"Không áp dụng"}
    ]'::jsonb
  END,
  '{}'::jsonb,
  CASE invoice.code
    WHEN 'client_output_invoice_issue' THEN 'Chỉ yêu cầu số và giá trị hóa đơn khi chọn Đã xuất.'
    ELSE 'Chỉ yêu cầu số và giá trị hóa đơn khi chọn Đã nhận.'
  END,
  10,
  '{}'::jsonb
FROM invoice_versions invoice
ON CONFLICT (form_version_id,field_key) DO NOTHING;

-- Official immutable Revision 6: Rev5 routing stays intact; only the 45
-- official client business forms point to their lean V2 definitions.
INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by,report_config_json
)
SELECT
  '40000000-0000-0000-0000-000000000025',
  source.name,
  'Revision 6: biểu mẫu tinh gọn theo bước; minh chứng dùng sổ Minh chứng, xác nhận bằng thao tác chốt hồ sơ và việc tiếp theo được quản lý bằng đầu việc thực.',
  source.project_type,
  6,
  'published',
  source.created_by,
  source.template_family_id,
  6,
  source.id,
  source.engine_version,
  encode(digest('official-client-workflow-rev6-lean-forms','sha256'),'hex'),
  now(),
  NULL,
  source.report_config_json
FROM pipeline_templates source
WHERE source.id='40000000-0000-0000-0000-000000000024'
  AND source.status='published'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  replace(source.id::text,'45400000','46400000')::uuid,
  '40000000-0000-0000-0000-000000000025',
  source.business_code,source.name,source.description,source.type,source.position_x,source.position_y,
  source.order_index,source.responsible_role_ids,source.required_permission,source.permissions,
  source.entry_conditions,source.exit_conditions,source.approval_rule,source.approval_policy_json,
  source.rework_enabled,source.replanning_enabled,source.fail_path,source.status
FROM pipeline_template_nodes source
WHERE source.template_id='40000000-0000-0000-0000-000000000024'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  replace(source.id::text,'45500000','46500000')::uuid,
  replace(source.node_id::text,'45400000','46400000')::uuid,
  source.business_code,source.name,source.description,source.step_type,source.required_role_ids,
  source.required_permission,source.checklist_required,source.document_required,source.photo_required,
  COALESCE(lean_version.id,source.form_version_id),
  source.form_required,source.surface_kind,source.domain_action_key,source.order_index
FROM pipeline_template_steps source
JOIN pipeline_template_nodes node ON node.id=source.node_id
LEFT JOIN workflow_form_versions source_version ON source_version.id=source.form_version_id
LEFT JOIN workflow_form_versions lean_version
  ON lean_version.template_id=source_version.template_id
  AND lean_version.version_no=2
  AND lean_version.schema_json->>'variant'='lean_capture_v2'
WHERE node.template_id='40000000-0000-0000-0000-000000000024'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_edges (
  id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority
)
SELECT
  replace(source.id::text,'45600000','46600000')::uuid,
  '40000000-0000-0000-0000-000000000025',
  replace(source.from_node_id::text,'45400000','46400000')::uuid,
  replace(source.to_node_id::text,'45400000','46400000')::uuid,
  source.condition_type,source.condition_config,source.label,source.priority
FROM pipeline_template_edges source
WHERE source.template_id='40000000-0000-0000-0000-000000000024'
ON CONFLICT (id) DO NOTHING;

-- Only projects that selected Rev5 and have not started adopt Rev6.
UPDATE projects project
SET pipeline_template_id='40000000-0000-0000-0000-000000000025',updated_at=now()
WHERE project.pipeline_template_id='40000000-0000-0000-0000-000000000024'
  AND NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=project.id);

DO $$
DECLARE
  lean_form_count integer;
  universal_field_count integer;
  empty_required_form_count integer;
  remapped_step_count integer;
  rev6_node_count integer;
  rev6_step_count integer;
BEGIN
  SELECT count(*) INTO lean_form_count
  FROM workflow_form_versions version
  WHERE version.schema_json->>'variant'='lean_capture_v2';
  IF lean_form_count <> 45 THEN
    RAISE EXCEPTION 'expected 45 lean workflow forms, found %',lean_form_count;
  END IF;

  SELECT count(*) INTO universal_field_count
  FROM workflow_form_fields field
  JOIN workflow_form_versions version ON version.id=field.form_version_id
  WHERE version.schema_json->>'variant'='lean_capture_v2'
    AND field.field_key IN ('scope_summary','work_result','linked_evidence_id','next_action','confirmed');
  IF universal_field_count <> 0 THEN
    RAISE EXCEPTION 'lean workflow forms still contain % universal fields',universal_field_count;
  END IF;

  SELECT count(*) INTO empty_required_form_count
  FROM workflow_form_versions version
  WHERE version.schema_json->>'variant'='lean_capture_v2'
    AND NOT EXISTS (
      SELECT 1
      FROM workflow_form_fields field
      WHERE field.form_version_id=version.id
        AND (field.required OR field.validation_json ? 'required_when')
    );
  IF empty_required_form_count <> 0 THEN
    RAISE EXCEPTION '% lean workflow forms have no effective required input',empty_required_form_count;
  END IF;

  SELECT count(*) INTO remapped_step_count
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  JOIN workflow_form_versions version ON version.id=step.form_version_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000025'
    AND version.schema_json->>'variant'='lean_capture_v2';
  IF remapped_step_count <> 43 THEN
    RAISE EXCEPTION 'expected 43 Rev6 step occurrences remapped to lean forms, found %',remapped_step_count;
  END IF;

  SELECT count(*) INTO rev6_node_count
  FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000025';
  SELECT count(*) INTO rev6_step_count
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000025';
  IF rev6_node_count <> 15 OR rev6_step_count <> 51 THEN
    RAISE EXCEPTION 'expected Rev6 to contain 15 nodes and 51 steps, found % nodes and % steps',rev6_node_count,rev6_step_count;
  END IF;
END $$;
