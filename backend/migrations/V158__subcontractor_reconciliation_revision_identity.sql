-- Retain every existing ledger and payload; allow a distinct amended revision.
ALTER TABLE subcontractor_checklist_reconciliations
  DROP CONSTRAINT subcontractor_reconciliations_work_item_unique,
  DROP CONSTRAINT subcontractor_reconciliations_form_unique,
  ADD CONSTRAINT subcontractor_reconciliations_source_revision_unique
    UNIQUE (project_id, work_item_id, form_instance_id, form_revision_no),
  ADD CONSTRAINT subcontractor_reconciliations_form_revision_unique
    UNIQUE (form_instance_id, form_revision_no);
