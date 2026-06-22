CREATE TABLE IF NOT EXISTS subcontractor_checklist_reconciliations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  form_revision_no INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',
  subcontractor_name TEXT,
  work_package TEXT,
  construction_area TEXT,
  summary_json JSONB NOT NULL DEFAULT '{}',
  comment TEXT,
  created_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT subcontractor_reconciliations_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT subcontractor_reconciliations_work_item_unique UNIQUE (work_item_id),
  CONSTRAINT subcontractor_reconciliations_form_unique UNIQUE (form_instance_id),
  CONSTRAINT subcontractor_reconciliations_status_check CHECK (status IN ('draft', 'submitted', 'reopened')),
  CONSTRAINT subcontractor_reconciliations_revision_nonnegative CHECK (form_revision_no >= 0)
);

CREATE INDEX IF NOT EXISTS idx_subcontractor_reconciliations_project_status
  ON subcontractor_checklist_reconciliations(project_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS subcontractor_checklist_reconciliation_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  reconciliation_id UUID NOT NULL REFERENCES subcontractor_checklist_reconciliations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  form_instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  item_index INTEGER NOT NULL,
  checklist_title TEXT NOT NULL,
  boq_item_id UUID REFERENCES boq_items(id),
  boq_item_ref TEXT,
  boq_work_item_name TEXT,
  boq_quantity NUMERIC(18,3),
  boq_unit TEXT,
  claimed_quantity NUMERIC(18,3),
  confirmed_quantity NUMERIC(18,3),
  variance_quantity NUMERIC(18,3),
  unit TEXT,
  status TEXT,
  evidence_id UUID REFERENCES evidence_records(id),
  owner_response TEXT,
  decision TEXT NOT NULL,
  due_date DATE,
  comment TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT subcontractor_reconciliation_items_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT subcontractor_reconciliation_items_position_unique UNIQUE (reconciliation_id, item_index),
  CONSTRAINT subcontractor_reconciliation_items_decision_check CHECK (decision IN (
    'accepted',
    'adjust_quantity',
    'rework_required',
    'waiting_evidence',
    'rejected'
  )),
  CONSTRAINT subcontractor_reconciliation_items_status_check CHECK (status IS NULL OR status IN (
    'done',
    'not_applicable',
    'needs_follow_up',
    'blocked'
  )),
  CONSTRAINT subcontractor_reconciliation_items_owner_response_check CHECK (owner_response IS NULL OR owner_response IN (
    'accepted',
    'adjust_quantity',
    'rework_required',
    'waiting_evidence'
  )),
  CONSTRAINT subcontractor_reconciliation_items_index_nonnegative CHECK (item_index >= 0),
  CONSTRAINT subcontractor_reconciliation_items_claimed_nonnegative CHECK (claimed_quantity IS NULL OR claimed_quantity >= 0),
  CONSTRAINT subcontractor_reconciliation_items_confirmed_nonnegative CHECK (confirmed_quantity IS NULL OR confirmed_quantity >= 0),
  CONSTRAINT subcontractor_reconciliation_items_boq_quantity_nonnegative CHECK (boq_quantity IS NULL OR boq_quantity >= 0)
);

CREATE INDEX IF NOT EXISTS idx_subcontractor_reconciliation_items_reconciliation
  ON subcontractor_checklist_reconciliation_items(reconciliation_id, item_index);

CREATE INDEX IF NOT EXISTS idx_subcontractor_reconciliation_items_boq
  ON subcontractor_checklist_reconciliation_items(boq_item_id)
  WHERE boq_item_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subcontractor_reconciliation_items_evidence
  ON subcontractor_checklist_reconciliation_items(evidence_id)
  WHERE evidence_id IS NOT NULL;
