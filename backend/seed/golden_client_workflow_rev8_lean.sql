\set ON_ERROR_STOP on

-- Additive runtime fixture for the 21-work-package Rev8 workflow. Reapplying
-- this file preserves any review data already entered in the fixture.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pipeline_templates
    WHERE id='40000000-0000-0000-0000-000000000027'
      AND revision_no=8 AND status='published' AND engine_version=4
  ) THEN
    RAISE EXCEPTION 'Published Rev8 engine-4 template is required before seeding the lean fixture';
  END IF;
  IF EXISTS (
    SELECT 1 FROM projects
    WHERE (id='30000000-0000-0000-0000-000000000028' OR code='GOLDEN-REV8-LEAN-001')
      AND NOT (id='30000000-0000-0000-0000-000000000028' AND code='GOLDEN-REV8-LEAN-001')
  ) THEN
    RAISE EXCEPTION 'Golden Rev8 fixture project ID or code is already used';
  END IF;
  IF EXISTS (
    SELECT 1 FROM pipeline_runs
    WHERE id='42000000-0000-0000-0000-000000000028'
      AND project_id<>'30000000-0000-0000-0000-000000000028'
  ) THEN
    RAISE EXCEPTION 'Golden Rev8 fixture run ID is already used';
  END IF;
END $$;

SELECT NOT EXISTS (
  SELECT 1 FROM projects
  WHERE id='30000000-0000-0000-0000-000000000028' AND code='GOLDEN-REV8-LEAN-001'
) AS golden_rev8_should_seed \gset

\if :golden_rev8_should_seed
BEGIN;

INSERT INTO projects (
  id,name,code,partner_id,client_name,address,planned_start_date,planned_end_date,
  estimated_budget,contract_amount,status,pipeline_template_id,pm_staff_id,
  progress_percent,workflow_entry_mode
)
SELECT
  '30000000-0000-0000-0000-000000000028',
  'Golden Rev8 – 21 gói công việc',
  'GOLDEN-REV8-LEAN-001',
  '90000000-0000-0000-0000-000000000001',
  'Công ty An Phát',
  'Fixture Rev8 tại node Khảo sát · TP.HCM',
  '2026-08-18','2027-03-31',3600000000,5200000000,'planning',template.id,
  '21000000-0000-0000-0000-000000000002',5,'survey_required'
FROM pipeline_templates template
WHERE template.id='40000000-0000-0000-0000-000000000027' AND template.status='published';

INSERT INTO project_members (project_id,staff_id,role_ids,assigned_nodes)
SELECT
  '30000000-0000-0000-0000-000000000028',staff.id,
  jsonb_build_array(role.display_name),'[]'::jsonb
FROM staff
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
SELECT
  '42000000-0000-0000-0000-000000000028',
  '30000000-0000-0000-0000-000000000028',template.id,'in_progress',now()-interval '10 minutes',
  template.engine_version,template.definition_hash
FROM pipeline_templates template
WHERE template.id='40000000-0000-0000-0000-000000000027' AND template.status='published';

CREATE TEMP TABLE golden_rev8_node_map ON COMMIT DROP AS
SELECT
  node.id AS template_node_id,gen_random_uuid() AS run_node_id,node.order_index,node.business_code,
  CASE WHEN node.business_code='project_intake' THEN 'completed'
       WHEN node.business_code='survey' THEN 'in_progress' ELSE 'pending' END AS runtime_status
FROM pipeline_template_nodes node
WHERE node.template_id='40000000-0000-0000-0000-000000000027';

INSERT INTO pipeline_run_nodes (
  id,run_id,project_id,template_node_id,business_code,name,description,type,order_index,
  responsible_role_ids,required_permission,permissions,exit_conditions,approval_rule,
  approval_policy_json,fail_path,status,started_at,completed_at
)
SELECT
  map.run_node_id,'42000000-0000-0000-0000-000000000028',
  '30000000-0000-0000-0000-000000000028',node.id,node.business_code,node.name,node.description,
  node.type,node.order_index,node.responsible_role_ids,node.required_permission,node.permissions,
  node.exit_conditions,node.approval_rule,node.approval_policy_json,node.fail_path,map.runtime_status,
  CASE WHEN node.business_code='project_intake' THEN now()-interval '10 minutes'
       WHEN node.business_code='survey' THEN now()-interval '5 minutes' END,
  CASE WHEN node.business_code='project_intake' THEN now()-interval '5 minutes' END
FROM golden_rev8_node_map map
JOIN pipeline_template_nodes node ON node.id=map.template_node_id;

