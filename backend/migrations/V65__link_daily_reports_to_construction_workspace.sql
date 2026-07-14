ALTER TABLE construction_checks
  ADD COLUMN IF NOT EXISTS source_daily_report_id UUID REFERENCES operation_daily_reports(id) ON DELETE SET NULL;

ALTER TABLE construction_action_items
  ADD COLUMN IF NOT EXISTS source_daily_report_id UUID REFERENCES operation_daily_reports(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_construction_checks_source_daily_report
  ON construction_checks(source_daily_report_id,created_at DESC)
  WHERE source_daily_report_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_construction_actions_source_daily_report
  ON construction_action_items(source_daily_report_id,created_at DESC)
  WHERE source_daily_report_id IS NOT NULL;
