\set ON_ERROR_STOP on

BEGIN;

-- Base demo seed intentionally owns the template rows, while migrations own the
-- product-grade form/surface contract. Reapply that current contract after the
-- base reset so a fresh V56 database does not fall back to generic forms.
CREATE TEMP TABLE official_step_binding (
  step_no INTEGER PRIMARY KEY,
  form_code TEXT,
  form_version_no INTEGER NOT NULL DEFAULT 1,
  surface_kind TEXT NOT NULL,
  domain_action_key TEXT
) ON COMMIT DROP;

INSERT INTO official_step_binding (step_no,form_code,form_version_no,surface_kind,domain_action_key) VALUES
(1,'client_survey_requirement_intake',1,'workflow_form',NULL),
(2,NULL,1,'evidence',NULL),
(3,'client_design_style_brief',1,'workflow_form',NULL),
(4,'client_site_measurement_record',1,'workflow_form',NULL),
(5,'client_site_risk_assessment',1,'workflow_form',NULL),
(6,'client_site_survey_minutes',1,'workflow_form',NULL),
(7,'client_boq_takeoff_summary',1,'workflow_form',NULL),
(8,'client_design_layout_submission',1,'workflow_form',NULL),
(9,'client_design_3d_submission',1,'workflow_form',NULL),
(10,'client_construction_drawing_submission',1,'workflow_form',NULL),
(11,'client_design_branch_decision',2,'workflow_form',NULL),
(12,'client_design_review_layout',1,'workflow_form',NULL),
(13,'client_design_review_3d',1,'workflow_form',NULL),
(14,'client_design_review_construction_drawing',1,'workflow_form',NULL),
(15,'client_design_owner_approval',1,'approval',NULL),
(16,'client_design_to_qs_handoff',1,'workflow_form',NULL),
(17,'client_boq_supplier_price_collection',1,'workflow_form',NULL),
(18,'client_boq_price_check',1,'workflow_form',NULL),
(19,'client_director_management_rate_review',1,'approval',NULL),
(20,'client_boq_owner_submission',1,'workflow_form','procurement.generate_from_boq'),
(21,'client_boq_owner_submission',1,'workflow_form','procurement.generate_from_boq'),
(22,'client_procurement_takeoff_review',1,'workflow_form',NULL),
(23,'client_procurement_sourcing',1,'workflow_form',NULL),
(24,'client_procurement_contract_advance_package',1,'contract',NULL),
(25,'client_procurement_input_approval',1,'approval','procurement.require_approved'),
(26,'client_price_negotiation_log',1,'workflow_form',NULL),
(27,'client_final_rate_confirmation',1,'workflow_form',NULL),
(28,'client_director_final_price_approval',1,'approval',NULL),
(29,'client_owner_contract_package',1,'contract',NULL),
(30,'client_contract_legal_finance_review',1,'contract',NULL),
(31,'client_owner_soft_file_submission',1,'contract',NULL),
(32,'client_internal_contract_approval',1,'approval',NULL),
(33,'client_hardcopy_dispatch',1,'contract',NULL),
(34,'client_construction_assignment_meeting',1,'workflow_form',NULL),
(35,'builtin_meeting_minutes',2,'workflow_form',NULL),
(36,'client_qa_construction_plan_checklist',1,'quality',NULL),
(37,'builtin_construction_daily_report',1,'quality',NULL),
(38,'builtin_subcontractor_checklist',1,'subcontractor_reconciliation','subcontractor.reconcile_checklist'),
(39,'client_material_incoming_acceptance_summary',1,'incoming_acceptance','acceptance.incoming_material'),
(40,'client_subcontractor_quantity_dossier',1,'workflow_form','subcontractor.prepare_quantity_dossier'),
(41,'client_director_subcontractor_quantity_approval',1,'approval',NULL),
(42,'client_subcontractor_payment_dossier',1,'finance',NULL),
(43,'client_payment_acceptance_approval',1,'approval',NULL),
(44,'client_payable_document_review',1,'finance',NULL),
(45,'client_subcontractor_payment_execution',1,'finance','finance.scaffold_incoming_acceptance'),
(46,'builtin_owner_client_checklist',1,'owner_client_reconciliation','owner.reconcile_acceptance'),
(47,'client_outgoing_acceptance_director_approval',1,'approval',NULL),
(48,'client_output_settlement_package',1,'finance',NULL),
(49,'client_output_invoice_issue',1,'finance','receivable.issue_invoice'),
(50,'client_output_payment_collection',1,'finance','finance.scaffold_outgoing_acceptance'),
(51,'client_input_invoice_closure',1,'finance','payable.verify_input_invoice'),
(52,'client_output_invoice_closure',1,'handover','handover.evaluate_readiness');

