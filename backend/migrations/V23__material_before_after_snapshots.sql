CREATE TABLE IF NOT EXISTS material_before_after_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  snapshot_date DATE,
  construction_area TEXT,
  summary_json JSONB NOT NULL DEFAULT '{}',
  comment TEXT,
  applied_to_site_balance BOOLEAN NOT NULL DEFAULT false,
  created_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT material_snapshots_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT material_snapshots_work_item_unique UNIQUE (work_item_id),
  CONSTRAINT material_snapshots_status_check CHECK (status IN ('draft', 'submitted', 'reopened'))
);

CREATE INDEX IF NOT EXISTS idx_material_snapshots_project_status
  ON material_before_after_snapshots(project_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS material_before_after_snapshot_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  snapshot_id UUID NOT NULL REFERENCES material_before_after_snapshots(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  item_index INTEGER NOT NULL,
  material_id UUID REFERENCES materials(id),
  material_code TEXT,
  material_name TEXT NOT NULL,
  unit TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT 'site_balance',
  before_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  added_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  after_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  waste_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  used_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  affects_site_balance BOOLEAN NOT NULL DEFAULT true,
  site_balance_id UUID REFERENCES site_material_balances(id),
  before_evidence_id UUID REFERENCES evidence_records(id),
  after_evidence_id UUID REFERENCES evidence_records(id),
  construction_area TEXT,
  condition TEXT NOT NULL DEFAULT 'usable',
  comment TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT material_snapshot_items_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT material_snapshot_items_position_unique UNIQUE (snapshot_id, item_index),
  CONSTRAINT material_snapshot_items_index_nonnegative CHECK (item_index >= 0),
  CONSTRAINT material_snapshot_items_source_check CHECK (source IN (
    'site_balance',
    'warehouse_issue',
    'subcontractor',
    'other'
  )),
  CONSTRAINT material_snapshot_items_condition_check CHECK (condition IN (
    'usable',
    'damaged',
    'mixed',
    'unknown'
  )),
  CONSTRAINT material_snapshot_items_before_nonnegative CHECK (before_quantity >= 0),
  CONSTRAINT material_snapshot_items_added_nonnegative CHECK (added_quantity >= 0),
  CONSTRAINT material_snapshot_items_after_nonnegative CHECK (after_quantity >= 0),
  CONSTRAINT material_snapshot_items_waste_nonnegative CHECK (waste_quantity >= 0),
  CONSTRAINT material_snapshot_items_used_nonnegative CHECK (used_quantity >= 0),
  CONSTRAINT material_snapshot_items_balance_material_required CHECK (
    NOT affects_site_balance OR (material_id IS NOT NULL AND site_balance_id IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_material_snapshot_items_snapshot
  ON material_before_after_snapshot_items(snapshot_id, item_index);

CREATE INDEX IF NOT EXISTS idx_material_snapshot_items_material
  ON material_before_after_snapshot_items(project_id, material_id)
  WHERE material_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_material_snapshot_items_site_balance
  ON material_before_after_snapshot_items(site_balance_id)
  WHERE site_balance_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_material_snapshot_items_before_evidence
  ON material_before_after_snapshot_items(before_evidence_id)
  WHERE before_evidence_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_material_snapshot_items_after_evidence
  ON material_before_after_snapshot_items(after_evidence_id)
  WHERE after_evidence_id IS NOT NULL;