INSERT INTO pipeline_run_edges (
  run_id,project_id,template_edge_id,from_run_node_id,to_run_node_id,
  condition_type,condition_config,label,priority
)
SELECT
  '42000000-0000-0000-0000-000000000028','30000000-0000-0000-0000-000000000028',
  edge.id,source.run_node_id,target.run_node_id,edge.condition_type,edge.condition_config,edge.label,edge.priority
FROM pipeline_template_edges edge
JOIN golden_rev8_node_map source ON source.template_node_id=edge.from_node_id
JOIN golden_rev8_node_map target ON target.template_node_id=edge.to_node_id
WHERE edge.template_id='40000000-0000-0000-0000-000000000027';

CREATE TEMP TABLE golden_rev8_work_map ON COMMIT DROP AS
SELECT
  node_map.run_node_id,node_map.business_code AS node_business_code,
  step.id AS template_step_id,gen_random_uuid() AS run_step_id,gen_random_uuid() AS work_item_id,
  step.business_code,step.name,step.description,step.step_type,step.required_role_ids,
  step.required_permission,step.checklist_required,step.document_required,step.photo_required,
  step.form_required,step.form_version_id,step.surface_kind,step.domain_action_key,step.order_index,
  CASE WHEN step.business_code='project_intake.confirm' THEN 'completed' ELSE 'pending' END AS step_status,
  CASE WHEN step.business_code='project_intake.confirm' THEN 'completed'
       WHEN node_map.business_code='survey' THEN 'ready' ELSE 'pending' END AS work_status
FROM golden_rev8_node_map node_map
JOIN pipeline_template_steps step ON step.node_id=node_map.template_node_id;

INSERT INTO pipeline_run_steps (
  id,run_node_id,project_id,template_step_id,business_code,name,description,step_type,
  required_role_ids,required_permission,checklist_required,document_required,photo_required,
  form_required,form_version_id,surface_kind,domain_action_key,order_index,status,submitted_by,submitted_at
)
SELECT
  run_step_id,run_node_id,'30000000-0000-0000-0000-000000000028',template_step_id,business_code,
  name,description,step_type,required_role_ids,required_permission,checklist_required,document_required,
  photo_required,form_required,form_version_id,surface_kind,domain_action_key,order_index,step_status,
  CASE WHEN step_status='completed' THEN '10000000-0000-0000-0000-000000000001'::uuid END,
  CASE WHEN step_status='completed' THEN now()-interval '5 minutes' END
FROM golden_rev8_work_map;

INSERT INTO work_items (
  id,project_id,pipeline_run_id,run_node_id,run_step_id,source_type,source_id,title,description,
  work_type,status,priority,role_ids,evidence_requirement_json,started_at,completed_at,created_by,updated_by
)
SELECT
  work_item_id,'30000000-0000-0000-0000-000000000028','42000000-0000-0000-0000-000000000028',
  run_node_id,run_step_id,'pipeline_run_step',run_step_id,name,description,
  CASE WHEN step_type='meeting_minutes' THEN 'checklist' ELSE step_type END,
  work_status,'normal',required_role_ids,
  jsonb_build_object(
    'checklist_required',checklist_required,'document_required',document_required,
    'photo_required',photo_required,'form_required',form_required,
    'form_version_id',form_version_id,'required_permission',required_permission
  ),
  CASE WHEN work_status='completed' THEN now()-interval '10 minutes' END,
  CASE WHEN work_status='completed' THEN now()-interval '5 minutes' END,
  '10000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001'
FROM golden_rev8_work_map;

INSERT INTO work_item_assignments (work_item_id,assignment_type,role_name,created_by)
SELECT map.work_item_id,'role',role_label,'10000000-0000-0000-0000-000000000001'
FROM golden_rev8_work_map map
CROSS JOIN LATERAL jsonb_array_elements_text(map.required_role_ids) role(role_label);

INSERT INTO work_item_activities (
  work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,to_status,
  comment,metadata_json,created_at
)
SELECT
  work_item_id,'30000000-0000-0000-0000-000000000028','work_item',work_item_id,
  '10000000-0000-0000-0000-000000000001','created',work_status,
  'Đầu việc Golden Rev8 được tạo từ template 21 gói công việc.',
  jsonb_build_object('source','golden_rev8_lean','business_code',business_code),now()-interval '10 minutes'
FROM golden_rev8_work_map;

