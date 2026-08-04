\set ON_ERROR_STOP on

BEGIN;

-- Fresh, untouched Rev3 runtime used to verify the client-BOQ entry branch.
-- Existing Golden Live project 022 remains unchanged for historical comparison.
INSERT INTO projects (
  id,name,code,partner_id,client_name,address,planned_start_date,planned_end_date,
  estimated_budget,contract_amount,status,pipeline_template_id,pm_staff_id,
  progress_percent,workflow_entry_mode
)
SELECT
  '30000000-0000-0000-0000-000000000023',
  'Golden Rev3 – Báo giá từ BOQ chủ đầu tư',
  'GOLDEN-REV3-BOQ-001',
  '90000000-0000-0000-0000-000000000001',
  'Công ty An Phát',
  'Quận 3, TP.HCM',
  '2026-08-04','2027-02-28',
  3100000000,4600000000,'planning',t.id,
  '21000000-0000-0000-0000-000000000002',0,'client_boq'
FROM pipeline_templates t
WHERE t.id='40000000-0000-0000-0000-000000000022' AND t.status='published';

INSERT INTO documents (
  id,project_id,target_type,target_id,name,file_url,document_type,status,uploaded_by,
  mime_type,file_size,storage_provider
) VALUES (
  '46000000-0000-0000-0000-000000000023',
  '30000000-0000-0000-0000-000000000023',
  'project_intake','30000000-0000-0000-0000-000000000023',
  'BOQ đầu vào từ chủ đầu tư.pdf','/fixtures/golden-rev3/client-boq.pdf','client_boq','active',
  '10000000-0000-0000-0000-000000000003','application/pdf',0,'external'
);

INSERT INTO project_members (project_id,staff_id,role_ids,assigned_nodes)
SELECT '30000000-0000-0000-0000-000000000023',s.id,jsonb_build_array(r.display_name),'[]'::jsonb
FROM staff s
JOIN users u ON u.id=s.user_id
JOIN user_roles ur ON ur.user_id=u.id
JOIN roles r ON r.id=ur.role_id
WHERE u.email IN (
  'admin@consflow.local','pm@consflow.local','qs@consflow.local',
  'accounting@consflow.local','supervisor@consflow.local','director@consflow.local',
  'procurement@consflow.local','qaqc@consflow.local','design@consflow.local',
  'sales@consflow.local','legal@consflow.local','contract@consflow.local'
);

INSERT INTO pipeline_runs (
  id,project_id,template_id,status,started_at,engine_version,template_definition_hash
)
SELECT
  '42000000-0000-0000-0000-000000000023',
  '30000000-0000-0000-0000-000000000023',t.id,'in_progress',now(),
  t.engine_version,t.definition_hash
FROM pipeline_templates t
WHERE t.id='40000000-0000-0000-0000-000000000022' AND t.status='published';

CREATE TEMP TABLE golden_rev3_node_map ON COMMIT DROP AS
SELECT n.id AS template_node_id,gen_random_uuid() AS run_node_id,n.order_index,
       CASE WHEN n.order_index=1 THEN 'in_progress' ELSE 'pending' END AS runtime_status
FROM pipeline_template_nodes n
WHERE n.template_id='40000000-0000-0000-0000-000000000022';

INSERT INTO pipeline_run_nodes (
  id,run_id,project_id,template_node_id,business_code,name,description,type,order_index,
  responsible_role_ids,required_permission,permissions,exit_conditions,approval_rule,
  approval_policy_json,fail_path,status,started_at
)
SELECT m.run_node_id,'42000000-0000-0000-0000-000000000023',
       '30000000-0000-0000-0000-000000000023',n.id,n.business_code,n.name,n.description,
       n.type,n.order_index,n.responsible_role_ids,n.required_permission,n.permissions,
       n.exit_conditions,n.approval_rule,n.approval_policy_json,n.fail_path,m.runtime_status,
       CASE WHEN n.order_index=1 THEN now() END
FROM golden_rev3_node_map m
JOIN pipeline_template_nodes n ON n.id=m.template_node_id;

INSERT INTO pipeline_run_edges (
  run_id,project_id,template_edge_id,from_run_node_id,to_run_node_id,
  condition_type,condition_config,label,priority
)
SELECT '42000000-0000-0000-0000-000000000023',
       '30000000-0000-0000-0000-000000000023',e.id,f.run_node_id,t.run_node_id,
       e.condition_type,e.condition_config,e.label,e.priority
FROM pipeline_template_edges e
JOIN golden_rev3_node_map f ON f.template_node_id=e.from_node_id
JOIN golden_rev3_node_map t ON t.template_node_id=e.to_node_id
WHERE e.template_id='40000000-0000-0000-0000-000000000022';

CREATE TEMP TABLE golden_rev3_step_map ON COMMIT DROP AS
SELECT n.id AS template_node_id,n.order_index AS node_order,m.run_node_id,
       s.id AS template_step_id,gen_random_uuid() AS run_step_id,s.business_code,
       s.name,s.description,s.step_type,s.required_role_ids,s.required_permission,
       s.checklist_required,s.document_required,s.photo_required,s.form_required,
       s.form_version_id,s.surface_kind,s.domain_action_key,s.order_index
