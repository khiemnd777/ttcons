-- Official workflow Revision 8: preserve the 15-node construction lifecycle
-- while reducing the template to 21 accountable work packages. Rev7 and all
-- historical runtimes remain immutable.

INSERT INTO workflow_form_templates (
  id,code,name,description,form_type,status,created_by
)
SELECT seed.id,seed.code,seed.name,seed.description,seed.form_type,'active',source.created_by
FROM (VALUES
  ('48000000-0000-0000-0000-000000000001'::uuid,'client_survey_work_package','Gói khảo sát hiện trạng Rev8','Một lần ghi nhận yêu cầu, phong cách, kích thước và rủi ro hiện trường.','generic_submission'),
  ('48000000-0000-0000-0000-000000000002'::uuid,'client_design_delivery_package','Gói hồ sơ thiết kế Rev8','Một hồ sơ tổng hợp để phát hành phương án, bản vẽ và kết quả review thiết kế.','generic_submission'),
  ('48000000-0000-0000-0000-000000000003'::uuid,'client_construction_execution_plan','Gói kế hoạch thi công Rev8','Một hồ sơ tổng hợp biên bản triển khai và checklist QA của kế hoạch thi công.','meeting_minutes'),
  ('48000000-0000-0000-0000-000000000004'::uuid,'client_subcontractor_acceptance_package','Gói nghiệm thu thầu phụ Rev8','Một hồ sơ tổng hợp checklist và khối lượng nghiệm thu thầu phụ.','checklist')
) seed(id,code,name,description,form_type)
LEFT JOIN workflow_form_templates source ON source.code='client_project_intake_routing'
ON CONFLICT (id) DO NOTHING;

INSERT INTO workflow_form_versions (
  id,template_id,version_no,status,schema_json,validation_json,published_at,created_by
)
SELECT
  seed.id,seed.template_id,1,'published',
  jsonb_build_object(
    'variant','operational_capture_v3',
    'source_revision',8,
    'capture_policy','work_package'
  ),
  jsonb_build_object(
    'source_template_id','40000000-0000-0000-0000-000000000027',
    'source_form_codes',seed.source_form_codes
  ),
  now(),source.created_by
FROM (VALUES
  ('48100000-0000-0000-0000-000000000001'::uuid,'48000000-0000-0000-0000-000000000001'::uuid,'["client_survey_requirement_intake","client_design_style_brief","client_site_measurement_record","client_site_risk_assessment"]'::jsonb),
  ('48100000-0000-0000-0000-000000000002'::uuid,'48000000-0000-0000-0000-000000000002'::uuid,'["client_design_layout_submission","client_design_3d_submission","client_construction_drawing_submission","client_design_review_layout","client_design_review_construction_drawing"]'::jsonb),
  ('48100000-0000-0000-0000-000000000003'::uuid,'48000000-0000-0000-0000-000000000003'::uuid,'["builtin_meeting_minutes","client_qa_construction_plan_checklist"]'::jsonb),
  ('48100000-0000-0000-0000-000000000004'::uuid,'48000000-0000-0000-0000-000000000004'::uuid,'["builtin_subcontractor_checklist","client_subcontractor_quantity_dossier"]'::jsonb)
) seed(id,template_id,source_form_codes)
LEFT JOIN workflow_form_templates source ON source.id=seed.template_id
ON CONFLICT (id) DO NOTHING;

