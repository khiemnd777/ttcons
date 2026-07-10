CREATE TABLE IF NOT EXISTS supplier_quotations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  quotation_code TEXT NOT NULL,
  supplier_partner_id UUID NOT NULL REFERENCES partners(id),
  supplier_name TEXT NOT NULL,
  quotation_date DATE NOT NULL,
  valid_until DATE,
  currency TEXT NOT NULL DEFAULT 'VND',
  payment_terms_days INTEGER NOT NULL DEFAULT 0,
  lead_time_days INTEGER NOT NULL DEFAULT 0,
  delivery_terms TEXT,
  warranty_terms TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  technical_status TEXT NOT NULL DEFAULT 'pending',
  commercial_status TEXT NOT NULL DEFAULT 'pending',
  notes TEXT,
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  qualified_by UUID REFERENCES users(id),
  qualified_at TIMESTAMPTZ,
  selected_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT supplier_quotations_project_code_unique UNIQUE (project_id, quotation_code),
  CONSTRAINT supplier_quotations_status_check CHECK (status IN ('draft','submitted','qualified','selected','rejected','expired','cancelled')),
  CONSTRAINT supplier_quotations_technical_check CHECK (technical_status IN ('pending','compliant','conditional','non_compliant')),
  CONSTRAINT supplier_quotations_commercial_check CHECK (commercial_status IN ('pending','compliant','non_compliant')),
  CONSTRAINT supplier_quotations_currency_check CHECK (currency ~ '^[A-Z]{3}$'),
  CONSTRAINT supplier_quotations_payment_terms_nonnegative CHECK (payment_terms_days >= 0),
  CONSTRAINT supplier_quotations_lead_time_nonnegative CHECK (lead_time_days >= 0),
  CONSTRAINT supplier_quotations_validity_check CHECK (valid_until IS NULL OR valid_until >= quotation_date)
);

CREATE INDEX IF NOT EXISTS idx_supplier_quotations_project_status
  ON supplier_quotations(project_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_supplier_quotations_supplier
  ON supplier_quotations(supplier_partner_id, project_id);

CREATE TABLE IF NOT EXISTS supplier_quotation_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  quotation_id UUID NOT NULL REFERENCES supplier_quotations(id) ON DELETE CASCADE,
  boq_item_id UUID NOT NULL REFERENCES boq_items(id),
  material_id UUID REFERENCES materials(id),
  description TEXT NOT NULL,
  unit TEXT NOT NULL,
  quantity NUMERIC(18,3) NOT NULL,
  unit_price NUMERIC(18,2) NOT NULL,
  discount_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  tax_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
  amount_before_tax NUMERIC(18,2) GENERATED ALWAYS AS ((quantity * unit_price) - discount_amount) STORED,
  total_amount NUMERIC(18,2) GENERATED ALWAYS AS (((quantity * unit_price) - discount_amount) * (1 + tax_rate / 100)) STORED,
  lead_time_days INTEGER NOT NULL DEFAULT 0,
  compliance_status TEXT NOT NULL DEFAULT 'pending',
  deviation_note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT supplier_quotation_items_quote_boq_unique UNIQUE (quotation_id, boq_item_id),
  CONSTRAINT supplier_quotation_items_quantity_positive CHECK (quantity > 0),
  CONSTRAINT supplier_quotation_items_unit_price_positive CHECK (unit_price > 0),
  CONSTRAINT supplier_quotation_items_discount_nonnegative CHECK (discount_amount >= 0),
  CONSTRAINT supplier_quotation_items_discount_within_amount CHECK (discount_amount <= quantity * unit_price),
  CONSTRAINT supplier_quotation_items_tax_rate_check CHECK (tax_rate >= 0 AND tax_rate <= 100),
  CONSTRAINT supplier_quotation_items_lead_time_nonnegative CHECK (lead_time_days >= 0),
  CONSTRAINT supplier_quotation_items_compliance_check CHECK (compliance_status IN ('pending','compliant','conditional','non_compliant'))
);

