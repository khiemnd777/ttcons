CREATE TABLE IF NOT EXISTS site_surplus_handlings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  handling_code TEXT NOT NULL UNIQUE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID NOT NULL,
  site_balance_id UUID NOT NULL REFERENCES site_material_balances(id),
  material_id UUID NOT NULL REFERENCES materials(id),
  material_snapshot_id UUID REFERENCES material_before_after_snapshots(id),
  material_snapshot_item_id UUID REFERENCES material_before_after_snapshot_items(id),
  stock_movement_id UUID REFERENCES stock_movements(id),
  status TEXT NOT NULL DEFAULT 'draft',
  handling_type TEXT NOT NULL DEFAULT 'return_to_warehouse',
  quantity NUMERIC(18,3) NOT NULL,
  unit TEXT NOT NULL,
  warehouse_id UUID REFERENCES warehouses(id),
  custody_location TEXT,
  condition TEXT NOT NULL DEFAULT 'usable',
  evidence_id UUID REFERENCES evidence_records(id),
  comment TEXT,
  payload_json JSONB NOT NULL DEFAULT '{}',
  created_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT site_surplus_handlings_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT site_surplus_handlings_status_check CHECK (status IN ('draft', 'submitted', 'cancelled')),
  CONSTRAINT site_surplus_handlings_type_check CHECK (handling_type IN (
    'return_to_warehouse',
    'return_to_subcontractor'
  )),
  CONSTRAINT site_surplus_handlings_condition_check CHECK (condition IN (
    'usable',
    'damaged',
    'mixed',
    'unknown'
  )),
  CONSTRAINT site_surplus_handlings_quantity_positive CHECK (quantity > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_site_surplus_handlings_draft_balance
  ON site_surplus_handlings(work_item_id, site_balance_id)
  WHERE status='draft';

CREATE INDEX IF NOT EXISTS idx_site_surplus_handlings_work_item
  ON site_surplus_handlings(work_item_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_site_surplus_handlings_project_status
  ON site_surplus_handlings(project_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_site_surplus_handlings_site_balance
  ON site_surplus_handlings(site_balance_id);

CREATE INDEX IF NOT EXISTS idx_site_surplus_handlings_stock_movement
  ON site_surplus_handlings(stock_movement_id)
  WHERE stock_movement_id IS NOT NULL;