WITH ranked_steps AS (
  SELECT s.id,row_number() OVER (ORDER BY n.order_index,s.order_index,s.id) AS step_no
  FROM pipeline_template_nodes n
  JOIN pipeline_template_steps s ON s.node_id=n.id
  WHERE n.template_id='40000000-0000-0000-0000-000000000020'
), resolved AS (
  SELECT rs.id,b.surface_kind,b.domain_action_key,v.id AS form_version_id
  FROM ranked_steps rs
  JOIN official_step_binding b ON b.step_no=rs.step_no
  LEFT JOIN workflow_form_templates t ON t.code=b.form_code
  LEFT JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=b.form_version_no
)
UPDATE pipeline_template_steps s
SET form_version_id=r.form_version_id,
    form_required=(r.form_version_id IS NOT NULL),
    surface_kind=r.surface_kind,
    domain_action_key=r.domain_action_key
FROM resolved r
WHERE s.id=r.id;

UPDATE pipeline_template_nodes
SET approval_policy_json=jsonb_build_object(
  'mode','any',
  'roles',responsible_role_ids,
  'quorum',1,
  'requester_cannot_approve',true,
  'comment_required',true,
  'signature_required',false
)
WHERE template_id='40000000-0000-0000-0000-000000000020';

-- Base seed resets RBAC after migrations have already run. Restore the additive
-- product-grade permissions introduced by V48, V51 and V54 so Golden Live can
-- actually enter Contract, Quality and Handover workspaces.
INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,module
FROM (VALUES
  ('contract.view','Xem hợp đồng','Xem hợp đồng, phiên bản, milestone, phụ lục và retention','contract'),
  ('contract.create','Tạo và sửa hợp đồng','Tạo hợp đồng và chỉnh sửa phiên bản nháp','contract'),
  ('contract.review','Review hợp đồng','Gửi và review phiên bản hợp đồng/phụ lục','contract'),
  ('contract.approve','Duyệt hợp đồng','Duyệt phiên bản, phụ lục và milestone','contract'),
  ('contract.sign','Ký và kích hoạt hợp đồng','Xác nhận ký và kích hoạt hợp đồng','contract'),
  ('contract.milestone_manage','Quản lý milestone hợp đồng','Quản lý điều kiện và mở nghĩa vụ AP/AR từ milestone','contract'),
  ('contract.retention_release','Giải phóng giữ lại','Mở nghĩa vụ thanh toán/thu tiền phần giữ lại','contract'),
  ('quality.view','Xem QA/QC','Xem ITP, RFI, NCR và công việc QA/QC lặp','quality'),
  ('quality.create','Lập hồ sơ QA/QC','Tạo/sửa ITP, RFI và NCR','quality'),
  ('quality.review','Review ITP','Gửi và review revision ITP','quality'),
  ('quality.approve','Duyệt/kích hoạt ITP','Duyệt và kích hoạt ITP revision','quality'),
  ('quality.inspect','Kiểm tra RFI','Thực hiện inspection và quyết định RFI','quality'),
  ('quality.ncr_manage','Quản lý NCR','Root cause, corrective action, reinspection và đóng NCR','quality'),
  ('quality.recurring_manage','Quản lý QA/QC lặp','Tạo lịch, sinh occurrence và hoàn tất occurrence','quality'),
  ('handover.verify','Xác minh hồ sơ bàn giao','Xác minh checklist, chứng từ và readiness trước bàn giao','handover')
) p(key,name,description,module)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.name='admin' AND p.module IN ('contract','quality')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id FROM roles r JOIN permissions p ON p.module='contract'
WHERE
  (r.name='director' AND p.key IN ('contract.view','contract.review','contract.approve','contract.sign','contract.milestone_manage','contract.retention_release')) OR
  (r.name='pm' AND p.key IN ('contract.view','contract.create','contract.review','contract.milestone_manage')) OR
  (r.name='qs' AND p.key IN ('contract.view','contract.create','contract.review','contract.milestone_manage')) OR
  (r.name='accounting' AND p.key IN ('contract.view','contract.review','contract.milestone_manage','contract.retention_release')) OR
  (r.name='client' AND p.key='contract.view')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id FROM roles r JOIN permissions p ON p.module='quality'
