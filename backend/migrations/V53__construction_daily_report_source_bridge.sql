ALTER TABLE operation_daily_reports
  ADD COLUMN IF NOT EXISTS workflow_form_instance_id UUID REFERENCES workflow_form_instances(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS work_item_id UUID,
  ADD COLUMN IF NOT EXISTS structured_json JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS primary_evidence_id UUID REFERENCES evidence_records(id);

ALTER TABLE operation_daily_reports
  ADD CONSTRAINT operation_daily_reports_work_item_project_fk
  FOREIGN KEY (work_item_id,project_id) REFERENCES work_items(id,project_id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_daily_reports_form_instance
  ON operation_daily_reports(workflow_form_instance_id)
  WHERE workflow_form_instance_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_operation_daily_reports_work_item
  ON operation_daily_reports(work_item_id,report_date DESC)
  WHERE work_item_id IS NOT NULL;
