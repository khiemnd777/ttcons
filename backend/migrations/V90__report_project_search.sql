CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_projects_report_search_trgm
  ON projects
  USING gin ((lower(btrim(code) || ' ' || btrim(name))) gin_trgm_ops)
  WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_projects_report_order
  ON projects ((lower(btrim(name))), (lower(btrim(code))), id)
  WHERE status <> 'cancelled';