FROM pipeline_template_nodes n
JOIN golden_rev3_node_map m ON m.template_node_id=n.id
JOIN pipeline_template_steps s ON s.node_id=n.id
WHERE n.template_id='40000000-0000-0000-0000-000000000022';

INSERT INTO pipeline_run_steps (
  id,run_node_id,project_id,template_step_id,business_code,name,description,step_type,
  required_role_ids,required_permission,checklist_required,document_required,photo_required,
  form_required,form_version_id,surface_kind,domain_action_key,order_index,status
)
SELECT run_step_id,run_node_id,'30000000-0000-0000-0000-000000000023',template_step_id,
       business_code,name,description,step_type,required_role_ids,required_permission,
       checklist_required,document_required,photo_required,form_required,form_version_id,
       surface_kind,domain_action_key,order_index,'pending'
FROM golden_rev3_step_map;

CREATE TEMP TABLE golden_rev3_work_map ON COMMIT DROP AS
SELECT sm.*,gen_random_uuid() AS work_item_id
FROM golden_rev3_step_map sm;

INSERT INTO work_items (
  id,project_id,pipeline_run_id,run_node_id,run_step_id,source_type,source_id,
  title,description,work_type,status,priority,role_ids,evidence_requirement_json,
  created_by,updated_by
)
SELECT work_item_id,'30000000-0000-0000-0000-000000000023',
       '42000000-0000-0000-0000-000000000023',run_node_id,run_step_id,
       'pipeline_run_step',run_step_id,name,description,
       CASE WHEN step_type='meeting_minutes' THEN 'checklist' ELSE step_type END,
       CASE WHEN node_order=1 THEN 'ready' ELSE 'pending' END,'normal',required_role_ids,
       jsonb_build_object(
         'checklist_required',checklist_required,'document_required',document_required,
         'photo_required',photo_required,'form_required',form_required,
         'form_version_id',form_version_id,'required_permission',required_permission
       ),
       '10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002'
FROM golden_rev3_work_map;

INSERT INTO work_item_assignments (work_item_id,assignment_type,role_name,created_by)
SELECT wm.work_item_id,'role',role_label,'10000000-0000-0000-0000-000000000002'
FROM golden_rev3_work_map wm
CROSS JOIN LATERAL jsonb_array_elements_text(wm.required_role_ids) role(role_label);

INSERT INTO work_item_activities (
  work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,to_status,comment,metadata_json
)
SELECT work_item_id,'30000000-0000-0000-0000-000000000023','work_item',work_item_id,
       '10000000-0000-0000-0000-000000000002','created',
       CASE WHEN node_order=1 THEN 'ready' ELSE 'pending' END,
       'Đầu việc Golden Rev3 được tạo từ official runtime; chưa thực hiện.',
       jsonb_build_object('source','golden_rev3_start','business_code',business_code)
FROM golden_rev3_work_map;

INSERT INTO workflow_form_instances (
  project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,template_step_id,
  form_template_id,form_version_id,form_type,title,status,current_revision,
  answers_snapshot_json,required_role_ids,created_by
)
SELECT '30000000-0000-0000-0000-000000000023',wm.work_item_id,
       '42000000-0000-0000-0000-000000000023',wm.run_node_id,wm.run_step_id,
       wm.template_step_id,v.template_id,v.id,t.form_type,wm.name,'draft',0,'{}'::jsonb,
       wm.required_role_ids,'10000000-0000-0000-0000-000000000002'
FROM golden_rev3_work_map wm
JOIN workflow_form_versions v ON v.id=wm.form_version_id
JOIN workflow_form_templates t ON t.id=v.template_id
WHERE wm.form_required;

INSERT INTO workflow_form_events (
  instance_id,project_id,work_item_id,actor_id,event_type,to_status,comment,metadata_json
)
SELECT i.id,i.project_id,i.work_item_id,'10000000-0000-0000-0000-000000000002',
       'created','draft','Biểu mẫu Golden Rev3 được tạo khi bắt đầu runtime.',
       '{"source":"golden_rev3_start","untouched":true}'::jsonb
FROM workflow_form_instances i
WHERE i.pipeline_run_id='42000000-0000-0000-0000-000000000023';

UPDATE pipeline_runs run
SET current_node_id=map.run_node_id
FROM golden_rev3_node_map map
WHERE run.id='42000000-0000-0000-0000-000000000023' AND map.order_index=1;

UPDATE projects
SET current_pipeline_run_id='42000000-0000-0000-0000-000000000023',updated_at=now()
WHERE id='30000000-0000-0000-0000-000000000023';

INSERT INTO audit_logs (
  project_id,actor_id,actor_name,event_type,target_type,target_id,description,metadata_json
) VALUES (
  '30000000-0000-0000-0000-000000000023',
  '10000000-0000-0000-0000-000000000002','Nguyễn Minh PM',
  'golden_rev3_started','project','30000000-0000-0000-0000-000000000023',
  'Khởi tạo Golden Rev3 ở nhánh Chủ đầu tư có BOQ; project 022 không bị thay đổi.',
  '{"template_id":"40000000-0000-0000-0000-000000000022","workflow_entry_mode":"client_boq"}'::jsonb
);

COMMIT;
