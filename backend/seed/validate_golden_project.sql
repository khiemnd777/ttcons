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
  FROM projects
  WHERE code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001')
    AND pipeline_template_id='40000000-0000-0000-0000-000000000020'
    AND status='completed' AND progress_percent=100;
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

  SELECT count(*) INTO actual FROM acceptance_records WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='completed' AND result='pass';
  IF actual <> 1 THEN RAISE EXCEPTION 'Final acceptance pass: expected 1, got %',actual; END IF;
  SELECT count(*) INTO actual FROM rework_requests WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='completed';
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
  SELECT count(*) INTO actual FROM payments WHERE project_id='30000000-0000-0000-0000-000000000020' AND status NOT IN ('paid','received');
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
    (SELECT count(*) FROM rework_requests WHERE project_id='30000000-0000-0000-0000-000000000020' AND status NOT IN ('completed','cancelled'))
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
  WHERE key LIKE 'contract.%' OR key LIKE 'quality.%' OR key='handover.verify';
  IF actual <> 15 THEN RAISE EXCEPTION 'Product-grade RBAC permissions after seed: expected 15, got %',actual; END IF;
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
    ('client@consflow.local','client')
  ) AND u.status='active';
  IF actual <> 5 THEN RAISE EXCEPTION 'Golden Live role accounts: expected 5, got %',actual; END IF;

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
  IF actual <> 12 THEN RAISE EXCEPTION 'Golden Live project members: expected 12, got %',actual; END IF;

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
WHERE p.code IN ('GOLDEN-REAL-001','GOLDEN-NO-DESIGN-001','GOLDEN-LIVE-001')
ORDER BY p.code;

SELECT 'GOLDEN_SAMPLE_VALIDATION_PASSED' AS result;
