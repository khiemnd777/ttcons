CREATE TABLE IF NOT EXISTS construction_handoffs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  pipeline_run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
  source_run_node_id UUID REFERENCES pipeline_run_nodes(id) ON DELETE SET NULL,
  target_run_node_id UUID NOT NULL REFERENCES pipeline_run_nodes(id) ON DELETE CASCADE,
  source_form_instance_id UUID REFERENCES workflow_form_instances(id) ON DELETE SET NULL,
  source_form_revision_no INTEGER NOT NULL DEFAULT 0,
  revision_no INTEGER NOT NULL DEFAULT 1,
  title TEXT NOT NULL,
  notes TEXT,
  status TEXT NOT NULL DEFAULT 'activated',
  source_snapshot_hash TEXT NOT NULL,
  source_snapshot_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  activated_by UUID REFERENCES users(id),
  activated_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT construction_handoff_revision_positive CHECK (revision_no > 0),
  CONSTRAINT construction_handoff_form_revision_nonnegative CHECK (source_form_revision_no >= 0),
  CONSTRAINT construction_handoff_status_check CHECK (status IN ('draft','activated','cancelled')),
  CONSTRAINT construction_handoff_snapshot_object_check CHECK (jsonb_typeof(source_snapshot_jsonb)='object'),
  CONSTRAINT construction_handoff_snapshot_unique UNIQUE (project_id,pipeline_run_id,source_snapshot_hash)
);

