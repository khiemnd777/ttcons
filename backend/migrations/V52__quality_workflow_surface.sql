ALTER TABLE pipeline_template_steps DROP CONSTRAINT IF EXISTS pipeline_template_steps_surface_kind_check;
ALTER TABLE pipeline_template_steps
  ADD CONSTRAINT pipeline_template_steps_surface_kind_check CHECK (
    surface_kind IS NULL OR surface_kind IN (
      'workflow_form','evidence','approval','incoming_acceptance','material_snapshot','surplus_handling',
      'outgoing_acceptance','owner_client_reconciliation','subcontractor_reconciliation','contract','quality','generic'
    )
  );

ALTER TABLE pipeline_run_steps DROP CONSTRAINT IF EXISTS pipeline_run_steps_surface_kind_check;
ALTER TABLE pipeline_run_steps
  ADD CONSTRAINT pipeline_run_steps_surface_kind_check CHECK (
    surface_kind IS NULL OR surface_kind IN (
      'workflow_form','evidence','approval','incoming_acceptance','material_snapshot','surplus_handling',
      'outgoing_acceptance','owner_client_reconciliation','subcontractor_reconciliation','contract','quality','generic'
    )
  );

UPDATE pipeline_template_steps
SET surface_kind='quality',
    required_permission=CASE id
      WHEN '41500000-0000-0000-0000-000000000036'::uuid THEN 'quality.create'
      WHEN '41500000-0000-0000-0000-000000000037'::uuid THEN 'quality.recurring_manage'
      ELSE required_permission
    END
WHERE id IN (
  '41500000-0000-0000-0000-000000000036'::uuid,
  '41500000-0000-0000-0000-000000000037'::uuid
);

UPDATE pipeline_run_steps rs
SET surface_kind=ts.surface_kind,
    required_permission=ts.required_permission
FROM pipeline_template_steps ts
WHERE ts.id=rs.template_step_id
  AND rs.status='pending'
  AND ts.id IN (
    '41500000-0000-0000-0000-000000000036'::uuid,
    '41500000-0000-0000-0000-000000000037'::uuid
  );
