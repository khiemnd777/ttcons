\set ON_ERROR_STOP on

BEGIN;

-- Rev4-only Browser UAT fixtures. Project 024 is a true entry-state runtime.
-- Project 025 is an explicit synthetic construction-stage snapshot for Operations UX;
-- it does not claim that earlier workflow nodes were completed through production gates.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pipeline_templates
    WHERE id='40000000-0000-0000-0000-000000000023'
      AND status='published' AND engine_version=3
  ) THEN
    RAISE EXCEPTION 'Published Rev4 engine-3 template is required before seeding UAT fixtures';
  END IF;
  IF EXISTS (
    SELECT 1 FROM projects
    WHERE id IN (
      '30000000-0000-0000-0000-000000000024',
      '30000000-0000-0000-0000-000000000025'
    ) OR code IN ('GOLDEN-REV4-ENTRY-001','GOLDEN-REV4-CONSTRUCTION-001')
  ) THEN
    RAISE EXCEPTION 'Rev4 UAT fixture IDs or codes are already in use';
  END IF;
END $$;

INSERT INTO projects (
  id,name,code,partner_id,client_name,address,planned_start_date,planned_end_date,
  estimated_budget,contract_amount,status,pipeline_template_id,pm_staff_id,
  progress_percent,workflow_entry_mode
)
SELECT fixture.id,fixture.name,fixture.code,
       '90000000-0000-0000-0000-000000000001','Công ty An Phát',fixture.address,
       '2026-08-05','2027-02-28',3200000000,4800000000,'planning',template.id,
       '21000000-0000-0000-0000-000000000002',0,'survey_required'
FROM pipeline_templates template
CROSS JOIN (VALUES
  ('30000000-0000-0000-0000-000000000024'::uuid,
   'Golden Rev4 – Kiểm tra khóa trước Thi công','GOLDEN-REV4-ENTRY-001',
   'Fixture Rev4 tại node đầu · TP.HCM'),
  ('30000000-0000-0000-0000-000000000025'::uuid,
   'Golden Rev4 – UAT vận hành Thi công','GOLDEN-REV4-CONSTRUCTION-001',
   'Fixture Rev4 tổng hợp tại Node Thi công · TP.HCM')
) AS fixture(id,name,code,address)
WHERE template.id='40000000-0000-0000-0000-000000000023'
  AND template.status='published';

INSERT INTO project_members (project_id,staff_id,role_ids,assigned_nodes)
SELECT fixture.project_id,staff.id,jsonb_build_array(role.display_name),'[]'::jsonb
FROM (VALUES
  ('30000000-0000-0000-0000-000000000024'::uuid),
  ('30000000-0000-0000-0000-000000000025'::uuid)
) AS fixture(project_id)
CROSS JOIN staff
JOIN users actor ON actor.id=staff.user_id
JOIN user_roles actor_role ON actor_role.user_id=actor.id
JOIN roles role ON role.id=actor_role.role_id
WHERE actor.email IN (
  'admin@consflow.local','pm@consflow.local','qs@consflow.local',
  'accounting@consflow.local','supervisor@consflow.local','director@consflow.local',
  'procurement@consflow.local','qaqc@consflow.local','design@consflow.local',
  'sales@consflow.local','legal@consflow.local','contract@consflow.local'
);

INSERT INTO pipeline_runs (
  id,project_id,template_id,status,started_at,engine_version,template_definition_hash
)
SELECT fixture.run_id,fixture.project_id,template.id,'in_progress',now(),
       template.engine_version,template.definition_hash
FROM pipeline_templates template
CROSS JOIN (VALUES
  ('42000000-0000-0000-0000-000000000024'::uuid,
   '30000000-0000-0000-0000-000000000024'::uuid),
  ('42000000-0000-0000-0000-000000000025'::uuid,
   '30000000-0000-0000-0000-000000000025'::uuid)
) AS fixture(run_id,project_id)
WHERE template.id='40000000-0000-0000-0000-000000000023'
  AND template.status='published';

CREATE TEMP TABLE golden_rev4_uat_node_map ON COMMIT DROP AS
SELECT fixture.project_id,fixture.run_id,fixture.stage,
       node.id AS template_node_id,gen_random_uuid() AS run_node_id,
       node.order_index,node.business_code,
       CASE
         WHEN fixture.stage='entry' AND node.business_code='project_intake' THEN 'in_progress'
         WHEN fixture.stage='construction' AND node.business_code='construction_execution' THEN 'in_progress'
         ELSE 'pending'
       END AS runtime_status