WHERE
  (r.name='director' AND p.key IN ('quality.view','quality.approve')) OR
  (r.name='pm' AND p.key IN ('quality.view','quality.create','quality.review','quality.approve','quality.ncr_manage','quality.recurring_manage')) OR
  (r.name='qa_qc' AND p.key IN ('quality.view','quality.create','quality.review','quality.inspect','quality.ncr_manage','quality.recurring_manage')) OR
  (r.name='supervisor' AND p.key IN ('quality.view','quality.create','quality.recurring_manage')) OR
  (r.name='construction_team' AND p.key='quality.view') OR
  (r.name='client' AND p.key='quality.view')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r CROSS JOIN permissions p
WHERE p.key='handover.verify' AND r.name IN ('admin','director','pm','qa_qc')
ON CONFLICT DO NOTHING;

-- Golden Sample Suite
-- Flagship executes the real "Có thiết kế" branch to final handover.
-- Companion executes the mutually exclusive "Không thiết kế" branch.
-- Together they cover every official node, step, form and primary surface without
-- pretending that a single construction run can execute two exclusive branches.

CREATE TEMP TABLE golden_projects (
  project_id UUID PRIMARY KEY,
  run_id UUID NOT NULL,
  project_code TEXT NOT NULL,
  branch_value TEXT NOT NULL
) ON COMMIT DROP;

INSERT INTO golden_projects (project_id,run_id,project_code,branch_value) VALUES
('30000000-0000-0000-0000-000000000020','42000000-0000-0000-0000-000000000020','GOLDEN-REAL-001','yes'),
('30000000-0000-0000-0000-000000000021','42000000-0000-0000-0000-000000000021','GOLDEN-NO-DESIGN-001','no');

INSERT INTO projects (
  id,name,code,partner_id,client_name,address,planned_start_date,planned_end_date,
  estimated_budget,contract_amount,status,pipeline_template_id,pm_staff_id,progress_percent
) VALUES
('30000000-0000-0000-0000-000000000020','Golden Project – Hoàn thiện Văn phòng An Phát','GOLDEN-REAL-001',
 '90000000-0000-0000-0000-000000000001','Công ty An Phát','Tầng 12, Quận 1, TP.HCM',
 '2026-01-05','2026-06-30',3200000000,4680000000,'completed','40000000-0000-0000-0000-000000000020',
 '21000000-0000-0000-0000-000000000002',100),
('30000000-0000-0000-0000-000000000021','Golden Branch – Cải tạo không thiết kế','GOLDEN-NO-DESIGN-001',
 '90000000-0000-0000-0000-000000000002','Anh Minh','TP. Thủ Đức, TP.HCM',
 '2026-02-01','2026-04-30',780000000,1150000000,'completed','40000000-0000-0000-0000-000000000020',
 '21000000-0000-0000-0000-000000000002',100);

INSERT INTO project_members (project_id,staff_id,role_ids,assigned_nodes)
SELECT gp.project_id,s.id,jsonb_build_array(r.display_name),'[]'::jsonb
FROM golden_projects gp
JOIN staff s ON s.id IN (
  '21000000-0000-0000-0000-000000000001','21000000-0000-0000-0000-000000000002',
  '21000000-0000-0000-0000-000000000003','21000000-0000-0000-0000-000000000004',
  '21000000-0000-0000-0000-000000000005','21000000-0000-0000-0000-000000000006',
  '21000000-0000-0000-0000-000000000007'
)
JOIN users u ON u.id=s.user_id
JOIN user_roles ur ON ur.user_id=u.id
JOIN roles r ON r.id=ur.role_id;

