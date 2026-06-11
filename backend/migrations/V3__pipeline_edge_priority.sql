ALTER TABLE pipeline_template_edges
  ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 100;

ALTER TABLE pipeline_run_edges
  ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 100;

CREATE INDEX IF NOT EXISTS idx_pipeline_template_edges_template_from_priority
  ON pipeline_template_edges(template_id, from_node_id, priority, created_at);

CREATE INDEX IF NOT EXISTS idx_pipeline_run_edges_run_from_priority
  ON pipeline_run_edges(run_id, from_run_node_id, priority, created_at);
