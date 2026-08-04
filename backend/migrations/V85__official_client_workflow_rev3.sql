-- Official immutable Revision 3 requested by the client. Existing published
-- revisions and project runtime snapshots are not updated.

INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by
)
SELECT
  '40000000-0000-0000-0000-000000000022',
  name,
  'Revision 3: chọn điểm vào theo hồ sơ khách hàng, chốt hợp đồng đầu ra trước đầu vào, ký trên App, luân chuyển hồ sơ giấy và chuẩn hóa vai trò.',
  project_type,
  3,
  'published',
  created_by,
  '40000000-0000-0000-0000-000000000020',
  3,
  '40000000-0000-0000-0000-000000000021',
  3,
  encode(digest('official-client-workflow-rev3','sha256'),'hex'),
  now(),
  NULL
FROM pipeline_templates
WHERE id='40000000-0000-0000-0000-000000000021'
ON CONFLICT (id) DO NOTHING;

WITH node_map(source_id,target_id,business_code,order_index) AS (
  VALUES
    ('42400000-0000-0000-0000-000000000001'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'survey',2),
    ('42400000-0000-0000-0000-000000000002'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'survey_review',3),
    ('42400000-0000-0000-0000-000000000003'::uuid,'43400000-0000-0000-0000-000000000004'::uuid,'design',4),
    ('42400000-0000-0000-0000-000000000004'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'boq_without_design',5),
    ('42400000-0000-0000-0000-000000000005'::uuid,'43400000-0000-0000-0000-000000000006'::uuid,'boq_with_design',6),
    ('42400000-0000-0000-0000-000000000008'::uuid,'43400000-0000-0000-0000-000000000007'::uuid,'outbound_contract',7),
    ('42400000-0000-0000-0000-000000000006'::uuid,'43400000-0000-0000-0000-000000000008'::uuid,'input_takeoff_purchase',8),
    ('42400000-0000-0000-0000-000000000007'::uuid,'43400000-0000-0000-0000-000000000009'::uuid,'price_negotiation',9),
    ('42400000-0000-0000-0000-000000000006'::uuid,'43400000-0000-0000-0000-000000000010'::uuid,'input_contract_advance',10),
    ('42400000-0000-0000-0000-000000000009'::uuid,'43400000-0000-0000-0000-000000000011'::uuid,'construction_planning',11),
    ('42400000-0000-0000-0000-000000000013'::uuid,'43400000-0000-0000-0000-000000000012'::uuid,'construction_execution',12),
    ('42400000-0000-0000-0000-000000000010'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming_acceptance',13),
    ('42400000-0000-0000-0000-000000000011'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'outgoing_acceptance',14),
    ('42400000-0000-0000-0000-000000000012'::uuid,'43400000-0000-0000-0000-000000000015'::uuid,'handover_finance',15)
)
INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  map.target_id,
  '40000000-0000-0000-0000-000000000022',
  map.business_code,
  source.name,
  source.description,
  source.type,
  80 + (map.order_index-1)*280,
  CASE map.business_code WHEN 'design' THEN 420 WHEN 'boq_with_design' THEN 420 ELSE 220 END,
  map.order_index,
  source.responsible_role_ids,
  source.required_permission,
  source.permissions,
  source.entry_conditions,
  source.exit_conditions,
  source.approval_rule,
  source.approval_policy_json,
  source.rework_enabled,
  source.replanning_enabled,
  source.fail_path,
  source.status
FROM node_map map
JOIN pipeline_template_nodes source ON source.id=map.source_id
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
VALUES (
  '43400000-0000-0000-0000-000000000001',
  '40000000-0000-0000-0000-000000000022',
  'project_intake',
  'Tiếp nhận dự án',
  'Xác nhận hồ sơ đầu vào và dùng lựa chọn đã chốt khi tạo công trình để vào nhánh Khảo sát hoặc BOQ có sẵn.',
  'project_intake',80,220,1,
  '["PM","QS","Giám sát","Thiết kế"]'::jsonb,
  'project.update',
  '["project.view","project.update","pipeline.view"]'::jsonb,
  '{}'::jsonb,
  '{"text":"Điểm bắt đầu và hồ sơ đầu vào đã được xác nhận"}'::jsonb,
  NULL,'{}'::jsonb,false,false,'none','active'
)
ON CONFLICT (id) DO NOTHING;

UPDATE pipeline_template_nodes
SET responsible_role_ids='["PM","QS","Kế toán","Pháp lý","Mua hàng","Bộ phận HĐ","Giám đốc"]'::jsonb,
    required_permission='contract.view',
    permissions='["contract.view","contract.create","contract.review","contract.approve","contract.sign","contract.dispatch","contract.scan_manage"]'::jsonb,
    exit_conditions='{"contract_active":true,"app_signature_required":true,"paper_dispatch_required":true,"returned_scan_required":true,"text":"Hợp đồng đầu ra đã ký trên App, gửi hồ sơ giấy và nhận lại bản scan"}'::jsonb,
    approval_rule='Duyệt hợp đồng theo thứ tự Bộ phận HĐ/QS, các bộ phận kiểm tra, PM và Giám đốc',
    approval_policy_json='{"mode":"ordered","stages":[{"roles":["QS","Bộ phận HĐ"],"quorum":1},{"roles":["Kế toán","Pháp lý","Mua hàng"],"quorum":3},{"roles":["PM"],"quorum":1},{"roles":["Giám đốc"],"quorum":1}],"requester_cannot_approve":true,"comment_required":true,"signature_required":false}'::jsonb,
    updated_at=now()
WHERE id='43400000-0000-0000-0000-000000000007';

UPDATE pipeline_template_nodes
SET name='Bóc tách và lấy giá đầu vào',
    description='Bóc khối lượng và lấy giá tổ đội, thầu phụ, nhà cung cấp sau khi hợp đồng đầu ra đã chốt.',
    responsible_role_ids='["QS","Giám sát","Mua hàng"]'::jsonb,
    required_permission='boq.update',
    permissions='["boq.view","boq.update","procurement.view","procurement.create"]'::jsonb,
    exit_conditions='{"text":"Khối lượng và giá đầu vào đã được tập hợp"}'::jsonb,
    approval_rule=NULL,
    approval_policy_json='{}'::jsonb,
    updated_at=now()
WHERE id='43400000-0000-0000-0000-000000000008';

UPDATE pipeline_template_nodes
SET description='Đàm phán và chốt đơn giá đầu vào trong một bước duy nhất.',
    responsible_role_ids='["Sale","QS","PM","Giám đốc","Kế toán","Mua hàng"]'::jsonb,
    updated_at=now()
WHERE id='43400000-0000-0000-0000-000000000009';

UPDATE pipeline_template_nodes
SET name='Hợp đồng và tạm ứng đầu vào',
    description='Ký hợp đồng, lập hồ sơ tạm ứng và rà soát đầu vào sau khi đã chốt giá.',
    responsible_role_ids='["Mua hàng","QS","Kế toán","Bộ phận HĐ","PM","Giám đốc"]'::jsonb,
    required_permission='contract.view',
    permissions='["contract.view","contract.create","contract.review","procurement.view","procurement.approve"]'::jsonb,
    updated_at=now()
WHERE id='43400000-0000-0000-0000-000000000010';

UPDATE pipeline_template_nodes
SET responsible_role_ids='["PM","QS","Thiết kế","QA/QC","Giám sát"]'::jsonb,
    description='Phân công thi công, lập hồ sơ chất lượng với chủ đầu tư và bàn giao danh sách hạng mục giám sát.',
    updated_at=now()
WHERE id='43400000-0000-0000-0000-000000000011';

UPDATE pipeline_template_nodes
SET responsible_role_ids='["PM","QS","Giám sát","QA/QC"]'::jsonb,
    description='Giám sát báo cáo theo hạng mục bằng checklist, hình ảnh và tiếp nhận phát sinh ngoài kế hoạch.',
    updated_at=now()
WHERE id='43400000-0000-0000-0000-000000000012';

UPDATE pipeline_template_nodes
SET responsible_role_ids=responsible_role_ids || '["Giám sát"]'::jsonb,
    updated_at=now()
WHERE id='43400000-0000-0000-0000-000000000013'
  AND NOT responsible_role_ids ? 'Giám sát';

-- Copy unchanged steps and give each one a stable business identity.
WITH step_map(source_id,target_node_id,business_code,target_order) AS (
  VALUES
    ('42500000-0000-0000-0000-000000000001'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'survey.client_requirements',1),
    ('42500000-0000-0000-0000-000000000002'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'survey.site_photos',2),
    ('42500000-0000-0000-0000-000000000003'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'survey.design_style',3),
    ('42500000-0000-0000-0000-000000000004'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'survey.dimensions',4),
    ('42500000-0000-0000-0000-000000000005'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'survey.risks',5),
    ('42500000-0000-0000-0000-000000000006'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'survey.minutes',6),
    ('42500000-0000-0000-0000-000000000007'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'survey_review.takeoff',1),
    ('42500000-0000-0000-0000-000000000008'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'survey_review.layout',2),
    ('42500000-0000-0000-0000-000000000009'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'survey_review.3d',3),
    ('42500000-0000-0000-0000-000000000010'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'survey_review.construction_drawing',4),
    ('42500000-0000-0000-0000-000000000011'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'survey_review.branch_decision',5),
    ('42500000-0000-0000-0000-000000000012'::uuid,'43400000-0000-0000-0000-000000000004'::uuid,'design.layout_3d_review',1),
    ('42500000-0000-0000-0000-000000000014'::uuid,'43400000-0000-0000-0000-000000000004'::uuid,'design.construction_drawing',2),
    ('42500000-0000-0000-0000-000000000015'::uuid,'43400000-0000-0000-0000-000000000004'::uuid,'design.client_approval',3),
    ('42500000-0000-0000-0000-000000000016'::uuid,'43400000-0000-0000-0000-000000000004'::uuid,'design.handover_to_qs',4),
    ('42500000-0000-0000-0000-000000000017'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'boq_without_design.pricing',1),
    ('42500000-0000-0000-0000-000000000018'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'boq_without_design.price_check',2),
    ('42500000-0000-0000-0000-000000000019'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'boq_without_design.management_norm',3),
    ('42500000-0000-0000-0000-000000000020'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'boq_without_design.client_send',4),
    ('42500000-0000-0000-0000-000000000021'::uuid,'43400000-0000-0000-0000-000000000006'::uuid,'boq_with_design.client_send',1),
    ('42500000-0000-0000-0000-000000000022'::uuid,'43400000-0000-0000-0000-000000000008'::uuid,'input.takeoff',1),
    ('42500000-0000-0000-0000-000000000023'::uuid,'43400000-0000-0000-0000-000000000008'::uuid,'input.price_sourcing',2),
    ('42500000-0000-0000-0000-000000000024'::uuid,'43400000-0000-0000-0000-000000000010'::uuid,'input.contract_advance',1),
    ('42500000-0000-0000-0000-000000000025'::uuid,'43400000-0000-0000-0000-000000000010'::uuid,'input.review',2),
    ('42500000-0000-0000-0000-000000000034'::uuid,'43400000-0000-0000-0000-000000000011'::uuid,'planning.assignment_meeting',1),
    ('42500000-0000-0000-0000-000000000035'::uuid,'43400000-0000-0000-0000-000000000011'::uuid,'planning.assignment_minutes',2),
    ('42500000-0000-0000-0000-000000000036'::uuid,'43400000-0000-0000-0000-000000000011'::uuid,'planning.client_quality_file',3),
    ('42500000-0000-0000-0000-000000000060'::uuid,'43400000-0000-0000-0000-000000000011'::uuid,'planning.construction_handoff',4),
    ('42500000-0000-0000-0000-000000000037'::uuid,'43400000-0000-0000-0000-000000000012'::uuid,'construction.item_reporting',1),
    ('42500000-0000-0000-0000-000000000038'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.subcontractor_acceptance',1),
    ('42500000-0000-0000-0000-000000000039'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.material_acceptance',2),
    ('42500000-0000-0000-0000-000000000040'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.quantity_file',3),
    ('42500000-0000-0000-0000-000000000041'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.quantity_review',4),
    ('42500000-0000-0000-0000-000000000042'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.payment_request',5),
    ('42500000-0000-0000-0000-000000000043'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.payment_approval',6),
    ('42500000-0000-0000-0000-000000000044'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.payment_check',7),
    ('42500000-0000-0000-0000-000000000045'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'incoming.accounting_handover',8),
    ('42500000-0000-0000-0000-000000000046'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'outgoing.acceptance_signoff',1),
    ('42500000-0000-0000-0000-000000000047'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'outgoing.file_signoff',2),
    ('42500000-0000-0000-0000-000000000048'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'outgoing.collection_request',3),
    ('42500000-0000-0000-0000-000000000049'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'outgoing.invoice_handover',4),
    ('42500000-0000-0000-0000-000000000050'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'outgoing.collection_tracking',5),
    ('42500000-0000-0000-0000-000000000051'::uuid,'43400000-0000-0000-0000-000000000015'::uuid,'closure.material_cost',1),
    ('42500000-0000-0000-0000-000000000052'::uuid,'43400000-0000-0000-0000-000000000015'::uuid,'closure.finance_confirm',2)
)
INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  replace(source.id::text,'42500000','43500000')::uuid,
  map.target_node_id,
  map.business_code,
  CASE WHEN source.id='42500000-0000-0000-0000-000000000012' THEN 'Trao đổi và hoàn thiện bản vẽ mặt bằng + 3D' ELSE source.name END,
  CASE WHEN source.id='42500000-0000-0000-0000-000000000012' THEN 'Gom nội dung 3.1 và 3.2: trao đổi, hoàn thiện bản vẽ bố trí mặt bằng và bản vẽ 3D trong một bước.' ELSE source.description END,
  source.step_type,
  source.required_role_ids,
  source.required_permission,
  source.checklist_required,
  source.document_required,
  source.photo_required,
  source.form_version_id,
  source.form_required,
  source.surface_kind,
  source.domain_action_key,
  map.target_order
FROM step_map map
JOIN pipeline_template_steps source ON source.id=map.source_id
ON CONFLICT (id) DO NOTHING;

-- Explicitly approved role additions.
UPDATE pipeline_template_steps
SET required_role_ids='["PM","Giám đốc","QS","Sale","Đội thi công","Giám sát","Thiết kế"]'::jsonb
WHERE id='43500000-0000-0000-0000-000000000001';

UPDATE pipeline_template_steps
SET required_role_ids='["QS","Giám sát"]'::jsonb
WHERE id='43500000-0000-0000-0000-000000000022';

UPDATE pipeline_template_steps
SET required_role_ids=required_role_ids || '["Giám sát"]'::jsonb
WHERE id IN (
  '43500000-0000-0000-0000-000000000034',
  '43500000-0000-0000-0000-000000000038'
)
  AND NOT required_role_ids ? 'Giám sát';

UPDATE pipeline_template_steps
SET name='Lập hồ sơ chất lượng với chủ đầu tư',
    description='QA/QC lập và quản lý hồ sơ chất lượng làm việc với chủ đầu tư; Giám sát phụ trách báo cáo hiện trường riêng.',
    required_role_ids='["QA/QC","PM"]'::jsonb,
    surface_kind='quality'
WHERE id='43500000-0000-0000-0000-000000000036';

INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  '43500000-0000-0000-0000-000000000100',
  '43400000-0000-0000-0000-000000000001',
  'project_intake.confirm',
  'Xác nhận điểm bắt đầu và hồ sơ đầu vào',
  'Xác nhận lựa chọn Khảo sát hoặc Chủ đầu tư có BOQ; lựa chọn này được khóa khi pipeline khởi chạy.',
  'task','["PM","QS","Giám sát","Thiết kế"]'::jsonb,'project.update',
  false,false,false,NULL,false,'generic',NULL,1
WHERE NOT EXISTS (SELECT 1 FROM pipeline_template_steps WHERE id='43500000-0000-0000-0000-000000000100');

-- Node 7 is represented by one step only.
INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  '43500000-0000-0000-0000-000000000026',
  '43400000-0000-0000-0000-000000000009',
  'price_negotiation.complete',
  'Đàm phán và chốt giá đầu vào',
  'Gom đàm phán, chốt định mức và review báo cáo đơn giá vào một bước có đủ dấu vết.',
  source.step_type,
  '["Sale","QS","PM","Giám đốc","Kế toán","Mua hàng"]'::jsonb,
  'boq.approve',true,false,false,source.form_version_id,true,
  source.surface_kind,NULL,1
FROM pipeline_template_steps source
WHERE source.id='42500000-0000-0000-0000-000000000026'
  AND NOT EXISTS (SELECT 1 FROM pipeline_template_steps WHERE id='43500000-0000-0000-0000-000000000026');

-- Contract steps use the existing workflow form/signature infrastructure.
WITH contract_step(source_id,target_id,business_code,step_name,step_description,permission_key,order_index,domain_action_key) AS (
  VALUES
    ('42500000-0000-0000-0000-000000000029'::uuid,'43500000-0000-0000-0000-000000000029'::uuid,'outbound_contract.prepare','Lập báo giá và hợp đồng','Bộ phận HĐ/QS lập báo giá, hợp đồng và hồ sơ tạm ứng đầu ra.','contract.create',1,NULL::text),
    ('42500000-0000-0000-0000-000000000030'::uuid,'43500000-0000-0000-0000-000000000030'::uuid,'outbound_contract.review','Rà soát hợp đồng liên phòng ban','QS, Kế toán, Pháp lý, Mua hàng, Bộ phận HĐ, PM và Giám đốc cùng rà soát hồ sơ.','contract.review',2,NULL::text),
    ('42500000-0000-0000-0000-000000000032'::uuid,'43500000-0000-0000-0000-000000000032'::uuid,'outbound_contract.app_sign','Ký duyệt nội bộ trên App','Dùng component chữ ký hiện có để ký biểu mẫu hợp đồng đã gửi; chữ ký được liên kết bất biến với version hợp đồng.','contract.sign',3,NULL::text),
    ('42500000-0000-0000-0000-000000000033'::uuid,'43500000-0000-0000-0000-000000000033'::uuid,'outbound_contract.paper_dispatch','In và gửi hồ sơ giấy cho chủ đầu tư','Ghi nhận người nhận, cách gửi, mã vận đơn và thời điểm gửi hồ sơ giấy.','contract.dispatch',4,NULL::text),
    ('42500000-0000-0000-0000-000000000033'::uuid,'43500000-0000-0000-0000-000000000061'::uuid,'outbound_contract.returned_scan','Nhận lại và scan hợp đồng đã ký','Ghi nhận chủ đầu tư đã ký nhận và liên kết bản scan trước khi mở các bước triển khai.','contract.scan_manage',5,'contract.require_execution_ready')
)
INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  seed.target_id,
  '43400000-0000-0000-0000-000000000007',
  seed.business_code,
  seed.step_name,
  seed.step_description,
  source.step_type,
  '["QS","Kế toán","Pháp lý","Mua hàng","Bộ phận HĐ","PM","Giám đốc"]'::jsonb,
  seed.permission_key,
  true,
  seed.business_code IN ('outbound_contract.prepare','outbound_contract.paper_dispatch','outbound_contract.returned_scan'),
  false,
  source.form_version_id,
  true,
  'contract',
  seed.domain_action_key,
  seed.order_index
FROM contract_step seed
JOIN pipeline_template_steps source ON source.id=seed.source_id
ON CONFLICT (id) DO NOTHING;

WITH edge_seed(id,from_node_id,to_node_id,condition_type,condition_config,label,priority) AS (
  VALUES
    ('43600000-0000-0000-0000-000000000001'::uuid,'43400000-0000-0000-0000-000000000001'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'project_entry_mode','{"value":"survey_required"}'::jsonb,'Cần khảo sát',10),
    ('43600000-0000-0000-0000-000000000002'::uuid,'43400000-0000-0000-0000-000000000001'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'project_entry_mode','{"value":"client_boq"}'::jsonb,'Chủ đầu tư có BOQ',20),
    ('43600000-0000-0000-0000-000000000003'::uuid,'43400000-0000-0000-0000-000000000002'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'always','{}'::jsonb,'Hoàn tất khảo sát',100),
    ('43600000-0000-0000-0000-000000000004'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'43400000-0000-0000-0000-000000000004'::uuid,'workflow_form_answer_equals','{"field_key":"needs_design","value":"yes"}'::jsonb,'Có thiết kế',10),
    ('43600000-0000-0000-0000-000000000005'::uuid,'43400000-0000-0000-0000-000000000003'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'workflow_form_answer_equals','{"field_key":"needs_design","value":"no"}'::jsonb,'Không thiết kế',20),
    ('43600000-0000-0000-0000-000000000006'::uuid,'43400000-0000-0000-0000-000000000004'::uuid,'43400000-0000-0000-0000-000000000006'::uuid,'always','{}'::jsonb,'Hoàn tất thiết kế',100),
    ('43600000-0000-0000-0000-000000000007'::uuid,'43400000-0000-0000-0000-000000000005'::uuid,'43400000-0000-0000-0000-000000000007'::uuid,'always','{}'::jsonb,'BOQ đã gửi',100),
    ('43600000-0000-0000-0000-000000000008'::uuid,'43400000-0000-0000-0000-000000000006'::uuid,'43400000-0000-0000-0000-000000000007'::uuid,'always','{}'::jsonb,'BOQ đã gửi',100),
    ('43600000-0000-0000-0000-000000000009'::uuid,'43400000-0000-0000-0000-000000000007'::uuid,'43400000-0000-0000-0000-000000000008'::uuid,'always','{}'::jsonb,'Hợp đồng đầu ra đã đủ điều kiện',100),
    ('43600000-0000-0000-0000-000000000010'::uuid,'43400000-0000-0000-0000-000000000008'::uuid,'43400000-0000-0000-0000-000000000009'::uuid,'always','{}'::jsonb,'Đã bóc tách và lấy giá',100),
    ('43600000-0000-0000-0000-000000000011'::uuid,'43400000-0000-0000-0000-000000000009'::uuid,'43400000-0000-0000-0000-000000000010'::uuid,'always','{}'::jsonb,'Đã chốt giá',100),
    ('43600000-0000-0000-0000-000000000012'::uuid,'43400000-0000-0000-0000-000000000010'::uuid,'43400000-0000-0000-0000-000000000011'::uuid,'always','{}'::jsonb,'Hồ sơ đầu vào hoàn tất',100),
    ('43600000-0000-0000-0000-000000000013'::uuid,'43400000-0000-0000-0000-000000000011'::uuid,'43400000-0000-0000-0000-000000000012'::uuid,'always','{}'::jsonb,'Bàn giao thi công',100),
    ('43600000-0000-0000-0000-000000000014'::uuid,'43400000-0000-0000-0000-000000000012'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'always','{}'::jsonb,'Thi công hoàn tất',100),
    ('43600000-0000-0000-0000-000000000015'::uuid,'43400000-0000-0000-0000-000000000013'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'always','{}'::jsonb,'Nghiệm thu đầu vào hoàn tất',100),
    ('43600000-0000-0000-0000-000000000016'::uuid,'43400000-0000-0000-0000-000000000014'::uuid,'43400000-0000-0000-0000-000000000015'::uuid,'always','{}'::jsonb,'Nghiệm thu đầu ra hoàn tất',100)
)
INSERT INTO pipeline_template_edges (
  id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority
)
SELECT id,'40000000-0000-0000-0000-000000000022',from_node_id,to_node_id,condition_type,condition_config,label,priority
FROM edge_seed
ON CONFLICT (id) DO NOTHING;

-- New and not-yet-started projects follow Revision 3. Active/historical runs keep
-- their template/runtime snapshot, including projects 020, 021 and 022.
UPDATE projects project
SET pipeline_template_id='40000000-0000-0000-0000-000000000022',updated_at=now()
WHERE project.pipeline_template_id='40000000-0000-0000-0000-000000000021'
  AND NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=project.id);