INSERT INTO pipeline_runs (
  id,project_id,template_id,status,started_at,completed_at,engine_version,template_definition_hash
)
SELECT gp.run_id,gp.project_id,t.id,'completed','2026-01-05 08:00+07','2026-06-30 17:00+07',
       t.engine_version,t.definition_hash
FROM golden_projects gp
JOIN pipeline_templates t ON t.id='40000000-0000-0000-0000-000000000020';

CREATE TEMP TABLE golden_node_map AS
SELECT gp.project_id,gp.run_id,gp.branch_value,n.id AS template_node_id,gen_random_uuid() AS run_node_id,
       CASE
         WHEN gp.branch_value='yes' AND n.order_index=4 THEN 'skipped'
         WHEN gp.branch_value='no' AND n.order_index IN (3,5) THEN 'skipped'
         ELSE 'completed'
       END AS runtime_status
FROM golden_projects gp
CROSS JOIN pipeline_template_nodes n
WHERE n.template_id='40000000-0000-0000-0000-000000000020';

INSERT INTO pipeline_run_nodes (
  id,run_id,project_id,template_node_id,name,description,type,order_index,
  responsible_role_ids,required_permission,exit_conditions,approval_rule,fail_path,
  status,started_at,completed_at,permissions,approval_policy_json
)
SELECT m.run_node_id,m.run_id,m.project_id,n.id,n.name,n.description,n.type,n.order_index,
       n.responsible_role_ids,n.required_permission,n.exit_conditions,n.approval_rule,n.fail_path,
       m.runtime_status,
       CASE WHEN m.runtime_status='completed' THEN '2026-01-05 08:00+07'::timestamptz + (n.order_index * interval '10 day') END,
       CASE WHEN m.runtime_status='completed' THEN '2026-01-05 17:00+07'::timestamptz + (n.order_index * interval '10 day') END,
       n.permissions,n.approval_policy_json
FROM golden_node_map m
JOIN pipeline_template_nodes n ON n.id=m.template_node_id;

INSERT INTO pipeline_run_edges (
  run_id,project_id,template_edge_id,from_run_node_id,to_run_node_id,
  condition_type,condition_config,label,priority
)
SELECT fm.run_id,fm.project_id,e.id,fm.run_node_id,tm.run_node_id,
       e.condition_type,e.condition_config,e.label,e.priority
FROM pipeline_template_edges e
JOIN golden_node_map fm ON fm.template_node_id=e.from_node_id
JOIN golden_node_map tm ON tm.template_node_id=e.to_node_id AND tm.project_id=fm.project_id
WHERE e.template_id='40000000-0000-0000-0000-000000000020';

CREATE TEMP TABLE golden_step_map AS
SELECT nm.project_id,nm.run_id,nm.run_node_id,nm.runtime_status,n.order_index AS node_order,
       s.id AS template_step_id,gen_random_uuid() AS run_step_id,s.form_version_id,
       s.name,s.description,s.step_type,s.required_role_ids,s.required_permission,
       s.checklist_required,s.document_required,s.photo_required,s.form_required,s.order_index,
       s.surface_kind,s.domain_action_key
FROM golden_node_map nm
JOIN pipeline_template_nodes n ON n.id=nm.template_node_id
JOIN pipeline_template_steps s ON s.node_id=n.id;

INSERT INTO pipeline_run_steps (
  id,run_node_id,project_id,template_step_id,name,description,step_type,
  required_role_ids,required_permission,checklist_required,document_required,photo_required,
  order_index,status,submitted_by,submitted_at,reviewed_at,form_required,form_version_id,
  surface_kind,domain_action_key
)
SELECT run_step_id,run_node_id,project_id,template_step_id,name,description,step_type,
       required_role_ids,required_permission,checklist_required,document_required,photo_required,
       order_index,CASE WHEN runtime_status='skipped' THEN 'skipped' ELSE 'submitted' END,
       CASE WHEN runtime_status='completed' THEN '10000000-0000-0000-0000-000000000002'::uuid END,
       CASE WHEN runtime_status='completed' THEN '2026-05-30 17:00+07'::timestamptz END,
       CASE WHEN runtime_status='completed' THEN '2026-05-31 09:00+07'::timestamptz END,
       form_required,form_version_id,surface_kind,domain_action_key
