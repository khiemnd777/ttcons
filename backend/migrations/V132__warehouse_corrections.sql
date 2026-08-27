ALTER TABLE stock_documents
  DROP CONSTRAINT IF EXISTS stock_documents_movement_type_check,
  DROP CONSTRAINT IF EXISTS stock_documents_status_check;

ALTER TABLE stock_documents
  ADD COLUMN IF NOT EXISTS correction_of_document_id UUID REFERENCES stock_documents(id),
  ADD COLUMN IF NOT EXISTS corrected_by_document_id UUID REFERENCES stock_documents(id),
  ADD COLUMN IF NOT EXISTS correction_reason TEXT,
  ADD COLUMN IF NOT EXISTS corrected_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS corrected_at TIMESTAMPTZ,
  ADD CONSTRAINT stock_documents_movement_type_check CHECK (movement_type IN (
    'receive',
    'issue',
    'transfer',
    'return_to_warehouse',
    'return_to_subcontractor',
    'adjustment_in',
    'adjustment_out'
  )),
  ADD CONSTRAINT stock_documents_status_check CHECK (status IN ('posted', 'corrected', 'cancelled'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_documents_single_correction
  ON stock_documents(correction_of_document_id)
  WHERE correction_of_document_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_documents_corrected_by
  ON stock_documents(corrected_by_document_id)
  WHERE corrected_by_document_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS site_material_adjustments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  material_id UUID NOT NULL REFERENCES materials(id),
  adjustment_type TEXT NOT NULL,
  quantity NUMERIC(18,3) NOT NULL,
  previous_used_quantity NUMERIC(18,3),
  target_used_quantity NUMERIC(18,3),
  previous_issued_quantity NUMERIC(18,3),
  target_issued_quantity NUMERIC(18,3),
  unit TEXT NOT NULL,
  reason TEXT NOT NULL,
  source_type TEXT,
  source_id UUID,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT site_material_adjustments_type_check CHECK (adjustment_type IN (
    'usage_increase',
    'usage_decrease',
    'issued_increase_correction',
    'issued_decrease_correction'
  )),
  CONSTRAINT site_material_adjustments_quantity_positive CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS idx_site_material_adjustments_project_material
  ON site_material_adjustments(project_id, material_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_site_material_adjustments_source
  ON site_material_adjustments(source_type, source_id)
  WHERE source_id IS NOT NULL;