WITH composite_sources(target_version_id,source_form_code,order_offset) AS (
  VALUES
    ('48100000-0000-0000-0000-000000000001'::uuid,'client_survey_requirement_intake',0),
    ('48100000-0000-0000-0000-000000000001'::uuid,'client_design_style_brief',100),
    ('48100000-0000-0000-0000-000000000001'::uuid,'client_site_measurement_record',200),
    ('48100000-0000-0000-0000-000000000001'::uuid,'client_site_risk_assessment',300),
    ('48100000-0000-0000-0000-000000000002'::uuid,'client_design_layout_submission',0),
    ('48100000-0000-0000-0000-000000000002'::uuid,'client_design_3d_submission',100),
    ('48100000-0000-0000-0000-000000000002'::uuid,'client_construction_drawing_submission',200),
    ('48100000-0000-0000-0000-000000000002'::uuid,'client_design_review_layout',300),
    ('48100000-0000-0000-0000-000000000002'::uuid,'client_design_review_construction_drawing',400),
    ('48100000-0000-0000-0000-000000000003'::uuid,'builtin_meeting_minutes',0),
    ('48100000-0000-0000-0000-000000000003'::uuid,'client_qa_construction_plan_checklist',100),
    ('48100000-0000-0000-0000-000000000004'::uuid,'builtin_subcontractor_checklist',0),
    ('48100000-0000-0000-0000-000000000004'::uuid,'client_subcontractor_quantity_dossier',100)
), source_versions AS (
  SELECT DISTINCT ON (template.code)
    template.code,version.id
  FROM pipeline_template_nodes node
  JOIN pipeline_template_steps step ON step.node_id=node.id
  JOIN workflow_form_versions version ON version.id=step.form_version_id
  JOIN workflow_form_templates template ON template.id=version.template_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000026'
  ORDER BY template.code,version.version_no DESC
)
INSERT INTO workflow_form_fields (
  form_version_id,field_key,label,field_type,required,options_json,
  validation_json,help_text,order_index,visibility_condition_json
)
SELECT
  source.target_version_id,field.field_key,field.label,field.field_type,field.required,
  field.options_json,field.validation_json,field.help_text,
  source.order_offset+field.order_index,field.visibility_condition_json
FROM composite_sources source
JOIN source_versions version ON version.code=source.source_form_code
JOIN workflow_form_fields field ON field.form_version_id=version.id
ON CONFLICT (form_version_id,field_key) DO NOTHING;

INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by,report_config_json
)
SELECT
  '40000000-0000-0000-0000-000000000027',
  source.name,
  'Revision 8: 15 node, 21 gói công việc có một vai trò chịu trách nhiệm chính; dữ liệu module nguồn được gom thành checklist readiness và tài chính chỉ khóa tại bước đóng công trình.',
  source.project_type,8,'published',source.created_by,source.template_family_id,8,source.id,
  source.engine_version,
  encode(digest('official-client-workflow-rev8-21-work-packages','sha256'),'hex'),
  now(),NULL,
  source.report_config_json || jsonb_build_object(
    'capture_policy','work_package',
    'template_step_count',21,
    'design_path_step_count',20,
    'no_design_path_step_count',18,
    'client_boq_path_step_count',15,
    'node_approval_count',2,
    'technical_handover_separate_from_finance',true
  )
FROM pipeline_templates source
WHERE source.id='40000000-0000-0000-0000-000000000026'
  AND source.status='published'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  replace(source.id::text,'47400000','48400000')::uuid,
  '40000000-0000-0000-0000-000000000027',
  source.business_code,source.name,source.description,source.type,source.position_x,source.position_y,
  source.order_index,source.responsible_role_ids,source.required_permission,source.permissions,
  source.entry_conditions,source.exit_conditions,
  CASE WHEN source.business_code IN ('survey_review','handover_finance') THEN source.approval_rule END,
  CASE WHEN source.business_code IN ('survey_review','handover_finance') THEN source.approval_policy_json ELSE '{}'::jsonb END,
  source.rework_enabled,source.replanning_enabled,source.fail_path,source.status
FROM pipeline_template_nodes source
WHERE source.template_id='40000000-0000-0000-0000-000000000026'
ON CONFLICT (id) DO NOTHING;