FROM golden_step_map;

CREATE TEMP TABLE golden_work_item_map AS
SELECT sm.*,gen_random_uuid() AS work_item_id
FROM golden_step_map sm;

INSERT INTO work_items (
  id,project_id,pipeline_run_id,run_node_id,run_step_id,source_type,source_id,title,description,
  work_type,status,priority,department,role_ids,assignee_staff_id,watcher_staff_ids,
  planned_start_at,due_at,started_at,completed_at,completion_payload_json,
  evidence_requirement_json,created_by,updated_by
)
SELECT wm.work_item_id,wm.project_id,wm.run_id,wm.run_node_id,wm.run_step_id,
       'pipeline_run_step',wm.run_step_id,wm.name,wm.description,
       CASE WHEN wm.step_type='meeting_minutes' THEN 'checklist' ELSE wm.step_type END,
       CASE WHEN wm.runtime_status='skipped' THEN 'cancelled' ELSE 'completed' END,
       CASE WHEN wm.step_type='approval' THEN 'high' ELSE 'normal' END,
       'Golden Project',wm.required_role_ids,
       CASE
         WHEN wm.required_permission LIKE 'accounting.%' THEN '21000000-0000-0000-0000-000000000004'::uuid
         WHEN wm.required_permission LIKE 'warehouse.%' THEN '21000000-0000-0000-0000-000000000005'::uuid
         WHEN wm.required_permission LIKE 'operation.%' THEN '21000000-0000-0000-0000-000000000006'::uuid
         ELSE '21000000-0000-0000-0000-000000000002'::uuid
       END,
       '[]'::jsonb,'2026-01-05 08:00+07','2026-06-25 17:00+07',
       CASE WHEN wm.runtime_status='completed' THEN '2026-01-05 08:00+07'::timestamptz + (wm.node_order * interval '10 day') END,
       CASE WHEN wm.runtime_status='completed' THEN '2026-01-05 17:00+07'::timestamptz + (wm.node_order * interval '10 day') END,
       jsonb_build_object('golden_sample',true,'result','Hoàn tất đúng hồ sơ và gate nghiệp vụ','branch',gp.branch_value),
       jsonb_build_object('checklist_required',wm.checklist_required,'document_required',wm.document_required,'photo_required',wm.photo_required),
       '10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002'
FROM golden_work_item_map wm
JOIN golden_projects gp ON gp.project_id=wm.project_id;

INSERT INTO work_item_assignments (work_item_id,assignment_type,staff_id,created_by)
SELECT work_item_id,'assignee',w.assignee_staff_id,'10000000-0000-0000-0000-000000000002'
FROM golden_work_item_map wm
JOIN work_items w ON w.id=wm.work_item_id
WHERE w.assignee_staff_id IS NOT NULL;

-- Every executed step receives accepted evidence. This makes the evidence-only
-- survey step visible and gives QA/QC, form and handover records a real source.
CREATE TEMP TABLE golden_evidence_map AS
SELECT wm.project_id,wm.work_item_id,wm.template_step_id,gen_random_uuid() AS evidence_id
FROM golden_work_item_map wm
WHERE wm.runtime_status='completed';

