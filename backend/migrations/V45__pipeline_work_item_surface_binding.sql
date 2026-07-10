ALTER TABLE pipeline_template_steps
  ADD COLUMN IF NOT EXISTS surface_kind TEXT,
  ADD COLUMN IF NOT EXISTS domain_action_key TEXT;

ALTER TABLE pipeline_run_steps
  ADD COLUMN IF NOT EXISTS surface_kind TEXT,
  ADD COLUMN IF NOT EXISTS domain_action_key TEXT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_template_steps_surface_kind_check'
  ) THEN
    ALTER TABLE pipeline_template_steps
      ADD CONSTRAINT pipeline_template_steps_surface_kind_check CHECK (
        surface_kind IS NULL OR surface_kind IN (
          'workflow_form', 'evidence', 'approval', 'incoming_acceptance',
          'material_snapshot', 'surplus_handling', 'outgoing_acceptance',
          'owner_client_reconciliation', 'subcontractor_reconciliation', 'generic'
        )
      );
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'pipeline_run_steps_surface_kind_check'
  ) THEN
    ALTER TABLE pipeline_run_steps
      ADD CONSTRAINT pipeline_run_steps_surface_kind_check CHECK (
        surface_kind IS NULL OR surface_kind IN (
          'workflow_form', 'evidence', 'approval', 'incoming_acceptance',
          'material_snapshot', 'surplus_handling', 'outgoing_acceptance',
          'owner_client_reconciliation', 'subcontractor_reconciliation', 'generic'
        )
      );
  END IF;
END $$;

UPDATE pipeline_template_steps s
SET surface_kind = CASE
      WHEN s.id='41500000-0000-0000-0000-000000000002' THEN 'evidence'
      WHEN s.id='41500000-0000-0000-0000-000000000038' THEN 'subcontractor_reconciliation'
      WHEN s.id='41500000-0000-0000-0000-000000000039' THEN 'incoming_acceptance'
      WHEN s.id='41500000-0000-0000-0000-000000000046' THEN 'owner_client_reconciliation'
      WHEN s.step_type='approval' THEN 'approval'
      WHEN COALESCE(s.form_required, false) THEN 'workflow_form'
      ELSE 'generic'
    END,
    domain_action_key = CASE
      WHEN s.id='41500000-0000-0000-0000-000000000038' THEN 'subcontractor.reconcile_checklist'
      WHEN s.id='41500000-0000-0000-0000-000000000039' THEN 'acceptance.incoming_material'
      WHEN s.id='41500000-0000-0000-0000-000000000040' THEN 'subcontractor.prepare_quantity_dossier'
      WHEN s.id='41500000-0000-0000-0000-000000000046' THEN 'owner.reconcile_acceptance'
      WHEN s.id='41500000-0000-0000-0000-000000000049' THEN 'receivable.issue_invoice'
      WHEN s.id='41500000-0000-0000-0000-000000000050' THEN 'receivable.record_payment'
      WHEN s.id='41500000-0000-0000-0000-000000000051' THEN 'payable.verify_input_invoice'
      WHEN s.id='41500000-0000-0000-0000-000000000052' THEN 'handover.close_output_invoice'
      ELSE NULL
    END
WHERE s.node_id IN (
  SELECT id FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000020'
);

UPDATE pipeline_run_steps rs
SET surface_kind=ts.surface_kind,
    domain_action_key=ts.domain_action_key
FROM pipeline_template_steps ts
WHERE ts.id=rs.template_step_id
  AND (rs.surface_kind IS DISTINCT FROM ts.surface_kind OR rs.domain_action_key IS DISTINCT FROM ts.domain_action_key);