FROM pipeline_template_nodes node
CROSS JOIN (VALUES
  ('30000000-0000-0000-0000-000000000024'::uuid,
   '42000000-0000-0000-0000-000000000024'::uuid,'entry'),
  ('30000000-0000-0000-0000-000000000025'::uuid,
   '42000000-0000-0000-0000-000000000025'::uuid,'construction')
) AS fixture(project_id,run_id,stage)
WHERE node.template_id='40000000-0000-0000-0000-000000000023';

INSERT INTO pipeline_run_nodes (
  id,run_id,project_id,template_node_id,business_code,name,description,type,order_index,
  responsible_role_ids,required_permission,permissions,exit_conditions,approval_rule,
  approval_policy_json,fail_path,status,started_at
)
SELECT map.run_node_id,map.run_id,map.project_id,node.id,node.business_code,node.name,
       node.description,node.type,node.order_index,node.responsible_role_ids,
       node.required_permission,node.permissions,node.exit_conditions,node.approval_rule,
       node.approval_policy_json,node.fail_path,map.runtime_status,
       CASE WHEN map.runtime_status='in_progress' THEN now() END
FROM golden_rev4_uat_node_map map
JOIN pipeline_template_nodes node ON node.id=map.template_node_id;

INSERT INTO pipeline_run_edges (
  run_id,project_id,template_edge_id,from_run_node_id,to_run_node_id,
  condition_type,condition_config,label,priority
)
SELECT source.run_id,source.project_id,edge.id,source.run_node_id,target.run_node_id,
       edge.condition_type,edge.condition_config,edge.label,edge.priority
FROM pipeline_template_edges edge
JOIN golden_rev4_uat_node_map source
  ON source.template_node_id=edge.from_node_id
JOIN golden_rev4_uat_node_map target
  ON target.template_node_id=edge.to_node_id
 AND target.project_id=source.project_id
WHERE edge.template_id='40000000-0000-0000-0000-000000000023';

CREATE TEMP TABLE golden_rev4_uat_step_map ON COMMIT DROP AS
SELECT node_map.project_id,node_map.run_id,node_map.stage,node_map.run_node_id,
       node_map.business_code AS node_business_code,node_map.order_index AS node_order,
       step.id AS template_step_id,gen_random_uuid() AS run_step_id,step.business_code,
       step.name,step.description,step.step_type,step.required_role_ids,
       step.required_permission,step.checklist_required,step.document_required,
       step.photo_required,step.form_required,step.form_version_id,step.surface_kind,
       step.domain_action_key,step.order_index
FROM golden_rev4_uat_node_map node_map
JOIN pipeline_template_steps step ON step.node_id=node_map.template_node_id;

INSERT INTO pipeline_run_steps (
  id,run_node_id,project_id,template_step_id,business_code,name,description,step_type,
  required_role_ids,required_permission,checklist_required,document_required,photo_required,
  form_required,form_version_id,surface_kind,domain_action_key,order_index,status
)
SELECT run_step_id,run_node_id,project_id,template_step_id,business_code,name,description,
       step_type,required_role_ids,required_permission,checklist_required,document_required,
       photo_required,form_required,form_version_id,surface_kind,domain_action_key,
       order_index,'pending'
FROM golden_rev4_uat_step_map;

CREATE TEMP TABLE golden_rev4_uat_work_map ON COMMIT DROP AS
SELECT step_map.*,gen_random_uuid() AS work_item_id,
       CASE
         WHEN stage='entry' AND node_business_code='project_intake' THEN 'ready'
         WHEN stage='construction' AND business_code='construction.item_reporting' THEN 'ready'
         ELSE 'pending'
       END AS work_status
FROM golden_rev4_uat_step_map step_map;

INSERT INTO work_items (
  id,project_id,pipeline_run_id,run_node_id,run_step_id,source_type,source_id,
  title,description,work_type,status,priority,role_ids,evidence_requirement_json,
  created_by,updated_by
)
SELECT work_item_id,project_id,run_id,run_node_id,run_step_id,
       'pipeline_run_step',run_step_id,name,description,
       CASE WHEN step_type='meeting_minutes' THEN 'checklist' ELSE step_type END,
       work_status,'normal',required_role_ids,
       jsonb_build_object(
         'checklist_required',checklist_required,'document_required',document_required,
         'photo_required',photo_required,'form_required',form_required,
         'form_version_id',form_version_id,'required_permission',required_permission
       ),
       '10000000-0000-0000-0000-000000000001',
       '10000000-0000-0000-0000-000000000001'
FROM golden_rev4_uat_work_map;

INSERT INTO work_item_assignments (work_item_id,assignment_type,role_name,created_by)
SELECT work_map.work_item_id,'role',role_label,
       '10000000-0000-0000-0000-000000000001'