CREATE INDEX IF NOT EXISTS idx_construction_handoffs_project
  ON construction_handoffs(project_id,status,revision_no DESC,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_construction_handoffs_source_node
  ON construction_handoffs(source_run_node_id,status) WHERE source_run_node_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_construction_handoffs_target_node
  ON construction_handoffs(target_run_node_id,status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_construction_handoffs_form_revision
  ON construction_handoffs(project_id,pipeline_run_id,source_form_instance_id,source_form_revision_no)
  WHERE source_form_instance_id IS NOT NULL AND status <> 'cancelled';

CREATE TABLE IF NOT EXISTS construction_handoff_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  handoff_id UUID NOT NULL REFERENCES construction_handoffs(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  source_field_key TEXT NOT NULL,
  source_item_index INTEGER NOT NULL,
  title TEXT NOT NULL,
  location TEXT,
  criteria_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  scheduled_at TIMESTAMPTZ,
  responsible_role_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  assigned_user_id UUID REFERENCES users(id),
  evidence_required BOOLEAN NOT NULL DEFAULT false,
  source_snapshot_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb,
  generated_work_item_id UUID REFERENCES work_items(id) ON DELETE SET NULL,
  generated_check_id UUID REFERENCES construction_checks(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'activated',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT construction_handoff_item_source_index_nonnegative CHECK (source_item_index >= 0),
  CONSTRAINT construction_handoff_item_criteria_array_check CHECK (jsonb_typeof(criteria_jsonb)='array'),
  CONSTRAINT construction_handoff_item_roles_array_check CHECK (jsonb_typeof(responsible_role_ids)='array'),
  CONSTRAINT construction_handoff_item_snapshot_object_check CHECK (jsonb_typeof(source_snapshot_jsonb)='object'),
  CONSTRAINT construction_handoff_item_status_check CHECK (status IN ('activated','completed','cancelled')),
  CONSTRAINT construction_handoff_item_source_unique UNIQUE (handoff_id,source_field_key,source_item_index),
  CONSTRAINT construction_handoff_item_work_unique UNIQUE (generated_work_item_id),
  CONSTRAINT construction_handoff_item_check_unique UNIQUE (generated_check_id)
);

CREATE INDEX IF NOT EXISTS idx_construction_handoff_items_handoff
  ON construction_handoff_items(handoff_id,source_item_index);
CREATE INDEX IF NOT EXISTS idx_construction_handoff_items_project
  ON construction_handoff_items(project_id,status,updated_at DESC);

ALTER TABLE construction_checks
  ADD COLUMN IF NOT EXISTS source_handoff_item_id UUID REFERENCES construction_handoff_items(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_construction_checks_source_handoff_item
  ON construction_checks(source_handoff_item_id) WHERE source_handoff_item_id IS NOT NULL;

ALTER TABLE work_items
  DROP CONSTRAINT IF EXISTS work_items_source_type_check;

ALTER TABLE work_items
  ADD CONSTRAINT work_items_source_type_check CHECK (source_type IN (
    'pipeline_run_step',
    'boq_item',
    'procurement_request',
    'stock_movement',
    'daily_report',
    'issue',
    'acceptance',
    'rework_request',
    'replanning_request',
    'handover',
    'accounting_budget',
    'payable',
    'receivable',
    'payment',
    'workflow_form_action_item',
    'construction_handoff_item'
  ));

INSERT INTO permissions (id,key,name,description,module)
VALUES (
  gen_random_uuid(),
  'construction.handoff.manage',
  'Bàn giao kế hoạch sang thi công',
  'Rà soát và kích hoạt hàng loạt hạng mục giám sát từ kế hoạch thi công',
  'construction'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r
JOIN permissions p ON p.key='construction.handoff.manage'
WHERE r.name IN ('admin','pm','qa_qc')
ON CONFLICT DO NOTHING;

-- Dedicated planning form used by revision 2 of the official client workflow.
INSERT INTO workflow_form_templates (id,code,name,description,form_type,status)
VALUES (
  '80000000-0000-0000-0000-000000000066',
  'client_construction_handoff_plan',
  'Kế hoạch giám sát và bàn giao thi công',
  'Lập danh sách hạng mục cần giám sát một lần để bàn giao sang không gian thi công.',
  'checklist',
  'active'
)
ON CONFLICT (code) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,form_type=EXCLUDED.form_type,status='active',updated_at=now();

INSERT INTO workflow_form_versions (id,template_id,version_no,status,schema_json,validation_json,published_at)
SELECT
  '80000000-0000-0000-0000-000000000067',
  id,
  1,
  'published',
  '{"source":"official_client_workflow","purpose":"planning_to_construction_handoff"}'::jsonb,
  '{"creates_construction_handoff_from":"inspection_items"}'::jsonb,
  now()
FROM workflow_form_templates
WHERE code='client_construction_handoff_plan'
ON CONFLICT (template_id,version_no) DO UPDATE
SET status='published',schema_json=EXCLUDED.schema_json,validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at,EXCLUDED.published_at),updated_at=now();

WITH version_row AS (
  SELECT v.id
  FROM workflow_form_versions v
  JOIN workflow_form_templates t ON t.id=v.template_id
  WHERE t.code='client_construction_handoff_plan' AND v.version_no=1
), field_seed(field_key,label,field_type,required,options_json,validation_json,help_text,order_index) AS (
  VALUES
    ('handoff_title','Tên đợt bàn giao','text',true,'[]'::jsonb,'{"min_length":3,"max_length":180}'::jsonb,'Ví dụ: Đợt thi công hoàn thiện tầng 12.',10),
    (
      'inspection_items',
      'Hạng mục giám sát',
      'structured_list',
      true,
      '[]'::jsonb,
      '{
        "min_items":1,
        "max_items":200,
        "item_fields":[
          {"key":"title","label":"Nội dung kiểm tra","type":"text","required":true,"min_length":3,"max_length":180},
          {"key":"location","label":"Vị trí","type":"text","required":false,"max_length":180},
          {"key":"criteria","label":"Tiêu chí đạt","type":"textarea","required":true,"min_length":3,"max_length":1500},
          {"key":"scheduled_date","label":"Ngày dự kiến","type":"date","required":false},
          {"key":"responsible_role","label":"Vai trò giám sát","type":"select","required":true,"options":[{"value":"QA/QC","label":"QA/QC"},{"value":"Giám sát","label":"Giám sát"},{"value":"PM","label":"PM"}]},
          {"key":"evidence_required","label":"Bắt buộc minh chứng","type":"checkbox","required":false}
        ]
      }'::jsonb,
      'Mỗi dòng sẽ sinh đúng một đầu việc và một kiểm tra khi kích hoạt bàn giao.',
      20
    ),
    ('handoff_notes','Lưu ý bàn giao','textarea',false,'[]'::jsonb,'{"max_length":3000}'::jsonb,NULL,30),
    ('confirmed','Xác nhận kế hoạch sẵn sàng bàn giao','checkbox',true,'[]'::jsonb,'{}'::jsonb,'Chỉ kích hoạt khi danh sách đã được rà soát.',40)
)
INSERT INTO workflow_form_fields (form_version_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index)
SELECT v.id,f.field_key,f.label,f.field_type,f.required,f.options_json,f.validation_json,f.help_text,f.order_index
FROM version_row v CROSS JOIN field_seed f
ON CONFLICT (form_version_id,field_key) DO UPDATE
SET label=EXCLUDED.label,field_type=EXCLUDED.field_type,required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,order_index=EXCLUDED.order_index;

-- Publish a new immutable revision. Existing pipeline runs continue referencing revision 1.
INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by
)
SELECT
  '40000000-0000-0000-0000-000000000021',
  name,
  'Revision 2: tách Kế hoạch thi công khỏi Thi công và bàn giao hạng mục giám sát theo lô.',
  project_type,
  2,
  'published',
  created_by,
  '40000000-0000-0000-0000-000000000020',
  2,
  '40000000-0000-0000-0000-000000000020',
  2,
  NULL,
  now(),
  NULL
FROM pipeline_templates
WHERE id='40000000-0000-0000-0000-000000000020'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,name,description,type,position_x,position_y,order_index,responsible_role_ids,
  required_permission,permissions,entry_conditions,exit_conditions,approval_rule,approval_policy_json,
  rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  replace(id::text,'41400000','42400000')::uuid,
  '40000000-0000-0000-0000-000000000021',
  CASE WHEN id='41400000-0000-0000-0000-000000000009' THEN 'Kế hoạch thi công' ELSE name END,
  CASE WHEN id='41400000-0000-0000-0000-000000000009'
       THEN 'Chốt phân công, QA/QC và danh sách hạng mục giám sát trước khi bàn giao thi công.'
       ELSE description END,
  CASE WHEN id='41400000-0000-0000-0000-000000000009' THEN 'construction_planning' ELSE type END,
  position_x + CASE WHEN order_index > 9 THEN 280 ELSE 0 END,
  position_y,
  order_index + CASE WHEN order_index > 9 THEN 1 ELSE 0 END,
  responsible_role_ids,
  required_permission,
  permissions,
  entry_conditions,
  CASE WHEN id='41400000-0000-0000-0000-000000000009'
       THEN '{"handoff_required":true,"text":"Kế hoạch giám sát đã được kích hoạt và bàn giao"}'::jsonb
       ELSE exit_conditions END,
  CASE WHEN id='41400000-0000-0000-0000-000000000009' THEN 'PM duyệt bàn giao thi công' ELSE approval_rule END,
  approval_policy_json,
  rework_enabled,
  replanning_enabled,
  fail_path,
  status
FROM pipeline_template_nodes
WHERE template_id='40000000-0000-0000-0000-000000000020'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,name,description,type,position_x,position_y,order_index,responsible_role_ids,
  required_permission,permissions,entry_conditions,exit_conditions,approval_rule,approval_policy_json,
  rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  '42400000-0000-0000-0000-000000000013',
  '40000000-0000-0000-0000-000000000021',
  'Thi công',
  'Giám sát, kiểm tra, khắc phục và báo cáo hiện trường từ danh sách đã bàn giao.',
  'construction',
  2320,
  260,
  10,
  responsible_role_ids,
  required_permission,
  permissions,
  '{}'::jsonb,
  '{"handoff_checks_completed":true,"text":"Các kiểm tra và khắc phục từ kế hoạch đã hoàn tất"}'::jsonb,
  'PM/QA review thi công',
  approval_policy_json,
  true,
  true,
  'rework',
  'active'
FROM pipeline_template_nodes
WHERE id='41400000-0000-0000-0000-000000000009'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,name,description,step_type,required_role_ids,required_permission,checklist_required,
  document_required,photo_required,form_version_id,form_required,surface_kind,domain_action_key,order_index
)
SELECT
  replace(s.id::text,'41500000','42500000')::uuid,
  replace(s.node_id::text,'41400000','42400000')::uuid,
  s.name,s.description,s.step_type,s.required_role_ids,s.required_permission,s.checklist_required,
  s.document_required,s.photo_required,s.form_version_id,s.form_required,s.surface_kind,s.domain_action_key,s.order_index
FROM pipeline_template_steps s
JOIN pipeline_template_nodes n ON n.id=s.node_id
WHERE n.template_id='40000000-0000-0000-0000-000000000020'
  AND s.id <> '41500000-0000-0000-0000-000000000037'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,name,description,step_type,required_role_ids,required_permission,checklist_required,
  document_required,photo_required,form_version_id,form_required,surface_kind,domain_action_key,order_index
)
SELECT
  '42500000-0000-0000-0000-000000000060',
  '42400000-0000-0000-0000-000000000009',
  'Chốt hạng mục giám sát và bàn giao thi công',
  'Lập một lần danh sách kiểm tra sẽ được sinh tự động cho đội hiện trường.',
  'checklist',
  '["PM","QA/QC","Giám sát"]'::jsonb,
  'construction.handoff.manage',
  true,false,false,
  v.id,
  true,
  'workflow_form',
  'construction.activate_handoff',
  4
