ALTER TABLE pipeline_template_nodes
  ADD COLUMN IF NOT EXISTS approval_policy_json JSONB NOT NULL DEFAULT '{}';

ALTER TABLE pipeline_run_nodes
  ADD COLUMN IF NOT EXISTS approval_policy_json JSONB NOT NULL DEFAULT '{}';

ALTER TABLE approval_requests
  ADD COLUMN IF NOT EXISTS policy_json JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS current_stage INTEGER NOT NULL DEFAULT 0;

ALTER TABLE approval_actions
  ADD COLUMN IF NOT EXISTS role_label TEXT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'approval_requests_current_stage_nonnegative'
  ) THEN
    ALTER TABLE approval_requests
      ADD CONSTRAINT approval_requests_current_stage_nonnegative CHECK (current_stage >= 0);
  END IF;
END $$;

UPDATE pipeline_template_nodes n
SET approval_policy_json = jsonb_build_object(
  'mode', 'any',
  'roles', COALESCE(n.responsible_role_ids, '[]'::jsonb),
  'quorum', 1,
  'requester_cannot_approve', true,
  'comment_required', true,
  'signature_required', false
)
WHERE n.template_id='40000000-0000-0000-0000-000000000020'
  AND NULLIF(BTRIM(COALESCE(n.approval_rule,'')), '') IS NOT NULL
  AND n.approval_policy_json='{}'::jsonb;

UPDATE pipeline_run_nodes rn
SET approval_policy_json=tn.approval_policy_json
FROM pipeline_template_nodes tn
WHERE tn.id=rn.template_node_id
  AND rn.approval_policy_json='{}'::jsonb;

WITH hard_actions(step_id, action_key) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000020'::uuid, 'procurement.generate_from_boq'),
    ('41500000-0000-0000-0000-000000000021'::uuid, 'procurement.generate_from_boq'),
    ('41500000-0000-0000-0000-000000000025'::uuid, 'procurement.require_approved'),
    ('41500000-0000-0000-0000-000000000045'::uuid, 'finance.scaffold_incoming_acceptance'),
    ('41500000-0000-0000-0000-000000000050'::uuid, 'finance.scaffold_outgoing_acceptance'),
    ('41500000-0000-0000-0000-000000000052'::uuid, 'handover.evaluate_readiness')
)
UPDATE pipeline_template_steps s
SET domain_action_key=a.action_key
FROM hard_actions a
WHERE s.id=a.step_id;

UPDATE pipeline_run_steps rs
SET domain_action_key=ts.domain_action_key
FROM pipeline_template_steps ts
WHERE ts.id=rs.template_step_id
  AND rs.domain_action_key IS DISTINCT FROM ts.domain_action_key;

CREATE INDEX IF NOT EXISTS idx_workflow_automation_required_gate
  ON workflow_automation_events(run_node_id, status, updated_at DESC)
  WHERE status IN ('blocked','failed');
