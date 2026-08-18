\set ON_ERROR_STOP on

DO $$
DECLARE
  actual BIGINT;
  expected BIGINT;
  amount NUMERIC;
BEGIN
  SELECT count(*) INTO actual FROM projects WHERE code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001');
  IF actual <> 2 THEN RAISE EXCEPTION 'Golden projects: expected 2, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM projects project
  JOIN pipeline_templates template ON template.id=project.pipeline_template_id
  WHERE project.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001')
    AND template.template_family_id='40000000-0000-0000-0000-000000000020'
    AND project.status='completed' AND project.progress_percent=100;
  IF actual <> 2 THEN RAISE EXCEPTION 'Golden projects are not completed on official template: %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_nodes n JOIN projects p ON p.id=n.project_id
  WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001');
  IF actual <> 24 THEN RAISE EXCEPTION 'Runtime node coverage: expected 24, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps s JOIN projects p ON p.id=s.project_id
  WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001');
  IF actual <> 104 THEN RAISE EXCEPTION 'Runtime step coverage: expected 104, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM work_items w JOIN projects p ON p.id=w.project_id
  WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001');
  IF actual <> 104 THEN RAISE EXCEPTION 'Work item coverage: expected 104, got %',actual; END IF;

  SELECT count(DISTINCT i.template_step_id) INTO actual
  FROM workflow_form_instances i
  JOIN projects p ON p.id=i.project_id
  WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001') AND i.status='submitted';
  IF actual <> 51 THEN RAISE EXCEPTION 'Official form-step coverage: expected 51, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps s
  JOIN projects p ON p.id=s.project_id
  WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001')
    AND s.status='submitted' AND s.form_required
    AND NOT EXISTS (
      SELECT 1 FROM workflow_form_instances i
      WHERE i.run_step_id=s.id AND i.form_version_id=s.form_version_id AND i.status='submitted'
    );
  IF actual <> 0 THEN RAISE EXCEPTION 'Executed required steps missing exact submitted form: %',actual; END IF;

  SELECT count(*) INTO actual
  FROM workflow_form_instances i
  JOIN projects p ON p.id=i.project_id
  JOIN workflow_form_fields f ON f.form_version_id=i.form_version_id AND f.required
  WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001')
    AND NOT EXISTS (SELECT 1 FROM workflow_form_answers a WHERE a.instance_id=i.id AND a.field_key=f.field_key);
  IF actual <> 0 THEN RAISE EXCEPTION 'Required form answers missing: %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps s
  JOIN projects p ON p.id=s.project_id
  WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001') AND s.status='submitted'
    AND NOT EXISTS (
      SELECT 1 FROM work_items w JOIN evidence_records e ON e.work_item_id=w.id
      WHERE w.run_step_id=s.id AND e.status='accepted'
    );
  IF actual <> 0 THEN RAISE EXCEPTION 'Executed steps missing accepted evidence: %',actual; END IF;

  SELECT count(*) INTO actual
  FROM workflow_form_pdf_exports x
  JOIN projects p ON p.id=x.project_id
  WHERE p.code='GOLDEN-REAL-001';
  IF actual <> 8 THEN RAISE EXCEPTION 'Golden PDF artifacts: expected 8, got %',actual; END IF;

  SELECT count(*) INTO actual FROM boq_items WHERE project_id='30000000-0000-0000-0000-000000000020' AND final_price_locked AND selected_quotation_item_id IS NOT NULL;
  IF actual <> 6 THEN RAISE EXCEPTION 'BOQ final quote linkage: expected 6, got %',actual; END IF;
  SELECT count(*) INTO actual FROM supplier_quotations WHERE project_id='30000000-0000-0000-0000-000000000020' AND status IN ('qualified','selected');
  IF actual <> 3 THEN RAISE EXCEPTION 'Qualified supplier quotations: expected 3, got %',actual; END IF;
  SELECT count(*) INTO actual FROM supplier_quotation_items qi JOIN supplier_quotations q ON q.id=qi.quotation_id WHERE q.project_id='30000000-0000-0000-0000-000000000020';
  IF actual <> 18 THEN RAISE EXCEPTION 'Supplier quote matrix items: expected 18, got %',actual; END IF;
  SELECT count(*) INTO actual FROM procurement_requests WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='received';
  IF actual <> 2 THEN RAISE EXCEPTION 'Received procurement requests: expected 2, got %',actual; END IF;
  SELECT count(*) INTO actual FROM incoming_acceptance_records WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='submitted' AND result IN ('pass','conditional_pass');
  IF actual <> 2 THEN RAISE EXCEPTION 'Incoming acceptances: expected 2, got %',actual; END IF;

  SELECT count(*) INTO actual FROM contracts WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='active' AND current_version_id IS NOT NULL;
  IF actual <> 3 THEN RAISE EXCEPTION 'Active contracts: expected 3, got %',actual; END IF;
  SELECT count(*) INTO actual FROM contract_versions v JOIN contracts c ON c.id=v.contract_id WHERE c.project_id='30000000-0000-0000-0000-000000000020' AND v.status='signed';
  IF actual <> 4 THEN RAISE EXCEPTION 'Signed contract versions: expected 4, got %',actual; END IF;
  SELECT count(*) INTO actual FROM contract_milestones m JOIN contracts c ON c.id=m.contract_id WHERE c.project_id='30000000-0000-0000-0000-000000000020' AND m.status='paid';
  IF actual <> 6 THEN RAISE EXCEPTION 'Paid contract milestones: expected 6, got %',actual; END IF;
  SELECT count(*) INTO actual FROM contract_retentions r JOIN contracts c ON c.id=r.contract_id WHERE c.project_id='30000000-0000-0000-0000-000000000020' AND r.status='released' AND r.outstanding_amount=0;
  IF actual <> 3 THEN RAISE EXCEPTION 'Released contract retentions: expected 3, got %',actual; END IF;

  SELECT count(*) INTO actual FROM quality_itp_plans WHERE project_id='30000000-0000-0000-0000-000000000020' AND status IN ('active','superseded');
  IF actual <> 2 THEN RAISE EXCEPTION 'ITP revision history: expected 2, got %',actual; END IF;
  SELECT count(*) INTO actual FROM quality_inspection_requests WHERE project_id='30000000-0000-0000-0000-000000000020';
  IF actual <> 3 THEN RAISE EXCEPTION 'RFI history: expected 3, got %',actual; END IF;
  SELECT count(*) INTO actual FROM quality_ncrs WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='closed' AND reinspection_rfi_id IS NOT NULL;
  IF actual <> 1 THEN RAISE EXCEPTION 'Closed NCR/reinspection chain: expected 1, got %',actual; END IF;
  SELECT count(*) INTO actual FROM construction_recurring_occurrences WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='completed';
  IF actual <> 3 THEN RAISE EXCEPTION 'Completed recurring QA occurrences: expected 3, got %',actual; END IF;
  SELECT count(*) INTO actual FROM construction_check_templates WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='active';
  IF actual <> 1 THEN RAISE EXCEPTION 'Simplified construction check templates: expected 1, got %',actual; END IF;
  SELECT count(*) INTO actual FROM construction_checks WHERE project_id='30000000-0000-0000-0000-000000000020';
  IF actual <> 4 THEN RAISE EXCEPTION 'Simplified construction checks: expected 4, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM construction_checks c
  JOIN operation_daily_reports r ON r.id=c.source_daily_report_id AND r.project_id=c.project_id
  WHERE c.project_id='30000000-0000-0000-0000-000000000020'
    AND (r.work_item_id IS NULL OR r.work_item_id=c.work_item_id);
  IF actual <> 4 THEN RAISE EXCEPTION 'Daily report to construction check links: expected 4 valid links, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM construction_check_evidence ce
  JOIN construction_checks c ON c.id=ce.check_id
  JOIN evidence_records e ON e.id=ce.evidence_id
  WHERE c.project_id='30000000-0000-0000-0000-000000000020'
    AND e.project_id=c.project_id AND e.work_item_id=c.work_item_id;
  IF actual <> 4 THEN RAISE EXCEPTION 'Construction check evidence links: expected 4 valid links, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM construction_check_evidence
  WHERE check_id='86700000-0000-0000-0000-000000000204' AND evidence_role='context';
  IF actual <> 1 THEN RAISE EXCEPTION 'Pending construction check context evidence: expected 1, got %',actual; END IF;
  SELECT count(*) INTO actual FROM construction_action_items WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='done';
  IF actual <> 1 THEN RAISE EXCEPTION 'Completed simplified construction actions: expected 1, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM construction_action_items a
  JOIN construction_checks c ON c.id=a.source_check_id
  JOIN operation_daily_reports r ON r.id=c.source_daily_report_id
  WHERE a.project_id='30000000-0000-0000-0000-000000000020' AND a.status='done';
  IF actual <> 1 THEN RAISE EXCEPTION 'Resolved action report trace: expected 1 inherited link, got %',actual; END IF;

  SELECT count(*) INTO actual FROM acceptance_records WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='completed' AND result='pass';
  IF actual <> 1 THEN RAISE EXCEPTION 'Final acceptance pass: expected 1, got %',actual; END IF;
  SELECT count(*) INTO actual FROM rework_requests WHERE project_id='30000000-0000-0000-0000-000000000020' AND status IN ('done','completed');
  IF actual <> 1 THEN RAISE EXCEPTION 'Completed rework history: expected 1, got %',actual; END IF;
  SELECT count(*) INTO actual FROM subcontractor_checklist_reconciliations WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='submitted';
  IF actual <> 1 THEN RAISE EXCEPTION 'Subcontractor reconciliation missing'; END IF;
  SELECT count(*) INTO actual FROM owner_client_checklist_reconciliations WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='submitted';
  IF actual <> 1 THEN RAISE EXCEPTION 'Owner/client reconciliation missing'; END IF;

  SELECT COALESCE(sum(surplus_quantity),0) INTO amount FROM site_material_balances WHERE project_id='30000000-0000-0000-0000-000000000020';
  IF amount <> 0 THEN RAISE EXCEPTION 'Site material surplus must be zero, got %',amount; END IF;
  SELECT count(*) INTO actual FROM site_material_balances WHERE project_id='30000000-0000-0000-0000-000000000020' AND surplus_status='handled';
  IF actual <> 6 THEN RAISE EXCEPTION 'Handled material balances: expected 6, got %',actual; END IF;
  SELECT count(*) INTO actual FROM site_surplus_handlings WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='submitted';
  IF actual <> 6 THEN RAISE EXCEPTION 'Surplus return history: expected 6, got %',actual; END IF;

  SELECT COALESCE(sum(remaining_amount),0) INTO amount FROM payables WHERE project_id='30000000-0000-0000-0000-000000000020' AND status <> 'deleted';
  IF amount <> 0 THEN RAISE EXCEPTION 'AP remaining must be zero, got %',amount; END IF;
  SELECT COALESCE(sum(remaining_amount),0) INTO amount FROM receivables WHERE project_id='30000000-0000-0000-0000-000000000020' AND status <> 'deleted';
  IF amount <> 0 THEN RAISE EXCEPTION 'AR remaining must be zero, got %',amount; END IF;
  SELECT count(*) INTO actual FROM payments WHERE project_id='30000000-0000-0000-0000-000000000020' AND status <> 'approved';
  IF actual <> 0 THEN RAISE EXCEPTION 'Non-terminal Golden payments: %',actual; END IF;
  SELECT count(*) INTO actual FROM input_invoices WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='verified';
  IF actual <> 1 THEN RAISE EXCEPTION 'Verified input invoice missing'; END IF;
  SELECT count(*) INTO actual FROM output_invoices WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='issued';
  IF actual <> 1 THEN RAISE EXCEPTION 'Issued output invoice missing'; END IF;

  SELECT COALESCE(sum(gross_amount),0) INTO amount FROM receivables WHERE project_id='30000000-0000-0000-0000-000000000020';
  IF amount <> 4680000000 THEN RAISE EXCEPTION 'Main contract AR reconciliation expected 4.68b, got %',amount; END IF;
  SELECT COALESCE(sum(gross_amount),0) INTO amount FROM payables WHERE project_id='30000000-0000-0000-0000-000000000020' AND vendor_type='subcontractor';
  IF amount <> 950000000 THEN RAISE EXCEPTION 'Subcontract AP reconciliation expected 950m, got %',amount; END IF;

  SELECT count(*) INTO actual
  FROM handover_records h
  WHERE h.project_id='30000000-0000-0000-0000-000000000020'
    AND h.status='handed_over' AND h.completed_at IS NOT NULL
    AND h.readiness_snapshot_json->>'ready'='true';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden handover is not completed and ready'; END IF;
  SELECT count(*) INTO actual FROM handover_checklist_items WHERE handover_id='73000000-0000-0000-0000-000000000201' AND required AND status='verified';
  IF actual <> 8 THEN RAISE EXCEPTION 'Verified handover checklist: expected 8, got %',actual; END IF;

  SELECT
    (SELECT count(*) FROM issues WHERE project_id='30000000-0000-0000-0000-000000000020' AND status NOT IN ('closed','cancelled')) +
    (SELECT count(*) FROM quality_ncrs WHERE project_id='30000000-0000-0000-0000-000000000020' AND status NOT IN ('closed','cancelled')) +
    (SELECT count(*) FROM rework_requests WHERE project_id='30000000-0000-0000-0000-000000000020' AND status NOT IN ('done','completed','cancelled')) +
    (SELECT count(*) FROM construction_action_items WHERE project_id='30000000-0000-0000-0000-000000000020' AND status IN ('open','waiting_confirmation'))
  INTO actual;
  IF actual <> 0 THEN RAISE EXCEPTION 'Open quality/handover blockers: %',actual; END IF;

  -- Official template policies must remain executable by the engine-v2 parser.
  SELECT count(*) INTO actual
  FROM pipeline_template_nodes n
  WHERE n.template_id='40000000-0000-0000-0000-000000000020'
    AND n.approval_policy_json->>'mode'='any'
    AND jsonb_typeof(n.approval_policy_json->'roles')='array'
    AND jsonb_array_length(n.approval_policy_json->'roles') > 0
    AND (n.approval_policy_json->>'quorum')::integer=1
    AND (n.approval_policy_json->>'requester_cannot_approve')::boolean
    AND (n.approval_policy_json->>'comment_required')::boolean
    AND NOT (n.approval_policy_json ? 'required_roles')
    AND NOT (n.approval_policy_json ? 'require_comment')
    AND NOT (n.approval_policy_json ? 'separate_requester');
  IF actual <> 12 THEN RAISE EXCEPTION 'Official approval policy contract: expected 12 valid nodes, got %',actual; END IF;

  SELECT count(*) INTO actual FROM permissions
  WHERE key LIKE 'contract.%' OR key LIKE 'quality.%' OR key LIKE 'construction.%' OR key='handover.verify';
  IF actual <> 29 THEN RAISE EXCEPTION 'Product-grade RBAC permissions after seed: expected 29, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE p.key='project.member.manage' AND r.name IN ('admin','director','pm');
  IF actual <> 3 THEN RAISE EXCEPTION 'Project member management role grants: expected 3, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE r.name='pm' AND p.key IN ('planning.view','planning.edit');
  IF actual <> 2 THEN RAISE EXCEPTION 'PM official planning permissions: expected 2, got %',actual; END IF;

  SELECT
    (SELECT count(*) FROM pipeline_template_steps WHERE name LIKE 'Trảo đổi%') +
    (SELECT count(*) FROM pipeline_run_steps WHERE name LIKE 'Trảo đổi%') +
    (SELECT count(*) FROM work_items WHERE title LIKE 'Trảo đổi%') +
    (SELECT count(*) FROM workflow_form_instances WHERE title LIKE 'Trảo đổi%')
  INTO actual;
  IF actual <> 0 THEN RAISE EXCEPTION 'Incorrect workflow label Trảo đổi remains in % record(s)',actual; END IF;

  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE r.name='qa_qc' AND p.key IN (
    'quality.view','quality.create','quality.review','quality.inspect',
    'quality.ncr_manage','quality.recurring_manage'
  );
  IF actual <> 6 THEN RAISE EXCEPTION 'QA/QC Golden Live permissions: expected 6, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE r.name='qa_qc' AND p.key LIKE 'construction.%';
  IF actual <> 9 THEN RAISE EXCEPTION 'QA/QC simplified construction permissions: expected 9, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE r.name IN ('admin','pm','qa_qc','supervisor','construction_team')
    AND p.key IN ('construction.check.edit','construction.action.edit');
  IF actual <> 10 THEN RAISE EXCEPTION 'Construction edit role grants: expected 10, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE p.key='contract.variation.propose' AND r.name IN ('admin','pm','supervisor');
  IF actual <> 3 THEN RAISE EXCEPTION 'Construction variation proposal grants: expected 3, got %',actual; END IF;
  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE r.name='supervisor' AND p.key IN ('contract.view','contract.create');
  IF actual <> 0 THEN RAISE EXCEPTION 'Supervisor must not receive broad contract view/create permissions'; END IF;
  SELECT count(*) INTO actual
  FROM role_permissions rp
  JOIN roles r ON r.id=rp.role_id
  JOIN permissions p ON p.id=rp.permission_id
  WHERE p.key='handover.verify' AND r.name IN ('admin','director','pm','qa_qc');
  IF actual <> 4 THEN RAISE EXCEPTION 'Handover verification role grants: expected 4, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM users u
  JOIN user_roles ur ON ur.user_id=u.id
  JOIN roles r ON r.id=ur.role_id
  WHERE (u.email,r.name) IN (
    ('procurement@consflow.local','procurement'),
    ('qaqc@consflow.local','qa_qc'),
    ('construction@consflow.local','construction_team'),
    ('subcontractor@consflow.local','subcontractor'),
    ('design@consflow.local','design'),
    ('sales@consflow.local','sales'),
    ('legal@consflow.local','legal'),
    ('contract@consflow.local','contract_officer')
  ) AND u.status='active';
  IF actual <> 8 THEN RAISE EXCEPTION 'Golden Live dedicated App role accounts: expected 8, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM users u
  JOIN staff s ON s.user_id=u.id
  JOIN user_roles ur ON ur.user_id=u.id
  JOIN roles r ON r.id=ur.role_id
  WHERE u.email='client@consflow.local'
    AND u.status='inactive'
    AND s.status='inactive'
    AND r.name='client'
    AND r.external_only
    AND NOT EXISTS (SELECT 1 FROM role_permissions rp WHERE rp.role_id=r.id);
  IF actual <> 1 THEN RAISE EXCEPTION 'Client negative-login fixture must remain inactive, external-only and permissionless'; END IF;

  SELECT count(*) INTO actual
  FROM projects p
  JOIN pipeline_runs r ON r.id=p.current_pipeline_run_id
  JOIN pipeline_templates t ON t.id=r.template_id
  WHERE p.id='30000000-0000-0000-0000-000000000022'
    AND p.code='GOLDEN-LIVE-001' AND p.status='planning' AND p.progress_percent=0
    AND r.status='in_progress' AND r.completed_at IS NULL
    AND r.engine_version=t.engine_version
    AND r.template_definition_hash=t.definition_hash
    AND t.id='40000000-0000-0000-0000-000000000020';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Live project/run contract is invalid'; END IF;

  SELECT count(*) INTO actual FROM project_members
  WHERE project_id='30000000-0000-0000-0000-000000000022' AND active;
  IF actual <> 14 THEN RAISE EXCEPTION 'Golden Live App project members: expected 14, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000022';
  IF actual <> 12 THEN RAISE EXCEPTION 'Golden Live nodes: expected 12, got %',actual; END IF;
  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000022' AND status='in_progress' AND order_index=1;
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Live must have node 1 in progress'; END IF;
  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000022' AND status='pending' AND order_index>1;
  IF actual <> 11 THEN RAISE EXCEPTION 'Golden Live pending nodes: expected 11, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000022' AND status='pending';
  IF actual <> 52 THEN RAISE EXCEPTION 'Golden Live pending steps: expected 52, got %',actual; END IF;
  SELECT count(*) INTO actual FROM work_items
  WHERE project_id='30000000-0000-0000-0000-000000000022' AND status='ready';
  IF actual <> 6 THEN RAISE EXCEPTION 'Golden Live ready work items: expected 6, got %',actual; END IF;
  SELECT count(*) INTO actual FROM work_items
  WHERE project_id='30000000-0000-0000-0000-000000000022' AND status='pending';
  IF actual <> 46 THEN RAISE EXCEPTION 'Golden Live pending work items: expected 46, got %',actual; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id='30000000-0000-0000-0000-000000000022'
    AND status='draft' AND current_revision=0 AND answers_snapshot_json='{}'::jsonb;
  IF actual <> 51 THEN RAISE EXCEPTION 'Golden Live draft forms: expected 51, got %',actual; END IF;
  SELECT count(*) INTO actual FROM workflow_form_answers a
  JOIN workflow_form_instances i ON i.id=a.instance_id
  WHERE i.project_id='30000000-0000-0000-0000-000000000022';
  IF actual <> 0 THEN RAISE EXCEPTION 'Golden Live must not have form answers, got %',actual; END IF;

  -- Rev4 is an immutable future-run definition. Its report metadata contains only
  -- client-to-business-code references; runtime/UAT state remains outside the template report.
  SELECT count(*) INTO actual
  FROM pipeline_templates t
  WHERE t.id='40000000-0000-0000-0000-000000000023'
    AND t.source_template_id='40000000-0000-0000-0000-000000000022'
    AND t.revision_no=4 AND t.version=4 AND t.status='published'
    AND jsonb_array_length(t.report_config_json->'node_references')=14
    AND NOT EXISTS (
      SELECT 1 FROM jsonb_object_keys(t.report_config_json) AS keys(key_name)
      WHERE key_name NOT IN ('node_reference_title','node_references')
    );
  IF actual <> 1 THEN RAISE EXCEPTION 'Official Rev4 template lineage/report definition is invalid'; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000023';
  IF actual <> 15 THEN RAISE EXCEPTION 'Official Rev4 nodes: expected 15, got %',actual; END IF;
  SELECT count(DISTINCT business_code) INTO actual
  FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000023' AND business_code IS NOT NULL;
  IF actual <> 15 THEN RAISE EXCEPTION 'Official Rev4 unique node business codes: expected 15, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_edges
  WHERE template_id='40000000-0000-0000-0000-000000000023';
  IF actual <> 16 THEN RAISE EXCEPTION 'Official Rev4 edges: expected 16, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_steps s
  JOIN pipeline_template_nodes n ON n.id=s.node_id
  WHERE n.template_id='40000000-0000-0000-0000-000000000023';
  IF actual <> 51 THEN RAISE EXCEPTION 'Official Rev4 steps: expected 51, got %',actual; END IF;
  SELECT count(DISTINCT s.business_code) INTO actual
  FROM pipeline_template_steps s
  JOIN pipeline_template_nodes n ON n.id=s.node_id
  WHERE n.template_id='40000000-0000-0000-0000-000000000023' AND s.business_code IS NOT NULL;
  IF actual <> 51 THEN RAISE EXCEPTION 'Official Rev4 unique step business codes: expected 51, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_steps s
  JOIN pipeline_template_nodes n ON n.id=s.node_id
  WHERE n.template_id='40000000-0000-0000-0000-000000000023'
    AND (
      (s.business_code='boq_without_design.pricing' AND s.required_role_ids='["QS","Mua hàng"]'::jsonb) OR
      (s.business_code='planning.client_quality_file' AND s.step_type='document_upload' AND s.surface_kind='quality' AND s.form_required AND s.required_role_ids='["QA/QC"]'::jsonb AND s.checklist_required AND s.document_required AND NOT s.photo_required) OR
      (s.business_code='construction.item_reporting' AND s.required_role_ids='["Giám sát"]'::jsonb AND s.checklist_required AND NOT s.document_required AND s.photo_required)
    );
  IF actual <> 3 THEN RAISE EXCEPTION 'Official Rev4 role/evidence responsibilities are invalid'; END IF;

  -- Historical projects 020/021/022 and their runtime snapshots must not be rebound by Rev4.
  SELECT count(*) INTO actual
  FROM projects p
  JOIN pipeline_runs r ON r.id=p.current_pipeline_run_id
  WHERE p.id IN (
      '30000000-0000-0000-0000-000000000020',
      '30000000-0000-0000-0000-000000000021',
      '30000000-0000-0000-0000-000000000022'
    )
    AND p.pipeline_template_id<>'40000000-0000-0000-0000-000000000023'
    AND r.template_id='40000000-0000-0000-0000-000000000020';
  IF actual <> 3 THEN RAISE EXCEPTION 'Historical projects 020/021/022 were unexpectedly rebound by Rev4'; END IF;

  -- Rev3 uses a new runtime fixture; historical project 022 must remain on its original snapshot.
  SELECT count(*) INTO actual
  FROM projects p
  JOIN pipeline_runs r ON r.id=p.current_pipeline_run_id
  WHERE p.id='30000000-0000-0000-0000-000000000023'
    AND p.code='GOLDEN-REV3-BOQ-001'
    AND p.workflow_entry_mode='client_boq'
    AND p.pipeline_template_id='40000000-0000-0000-0000-000000000022'
    AND r.template_id='40000000-0000-0000-0000-000000000022'
    AND r.engine_version=3 AND r.status='in_progress';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Rev3 project/run contract is invalid'; END IF;

  SELECT count(*) INTO actual FROM documents
  WHERE project_id='30000000-0000-0000-0000-000000000023'
    AND document_type='client_boq' AND status='active';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Rev3 client BOQ evidence: expected 1, got %',actual; END IF;

  SELECT count(*) INTO actual FROM project_members
  WHERE project_id='30000000-0000-0000-0000-000000000023' AND active;
  IF actual <> 12 THEN RAISE EXCEPTION 'Golden Rev3 project members: expected 12, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000023' AND business_code IS NOT NULL;
  IF actual <> 15 THEN RAISE EXCEPTION 'Golden Rev3 coded runtime nodes: expected 15, got %',actual; END IF;

  SELECT count(DISTINCT business_code) INTO actual FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000023' AND business_code IS NOT NULL;
  IF actual <> 51 THEN RAISE EXCEPTION 'Golden Rev3 unique coded runtime steps: expected 51, got %',actual; END IF;

  SELECT count(*) INTO actual FROM work_items
  WHERE project_id='30000000-0000-0000-0000-000000000023';
  IF actual <> 51 THEN RAISE EXCEPTION 'Golden Rev3 work items: expected 51, got %',actual; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id='30000000-0000-0000-0000-000000000023'
    AND status='draft' AND current_revision=0;
  IF actual <> 49 THEN RAISE EXCEPTION 'Golden Rev3 draft forms: expected 49, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM projects p JOIN pipeline_runs r ON r.id=p.current_pipeline_run_id
  WHERE p.id='30000000-0000-0000-0000-000000000022'
    AND p.pipeline_template_id<>'40000000-0000-0000-0000-000000000022'
    AND r.template_id='40000000-0000-0000-0000-000000000020';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Live 022 history was unexpectedly rebound to Rev3'; END IF;

  -- Rev4-only UAT fixtures: 024 is a production-shaped entry state, while 025 is
  -- an explicitly synthetic construction-stage snapshot for Operations Browser UAT.
  SELECT count(*) INTO actual
  FROM projects p
  JOIN pipeline_runs r ON r.id=p.current_pipeline_run_id
  JOIN pipeline_templates t ON t.id=r.template_id
  WHERE p.id IN (
      '30000000-0000-0000-0000-000000000024',
      '30000000-0000-0000-0000-000000000025'
    )
    AND p.pipeline_template_id='40000000-0000-0000-0000-000000000023'
    AND r.template_id='40000000-0000-0000-0000-000000000023'
    AND r.engine_version=3 AND r.status='in_progress'
    AND r.template_definition_hash=t.definition_hash;
  IF actual <> 2 THEN RAISE EXCEPTION 'Rev4 UAT project/run snapshot contract is invalid'; END IF;

  SELECT count(*) INTO actual FROM project_members
  WHERE project_id IN (
      '30000000-0000-0000-0000-000000000024',
      '30000000-0000-0000-0000-000000000025'
    ) AND active;
  IF actual <> 24 THEN RAISE EXCEPTION 'Rev4 UAT project members: expected 24, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id IN (
      '30000000-0000-0000-0000-000000000024',
      '30000000-0000-0000-0000-000000000025'
    );
  IF actual <> 30 THEN RAISE EXCEPTION 'Rev4 UAT runtime nodes: expected 30, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_steps
  WHERE project_id IN (
      '30000000-0000-0000-0000-000000000024',
      '30000000-0000-0000-0000-000000000025'
    );
  IF actual <> 102 THEN RAISE EXCEPTION 'Rev4 UAT runtime steps: expected 102, got %',actual; END IF;

  SELECT count(*) INTO actual FROM work_items
  WHERE project_id IN (
      '30000000-0000-0000-0000-000000000024',
      '30000000-0000-0000-0000-000000000025'
    );
  IF actual <> 102 THEN RAISE EXCEPTION 'Rev4 UAT work items: expected 102, got %',actual; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id IN (
      '30000000-0000-0000-0000-000000000024',
      '30000000-0000-0000-0000-000000000025'
    ) AND status='draft' AND current_revision=0;
  IF actual <> 98 THEN RAISE EXCEPTION 'Rev4 UAT draft forms: expected 98, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_runs run
  JOIN pipeline_run_nodes node ON node.id=run.current_node_id
  WHERE run.project_id='30000000-0000-0000-0000-000000000024'
    AND node.business_code='project_intake' AND node.status='in_progress';
  IF actual <> 1 THEN RAISE EXCEPTION 'Rev4 entry fixture 024 is not at project_intake'; END IF;

  SELECT count(*) INTO actual
  FROM work_items item
  JOIN pipeline_run_steps step ON step.id=item.run_step_id
  JOIN pipeline_run_nodes node ON node.id=item.run_node_id
  WHERE item.project_id='30000000-0000-0000-0000-000000000024'
    AND item.status='ready' AND node.business_code='project_intake'
    AND step.business_code='project_intake.confirm';
  IF actual <> 1 THEN RAISE EXCEPTION 'Rev4 entry fixture 024 must expose exactly one entry work item'; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_runs run
  JOIN pipeline_run_nodes node ON node.id=run.current_node_id
  WHERE run.project_id='30000000-0000-0000-0000-000000000025'
    AND node.business_code='construction_execution' AND node.status='in_progress';
  IF actual <> 1 THEN RAISE EXCEPTION 'Rev4 construction fixture 025 is not at construction_execution'; END IF;

  SELECT count(*) INTO actual
  FROM work_items item
  JOIN pipeline_run_steps step ON step.id=item.run_step_id
  WHERE item.project_id='30000000-0000-0000-0000-000000000025'
    AND item.status='ready'
    AND step.business_code='construction.item_reporting'
    AND item.role_ids='["Giám sát"]'::jsonb
    AND item.evidence_requirement_json->>'checklist_required'='true'
    AND item.evidence_requirement_json->>'document_required'='false'
    AND item.evidence_requirement_json->>'photo_required'='true';
  IF actual <> 1 THEN RAISE EXCEPTION 'Rev4 construction fixture 025 eligible reporting item is invalid'; END IF;

  SELECT count(*) INTO actual FROM work_items
  WHERE project_id='30000000-0000-0000-0000-000000000025' AND status='ready';
  IF actual <> 1 THEN RAISE EXCEPTION 'Rev4 construction fixture 025 must expose exactly one ready work item, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000025'
    AND status<>'pending' AND business_code<>'construction_execution';
  IF actual <> 0 THEN RAISE EXCEPTION 'Rev4 construction fixture 025 must not fabricate completed workflow nodes'; END IF;

  SELECT count(*) INTO actual FROM contracts
  WHERE id='47000000-0000-0000-0000-000000000025'
    AND project_id='30000000-0000-0000-0000-000000000025'
    AND contract_code='HD-REV4-UAT-001' AND contract_type='main_client'
    AND direction='receivable' AND status='active';
  IF actual <> 1 THEN RAISE EXCEPTION 'Rev4 construction fixture 025 active client contract is invalid'; END IF;

  SELECT
    (SELECT count(*) FROM operation_daily_reports WHERE project_id='30000000-0000-0000-0000-000000000025') +
    (SELECT count(*) FROM construction_checks WHERE project_id='30000000-0000-0000-0000-000000000025') +
    (SELECT count(*) FROM construction_action_items WHERE project_id='30000000-0000-0000-0000-000000000025') +
    (SELECT count(*) FROM contract_variations WHERE contract_id='47000000-0000-0000-0000-000000000025')
  INTO actual;
  IF actual <> 0 THEN RAISE EXCEPTION 'Rev4 construction fixture 025 must begin without UAT-generated Operations records, got %',actual; END IF;

  -- Rev6 lean-form runtime fixture: the intake decision is truthfully completed,
  -- survey is current, and all later runtime work remains untouched.
  SELECT count(*) INTO actual
  FROM projects project
  JOIN pipeline_runs run ON run.id=project.current_pipeline_run_id
  JOIN pipeline_templates template ON template.id=run.template_id
  JOIN pipeline_run_nodes current_node ON current_node.id=run.current_node_id
  WHERE project.id='30000000-0000-0000-0000-000000000026'
    AND project.code='GOLDEN-REV6-LEAN-001'
    AND project.pipeline_template_id='40000000-0000-0000-0000-000000000025'
    AND project.workflow_entry_mode='survey_required'
    AND project.status='planning' AND project.progress_percent=2
    AND run.id='42000000-0000-0000-0000-000000000026'
    AND run.template_id='40000000-0000-0000-0000-000000000025'
    AND run.engine_version=4 AND run.status='in_progress'
    AND run.template_definition_hash=template.definition_hash
    AND current_node.business_code='survey' AND current_node.status='in_progress';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Rev6 lean project/run snapshot contract is invalid'; END IF;

  SELECT count(*) INTO actual FROM project_members
  WHERE project_id='30000000-0000-0000-0000-000000000026' AND active;
  SELECT count(*) INTO expected
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
  IF actual <> expected THEN RAISE EXCEPTION 'Golden Rev6 project members: expected %, got %',expected,actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000026';
  IF actual <> 15 THEN RAISE EXCEPTION 'Golden Rev6 runtime nodes: expected 15, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000026'
    AND status='completed' AND business_code='project_intake';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Rev6 intake node must be completed'; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000026'
    AND status='pending' AND business_code NOT IN ('project_intake','survey');
  IF actual <> 13 THEN RAISE EXCEPTION 'Golden Rev6 later pending nodes: expected 13, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000026';
  IF actual <> 51 THEN RAISE EXCEPTION 'Golden Rev6 runtime steps: expected 51, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps run_step
  JOIN pipeline_template_steps template_step ON template_step.id=run_step.template_step_id
  WHERE run_step.project_id='30000000-0000-0000-0000-000000000026'
    AND (
      run_step.form_required<>template_step.form_required
      OR run_step.form_version_id IS DISTINCT FROM template_step.form_version_id
    );
  IF actual <> 0 THEN RAISE EXCEPTION 'Golden Rev6 runtime form snapshots drift from Rev6 template: %',actual; END IF;

  SELECT count(*) INTO actual FROM work_items
  WHERE project_id='30000000-0000-0000-0000-000000000026';
  IF actual <> 51 THEN RAISE EXCEPTION 'Golden Rev6 work items: expected 51, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM work_items item
  JOIN pipeline_run_steps step ON step.id=item.run_step_id
  WHERE item.project_id='30000000-0000-0000-0000-000000000026'
    AND item.status='completed' AND step.business_code='project_intake.confirm';
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Rev6 intake work item must be completed'; END IF;

  SELECT count(*) INTO actual
  FROM work_items item
  JOIN pipeline_run_nodes node ON node.id=item.run_node_id
  WHERE item.project_id='30000000-0000-0000-0000-000000000026'
    AND item.status='ready' AND node.business_code='survey';
  IF actual <> 6 THEN RAISE EXCEPTION 'Golden Rev6 ready survey work items: expected 6, got %',actual; END IF;

  SELECT count(*) INTO actual FROM work_items
  WHERE project_id='30000000-0000-0000-0000-000000000026' AND status='pending';
  IF actual <> 44 THEN RAISE EXCEPTION 'Golden Rev6 pending work items: expected 44, got %',actual; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id='30000000-0000-0000-0000-000000000026';
  IF actual <> 50 THEN RAISE EXCEPTION 'Golden Rev6 form instances: expected 50, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM workflow_form_instances instance
  JOIN pipeline_run_steps step ON step.id=instance.run_step_id
  WHERE instance.project_id='30000000-0000-0000-0000-000000000026'
    AND step.business_code='project_intake.confirm'
    AND instance.status='submitted' AND instance.current_revision=1
    AND instance.answers_snapshot_json='{"entry_route":"survey_required"}'::jsonb
    AND EXISTS (
      SELECT 1
      FROM workflow_form_answers answer
      WHERE answer.instance_id=instance.id
        AND answer.field_key='entry_route'
        AND answer.value_json='"survey_required"'::jsonb
        AND answer.revision_no=1
    );
  IF actual <> 1 THEN RAISE EXCEPTION 'Golden Rev6 submitted intake routing answer is invalid'; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id='30000000-0000-0000-0000-000000000026'
    AND status='draft' AND current_revision=0 AND answers_snapshot_json='{}'::jsonb;
  IF actual <> 49 THEN RAISE EXCEPTION 'Golden Rev6 draft forms: expected 49, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM workflow_form_instances instance
  JOIN workflow_form_versions version ON version.id=instance.form_version_id
  WHERE instance.project_id='30000000-0000-0000-0000-000000000026'
    AND version.schema_json->>'variant'='lean_capture_v2';
  IF actual <> 43 THEN RAISE EXCEPTION 'Golden Rev6 lean form instances: expected 43, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM workflow_form_instances instance
  JOIN workflow_form_versions version ON version.id=instance.form_version_id
  JOIN workflow_form_fields field ON field.form_version_id=version.id
  WHERE instance.project_id='30000000-0000-0000-0000-000000000026'
    AND version.schema_json->>'variant'='lean_capture_v2'
    AND field.field_key IN (
      'scope_summary','work_result','linked_evidence_id','next_action','confirmed'
    );
  IF actual <> 0 THEN RAISE EXCEPTION 'Golden Rev6 lean runtime still exposes % universal fields',actual; END IF;

  -- Rev7 operational fixture: duplicate capture is removed while source-only
  -- steps keep explicit action keys and no empty workflow forms.
  SELECT count(*) INTO actual
  FROM projects project
  JOIN pipeline_runs run ON run.id=project.current_pipeline_run_id
  JOIN pipeline_templates template ON template.id=run.template_id
  JOIN pipeline_run_nodes current_node ON current_node.id=run.current_node_id
  WHERE project.id='30000000-0000-0000-0000-000000000027'
    AND project.code='GOLDEN-REV7-OPS-001'
    AND project.pipeline_template_id='40000000-0000-0000-0000-000000000026'
    AND run.id='42000000-0000-0000-0000-000000000027'
    AND run.template_id='40000000-0000-0000-0000-000000000026'
    AND run.engine_version=4 AND run.template_definition_hash=template.definition_hash
    AND current_node.business_code='survey' AND current_node.status='in_progress';
  IF actual<>1 THEN RAISE EXCEPTION 'Golden Rev7 operational project/run snapshot contract is invalid'; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000027';
  IF actual<>15 THEN RAISE EXCEPTION 'Golden Rev7 runtime nodes: expected 15, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000027';
  IF actual<>50 THEN RAISE EXCEPTION 'Golden Rev7 runtime steps: expected 50, got %',actual; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id='30000000-0000-0000-0000-000000000027';
  IF actual<>27 THEN RAISE EXCEPTION 'Golden Rev7 form instances: expected 27, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000027'
    AND form_version_id IS NULL AND domain_action_key IS NOT NULL;
  IF actual<>22 THEN RAISE EXCEPTION 'Golden Rev7 source/action-only steps: expected 22, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM workflow_form_instances instance
  JOIN workflow_form_versions version ON version.id=instance.form_version_id
  JOIN workflow_form_fields field ON field.form_version_id=version.id
  WHERE instance.project_id='30000000-0000-0000-0000-000000000027'
    AND field.field_key IN (
      'confirmed','progress_percent','site_photo_evidence_id','output_invoice_no',
      'boq_version','quote_version','invoice_status'
    );
  IF actual<>0 THEN RAISE EXCEPTION 'Golden Rev7 still exposes % removed administrative fields',actual; END IF;

  -- Rev8 lean fixture: the same 15-node lifecycle is expressed as 21
  -- accountable work packages with exactly one primary role per step.
  SELECT count(*) INTO actual
  FROM projects project
  JOIN pipeline_runs run ON run.id=project.current_pipeline_run_id
  JOIN pipeline_templates template ON template.id=run.template_id
  JOIN pipeline_run_nodes current_node ON current_node.id=run.current_node_id
  WHERE project.id='30000000-0000-0000-0000-000000000028'
    AND project.code='GOLDEN-REV8-LEAN-001'
    AND project.pipeline_template_id='40000000-0000-0000-0000-000000000027'
    AND run.id='42000000-0000-0000-0000-000000000028'
    AND run.template_id='40000000-0000-0000-0000-000000000027'
    AND run.engine_version=4 AND run.template_definition_hash=template.definition_hash
    AND current_node.business_code='survey' AND current_node.status='in_progress';
  IF actual<>1 THEN RAISE EXCEPTION 'Golden Rev8 lean project/run snapshot contract is invalid'; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000028';
  IF actual<>15 THEN RAISE EXCEPTION 'Golden Rev8 runtime nodes: expected 15, got %',actual; END IF;

  SELECT count(*) INTO actual FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000028';
  IF actual<>21 THEN RAISE EXCEPTION 'Golden Rev8 runtime steps: expected 21, got %',actual; END IF;

  SELECT count(*) INTO actual FROM workflow_form_instances
  WHERE project_id='30000000-0000-0000-0000-000000000028';
  IF actual<>15 THEN RAISE EXCEPTION 'Golden Rev8 form instances: expected 15, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000028'
    AND form_version_id IS NULL AND domain_action_key IS NOT NULL;
  IF actual<>6 THEN RAISE EXCEPTION 'Golden Rev8 source/action-only steps: expected 6, got %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_steps
  WHERE project_id='30000000-0000-0000-0000-000000000028'
    AND jsonb_array_length(required_role_ids)<>1;
  IF actual<>0 THEN RAISE EXCEPTION 'Golden Rev8 steps without one primary role: %',actual; END IF;

  SELECT count(*) INTO actual
  FROM pipeline_run_nodes
  WHERE project_id='30000000-0000-0000-0000-000000000028'
    AND approval_policy_json<>'{}'::jsonb;
  IF actual<>2 THEN RAISE EXCEPTION 'Golden Rev8 approval nodes: expected 2, got %',actual; END IF;

  SELECT
    (SELECT count(*) FROM evidence_records WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM documents WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM approval_requests WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM boq_items WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM supplier_quotations WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM procurement_requests WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM contracts WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM stock_movements WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM site_material_balances WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM operation_daily_reports WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM quality_itp_plans WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM quality_inspection_requests WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM quality_ncrs WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM construction_check_templates WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM construction_checks WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM construction_action_items WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM incoming_acceptance_records WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM acceptance_records WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM rework_requests WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM payables WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM receivables WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM payments WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM input_invoices WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM output_invoices WHERE project_id='30000000-0000-0000-0000-000000000022') +
    (SELECT count(*) FROM handover_records WHERE project_id='30000000-0000-0000-0000-000000000022')
  INTO actual;
  IF actual <> 0 THEN RAISE EXCEPTION 'Golden Live must not contain pre-completed domain ledger rows, got %',actual; END IF;
END $$;

SELECT
  p.code,
  (SELECT count(*) FROM pipeline_run_nodes n WHERE n.project_id=p.id) AS nodes,
  (SELECT count(*) FROM pipeline_run_steps s WHERE s.project_id=p.id) AS steps,
  (SELECT count(*) FROM work_items w WHERE w.project_id=p.id) AS work_items,
  (SELECT count(*) FROM workflow_form_instances f WHERE f.project_id=p.id AND f.status='draft') AS draft_forms,
  (SELECT count(*) FROM workflow_form_instances f WHERE f.project_id=p.id AND f.status='submitted') AS submitted_forms,
  (SELECT count(*) FROM evidence_records e WHERE e.project_id=p.id AND e.status='accepted') AS accepted_evidence
FROM projects p
WHERE p.code IN (
  'GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001','GOLDEN-LIVE-001','GOLDEN-REV3-BOQ-001',
  'GOLDEN-REV4-ENTRY-001','GOLDEN-REV4-CONSTRUCTION-001','GOLDEN-REV6-LEAN-001'
)
ORDER BY p.code;

SELECT 'GOLDEN_SAMPLE_VALIDATION_PASSED' AS result;