UPDATE pipeline_template_nodes
SET
  responsible_role_ids=CASE business_code
    WHEN 'project_intake' THEN '["PM"]'::jsonb
    WHEN 'survey' THEN '["Giám sát"]'::jsonb
    WHEN 'survey_review' THEN '["PM"]'::jsonb
    WHEN 'design' THEN '["Thiết kế"]'::jsonb
    WHEN 'boq_without_design' THEN '["QS"]'::jsonb
    WHEN 'boq_with_design' THEN '["QS"]'::jsonb
    WHEN 'outbound_contract' THEN '["Bộ phận HĐ"]'::jsonb
    WHEN 'input_takeoff_purchase' THEN '["Mua hàng"]'::jsonb
    WHEN 'price_negotiation' THEN '["Mua hàng"]'::jsonb
    WHEN 'input_contract_advance' THEN '["Mua hàng"]'::jsonb
    WHEN 'construction_planning' THEN '["PM"]'::jsonb
    WHEN 'construction_execution' THEN '["Giám sát"]'::jsonb
    WHEN 'incoming_acceptance' THEN '["QA/QC"]'::jsonb
    WHEN 'outgoing_acceptance' THEN '["QA/QC"]'::jsonb
    WHEN 'handover_finance' THEN '["PM"]'::jsonb
    ELSE responsible_role_ids
  END,
  description=CASE business_code
    WHEN 'survey' THEN 'Một gói khảo sát hiện trạng và một biên bản chốt; ảnh hiện trường thuộc cùng gói khảo sát.'
    WHEN 'survey_review' THEN 'PM chốt nhánh có thiết kế hoặc không thiết kế từ một biên bản quyết định.'
    WHEN 'design' THEN 'Thiết kế phát hành một gói hồ sơ và PM bàn giao bản phát hành cho QS.'
    WHEN 'boq_without_design' THEN 'QS hoàn tất và phát hành BOQ không thiết kế từ module BOQ.'
    WHEN 'boq_with_design' THEN 'QS hoàn tất và phát hành BOQ có thiết kế từ module BOQ.'
    WHEN 'outbound_contract' THEN 'Bộ phận HĐ chịu trách nhiệm một gói hợp đồng; checklist nguồn theo dõi version, ký App, hồ sơ giấy và kích hoạt.'
    WHEN 'input_takeoff_purchase' THEN 'QS chốt bóc tách, Mua hàng hoàn tất báo giá nhà cung cấp tại module nguồn.'
    WHEN 'price_negotiation' THEN 'Mua hàng chốt lựa chọn báo giá sau đàm phán.'
    WHEN 'input_contract_advance' THEN 'Mua hàng hoàn tất một gói hợp đồng đầu vào và procurement đã duyệt.'
    WHEN 'construction_planning' THEN 'PM lập một gói kế hoạch thi công và kích hoạt bàn giao ra hiện trường.'
    WHEN 'construction_execution' THEN 'Giám sát cập nhật báo cáo thi công liên tục; lỗi, NCR và rework ở module nguồn.'
    WHEN 'incoming_acceptance' THEN 'Hai gói nghiệm thu kỹ thuật: thầu phụ/khối lượng và vật tư; tài chính không chặn tiến độ kỹ thuật.'
    WHEN 'outgoing_acceptance' THEN 'Một gói nghiệm thu cuối và đối chiếu chủ đầu tư; tài chính được theo dõi ngoài critical path.'
    WHEN 'handover_finance' THEN 'Bàn giao vật lý được hoàn tất độc lập trước; tài chính công trình được khóa ở bước cuối.'
    ELSE description
  END,
  exit_conditions=CASE business_code
    WHEN 'survey' THEN '{"text":"Gói khảo sát và biên bản khảo sát đã hoàn tất"}'::jsonb
    WHEN 'design' THEN '{"text":"Gói thiết kế đã phát hành và bàn giao QS"}'::jsonb
    WHEN 'outbound_contract' THEN '{"text":"Hợp đồng chủ đầu tư đủ điều kiện triển khai"}'::jsonb
    WHEN 'incoming_acceptance' THEN '{"text":"Nghiệm thu kỹ thuật đầu vào đã hoàn tất"}'::jsonb
    WHEN 'outgoing_acceptance' THEN '{"text":"Nghiệm thu cuối và đối chiếu chủ đầu tư đã hoàn tất"}'::jsonb
    WHEN 'handover_finance' THEN '{"handover_included":true,"financial_closure_included":true,"separate_gates":true,"text":"Bàn giao vật lý và khóa tài chính công trình đã hoàn tất"}'::jsonb
    ELSE exit_conditions
  END,
  updated_at=now()
WHERE template_id='40000000-0000-0000-0000-000000000027';

