CREATE TABLE IF NOT EXISTS stock_documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_code TEXT NOT NULL UNIQUE,
  project_id UUID REFERENCES projects(id),
  movement_type TEXT NOT NULL,
  source_warehouse_id UUID REFERENCES warehouses(id),
  target_warehouse_id UUID REFERENCES warehouses(id),
  reference_type TEXT,
  reference_id UUID,
  document_date DATE NOT NULL DEFAULT CURRENT_DATE,
  note TEXT,
  status TEXT NOT NULL DEFAULT 'posted',
  created_by UUID REFERENCES users(id),
  posted_by UUID REFERENCES users(id),
  posted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT stock_documents_movement_type_check CHECK (movement_type IN (
    'receive',
    'issue',
    'transfer',
    'return_to_warehouse',
    'return_to_subcontractor'
  )),
  CONSTRAINT stock_documents_status_check CHECK (status IN ('posted', 'cancelled')),
  CONSTRAINT stock_documents_transfer_warehouses_check CHECK (
    movement_type <> 'transfer'
    OR (
      source_warehouse_id IS NOT NULL
      AND target_warehouse_id IS NOT NULL
      AND source_warehouse_id <> target_warehouse_id
    )
  )
);

CREATE TABLE IF NOT EXISTS stock_document_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id UUID NOT NULL REFERENCES stock_documents(id) ON DELETE CASCADE,
  material_id UUID NOT NULL REFERENCES materials(id),
  quantity NUMERIC(18,3) NOT NULL,
  unit TEXT NOT NULL,
  reason TEXT,
  source_line_type TEXT,
  source_line_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT stock_document_items_quantity_positive CHECK (quantity > 0)
);

ALTER TABLE stock_movements
  ADD COLUMN IF NOT EXISTS stock_document_id UUID REFERENCES stock_documents(id),
  ADD COLUMN IF NOT EXISTS target_warehouse_id UUID REFERENCES warehouses(id);

CREATE INDEX IF NOT EXISTS idx_stock_documents_project_date
  ON stock_documents(project_id, document_date DESC, created_at DESC)
  WHERE status <> 'cancelled';

CREATE INDEX IF NOT EXISTS idx_stock_documents_reference
  ON stock_documents(reference_type, reference_id)
  WHERE reference_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_document_items_document
  ON stock_document_items(document_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_stock_document_items_source_line
  ON stock_document_items(source_line_type, source_line_id)
  WHERE source_line_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_movements_document
  ON stock_movements(stock_document_id)
  WHERE stock_document_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_movements_target_warehouse_material
  ON stock_movements(target_warehouse_id, material_id, created_at)
  WHERE target_warehouse_id IS NOT NULL;
