ALTER TABLE pipeline_template_steps DROP CONSTRAINT IF EXISTS pipeline_template_steps_surface_kind_check;
ALTER TABLE pipeline_template_steps
  ADD CONSTRAINT pipeline_template_steps_surface_kind_check CHECK (
    surface_kind IS NULL OR surface_kind IN (
      'workflow_form','evidence','approval','incoming_acceptance','material_snapshot','surplus_handling',
      'outgoing_acceptance','owner_client_reconciliation','subcontractor_reconciliation','contract','quality',
      'finance','handover','generic'
    )
  );

ALTER TABLE pipeline_run_steps DROP CONSTRAINT IF EXISTS pipeline_run_steps_surface_kind_check;
ALTER TABLE pipeline_run_steps
  ADD CONSTRAINT pipeline_run_steps_surface_kind_check CHECK (
    surface_kind IS NULL OR surface_kind IN (
      'workflow_form','evidence','approval','incoming_acceptance','material_snapshot','surplus_handling',
      'outgoing_acceptance','owner_client_reconciliation','subcontractor_reconciliation','contract','quality',
      'finance','handover','generic'
    )
  );

UPDATE pipeline_template_steps
SET surface_kind=CASE WHEN id='41500000-0000-0000-0000-000000000052'::uuid THEN 'handover' ELSE 'finance' END
WHERE id IN (
  '41500000-0000-0000-0000-000000000042'::uuid,
  '41500000-0000-0000-0000-000000000044'::uuid,
  '41500000-0000-0000-0000-000000000045'::uuid,
  '41500000-0000-0000-0000-000000000048'::uuid,
  '41500000-0000-0000-0000-000000000049'::uuid,
  '41500000-0000-0000-0000-000000000050'::uuid,
  '41500000-0000-0000-0000-000000000051'::uuid,
  '41500000-0000-0000-0000-000000000052'::uuid
);

UPDATE pipeline_run_steps rs
SET surface_kind=ts.surface_kind
FROM pipeline_template_steps ts
WHERE ts.id=rs.template_step_id
  AND rs.status='pending'
  AND ts.id IN (
    '41500000-0000-0000-0000-000000000042'::uuid,
    '41500000-0000-0000-0000-000000000044'::uuid,
    '41500000-0000-0000-0000-000000000045'::uuid,
    '41500000-0000-0000-0000-000000000048'::uuid,
    '41500000-0000-0000-0000-000000000049'::uuid,
    '41500000-0000-0000-0000-000000000050'::uuid,
    '41500000-0000-0000-0000-000000000051'::uuid,
    '41500000-0000-0000-0000-000000000052'::uuid
  );
