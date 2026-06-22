CREATE UNIQUE INDEX IF NOT EXISTS ux_payables_id_project
  ON payables(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_procurement_requests_id_project
  ON procurement_requests(id, project_id);

CREATE TABLE IF NOT EXISTS input_invoices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  payable_id UUID NOT NULL,
  procurement_request_id UUID,
  document_id UUID,
  invoice_number TEXT NOT NULL,
  invoice_symbol TEXT,
  invoice_date DATE NOT NULL,
  vendor_name TEXT NOT NULL,
  vendor_tax_code TEXT,
  amount_before_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
  vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  total_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'VND',
  status TEXT NOT NULL DEFAULT 'draft',
  notes TEXT,
  storage_provider TEXT NOT NULL DEFAULT 'local',
  storage_key TEXT NOT NULL,
  mime_type TEXT NOT NULL DEFAULT 'application/pdf',
  file_name TEXT NOT NULL,
  file_size BIGINT NOT NULL,
  checksum_sha256 TEXT NOT NULL,
  uploaded_by UUID REFERENCES users(id),
  verified_by UUID REFERENCES users(id),
  verified_at TIMESTAMPTZ,
  rejected_by UUID REFERENCES users(id),
  rejected_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT input_invoices_payable_project_fk
    FOREIGN KEY (payable_id, project_id) REFERENCES payables(id, project_id) ON DELETE CASCADE,
  CONSTRAINT input_invoices_procurement_project_fk
    FOREIGN KEY (procurement_request_id, project_id) REFERENCES procurement_requests(id, project_id),
  CONSTRAINT input_invoices_document_project_fk
    FOREIGN KEY (document_id, project_id) REFERENCES documents(id, project_id),
  CONSTRAINT input_invoices_status_check CHECK (status IN ('draft', 'verified', 'rejected', 'cancelled', 'deleted')),
  CONSTRAINT input_invoices_currency_check CHECK (currency IN ('VND')),
  CONSTRAINT input_invoices_storage_provider_check CHECK (storage_provider IN ('local')),
  CONSTRAINT input_invoices_mime_check CHECK (mime_type = 'application/pdf'),
  CONSTRAINT input_invoices_file_size_positive CHECK (file_size > 0),
  CONSTRAINT input_invoices_checksum_check CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT input_invoices_amount_before_vat_nonnegative CHECK (amount_before_vat >= 0),
  CONSTRAINT input_invoices_vat_nonnegative CHECK (vat_amount >= 0),
  CONSTRAINT input_invoices_total_positive CHECK (total_amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_input_invoices_project_status
  ON input_invoices(project_id, status, invoice_date DESC, created_at DESC)
  WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_input_invoices_payable_status
  ON input_invoices(payable_id, status, invoice_date DESC, created_at DESC)
  WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_input_invoices_procurement
  ON input_invoices(procurement_request_id)
  WHERE procurement_request_id IS NOT NULL AND status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_input_invoices_document
  ON input_invoices(document_id)
  WHERE document_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_input_invoices_vendor_number
  ON input_invoices(project_id, vendor_tax_code, invoice_number)
  WHERE vendor_tax_code IS NOT NULL
    AND vendor_tax_code <> ''
    AND status NOT IN ('deleted', 'cancelled');
