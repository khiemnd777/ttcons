ALTER TABLE acceptance_records
  ADD COLUMN IF NOT EXISTS acceptance_code TEXT,
  ADD COLUMN IF NOT EXISTS work_item_id UUID,
  ADD COLUMN IF NOT EXISTS form_instance_id UUID REFERENCES workflow_form_instances(id),
  ADD COLUMN IF NOT EXISTS pdf_export_id UUID REFERENCES workflow_form_pdf_exports(id),
  ADD COLUMN IF NOT EXISTS acceptance_date DATE,
  ADD COLUMN IF NOT EXISTS location TEXT,
  ADD COLUMN IF NOT EXISTS client_representative TEXT,
  ADD COLUMN IF NOT EXISTS contractor_representative TEXT,
  ADD COLUMN IF NOT EXISTS scope_summary TEXT,
  ADD COLUMN IF NOT EXISTS signature_status TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS document_status TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS summary_json JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS submitted_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

UPDATE acceptance_records
SET acceptance_code = 'OA-' || upper(substr(id::text, 1, 8))
WHERE acceptance_code IS NULL OR acceptance_code = '';

ALTER TABLE acceptance_records
  ALTER COLUMN acceptance_code SET NOT NULL;

ALTER TABLE acceptance_records
  ALTER COLUMN acceptance_code SET DEFAULT ('OA-' || upper(substr(gen_random_uuid()::text, 1, 8)));

ALTER TABLE acceptance_records
  DROP CONSTRAINT IF EXISTS acceptance_records_work_item_project_fk,
  ADD CONSTRAINT acceptance_records_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id);

ALTER TABLE acceptance_records
  DROP CONSTRAINT IF EXISTS acceptance_records_result_check,
  ADD CONSTRAINT acceptance_records_result_check CHECK (
    result IS NULL OR result IN ('pass', 'conditional_pass', 'fail')
  );

ALTER TABLE acceptance_records
  DROP CONSTRAINT IF EXISTS acceptance_records_signature_status_check,
  ADD CONSTRAINT acceptance_records_signature_status_check CHECK (
    signature_status IN ('pending', 'signed', 'waived')
  );

ALTER TABLE acceptance_records
  DROP CONSTRAINT IF EXISTS acceptance_records_document_status_check,
  ADD CONSTRAINT acceptance_records_document_status_check CHECK (
    document_status IN ('pending', 'linked', 'waived')
  );

CREATE UNIQUE INDEX IF NOT EXISTS ux_acceptance_records_code
  ON acceptance_records(acceptance_code);

CREATE INDEX IF NOT EXISTS idx_acceptance_records_work_item
  ON acceptance_records(work_item_id, status, updated_at DESC)
  WHERE work_item_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_acceptance_records_form_instance
  ON acceptance_records(form_instance_id)
  WHERE form_instance_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_acceptance_records_pdf_export
  ON acceptance_records(pdf_export_id)
  WHERE pdf_export_id IS NOT NULL;

ALTER TABLE acceptance_checklist_items
  ADD COLUMN IF NOT EXISTS item_index INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS evidence_id UUID REFERENCES evidence_records(id),
  ADD COLUMN IF NOT EXISTS decision TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS severity TEXT NOT NULL DEFAULT 'medium',
  ADD COLUMN IF NOT EXISTS required_action TEXT,
  ADD COLUMN IF NOT EXISTS due_date DATE,
  ADD COLUMN IF NOT EXISTS payload_json JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

WITH ranked AS (
  SELECT id, row_number() OVER (PARTITION BY acceptance_id ORDER BY created_at, id) - 1 AS next_index
  FROM acceptance_checklist_items
)
UPDATE acceptance_checklist_items item
SET item_index = ranked.next_index
FROM ranked
WHERE item.id = ranked.id;

UPDATE acceptance_checklist_items
SET decision = CASE
  WHEN is_passed THEN 'pass'
  WHEN decision IS NULL OR decision = '' THEN 'pending'
  ELSE decision
END;

ALTER TABLE acceptance_checklist_items
  DROP CONSTRAINT IF EXISTS acceptance_checklist_items_index_nonnegative,
  ADD CONSTRAINT acceptance_checklist_items_index_nonnegative CHECK (item_index >= 0);

ALTER TABLE acceptance_checklist_items
  DROP CONSTRAINT IF EXISTS acceptance_checklist_items_decision_check,
  ADD CONSTRAINT acceptance_checklist_items_decision_check CHECK (
    decision IN ('pending', 'pass', 'conditional_pass', 'fail')
  );

ALTER TABLE acceptance_checklist_items
  DROP CONSTRAINT IF EXISTS acceptance_checklist_items_severity_check,
  ADD CONSTRAINT acceptance_checklist_items_severity_check CHECK (
    severity IN ('low', 'medium', 'high', 'critical')
  );

CREATE UNIQUE INDEX IF NOT EXISTS ux_acceptance_checklist_items_position
  ON acceptance_checklist_items(acceptance_id, item_index);

CREATE INDEX IF NOT EXISTS idx_acceptance_checklist_items_evidence
  ON acceptance_checklist_items(evidence_id)
  WHERE evidence_id IS NOT NULL;
