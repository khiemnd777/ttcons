CREATE TABLE IF NOT EXISTS incoming_acceptance_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  acceptance_code TEXT NOT NULL UNIQUE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID,
  procurement_request_id UUID REFERENCES procurement_requests(id),
  warehouse_id UUID REFERENCES warehouses(id),
  source_type TEXT NOT NULL DEFAULT 'supplier',
  supplier_name TEXT,
  subcontractor_name TEXT,
  delivery_reference TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  result TEXT,
  received_at DATE,
  summary_json JSONB NOT NULL DEFAULT '{}',
  comment TEXT,
  created_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT incoming_acceptance_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT incoming_acceptance_status_check CHECK (status IN ('draft', 'submitted', 'cancelled')),
  CONSTRAINT incoming_acceptance_result_check CHECK (result IS NULL OR result IN (
    'pass',
    'conditional_pass',
    'fail'
  )),
  CONSTRAINT incoming_acceptance_source_check CHECK (source_type IN (
    'supplier',
    'subcontractor',
    'procurement',
    'other'
  ))
);

CREATE INDEX IF NOT EXISTS idx_incoming_acceptance_project_status
  ON incoming_acceptance_records(project_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_incoming_acceptance_work_item
  ON incoming_acceptance_records(work_item_id, status, updated_at DESC)
  WHERE work_item_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_incoming_acceptance_procurement
  ON incoming_acceptance_records(procurement_request_id)
  WHERE procurement_request_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS incoming_acceptance_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  incoming_acceptance_id UUID NOT NULL REFERENCES incoming_acceptance_records(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID,
  item_index INTEGER NOT NULL,
  material_id UUID REFERENCES materials(id),
  material_code TEXT,
  material_name TEXT NOT NULL,
  procurement_request_item_id UUID REFERENCES procurement_request_items(id),
  delivered_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  accepted_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  rejected_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  unit TEXT NOT NULL,
  condition TEXT NOT NULL DEFAULT 'usable',
  defect_note TEXT,
  evidence_id UUID REFERENCES evidence_records(id),
  decision TEXT NOT NULL DEFAULT 'pending',
  comment TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT incoming_acceptance_items_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT incoming_acceptance_items_position_unique UNIQUE (incoming_acceptance_id, item_index),
  CONSTRAINT incoming_acceptance_items_index_nonnegative CHECK (item_index >= 0),
  CONSTRAINT incoming_acceptance_items_condition_check CHECK (condition IN (
    'usable',
    'damaged',
    'mixed',
    'unknown'
  )),
  CONSTRAINT incoming_acceptance_items_decision_check CHECK (decision IN (
    'pending',
    'accepted',
    'conditional_acceptance',
    'rejected'
  )),
  CONSTRAINT incoming_acceptance_items_delivered_nonnegative CHECK (delivered_quantity >= 0),
  CONSTRAINT incoming_acceptance_items_accepted_nonnegative CHECK (accepted_quantity >= 0),
  CONSTRAINT incoming_acceptance_items_rejected_nonnegative CHECK (rejected_quantity >= 0),
  CONSTRAINT incoming_acceptance_items_accepted_rejected_within_delivered CHECK (
    accepted_quantity + rejected_quantity <= delivered_quantity
  )
);

CREATE INDEX IF NOT EXISTS idx_incoming_acceptance_items_record
  ON incoming_acceptance_items(incoming_acceptance_id, item_index);

CREATE INDEX IF NOT EXISTS idx_incoming_acceptance_items_material
  ON incoming_acceptance_items(project_id, material_id)
  WHERE material_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_incoming_acceptance_items_evidence
  ON incoming_acceptance_items(evidence_id)
  WHERE evidence_id IS NOT NULL;
