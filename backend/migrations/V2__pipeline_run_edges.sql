CREATE TABLE IF NOT EXISTS pipeline_run_edges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  template_edge_id UUID REFERENCES pipeline_template_edges(id),
  from_run_node_id UUID NOT NULL REFERENCES pipeline_run_nodes(id) ON DELETE CASCADE,
  to_run_node_id UUID NOT NULL REFERENCES pipeline_run_nodes(id) ON DELETE CASCADE,
  condition_type TEXT NOT NULL DEFAULT 'always',
  condition_config JSONB NOT NULL DEFAULT '{}',
  label TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_run_edges_run_from ON pipeline_run_edges(run_id, from_run_node_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_run_edges_run_to ON pipeline_run_edges(run_id, to_run_node_id);
