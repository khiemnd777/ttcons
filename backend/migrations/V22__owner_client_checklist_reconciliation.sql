CREATE TABLE IF NOT EXISTS owner_client_checklist_reconciliations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  form_revision_no INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',
  client_representative TEXT,
  company_representative TEXT,
  handover_scope TEXT,
  scope_detail TEXT,
  summary_json JSONB NOT NULL DEFAULT '{}',
  comment TEXT,
  created_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT owner_client_reconciliations_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT owner_client_reconciliations_work_item_unique UNIQUE (work_item_id),
  CONSTRAINT owner_client_reconciliations_form_unique UNIQUE (form_instance_id),
  CONSTRAINT owner_client_reconciliations_status_check CHECK (status IN ('draft', 'submitted', 'reopened')),
  CONSTRAINT owner_client_reconciliations_revision_nonnegative CHECK (form_revision_no >= 0)
);

CREATE INDEX IF NOT EXISTS idx_owner_client_reconciliations_project_status
  ON owner_client_checklist_reconciliations(project_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS owner_client_checklist_reconciliation_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  reconciliation_id UUID NOT NULL REFERENCES owner_client_checklist_reconciliations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  item_index INTEGER NOT NULL,
  checklist_title TEXT NOT NULL,
  output_ref TEXT,
  status TEXT,
  client_decision TEXT,
  severity TEXT,
  required_action TEXT,
  due_date DATE,
  evidence_id UUID REFERENCES evidence_records(id),
  acceptance_id UUID REFERENCES acceptance_records(id),
  acceptance_title TEXT,
  acceptance_type TEXT,
  acceptance_status TEXT,
  acceptance_result TEXT,
  decision TEXT NOT NULL,
  comment TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT owner_client_reconciliation_items_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT owner_client_reconciliation_items_position_unique UNIQUE (reconciliation_id, item_index),
  CONSTRAINT owner_client_reconciliation_items_index_nonnegative CHECK (item_index >= 0),
  CONSTRAINT owner_client_reconciliation_items_status_check CHECK (status IS NULL OR status IN (
    'done',
    'not_applicable',
    'needs_follow_up',
    'blocked'
  )),
  CONSTRAINT owner_client_reconciliation_items_client_decision_check CHECK (client_decision IS NULL OR client_decision IN (
    'pass',
    'conditional_pass',
    'fail',
    'not_applicable'
  )),
  CONSTRAINT owner_client_reconciliation_items_severity_check CHECK (severity IS NULL OR severity IN (
    'low',
    'medium',
    'high',
    'critical'
  )),
  CONSTRAINT owner_client_reconciliation_items_decision_check CHECK (decision IN (
    'accepted',
    'conditional_acceptance',
    'rework_required',
    'waiting_internal_acceptance',
    'rejected',
    'not_applicable'
  ))
);

CREATE INDEX IF NOT EXISTS idx_owner_client_reconciliation_items_reconciliation
  ON owner_client_checklist_reconciliation_items(reconciliation_id, item_index);

CREATE INDEX IF NOT EXISTS idx_owner_client_reconciliation_items_acceptance
  ON owner_client_checklist_reconciliation_items(acceptance_id)
  WHERE acceptance_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_owner_client_reconciliation_items_evidence
  ON owner_client_checklist_reconciliation_items(evidence_id)
  WHERE evidence_id IS NOT NULL;
