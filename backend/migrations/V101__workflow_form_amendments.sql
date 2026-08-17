ALTER TABLE workflow_form_events
  DROP CONSTRAINT IF EXISTS workflow_form_events_type_check;

ALTER TABLE workflow_form_events
  ADD CONSTRAINT workflow_form_events_type_check CHECK (event_type IN (
    'created',
    'draft_saved',
    'submitted',
    'reopened',
    'amended',
    'cancelled',
    'signed',
    'seal_applied',
    'pdf_exported',
    'pdf_export_deleted'
  ));

ALTER TABLE workflow_form_signatures
  DROP CONSTRAINT IF EXISTS workflow_form_signatures_unique_signer_purpose;

ALTER TABLE workflow_form_signatures
  ADD CONSTRAINT workflow_form_signatures_unique_signer_purpose_revision
  UNIQUE (form_instance_id, form_revision_no, signer_user_id, purpose);

ALTER TABLE workflow_form_seal_applications
  DROP CONSTRAINT IF EXISTS workflow_form_seal_applications_unique_form_purpose_asset;

ALTER TABLE workflow_form_seal_applications
  ADD CONSTRAINT workflow_form_seal_applications_unique_form_revision_purpose_asset
  UNIQUE (form_instance_id, form_revision_no, purpose, seal_asset_id);

CREATE INDEX IF NOT EXISTS idx_workflow_form_signatures_instance_revision
  ON workflow_form_signatures(form_instance_id, form_revision_no, signed_at DESC, id);

CREATE INDEX IF NOT EXISTS idx_workflow_form_seal_applications_instance_revision
  ON workflow_form_seal_applications(form_instance_id, form_revision_no, applied_at DESC, id);