INSERT INTO evidence_records (
  id,work_item_id,project_id,target_type,target_id,evidence_type,document_type,name,title,
  description,file_url,mime_type,file_size,caption,revision_no,visibility,payload_json,status,
  uploaded_by,submitted_by,reviewed_by,reviewed_at,storage_provider,storage_key,checksum_sha256
)
SELECT em.evidence_id,em.work_item_id,em.project_id,'work_item',em.work_item_id,
       CASE WHEN wm.photo_required THEN 'photo' ELSE 'document' END,
       CASE WHEN wm.photo_required THEN 'photo' ELSE 'record' END,
       'Minh chứng – ' || wm.name,'Hồ sơ đã kiểm tra – ' || wm.name,
       'Minh chứng Golden Sample đã được PM và QA/QC kiểm tra, truy vết theo đúng work item.',
       'https://example.com/golden/evidence/' || em.evidence_id || '.pdf','application/pdf',637,
       'Hồ sơ thực hiện: ' || wm.name,'1','project',
       jsonb_build_object('golden_sample',true,'template_step_id',wm.template_step_id),
       'accepted','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006',
       '10000000-0000-0000-0000-000000000002','2026-05-31 10:00+07','external',NULL,
       '7082569b4e07fb42a1e0f001d618891eac1360d113d9eb954b82f0213cb012ce'
FROM golden_evidence_map em
JOIN golden_work_item_map wm ON wm.work_item_id=em.work_item_id;

CREATE TEMP TABLE golden_form_map AS
SELECT wm.*,gen_random_uuid() AS instance_id,t.id AS form_template_id,t.code AS form_code,t.form_type
FROM golden_work_item_map wm
JOIN workflow_form_versions v ON v.id=wm.form_version_id
JOIN workflow_form_templates t ON t.id=v.template_id
WHERE wm.runtime_status='completed' AND wm.form_required;

INSERT INTO workflow_form_instances (
  id,project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,template_step_id,
  form_template_id,form_version_id,form_type,title,status,current_revision,
  answers_snapshot_json,required_role_ids,created_by,submitted_by,submitted_at
)
SELECT instance_id,project_id,work_item_id,run_id,run_node_id,run_step_id,template_step_id,
       form_template_id,form_version_id,form_type,name,'submitted',1,'{}'::jsonb,
       required_role_ids,'10000000-0000-0000-0000-000000000002',
       '10000000-0000-0000-0000-000000000002','2026-05-30 16:00+07'
FROM golden_form_map;

INSERT INTO workflow_form_answers (
  instance_id,field_key,field_type,value_json,revision_no,updated_by
)
SELECT fm.instance_id,f.field_key,f.field_type,
       CASE f.field_type
         WHEN 'checkbox' THEN 'true'::jsonb
         WHEN 'date' THEN to_jsonb('2026-05-30'::text)
         WHEN 'number' THEN '1'::jsonb
         WHEN 'select' THEN COALESCE(f.options_json->0->'value',to_jsonb('other'::text))
         WHEN 'evidence_reference' THEN to_jsonb(em.evidence_id::text)
         WHEN 'structured_list' THEN jsonb_build_array(jsonb_build_object(
              'title','Hạng mục Golden đã hoàn tất','status','done','result','pass',
              'comment','Đã kiểm tra tại hiện trường và đối chiếu hồ sơ nguồn.'))
         WHEN 'textarea' THEN to_jsonb(('Golden Project: ' || f.label ||
              ' đã được thực hiện, kiểm tra và xác nhận; hồ sơ nguồn đính kèm đầy đủ.')::text)
         ELSE to_jsonb(('Golden Project – ' || f.label || ': đã xác nhận')::text)
       END,
       1,'10000000-0000-0000-0000-000000000002'
FROM golden_form_map fm
JOIN workflow_form_fields f ON f.form_version_id=fm.form_version_id
LEFT JOIN golden_evidence_map em ON em.work_item_id=fm.work_item_id;

-- The branch decision is the one field whose value controls the engine path.
UPDATE workflow_form_answers a
SET value_json=to_jsonb(gp.branch_value),updated_at=now()
FROM golden_form_map fm
JOIN golden_projects gp ON gp.project_id=fm.project_id
WHERE a.instance_id=fm.instance_id
  AND fm.form_code='client_design_branch_decision'
  AND a.field_key='needs_design';

UPDATE workflow_form_instances i
SET answers_snapshot_json=x.answers,updated_at=now()
FROM (
  SELECT instance_id,jsonb_object_agg(field_key,value_json) AS answers
  FROM workflow_form_answers
  WHERE instance_id IN (SELECT instance_id FROM golden_form_map)
  GROUP BY instance_id
) x
WHERE i.id=x.instance_id;