WITH step_defs(
  id,node_code,business_code,name,description,step_type,role_name,permission_key,
  checklist_required,document_required,photo_required,source_business_code,composite_form_version_id,
  surface_kind,domain_action_key,order_index
) AS (VALUES
  ('48500000-0000-0000-0000-000000000001'::uuid,'project_intake','project_intake.confirm','Xác nhận điểm bắt đầu','PM xác nhận điểm bắt đầu và bộ hồ sơ đầu vào của công trình.','task','PM','project.update',true,false,false,'project_intake.confirm',NULL::uuid,'workflow_form',NULL::text,1),
  ('48500000-0000-0000-0000-000000000002'::uuid,'survey','survey.work_package','Ghi nhận gói khảo sát hiện trạng','Giám sát ghi một lần yêu cầu, phong cách, kích thước, rủi ro và ảnh hiện trường.','task','Giám sát','planning.edit',true,false,true,NULL,'48100000-0000-0000-0000-000000000001'::uuid,'workflow_form',NULL,1),
  ('48500000-0000-0000-0000-000000000003'::uuid,'survey','survey.minutes','Chốt biên bản khảo sát','PM chốt biên bản khảo sát và các điểm cần xử lý tiếp theo.','document_upload','PM','planning.edit',true,true,false,'survey.minutes',NULL,'workflow_form',NULL,2),
  ('48500000-0000-0000-0000-000000000004'::uuid,'survey_review','survey_review.branch_decision','Chốt nhánh xử lý','PM xác nhận dự án đi nhánh có thiết kế hoặc không thiết kế.','meeting_minutes','PM','planning.edit',true,false,false,'survey_review.branch_decision',NULL,'workflow_form',NULL,1),
  ('48500000-0000-0000-0000-000000000005'::uuid,'design','design.delivery_package','Phát hành gói hồ sơ thiết kế','Thiết kế tổng hợp phương án, bản vẽ và kết quả review thành một gói phát hành.','document_upload','Thiết kế','planning.edit',true,true,false,NULL,'48100000-0000-0000-0000-000000000002'::uuid,'workflow_form',NULL,1),
  ('48500000-0000-0000-0000-000000000006'::uuid,'design','design.release_to_qs','Bàn giao bản phát hành cho QS','PM xác nhận hồ sơ thiết kế nguồn đã phát hành và bàn giao đúng revision cho QS.','task','PM','planning.edit',true,false,false,'design.handover_to_qs',NULL,'workflow_form','source.design_documents_ready',2),
  ('48500000-0000-0000-0000-000000000007'::uuid,'boq_without_design','boq_without_design.release_package','Phát hành BOQ không thiết kế','QS chốt bản phát hành BOQ và gửi chủ đầu tư; hệ thống tạo yêu cầu mua từ BOQ đã khóa.','document_upload','QS','boq.update',true,true,false,'boq_without_design.client_send',NULL,'workflow_form','procurement.generate_from_boq',1),
  ('48500000-0000-0000-0000-000000000008'::uuid,'boq_with_design','boq_with_design.release_package','Phát hành BOQ có thiết kế','QS chốt bản phát hành BOQ và gửi chủ đầu tư; hệ thống tạo yêu cầu mua từ BOQ đã khóa.','document_upload','QS','boq.update',true,true,false,'boq_with_design.client_send',NULL,'workflow_form','procurement.generate_from_boq',1),
  ('48500000-0000-0000-0000-000000000009'::uuid,'outbound_contract','outbound_contract.execution_package','Hoàn tất gói hợp đồng chủ đầu tư','Bộ phận HĐ điều phối version, ký App, gửi/nhận hồ sơ giấy và kích hoạt hợp đồng.','document_upload','Bộ phận HĐ','contract.dispatch',true,true,false,'outbound_contract.paper_dispatch',NULL,'contract','contract.require_execution_ready',1),
  ('48500000-0000-0000-0000-000000000010'::uuid,'input_takeoff_purchase','input.takeoff','Chốt bóc khối lượng đầu vào','QS xác nhận bản phát hành BOQ dùng cho mua hàng đầu vào.','task','QS','boq.update',false,false,false,NULL,NULL,'workflow_form','source.boq_release_ready',1),
  ('48500000-0000-0000-0000-000000000011'::uuid,'input_takeoff_purchase','input.price_sourcing','Hoàn tất lấy giá nhà cung cấp','Mua hàng hoàn tất báo giá nhà cung cấp đủ điều kiện tại module mua sắm.','material_check','Mua hàng','procurement.create',false,false,false,NULL,NULL,'workflow_form','source.supplier_quotation_ready',2),
  ('48500000-0000-0000-0000-000000000012'::uuid,'price_negotiation','price_negotiation.complete','Chốt lựa chọn giá đầu vào','Mua hàng chốt lựa chọn báo giá sau đàm phán.','budget_check','Mua hàng','boq.approve',false,false,false,NULL,NULL,'workflow_form','source.supplier_selection_ready',1),
  ('48500000-0000-0000-0000-000000000013'::uuid,'input_contract_advance','input.contract_package','Hoàn tất gói hợp đồng đầu vào','Mua hàng hoàn tất procurement được duyệt và hợp đồng đầu vào đủ điều kiện.','document_upload','Mua hàng','procurement.create',false,false,false,NULL,NULL,'contract','source.input_contract_package_ready',1),
  ('48500000-0000-0000-0000-000000000014'::uuid,'construction_planning','planning.execution_package','Lập gói kế hoạch thi công','PM ghi một lần biên bản triển khai và checklist QA của kế hoạch thi công.','document_upload','PM','operation.report',true,true,false,NULL,'48100000-0000-0000-0000-000000000003'::uuid,'workflow_form',NULL,1),
  ('48500000-0000-0000-0000-000000000015'::uuid,'construction_planning','planning.construction_handoff','Kích hoạt bàn giao thi công','PM chốt hạng mục giám sát và kích hoạt bàn giao ra hiện trường.','checklist','PM','construction.handoff.manage',true,false,false,'planning.construction_handoff',NULL,'workflow_form','construction.activate_handoff',2),
  ('48500000-0000-0000-0000-000000000016'::uuid,'construction_execution','construction.item_reporting','Báo cáo thi công liên tục','Giám sát cập nhật tiến độ, nhật ký, vấn đề và minh chứng theo diễn biến thực tế.','daily_report','Giám sát','operation.report',true,false,false,'construction.item_reporting',NULL,'quality',NULL,1),
  ('48500000-0000-0000-0000-000000000017'::uuid,'incoming_acceptance','incoming.subcontractor_package','Nghiệm thu gói thầu phụ và khối lượng','QS tổng hợp checklist và khối lượng thầu phụ; trạng thái nguồn được kiểm tra trước khi chốt.','acceptance_check','QS','acceptance.create',true,false,false,NULL,'48100000-0000-0000-0000-000000000004'::uuid,'subcontractor_reconciliation','source.subcontractor_acceptance_package_ready',1),
  ('48500000-0000-0000-0000-000000000018'::uuid,'incoming_acceptance','incoming.material_acceptance','Nghiệm thu vật tư đầu vào','QA/QC chốt hồ sơ nghiệm thu vật tư nguồn.','acceptance_check','QA/QC','acceptance.create',true,false,false,'incoming.material_acceptance',NULL,'incoming_acceptance','acceptance.incoming_material',2),
  ('48500000-0000-0000-0000-000000000019'::uuid,'outgoing_acceptance','outgoing.final_package','Chốt gói nghiệm thu cuối','QA/QC chốt đối chiếu chủ đầu tư và hồ sơ nghiệm thu cuối đạt.','acceptance_check','QA/QC','acceptance.approve',true,false,false,'outgoing.acceptance_signoff',NULL,'owner_client_reconciliation','source.final_acceptance_package_ready',1),
  ('48500000-0000-0000-0000-000000000020'::uuid,'handover_finance','closure.handover','Hoàn tất bàn giao vật lý','PM hoàn tất hồ sơ bàn giao sau khi chất lượng, tồn tại và vật tư dư đã đóng; tài chính không chặn bước này.','handover_check','PM','handover.approve',false,false,false,NULL,NULL,'handover','source.handover_completed',1),
  ('48500000-0000-0000-0000-000000000021'::uuid,'handover_finance','closure.finance','Khóa tài chính công trình','Kế toán xác nhận chi phí vật tư và các đề nghị thu/chi đã về trạng thái kết thúc.','accounting_check','Kế toán','project_finance.report_view',false,false,false,NULL,NULL,'finance','source.project_finance_close_ready',2)
), source_steps AS (
  SELECT step.business_code,step.form_version_id
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000026'
)
INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  definition.id,node.id,definition.business_code,definition.name,definition.description,
  definition.step_type,jsonb_build_array(definition.role_name),definition.permission_key,
  definition.checklist_required,definition.document_required,definition.photo_required,
  COALESCE(definition.composite_form_version_id,source.form_version_id),
  COALESCE(definition.composite_form_version_id,source.form_version_id) IS NOT NULL,
  definition.surface_kind,definition.domain_action_key,definition.order_index
