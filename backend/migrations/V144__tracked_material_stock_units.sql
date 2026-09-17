ALTER TABLE materials
  ADD COLUMN IF NOT EXISTS tracking_mode TEXT NOT NULL DEFAULT 'aggregate',
  ADD COLUMN IF NOT EXISTS form_factor TEXT,
  ADD COLUMN IF NOT EXISTS physical_tracking_initialized_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS physical_tracking_initialized_by UUID REFERENCES users(id);

ALTER TABLE materials
  DROP CONSTRAINT IF EXISTS materials_tracking_mode_check,
  ADD CONSTRAINT materials_tracking_mode_check
    CHECK (tracking_mode IN ('aggregate', 'physical_unit')),
  DROP CONSTRAINT IF EXISTS materials_form_factor_check,
  ADD CONSTRAINT materials_form_factor_check
    CHECK (form_factor IS NULL OR form_factor IN ('segment', 'roll', 'bar', 'sheet')),
  DROP CONSTRAINT IF EXISTS materials_physical_form_factor_check,
  ADD CONSTRAINT materials_physical_form_factor_check
    CHECK (tracking_mode <> 'physical_unit' OR form_factor IS NOT NULL);

CREATE TABLE IF NOT EXISTS material_stock_units (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  unit_code TEXT NOT NULL UNIQUE,
  material_id UUID NOT NULL REFERENCES materials(id),
  root_unit_id UUID REFERENCES material_stock_units(id),
  parent_unit_id UUID REFERENCES material_stock_units(id),
  origin_stock_document_item_id UUID REFERENCES stock_document_items(id),
  location_type TEXT NOT NULL,
  warehouse_id UUID REFERENCES warehouses(id),
  project_id UUID REFERENCES projects(id),
  work_item_id UUID REFERENCES work_items(id),
  quantity NUMERIC(18,3) NOT NULL,
  unit TEXT NOT NULL,
  length_mm NUMERIC(18,3),
  width_mm NUMERIC(18,3),
  thickness_mm NUMERIC(18,3),
  condition TEXT NOT NULL DEFAULT 'usable',
  status TEXT NOT NULL,
  custody_location TEXT,
  version INTEGER NOT NULL DEFAULT 1,
  created_by UUID REFERENCES users(id),
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT material_stock_units_quantity_nonnegative CHECK (quantity >= 0),
  CONSTRAINT material_stock_units_dimensions_positive CHECK (
    (length_mm IS NULL OR length_mm > 0)
    AND (width_mm IS NULL OR width_mm > 0)
    AND (thickness_mm IS NULL OR thickness_mm > 0)
  ),
  CONSTRAINT material_stock_units_location_type_check
    CHECK (location_type IN ('warehouse', 'project', 'external', 'consumed')),
  CONSTRAINT material_stock_units_condition_check
    CHECK (condition IN ('usable', 'damaged', 'mixed', 'unknown')),
  CONSTRAINT material_stock_units_status_check
    CHECK (status IN ('available', 'issued', 'consumed', 'quarantined', 'external', 'cancelled')),
  CONSTRAINT material_stock_units_location_check CHECK (
    (location_type='warehouse' AND warehouse_id IS NOT NULL AND project_id IS NULL)
    OR (location_type='project' AND project_id IS NOT NULL AND warehouse_id IS NULL)
    OR (location_type IN ('external','consumed') AND warehouse_id IS NULL AND project_id IS NULL)
  )
);

CREATE TABLE IF NOT EXISTS material_stock_unit_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stock_unit_id UUID NOT NULL REFERENCES material_stock_units(id),
  event_type TEXT NOT NULL,
  quantity NUMERIC(18,3) NOT NULL,
  before_quantity NUMERIC(18,3) NOT NULL,
  after_quantity NUMERIC(18,3) NOT NULL,
  from_location_type TEXT,
  to_location_type TEXT,
  from_warehouse_id UUID REFERENCES warehouses(id),
  to_warehouse_id UUID REFERENCES warehouses(id),
  from_project_id UUID REFERENCES projects(id),
  to_project_id UUID REFERENCES projects(id),
  source_type TEXT NOT NULL,
  source_id UUID,
  actor_id UUID REFERENCES users(id),
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT material_stock_unit_events_type_check CHECK (
    event_type IN ('initialize','receive','split','issue','transfer','use','waste','return_to_warehouse','return_to_subcontractor','reclassify','reverse')
  ),
  CONSTRAINT material_stock_unit_events_quantities_nonnegative CHECK (
    quantity >= 0 AND before_quantity >= 0 AND after_quantity >= 0
  )
);

CREATE TABLE IF NOT EXISTS stock_movement_unit_allocations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stock_movement_id UUID NOT NULL REFERENCES stock_movements(id),
  stock_document_item_id UUID REFERENCES stock_document_items(id),
  stock_unit_id UUID NOT NULL REFERENCES material_stock_units(id),
  stock_unit_event_id UUID NOT NULL REFERENCES material_stock_unit_events(id),
  quantity NUMERIC(18,3) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT stock_movement_unit_allocations_quantity_positive CHECK (quantity > 0),
  CONSTRAINT stock_movement_unit_allocations_event_unique UNIQUE (stock_unit_event_id)
);

ALTER TABLE material_before_after_snapshot_items
  ADD COLUMN IF NOT EXISTS stock_unit_usage_json JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE site_surplus_handlings
  ADD COLUMN IF NOT EXISTS stock_unit_allocations_json JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE site_material_balances
  ADD COLUMN IF NOT EXISTS waste_quantity NUMERIC(18,3) NOT NULL DEFAULT 0;

ALTER TABLE site_material_balances
  DROP CONSTRAINT IF EXISTS site_material_balances_waste_quantity_nonnegative,
  ADD CONSTRAINT site_material_balances_waste_quantity_nonnegative CHECK (waste_quantity >= 0);

CREATE INDEX IF NOT EXISTS idx_material_stock_units_material_location
  ON material_stock_units(material_id, location_type, warehouse_id, project_id, status);

CREATE INDEX IF NOT EXISTS idx_material_stock_units_root
  ON material_stock_units(root_unit_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_material_stock_units_parent
  ON material_stock_units(parent_unit_id, created_at, id)
  WHERE parent_unit_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_material_stock_unit_events_unit
  ON material_stock_unit_events(stock_unit_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_material_stock_unit_events_source
  ON material_stock_unit_events(source_type, source_id, created_at DESC)
  WHERE source_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_movement_unit_allocations_movement
  ON stock_movement_unit_allocations(stock_movement_id, created_at, id);
