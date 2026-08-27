-- Replace duplicate project-accounting capture with an operational handoff log.
-- Existing project_material_costs/project_cash_requests and Rev8 runtimes remain
-- immutable compatibility sources. New projects adopt official workflow Rev9.

CREATE TABLE IF NOT EXISTS accounting_handoffs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  direction TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  source_snapshot_json JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'handed_off',
  note TEXT,
  external_reference TEXT,
  return_reason TEXT,
  handed_off_by UUID NOT NULL REFERENCES users(id),
  handed_off_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  acknowledged_by UUID REFERENCES users(id),
  acknowledged_at TIMESTAMPTZ,
  returned_by UUID REFERENCES users(id),
  returned_at TIMESTAMPTZ,
  cancelled_by UUID REFERENCES users(id),
  cancelled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT accounting_handoffs_source_unique UNIQUE (project_id,source_type,source_id),
  CONSTRAINT accounting_handoffs_direction_check CHECK (direction IN ('payable','receivable')),
  CONSTRAINT accounting_handoffs_source_type_check CHECK (source_type IN ('procurement_request','contract_milestone')),
  CONSTRAINT accounting_handoffs_status_check CHECK (status IN ('handed_off','acknowledged','returned','cancelled')),
  CONSTRAINT accounting_handoffs_snapshot_object_check CHECK (jsonb_typeof(source_snapshot_json)='object'),
  CONSTRAINT accounting_handoffs_acknowledged_check CHECK (
    status <> 'acknowledged' OR (acknowledged_by IS NOT NULL AND acknowledged_at IS NOT NULL)
  ),
  CONSTRAINT accounting_handoffs_returned_check CHECK (
    status <> 'returned' OR (
      returned_by IS NOT NULL AND returned_at IS NOT NULL AND NULLIF(btrim(return_reason),'') IS NOT NULL
    )
  ),
  CONSTRAINT accounting_handoffs_cancelled_check CHECK (
    status <> 'cancelled' OR (cancelled_by IS NOT NULL AND cancelled_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_accounting_handoffs_project_status
  ON accounting_handoffs(project_id,status,updated_at DESC,id);

INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,'accounting_handoff'
FROM (VALUES
  ('accounting_handoff.view','Xem bàn giao kế toán','Xem readiness và lịch sử bàn giao hồ sơ nguồn của công trình'),
  ('accounting_handoff.submit','Bàn giao hồ sơ kế toán','Bàn giao hoặc bàn giao lại hồ sơ nghiệp vụ đủ điều kiện cho Kế toán'),
  ('accounting_handoff.acknowledge','Xác nhận hồ sơ kế toán','Kế toán xác nhận đã tiếp nhận hoặc trả lại hồ sơ cần bổ sung')
) permission_seed(key,name,description)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON
  (role.name IN ('admin','developer') AND permission.key LIKE 'accounting_handoff.%') OR
  (role.name='accounting' AND permission.key IN (
    'accounting_handoff.view','accounting_handoff.submit','accounting_handoff.acknowledge'
  )) OR
  (role.name IN ('pm','procurement') AND permission.key IN (
    'accounting_handoff.view','accounting_handoff.submit'
  )) OR
  (role.name IN ('director','qs') AND permission.key='accounting_handoff.view')
ON CONFLICT DO NOTHING;

UPDATE roles
SET description='Tiếp nhận hồ sơ công trình và xử lý nghiệp vụ kế toán trên MISA'
WHERE name='accounting';

-- Publish Rev9 by cloning Rev8. Only the final finance work package changes;
-- the graph, forms, approvals and every historical runtime remain untouched.
INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by,report_config_json
)
SELECT
  '40000000-0000-0000-0000-000000000028',source.name,
  'Revision 9: giữ nguyên 15 node và 21 gói công việc của Rev8; bước cuối chỉ xác nhận bàn giao hồ sơ nghiệp vụ cho Kế toán, không khóa sổ hay tính toán thay MISA.',
  source.project_type,9,'published',source.created_by,source.template_family_id,9,source.id,
  source.engine_version,
  encode(digest('official-client-workflow-rev9-accounting-handoff','sha256'),'hex'),
  now(),NULL,
  source.report_config_json || jsonb_build_object(
    'source_revision',8,
    'accounting_handoff_only',true,
    'misa_system_of_record',true,
    'project_finance_calculation',false
  )
FROM pipeline_templates source
WHERE source.id='40000000-0000-0000-0000-000000000027'
  AND source.status='published'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  replace(source.id::text,'48400000','49400000')::uuid,
  '40000000-0000-0000-0000-000000000028',
  source.business_code,source.name,
  CASE WHEN source.business_code='handover_finance'
    THEN 'Bàn giao vật lý được hoàn tất độc lập; hồ sơ thương mại đủ điều kiện được bàn giao và Kế toán xác nhận tiếp nhận.'
    ELSE source.description END,
  source.type,source.position_x,source.position_y,source.order_index,
  source.responsible_role_ids,source.required_permission,source.permissions,
  source.entry_conditions,
  CASE WHEN source.business_code='handover_finance'
    THEN '{"handover_included":true,"accounting_handoff_included":true,"separate_gates":true,"text":"Bàn giao vật lý và bàn giao hồ sơ kế toán đã hoàn tất"}'::jsonb
    ELSE source.exit_conditions END,
  source.approval_rule,source.approval_policy_json,source.rework_enabled,
  source.replanning_enabled,source.fail_path,source.status
FROM pipeline_template_nodes source
WHERE source.template_id='40000000-0000-0000-0000-000000000027'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  replace(source.id::text,'48500000','49500000')::uuid,
  replace(source.node_id::text,'48400000','49400000')::uuid,
  source.business_code,
  CASE WHEN source.business_code='closure.finance' THEN 'Xác nhận bàn giao hồ sơ kế toán' ELSE source.name END,
  CASE WHEN source.business_code='closure.finance'
    THEN 'Kế toán xác nhận đã tiếp nhận toàn bộ hồ sơ mua sắm và mốc thu đủ điều kiện; hạch toán, hóa đơn, thuế và thanh toán tiếp tục thực hiện trên MISA.'
    ELSE source.description END,
  source.step_type,source.required_role_ids,
  CASE WHEN source.business_code='closure.finance' THEN 'accounting_handoff.acknowledge' ELSE source.required_permission END,
  source.checklist_required,source.document_required,source.photo_required,
  source.form_version_id,source.form_required,source.surface_kind,
  CASE WHEN source.business_code='closure.finance' THEN 'source.accounting_handoff_close_ready' ELSE source.domain_action_key END,
  source.order_index
FROM pipeline_template_steps source
JOIN pipeline_template_nodes node ON node.id=source.node_id
WHERE node.template_id='40000000-0000-0000-0000-000000000027'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_edges (
  id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority
)
SELECT
  replace(source.id::text,'48600000','49600000')::uuid,
  '40000000-0000-0000-0000-000000000028',
  replace(source.from_node_id::text,'48400000','49400000')::uuid,
  replace(source.to_node_id::text,'48400000','49400000')::uuid,
  source.condition_type,source.condition_config,source.label,source.priority
FROM pipeline_template_edges source
WHERE source.template_id='40000000-0000-0000-0000-000000000027'
ON CONFLICT (id) DO NOTHING;

-- Only not-yet-started projects that selected Rev8 adopt Rev9.
UPDATE projects project
SET pipeline_template_id='40000000-0000-0000-0000-000000000028',updated_at=now()
WHERE project.pipeline_template_id='40000000-0000-0000-0000-000000000027'
  AND NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=project.id)
  AND NOT EXISTS (SELECT 1 FROM project_material_costs cost WHERE cost.project_id=project.id AND cost.status <> 'cancelled')
  AND NOT EXISTS (SELECT 1 FROM project_cash_requests request WHERE request.project_id=project.id AND request.status <> 'cancelled');

DO $$
DECLARE
  actual integer;
BEGIN
  SELECT count(*) INTO actual FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000028';
  IF actual<>15 THEN RAISE EXCEPTION 'expected Rev9 to contain 15 nodes, found %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000028';
  IF actual<>21 THEN RAISE EXCEPTION 'expected Rev9 to contain 21 steps, found %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000028'
    AND step.business_code='closure.finance'
    AND step.domain_action_key='source.accounting_handoff_close_ready'
    AND step.required_permission='accounting_handoff.acknowledge';
  IF actual<>1 THEN RAISE EXCEPTION 'expected Rev9 to contain one accounting handoff closure step, found %',actual; END IF;
END $$;