INSERT INTO workflow_form_events (
  instance_id,project_id,work_item_id,actor_id,event_type,to_status,comment,metadata_json,created_at
)
SELECT instance_id,project_id,work_item_id,'10000000-0000-0000-0000-000000000002',
       'created','draft','Khởi tạo hồ sơ Golden Project','{"golden_sample":true}'::jsonb,'2026-05-30 15:00+07'
FROM golden_form_map;

INSERT INTO workflow_form_events (
  instance_id,project_id,work_item_id,actor_id,event_type,from_status,to_status,comment,metadata_json,created_at
)
SELECT instance_id,project_id,work_item_id,'10000000-0000-0000-0000-000000000002',
       'submitted','draft','submitted','Đã kiểm tra và nộp hồ sơ nghiệp vụ','{"golden_sample":true,"validation":"passed"}'::jsonb,
       '2026-05-30 16:00+07'
FROM golden_form_map;

-- Every submitted Golden form has a traceable signature and immutable PDF artifact.
INSERT INTO workflow_form_signatures (
  project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,purpose,
  signer_user_id,signer_name,signer_roles_json,comment,payload_json,payload_hash,visual_storage_provider
)
SELECT project_id,work_item_id,instance_id,form_version_id,1,'approved_by',
       '10000000-0000-0000-0000-000000000002','Nguyễn Minh PM','["PM"]'::jsonb,
       'Ký xác nhận hồ sơ Golden Project',jsonb_build_object('golden_sample',true,'form_code',form_code),
       encode(digest(instance_id::text || ':golden-signature','sha256'),'hex'),'none'
FROM golden_form_map;

INSERT INTO workflow_form_pdf_exports (
  project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,title,file_name,
  storage_provider,storage_key,mime_type,file_size,checksum_sha256,payload_hash,metadata_json,
  generated_by,generated_by_name
)
SELECT project_id,work_item_id,instance_id,form_version_id,1,'Hồ sơ Golden – ' || name,
       project_code || '-' || replace(form_code,'_','-') || '.pdf','local',
       'workflow-form-pdfs/golden-project/' || form_code || '.pdf',
       'application/pdf',
       CASE fm.form_code
         WHEN 'client_site_survey_minutes' THEN 48086
         WHEN 'client_procurement_contract_advance_package' THEN 47537
         WHEN 'client_owner_contract_package' THEN 47725
         WHEN 'builtin_meeting_minutes' THEN 47452
         WHEN 'builtin_subcontractor_checklist' THEN 47358
         WHEN 'builtin_owner_client_checklist' THEN 47693
         WHEN 'client_output_settlement_package' THEN 47231
         WHEN 'client_output_invoice_closure' THEN 47244
       END,
       CASE fm.form_code
         WHEN 'client_site_survey_minutes' THEN 'be36110502f3e7656053b0c4bc13d030231aa83153b2823ee684900e3684954e'
         WHEN 'client_procurement_contract_advance_package' THEN '9f3e8309b504e5278950d2355717eb417a81e5f1efa2641ded09e51e9eef8b50'
         WHEN 'client_owner_contract_package' THEN '0475d85b62c99866bb255e935a018e3b93291a58b16e3489532717346b377724'
         WHEN 'builtin_meeting_minutes' THEN '48f22ba2e7b15294d468560aae786a58e64c4aabdab7bb6dff870ac3f0ce93d9'
         WHEN 'builtin_subcontractor_checklist' THEN '177c903d72e68792d1fd71fefc1aefa8f7930a8f5602e1211e6227bda8ff0f6b'
         WHEN 'builtin_owner_client_checklist' THEN 'efb57270985ba212f54f5ffaa8e16538a2a6ab98d39f3de478093d4ee01f8942'
         WHEN 'client_output_settlement_package' THEN '8783e51407c661d954e8081cce812b26b1a06d661e355efeae5bb11b131ede99'
         WHEN 'client_output_invoice_closure' THEN '490615e7443a34c268d524defd2091e97ba692c7e04fcbf5cc6e55a8164d51a8'
       END,
       encode(digest(instance_id::text || ':golden-pdf','sha256'),'hex'),
       jsonb_build_object('golden_sample',true,'form_code',form_code),
       '10000000-0000-0000-0000-000000000002','Nguyễn Minh PM'
