CREATE TABLE IF NOT EXISTS workflow_automation_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  pipeline_run_id UUID REFERENCES pipeline_runs(id) ON DELETE CASCADE,
  run_node_id UUID REFERENCES pipeline_run_nodes(id) ON DELETE SET NULL,
  run_step_id UUID REFERENCES pipeline_run_steps(id) ON DELETE SET NULL,
  work_item_id UUID REFERENCES work_items(id) ON DELETE SET NULL,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  action_key TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL,
  target_type TEXT,
  target_id UUID,
  blocker_reason TEXT,
  error_message TEXT,
  metadata_json JSONB NOT NULL DEFAULT '{}',
  actor_id UUID REFERENCES users(id),
  actor_name TEXT,
  run_count INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_automation_source_type_check CHECK (source_type IN (
    'work_item',
    'pipeline_run_step',
    'pipeline_run_node',
    'workflow_form'
  )),
  CONSTRAINT workflow_automation_status_check CHECK (status IN (
    'succeeded',
    'blocked',
    'failed',
    'skipped'
  )),
  CONSTRAINT workflow_automation_action_key_not_blank CHECK (btrim(action_key) <> ''),
  CONSTRAINT workflow_automation_idempotency_key_not_blank CHECK (btrim(idempotency_key) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_workflow_automation_idempotency
  ON workflow_automation_events(idempotency_key);

CREATE UNIQUE INDEX IF NOT EXISTS ux_workflow_automation_source_action
  ON workflow_automation_events(source_type, source_id, action_key);

CREATE INDEX IF NOT EXISTS idx_workflow_automation_project_created
  ON workflow_automation_events(project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_workflow_automation_work_item
  ON workflow_automation_events(work_item_id, action_key, updated_at DESC)
  WHERE work_item_id IS NOT NULL;

UPDATE pipeline_template_edges
SET condition_type='procurement_approved',
    condition_config='{}'::jsonb,
    label='Đầu vào đã được duyệt'
WHERE id='41600000-0000-0000-0000-000000000007'
  AND template_id='40000000-0000-0000-0000-000000000020';

UPDATE pipeline_run_edges run_edge
SET condition_type='procurement_approved',
    condition_config='{}'::jsonb,
    label='Đầu vào đã được duyệt'
FROM pipeline_runs run
WHERE run_edge.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000020'
  AND run_edge.template_edge_id='41600000-0000-0000-0000-000000000007';