INSERT INTO workflow_form_instances (
  project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,template_step_id,
  form_template_id,form_version_id,form_type,title,status,current_revision,
  answers_snapshot_json,required_role_ids,created_by,submitted_by,submitted_at
)
SELECT
  '30000000-0000-0000-0000-000000000028',map.work_item_id,
  '42000000-0000-0000-0000-000000000028',map.run_node_id,map.run_step_id,map.template_step_id,
  version.template_id,version.id,template.form_type,map.name,
  CASE WHEN map.business_code='project_intake.confirm' THEN 'submitted' ELSE 'draft' END,
  CASE WHEN map.business_code='project_intake.confirm' THEN 1 ELSE 0 END,
  CASE WHEN map.business_code='project_intake.confirm' THEN '{"entry_route":"survey_required"}'::jsonb ELSE '{}'::jsonb END,
  map.required_role_ids,'10000000-0000-0000-0000-000000000001',
  CASE WHEN map.business_code='project_intake.confirm' THEN '10000000-0000-0000-0000-000000000001'::uuid END,
  CASE WHEN map.business_code='project_intake.confirm' THEN now()-interval '5 minutes' END
FROM golden_rev8_work_map map
JOIN workflow_form_versions version ON version.id=map.form_version_id
JOIN workflow_form_templates template ON template.id=version.template_id
WHERE map.form_required;

INSERT INTO workflow_form_answers (
  instance_id,field_key,field_type,value_json,revision_no,updated_by,updated_at
)
SELECT instance.id,'entry_route','select','"survey_required"'::jsonb,1,
  '10000000-0000-0000-0000-000000000001',now()-interval '5 minutes'
FROM workflow_form_instances instance
JOIN pipeline_run_steps step ON step.id=instance.run_step_id
WHERE instance.pipeline_run_id='42000000-0000-0000-0000-000000000028'
  AND step.business_code='project_intake.confirm';

INSERT INTO workflow_form_events (
  instance_id,project_id,work_item_id,actor_id,event_type,from_status,to_status,
  comment,metadata_json,created_at
)
SELECT
  instance.id,instance.project_id,instance.work_item_id,'10000000-0000-0000-0000-000000000001',
  'created',NULL,'draft','Biểu mẫu Golden Rev8 được tạo khi khởi động runtime.',
  '{"source":"golden_rev8_lean"}'::jsonb,now()-interval '10 minutes'
FROM workflow_form_instances instance
WHERE instance.pipeline_run_id='42000000-0000-0000-0000-000000000028';

UPDATE pipeline_runs run
SET current_node_id=map.run_node_id
FROM golden_rev8_node_map map
WHERE run.id='42000000-0000-0000-0000-000000000028' AND map.business_code='survey';

UPDATE projects
SET current_pipeline_run_id='42000000-0000-0000-0000-000000000028',updated_at=now()
WHERE id='30000000-0000-0000-0000-000000000028';

INSERT INTO audit_logs (
  project_id,actor_id,actor_name,event_type,target_type,target_id,description,metadata_json,created_at
)
VALUES (
  '30000000-0000-0000-0000-000000000028','10000000-0000-0000-0000-000000000001',
  'Admin hệ thống','golden_rev8_started','pipeline_run','42000000-0000-0000-0000-000000000028',
  'Khởi tạo Golden Rev8 với 21 gói công việc và hai step khảo sát đang sẵn sàng.',
  '{"template_id":"40000000-0000-0000-0000-000000000027","revision_no":8}'::jsonb,
  now()-interval '10 minutes'
);

COMMIT;
\else
\echo 'GOLDEN-REV8-LEAN-001 already exists; preserving the current fixture state.'
\endif

DO $$
DECLARE
  actual integer;
BEGIN
  SELECT count(*) INTO actual
  FROM projects project
  JOIN pipeline_runs run ON run.id=project.current_pipeline_run_id
  WHERE project.id='30000000-0000-0000-0000-000000000028'
    AND project.code='GOLDEN-REV8-LEAN-001'
    AND project.pipeline_template_id='40000000-0000-0000-0000-000000000027'
    AND run.id='42000000-0000-0000-0000-000000000028'
    AND run.template_id='40000000-0000-0000-0000-000000000027'
    AND run.engine_version=4;
  IF actual<>1 THEN RAISE EXCEPTION 'Golden Rev8 project/runtime contract is invalid'; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000028';
  IF actual<>15 THEN RAISE EXCEPTION 'Golden Rev8 expected 15 runtime nodes, found %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000028';
  IF actual<>21 THEN RAISE EXCEPTION 'Golden Rev8 expected 21 runtime steps, found %',actual; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id='30000000-0000-0000-0000-000000000028';
  IF actual<>15 THEN RAISE EXCEPTION 'Golden Rev8 expected 15 form instances, found %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000028'
    AND form_version_id IS NULL AND domain_action_key IS NOT NULL;
  IF actual<>6 THEN RAISE EXCEPTION 'Golden Rev8 expected 6 source/action-only steps, found %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000028'
    AND jsonb_array_length(required_role_ids)<>1;
  IF actual<>0 THEN RAISE EXCEPTION 'Golden Rev8 has % steps without exactly one primary role',actual; END IF;
END $$;

SELECT 'GOLDEN_REV8_LEAN_FIXTURE_READY' AS result;
