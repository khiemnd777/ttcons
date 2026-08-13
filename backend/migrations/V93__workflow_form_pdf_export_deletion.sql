CREATE TABLE IF NOT EXISTS workflow_form_pdf_export_deletions (
  pdf_export_id UUID PRIMARY KEY REFERENCES workflow_form_pdf_exports(id) ON DELETE RESTRICT,
  status TEXT NOT NULL DEFAULT 'pending',
  requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
  requested_by_name TEXT,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  attempt_count INTEGER NOT NULL DEFAULT 1,
  last_error TEXT,
  storage_deleted_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_pdf_export_deletions_status_check
    CHECK (status IN ('pending', 'failed', 'deleted')),
  CONSTRAINT workflow_form_pdf_export_deletions_attempt_count_positive
    CHECK (attempt_count > 0),
  CONSTRAINT workflow_form_pdf_export_deletions_completed_check CHECK (
    (status = 'deleted' AND storage_deleted_at IS NOT NULL AND completed_at IS NOT NULL)
    OR
    (status <> 'deleted' AND completed_at IS NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_workflow_form_pdf_export_deletions_status
  ON workflow_form_pdf_export_deletions(status, updated_at DESC);

ALTER TABLE workflow_form_events
  DROP CONSTRAINT IF EXISTS workflow_form_events_type_check;

ALTER TABLE workflow_form_events
  ADD CONSTRAINT workflow_form_events_type_check CHECK (event_type IN (
    'created',
    'draft_saved',
    'submitted',
    'reopened',
    'cancelled',
    'signed',
    'pdf_exported',
    'pdf_export_deleted'
  ));

CREATE OR REPLACE FUNCTION prevent_acceptance_reference_to_deleting_pdf_export()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.pdf_export_id IS NOT NULL
    AND EXISTS (
      SELECT 1
      FROM workflow_form_pdf_export_deletions deletion
      WHERE deletion.pdf_export_id = NEW.pdf_export_id
        AND deletion.status IN ('pending', 'deleted')
    )
  THEN
    RAISE EXCEPTION 'acceptance cannot reference a deleting or deleted workflow form PDF export'
      USING ERRCODE = '23514', CONSTRAINT = 'acceptance_pdf_export_available';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_acceptance_pdf_export_available ON acceptance_records;

CREATE CONSTRAINT TRIGGER trg_acceptance_pdf_export_available
AFTER INSERT OR UPDATE ON acceptance_records
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION prevent_acceptance_reference_to_deleting_pdf_export();

CREATE OR REPLACE FUNCTION prevent_referenced_pdf_export_deletion()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.status IN ('pending', 'deleted')
    AND EXISTS (
      SELECT 1
      FROM acceptance_records acceptance
      WHERE acceptance.pdf_export_id = NEW.pdf_export_id
    )
  THEN
    RAISE EXCEPTION 'referenced workflow form PDF export cannot be deleted'
      USING ERRCODE = '23514', CONSTRAINT = 'workflow_form_pdf_export_deletion_unreferenced';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_workflow_form_pdf_export_deletion_unreferenced
  ON workflow_form_pdf_export_deletions;

CREATE CONSTRAINT TRIGGER trg_workflow_form_pdf_export_deletion_unreferenced
AFTER INSERT OR UPDATE ON workflow_form_pdf_export_deletions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION prevent_referenced_pdf_export_deletion();