FROM golden_rev4_uat_work_map work_map
CROSS JOIN LATERAL jsonb_array_elements_text(work_map.required_role_ids) role(role_label);

INSERT INTO work_item_activities (
  work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,to_status,
  comment,metadata_json
)
SELECT work_item_id,project_id,'work_item',work_item_id,
       '10000000-0000-0000-0000-000000000001','created',work_status,
       CASE WHEN stage='construction'
         THEN 'Đầu việc Rev4 được tạo cho fixture UAT tổng hợp tại Node Thi công.'
         ELSE 'Đầu việc Rev4 được tạo từ runtime tại node đầu.'
       END,
       jsonb_build_object('source','golden_rev4_uat','business_code',business_code,
                          'synthetic_stage',stage='construction')
FROM golden_rev4_uat_work_map;

INSERT INTO workflow_form_instances (
  project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,template_step_id,
  form_template_id,form_version_id,form_type,title,status,current_revision,
  answers_snapshot_json,required_role_ids,created_by
)
SELECT work_map.project_id,work_map.work_item_id,work_map.run_id,work_map.run_node_id,
       work_map.run_step_id,work_map.template_step_id,version.template_id,version.id,
       template.form_type,work_map.name,'draft',0,'{}'::jsonb,
       work_map.required_role_ids,'10000000-0000-0000-0000-000000000001'
FROM golden_rev4_uat_work_map work_map
JOIN workflow_form_versions version ON version.id=work_map.form_version_id
JOIN workflow_form_templates template ON template.id=version.template_id
WHERE work_map.form_required;

INSERT INTO workflow_form_events (
  instance_id,project_id,work_item_id,actor_id,event_type,to_status,comment,metadata_json
)
SELECT instance.id,instance.project_id,instance.work_item_id,
       '10000000-0000-0000-0000-000000000001','created','draft',
       'Biểu mẫu Rev4 được tạo cho fixture UAT.',
       jsonb_build_object('source','golden_rev4_uat',
                          'synthetic_stage',instance.project_id='30000000-0000-0000-0000-000000000025')
FROM workflow_form_instances instance
WHERE instance.pipeline_run_id IN (
  '42000000-0000-0000-0000-000000000024',
  '42000000-0000-0000-0000-000000000025'
);

UPDATE pipeline_runs run
SET current_node_id=node_map.run_node_id
FROM golden_rev4_uat_node_map node_map
WHERE run.id=node_map.run_id AND node_map.runtime_status='in_progress';

UPDATE projects project
SET current_pipeline_run_id=fixture.run_id,updated_at=now()
FROM (VALUES
  ('30000000-0000-0000-0000-000000000024'::uuid,
   '42000000-0000-0000-0000-000000000024'::uuid),
  ('30000000-0000-0000-0000-000000000025'::uuid,
   '42000000-0000-0000-0000-000000000025'::uuid)
) AS fixture(project_id,run_id)
WHERE project.id=fixture.project_id;

INSERT INTO contracts (
  id,project_id,contract_code,title,contract_type,direction,counterparty_name,
  currency,original_amount,current_amount,status,created_by,activated_by,activated_at
) VALUES (
  '47000000-0000-0000-0000-000000000025',
  '30000000-0000-0000-0000-000000000025',
  'HD-REV4-UAT-001','Hợp đồng chủ đầu tư · Fixture Rev4','main_client','receivable',
  'Công ty An Phát','VND',4800000000,4800000000,'active',
  '10000000-0000-0000-0000-000000000001',
  '10000000-0000-0000-0000-000000000001',now()
);

INSERT INTO audit_logs (
  project_id,actor_id,actor_name,event_type,target_type,target_id,description,metadata_json
) VALUES
  (
    '30000000-0000-0000-0000-000000000024',
    '10000000-0000-0000-0000-000000000001','Admin hệ thống',
    'golden_rev4_uat_started','project','30000000-0000-0000-0000-000000000024',
    'Khởi tạo fixture Rev4 engine 3 tại node đầu để kiểm tra khóa trước Thi công.',
    '{"template_id":"40000000-0000-0000-0000-000000000023","engine_version":3,"synthetic_stage":false}'::jsonb
  ),
  (
    '30000000-0000-0000-0000-000000000025',
    '10000000-0000-0000-0000-000000000001','Admin hệ thống',
    'golden_rev4_uat_started','project','30000000-0000-0000-0000-000000000025',
    'Khởi tạo fixture Rev4 engine 3 tổng hợp tại Node Thi công để kiểm tra Operations UX.',
    '{"template_id":"40000000-0000-0000-0000-000000000023","engine_version":3,"synthetic_stage":true}'::jsonb
  );

COMMIT;