CREATE INDEX IF NOT EXISTS idx_supplier_quotation_items_boq
  ON supplier_quotation_items(boq_item_id, quotation_id);

CREATE TABLE IF NOT EXISTS supplier_quote_selections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  boq_item_id UUID NOT NULL REFERENCES boq_items(id),
  quotation_item_id UUID NOT NULL REFERENCES supplier_quotation_items(id),
  status TEXT NOT NULL DEFAULT 'active',
  decision_reason TEXT NOT NULL,
  selected_by UUID REFERENCES users(id),
  selected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  superseded_at TIMESTAMPTZ,
  CONSTRAINT supplier_quote_selections_status_check CHECK (status IN ('active','superseded')),
  CONSTRAINT supplier_quote_selections_superseded_check CHECK (
    (status='active' AND superseded_at IS NULL) OR
    (status='superseded' AND superseded_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_quote_selections_active_boq
  ON supplier_quote_selections(boq_item_id)
  WHERE status='active';

CREATE INDEX IF NOT EXISTS idx_supplier_quote_selections_project
  ON supplier_quote_selections(project_id, selected_at DESC);

ALTER TABLE boq_items
  ADD COLUMN IF NOT EXISTS selected_quotation_item_id UUID REFERENCES supplier_quotation_items(id),
  ADD COLUMN IF NOT EXISTS price_waiver_reason TEXT;

ALTER TABLE procurement_request_items
  ADD COLUMN IF NOT EXISTS supplier_quotation_item_id UUID REFERENCES supplier_quotation_items(id),
  ADD COLUMN IF NOT EXISTS quoted_unit_price NUMERIC(18,2),
  ADD COLUMN IF NOT EXISTS price_variance_amount NUMERIC(18,2);

ALTER TABLE procurement_request_items
  ADD CONSTRAINT procurement_request_items_quoted_price_positive CHECK (quoted_unit_price IS NULL OR quoted_unit_price > 0);

ALTER TABLE procurement_requests
  ADD COLUMN IF NOT EXISTS supplier_partner_id UUID REFERENCES partners(id),
  ADD COLUMN IF NOT EXISTS supplier_name TEXT,
  ADD COLUMN IF NOT EXISTS order_reference TEXT,
  ADD COLUMN IF NOT EXISTS expected_delivery_date DATE,
  ADD COLUMN IF NOT EXISTS ordered_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_procurement_requests_supplier
  ON procurement_requests(project_id, supplier_partner_id, status);

CREATE INDEX IF NOT EXISTS idx_procurement_items_quotation_item
  ON procurement_request_items(supplier_quotation_item_id)
  WHERE supplier_quotation_item_id IS NOT NULL;

CREATE OR REPLACE FUNCTION prevent_supplier_commercial_history_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'supplier commercial history is immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_supplier_quote_selection_no_update_delete ON supplier_quote_selections;
CREATE TRIGGER trg_supplier_quote_selection_no_update_delete
BEFORE DELETE ON supplier_quote_selections
FOR EACH ROW EXECUTE FUNCTION prevent_supplier_commercial_history_mutation();

CREATE OR REPLACE FUNCTION prevent_supplier_quotation_item_locked_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM supplier_quotations q
    WHERE q.id=OLD.quotation_id AND q.status <> 'draft'
  ) THEN
    RAISE EXCEPTION 'submitted supplier quotation items are immutable';
  END IF;
  RETURN COALESCE(NEW, OLD);
END;
$$;

DROP TRIGGER IF EXISTS trg_supplier_quotation_item_locked_mutation ON supplier_quotation_items;
CREATE TRIGGER trg_supplier_quotation_item_locked_mutation
BEFORE UPDATE OR DELETE ON supplier_quotation_items
FOR EACH ROW EXECUTE FUNCTION prevent_supplier_quotation_item_locked_mutation();
