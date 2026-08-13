CREATE TABLE IF NOT EXISTS workflow_form_pdf_confirmation_preferences (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  purpose TEXT NOT NULL,
  include_signatures BOOLEAN NOT NULL DEFAULT TRUE,
  include_seals BOOLEAN NOT NULL DEFAULT TRUE,
  updated_by UUID REFERENCES users(id),
  updated_by_name TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_pdf_confirmation_preferences_purpose_check CHECK (purpose IN (
    'internal_ack',
    'prepared_by',
    'reviewed_by',
    'approved_by',
    'received_by'
  )),
  CONSTRAINT workflow_form_pdf_confirmation_preferences_at_least_one_check
    CHECK (include_signatures OR include_seals),
  CONSTRAINT workflow_form_pdf_confirmation_preferences_instance_purpose_unique
    UNIQUE (form_instance_id, purpose)
);

CREATE INDEX IF NOT EXISTS idx_workflow_form_pdf_confirmation_preferences_instance
  ON workflow_form_pdf_confirmation_preferences(form_instance_id, purpose);
