CREATE INDEX IF NOT EXISTS idx_project_explorer_outbox_published_retention
  ON project_explorer_invalidation_outbox(published_at, id)
  WHERE published_at IS NOT NULL;