FROM workflow_form_versions v
JOIN workflow_form_templates t ON t.id=v.template_id
WHERE t.code='client_construction_handoff_plan' AND v.version_no=1
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,name,description,step_type,required_role_ids,required_permission,checklist_required,
  document_required,photo_required,form_version_id,form_required,surface_kind,domain_action_key,order_index
)
SELECT
  '42500000-0000-0000-0000-000000000037',
  '42400000-0000-0000-0000-000000000013',
  s.name,s.description,s.step_type,s.required_role_ids,s.required_permission,s.checklist_required,
  s.document_required,s.photo_required,s.form_version_id,s.form_required,s.surface_kind,s.domain_action_key,1
FROM pipeline_template_steps s
WHERE s.id='41500000-0000-0000-0000-000000000037'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_edges (id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority)
SELECT
  replace(e.id::text,'41600000','42600000')::uuid,
  '40000000-0000-0000-0000-000000000021',
  CASE WHEN e.from_node_id='41400000-0000-0000-0000-000000000009'
       THEN '42400000-0000-0000-0000-000000000013'::uuid
       ELSE replace(e.from_node_id::text,'41400000','42400000')::uuid END,
  replace(e.to_node_id::text,'41400000','42400000')::uuid,
  e.condition_type,e.condition_config,e.label,e.priority
FROM pipeline_template_edges e
WHERE e.template_id='40000000-0000-0000-0000-000000000020'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_edges (id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority)
VALUES (
  '42600000-0000-0000-0000-000000000060',
  '40000000-0000-0000-0000-000000000021',
  '42400000-0000-0000-0000-000000000009',
  '42400000-0000-0000-0000-000000000013',
  'always',
  '{}'::jsonb,
  'Bàn giao thi công',
  100
)
ON CONFLICT (id) DO NOTHING;

-- Future runs use revision 2; active and historical runs keep their immutable template snapshot.
UPDATE projects
SET pipeline_template_id='40000000-0000-0000-0000-000000000021',updated_at=now()
WHERE pipeline_template_id='40000000-0000-0000-0000-000000000020';
