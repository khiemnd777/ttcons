-- Official immutable Revision 4 approved after the client-note/PDF audit.
-- Published Revision 3 and every existing project runtime snapshot remain unchanged.

ALTER TABLE pipeline_templates
  ADD COLUMN IF NOT EXISTS report_config_json JSONB NOT NULL DEFAULT '{}'::jsonb;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname='pipeline_templates_report_config_object'
  ) THEN
    ALTER TABLE pipeline_templates
      ADD CONSTRAINT pipeline_templates_report_config_object
      CHECK (jsonb_typeof(report_config_json)='object');
  END IF;
END $$;

INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by,report_config_json
)
SELECT
  '40000000-0000-0000-0000-000000000023',
  source.name,
  'Revision 4: bổ sung Mua hàng ở bước lập giá, tách QA/QC khỏi báo cáo Giám sát, bắt buộc hồ sơ/hình ảnh đúng trách nhiệm và hoàn thiện báo cáo định nghĩa workflow.',
  source.project_type,
  4,
  'published',
  source.created_by,
  source.template_family_id,
  4,
  source.id,
  source.engine_version,
  encode(digest('official-client-workflow-rev4','sha256'),'hex'),
  now(),
  NULL,
  '{
    "node_reference_title": "Đối chiếu đánh số client -> Rev4",
    "node_references": [
      {"source_ref":"Node 1","target_business_code":"survey","note":"Khảo sát"},
      {"source_ref":"Node 2","target_business_code":"survey_review","note":"Họp báo cáo khảo sát"},
      {"source_ref":"Node 3","target_business_code":"design","note":"Thiết kế"},
      {"source_ref":"Node 4","target_business_code":"boq_without_design","note":"BOQ không thiết kế"},
      {"source_ref":"Node 5","target_business_code":"boq_with_design","note":"BOQ có thiết kế"},
      {"source_ref":"Node 6","target_business_code":"input_takeoff_purchase","note":"Bóc tách và lấy giá đầu vào"},
      {"source_ref":"Node 6","target_business_code":"input_contract_advance","note":"Hợp đồng và tạm ứng đầu vào"},
      {"source_ref":"Node 7","target_business_code":"price_negotiation","note":"Đàm phán giá"},
      {"source_ref":"Node 8","target_business_code":"outbound_contract","note":"Chốt hợp đồng với chủ đầu tư"},
      {"source_ref":"Node 9","target_business_code":"construction_planning","note":"Kế hoạch thi công"},
      {"source_ref":"Node 10","target_business_code":"construction_execution","note":"Thi công"},
      {"source_ref":"Node 11","target_business_code":"incoming_acceptance","note":"Nghiệm thu đầu vào"},
      {"source_ref":"Node 12","target_business_code":"outgoing_acceptance","note":"Nghiệm thu đầu ra"},
      {"source_ref":"Node 13","target_business_code":"handover_finance","note":"Bàn giao và tài chính công trình"}
    ]
  }'::jsonb
FROM pipeline_templates source
WHERE source.id='40000000-0000-0000-0000-000000000022'
  AND source.status='published'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  replace(source.id::text,'43400000','44400000')::uuid,
  '40000000-0000-0000-0000-000000000023',
  source.business_code,source.name,source.description,source.type,source.position_x,source.position_y,
  source.order_index,source.responsible_role_ids,source.required_permission,source.permissions,
  source.entry_conditions,source.exit_conditions,source.approval_rule,source.approval_policy_json,
  source.rework_enabled,source.replanning_enabled,source.fail_path,source.status
FROM pipeline_template_nodes source
WHERE source.template_id='40000000-0000-0000-0000-000000000022'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  replace(source.id::text,'43500000','44500000')::uuid,
  replace(source.node_id::text,'43400000','44400000')::uuid,
  source.business_code,source.name,source.description,source.step_type,source.required_role_ids,
  source.required_permission,source.checklist_required,source.document_required,source.photo_required,
  source.form_version_id,source.form_required,source.surface_kind,source.domain_action_key,source.order_index
FROM pipeline_template_steps source
JOIN pipeline_template_nodes node ON node.id=source.node_id
WHERE node.template_id='40000000-0000-0000-0000-000000000022'
ON CONFLICT (id) DO NOTHING;

UPDATE pipeline_template_steps step
SET required_role_ids='["QS","Mua hàng"]'::jsonb
FROM pipeline_template_nodes node
WHERE node.id=step.node_id
  AND node.template_id='40000000-0000-0000-0000-000000000023'
  AND step.business_code='boq_without_design.pricing';

UPDATE pipeline_template_steps step
SET required_role_ids='["QA/QC"]'::jsonb,
    checklist_required=true,
    document_required=true,
    photo_required=false,
    description='QA/QC lập và quản lý hồ sơ chất lượng làm việc với chủ đầu tư; Giám sát phụ trách báo cáo hiện trường ở bước riêng.'
FROM pipeline_template_nodes node
WHERE node.id=step.node_id
  AND node.template_id='40000000-0000-0000-0000-000000000023'
  AND step.business_code='planning.client_quality_file';

UPDATE pipeline_template_steps step
SET required_role_ids='["Giám sát"]'::jsonb,
    checklist_required=true,
    document_required=false,
    photo_required=true,
    description='Giám sát lập báo cáo theo hạng mục bằng checklist và hình ảnh; QA/QC quản lý hồ sơ chất lượng ở bước riêng.'
FROM pipeline_template_nodes node
WHERE node.id=step.node_id
  AND node.template_id='40000000-0000-0000-0000-000000000023'
  AND step.business_code='construction.item_reporting';

INSERT INTO pipeline_template_edges (
  id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority
)
SELECT
  replace(source.id::text,'43600000','44600000')::uuid,
  '40000000-0000-0000-0000-000000000023',
  replace(source.from_node_id::text,'43400000','44400000')::uuid,
  replace(source.to_node_id::text,'43400000','44400000')::uuid,
  source.condition_type,source.condition_config,source.label,source.priority
FROM pipeline_template_edges source
WHERE source.template_id='40000000-0000-0000-0000-000000000022'
ON CONFLICT (id) DO NOTHING;

-- Only definitions selected by projects that have never started move forward.
-- Every existing run keeps its exact template/runtime snapshot.
UPDATE projects project
SET pipeline_template_id='40000000-0000-0000-0000-000000000023',updated_at=now()
WHERE project.pipeline_template_id='40000000-0000-0000-0000-000000000022'
  AND NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=project.id);
