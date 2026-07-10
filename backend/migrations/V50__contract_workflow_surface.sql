ALTER TABLE pipeline_template_steps DROP CONSTRAINT IF EXISTS pipeline_template_steps_surface_kind_check;
ALTER TABLE pipeline_template_steps
  ADD CONSTRAINT pipeline_template_steps_surface_kind_check CHECK (
    surface_kind IS NULL OR surface_kind IN (
      'workflow_form','evidence','approval','incoming_acceptance','material_snapshot','surplus_handling',
      'outgoing_acceptance','owner_client_reconciliation','subcontractor_reconciliation','contract','generic'
    )
  );

ALTER TABLE pipeline_run_steps DROP CONSTRAINT IF EXISTS pipeline_run_steps_surface_kind_check;
ALTER TABLE pipeline_run_steps
  ADD CONSTRAINT pipeline_run_steps_surface_kind_check CHECK (
    surface_kind IS NULL OR surface_kind IN (
      'workflow_form','evidence','approval','incoming_acceptance','material_snapshot','surplus_handling',
      'outgoing_acceptance','owner_client_reconciliation','subcontractor_reconciliation','contract','generic'
    )
  );

UPDATE pipeline_template_steps
SET surface_kind='contract',
    required_permission=CASE id
      WHEN '41500000-0000-0000-0000-000000000024'::uuid THEN 'contract.create'
      WHEN '41500000-0000-0000-0000-000000000029'::uuid THEN 'contract.create'
      WHEN '41500000-0000-0000-0000-000000000030'::uuid THEN 'contract.review'
      WHEN '41500000-0000-0000-0000-000000000031'::uuid THEN 'contract.review'
      WHEN '41500000-0000-0000-0000-000000000033'::uuid THEN 'contract.sign'
      ELSE required_permission
    END
WHERE id IN (
  '41500000-0000-0000-0000-000000000024'::uuid,
  '41500000-0000-0000-0000-000000000029'::uuid,
  '41500000-0000-0000-0000-000000000030'::uuid,
  '41500000-0000-0000-0000-000000000031'::uuid,
  '41500000-0000-0000-0000-000000000033'::uuid
);

UPDATE pipeline_template_steps
SET required_permission='contract.approve'
WHERE id='41500000-0000-0000-0000-000000000032'::uuid;

UPDATE pipeline_run_steps rs
SET surface_kind=ts.surface_kind,
    required_permission=ts.required_permission
FROM pipeline_template_steps ts
WHERE ts.id=rs.template_step_id
  AND rs.status='pending'
  AND ts.id IN (
    '41500000-0000-0000-0000-000000000024'::uuid,
    '41500000-0000-0000-0000-000000000029'::uuid,
    '41500000-0000-0000-0000-000000000030'::uuid,
    '41500000-0000-0000-0000-000000000031'::uuid,
    '41500000-0000-0000-0000-000000000032'::uuid,
    '41500000-0000-0000-0000-000000000033'::uuid
  );