FROM golden_form_map fm
JOIN golden_projects gp USING (project_id)
WHERE gp.project_code='GOLDEN-REAL-001'
  AND fm.form_code IN (
    'client_site_survey_minutes','client_procurement_contract_advance_package',
    'client_owner_contract_package','builtin_meeting_minutes',
    'builtin_subcontractor_checklist','builtin_owner_client_checklist',
    'client_output_settlement_package','client_output_invoice_closure'
  );

CREATE TEMP TABLE golden_approval_map AS
SELECT wm.work_item_id,wm.project_id,gen_random_uuid() AS approval_id
FROM golden_work_item_map wm
WHERE wm.runtime_status='completed'
  AND (wm.step_type='approval' OR wm.surface_kind='approval');

INSERT INTO approval_requests (
  id,target_type,target_id,project_id,requested_by,status,reason,required_roles,
  resolved_at,policy_json,current_stage
)
SELECT approval_id,'work_item',work_item_id,project_id,
       '10000000-0000-0000-0000-000000000002','approved',
       'Phê duyệt hồ sơ Golden Project sau khi kiểm tra chứng từ và hiện trường.',
       '["Giám đốc"]'::jsonb,'2026-05-31 11:00+07',
       '{"mode":"any","roles":["Giám đốc"],"quorum":1,"requester_cannot_approve":true,"comment_required":true,"signature_required":false}'::jsonb,1
FROM golden_approval_map;

INSERT INTO approval_actions (
  approval_request_id,user_id,role_id,action,comment,role_label,created_at
)
SELECT am.approval_id,'10000000-0000-0000-0000-000000000007',r.id,'approved',
       'Đã đối chiếu hồ sơ, khối lượng, chi phí và điều kiện đầu ra; chấp thuận.',
       r.display_name,'2026-05-31 11:00+07'
FROM golden_approval_map am
JOIN roles r ON r.name='director';

UPDATE work_items w
SET approval_request_id=am.approval_id
FROM golden_approval_map am
WHERE w.id=am.work_item_id;

INSERT INTO work_item_activities (
  work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,to_status,comment,metadata_json,created_at
)
SELECT wm.work_item_id,wm.project_id,'work_item',wm.work_item_id,
       '10000000-0000-0000-0000-000000000002','completed',
       CASE WHEN wm.runtime_status='skipped' THEN 'cancelled' ELSE 'completed' END,
       CASE WHEN wm.runtime_status='skipped'
            THEN 'Bỏ qua đúng nhánh loại trừ của workflow.'
            ELSE 'Hoàn tất work item Golden với form, evidence và gate tương ứng.' END,
       jsonb_build_object('golden_sample',true,'template_step_id',wm.template_step_id),
       '2026-05-31 17:00+07'
FROM golden_work_item_map wm;

UPDATE pipeline_runs pr
SET current_node_id=nm.run_node_id
FROM golden_node_map nm
JOIN pipeline_template_nodes n ON n.id=nm.template_node_id
WHERE pr.id=nm.run_id AND n.order_index=12;

UPDATE projects p
SET current_pipeline_run_id=gp.run_id,updated_at=now()
FROM golden_projects gp
WHERE p.id=gp.project_id;

INSERT INTO audit_logs (
  project_id,actor_id,actor_name,event_type,target_type,target_id,description,metadata_json,created_at
)
SELECT project_id,'10000000-0000-0000-0000-000000000001','Admin hệ thống',
       'golden_workflow_completed','project',project_id,
       'Golden Sample hoàn tất đúng template chính thức và nhánh runtime đã chọn.',
       jsonb_build_object('golden_sample',true,'project_code',project_code,'branch',branch_value,
                          'template_id','40000000-0000-0000-0000-000000000020'),
       '2026-06-30 17:00+07'
FROM golden_projects;

COMMIT;
