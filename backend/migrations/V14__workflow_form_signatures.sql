ALTER TABLE workflow_form_events
  DROP CONSTRAINT IF EXISTS workflow_form_events_type_check;

ALTER TABLE workflow_form_events
  ADD CONSTRAINT workflow_form_events_type_check CHECK (event_type IN (
    'created',
    'draft_saved',
    'submitted',
    'reopened',
    'cancelled',
    'signed'
  ));

CREATE TABLE IF NOT EXISTS workflow_form_signatures (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  form_version_id UUID NOT NULL REFERENCES workflow_form_versions(id),
  form_revision_no INTEGER NOT NULL,
  purpose TEXT NOT NULL DEFAULT 'internal_ack',
  signer_user_id UUID NOT NULL REFERENCES users(id),
  signer_name TEXT NOT NULL,
  signer_roles_json JSONB NOT NULL DEFAULT '[]',
  comment TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL,
  signed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_signatures_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT workflow_form_signatures_revision_nonnegative CHECK (form_revision_no >= 0),
  CONSTRAINT workflow_form_signatures_purpose_check CHECK (purpose IN (
    'internal_ack',
    'prepared_by',
    'reviewed_by',
    'approved_by',
    'received_by'
  )),
  CONSTRAINT workflow_form_signatures_hash_check CHECK (payload_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT workflow_form_signatures_unique_signer_purpose UNIQUE (form_instance_id, signer_user_id, purpose)
);

CREATE INDEX IF NOT EXISTS idx_workflow_form_signatures_instance
  ON workflow_form_signatures(form_instance_id, signed_at DESC);

CREATE INDEX IF NOT EXISTS idx_workflow_form_signatures_project
  ON workflow_form_signatures(project_id, signed_at DESC);

CREATE OR REPLACE FUNCTION prevent_workflow_form_signature_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'workflow_form_signatures are immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_workflow_form_signatures_immutable ON workflow_form_signatures;

CREATE TRIGGER trg_workflow_form_signatures_immutable
BEFORE UPDATE OR DELETE ON workflow_form_signatures
FOR EACH ROW
EXECUTE FUNCTION prevent_workflow_form_signature_mutation();