FROM step_defs definition
JOIN pipeline_template_nodes node
  ON node.template_id='40000000-0000-0000-0000-000000000027'
 AND node.business_code=definition.node_code
LEFT JOIN source_steps source ON source.business_code=definition.source_business_code
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_edges (
  id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority
)
SELECT
  replace(source.id::text,'47600000','48600000')::uuid,
  '40000000-0000-0000-0000-000000000027',
  replace(source.from_node_id::text,'47400000','48400000')::uuid,
  replace(source.to_node_id::text,'47400000','48400000')::uuid,
  source.condition_type,source.condition_config,source.label,source.priority
FROM pipeline_template_edges source
WHERE source.template_id='40000000-0000-0000-0000-000000000026'
ON CONFLICT (id) DO NOTHING;

-- Only projects that selected Rev7 and still have no runtime adopt Rev8.
UPDATE projects project
SET pipeline_template_id='40000000-0000-0000-0000-000000000027',updated_at=now()
WHERE project.pipeline_template_id='40000000-0000-0000-0000-000000000026'
  AND NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=project.id);

DO $$
DECLARE
  actual integer;
BEGIN
  SELECT count(*) INTO actual
  FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000027';
  IF actual<>15 THEN RAISE EXCEPTION 'expected Rev8 to contain 15 nodes, found %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000027';
  IF actual<>21 THEN RAISE EXCEPTION 'expected Rev8 to contain 21 steps, found %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000027'
    AND approval_policy_json<>'{}'::jsonb;
  IF actual<>2 THEN RAISE EXCEPTION 'expected Rev8 to contain 2 node approval policies, found %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000027'
    AND jsonb_array_length(step.required_role_ids)<>1;
  IF actual<>0 THEN RAISE EXCEPTION 'Rev8 contains % steps without exactly one primary role',actual; END IF;

  SELECT count(*) INTO actual
  FROM (
    SELECT step.id
    FROM pipeline_template_steps step
    JOIN pipeline_template_nodes node ON node.id=step.node_id
    LEFT JOIN workflow_form_fields field ON field.form_version_id=step.form_version_id
    WHERE node.template_id='40000000-0000-0000-0000-000000000027'
      AND step.form_required
    GROUP BY step.id
    HAVING count(field.id)=0
  ) empty_forms;
  IF actual<>0 THEN RAISE EXCEPTION 'Rev8 contains % required forms without fields',actual; END IF;

  SELECT sum(step_count)::integer INTO actual
  FROM (VALUES
    ('project_intake',1),('survey',2),('survey_review',1),('design',2),('boq_with_design',1),
    ('outbound_contract',1),('input_takeoff_purchase',2),('price_negotiation',1),
    ('input_contract_advance',1),('construction_planning',2),('construction_execution',1),
    ('incoming_acceptance',2),('outgoing_acceptance',1),('handover_finance',2)
  ) design_path(node_code,step_count);
  IF actual<>20 THEN RAISE EXCEPTION 'expected Rev8 design path to contain 20 steps, found %',actual; END IF;

  SELECT sum(step_count)::integer INTO actual
  FROM (VALUES
    ('project_intake',1),('survey',2),('survey_review',1),('boq_without_design',1),
    ('outbound_contract',1),('input_takeoff_purchase',2),('price_negotiation',1),
    ('input_contract_advance',1),('construction_planning',2),('construction_execution',1),
    ('incoming_acceptance',2),('outgoing_acceptance',1),('handover_finance',2)
  ) no_design_path(node_code,step_count);
  IF actual<>18 THEN RAISE EXCEPTION 'expected Rev8 no-design path to contain 18 steps, found %',actual; END IF;

  SELECT sum(step_count)::integer INTO actual
  FROM (VALUES
    ('project_intake',1),('boq_without_design',1),('outbound_contract',1),
    ('input_takeoff_purchase',2),('price_negotiation',1),('input_contract_advance',1),
    ('construction_planning',2),('construction_execution',1),('incoming_acceptance',2),
    ('outgoing_acceptance',1),('handover_finance',2)
  ) client_boq_path(node_code,step_count);
  IF actual<>15 THEN RAISE EXCEPTION 'expected Rev8 client-BOQ path to contain 15 steps, found %',actual; END IF;
END $$;
