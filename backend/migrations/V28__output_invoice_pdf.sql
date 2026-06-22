CREATE UNIQUE INDEX IF NOT EXISTS ux_receivables_id_project
  ON receivables(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payments_id_project
  ON payments(id, project_id);

CREATE TABLE IF NOT EXISTS output_invoices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  receivable_id UUID NOT NULL,
  payment_id UUID,
  document_id UUID,
  invoice_number TEXT NOT NULL,
  invoice_symbol TEXT,
  invoice_date DATE NOT NULL,
  client_name TEXT NOT NULL,
  client_tax_code TEXT,
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
  issued_by UUID REFERENCES users(id),
  issued_at TIMESTAMPTZ,
  voided_by UUID REFERENCES users(id),
  voided_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT output_invoices_receivable_project_fk
    FOREIGN KEY (receivable_id, project_id) REFERENCES receivables(id, project_id) ON DELETE CASCADE,
  CONSTRAINT output_invoices_payment_project_fk
    FOREIGN KEY (payment_id, project_id) REFERENCES payments(id, project_id),
  CONSTRAINT output_invoices_document_project_fk
    FOREIGN KEY (document_id, project_id) REFERENCES documents(id, project_id),
  CONSTRAINT output_invoices_status_check CHECK (status IN ('draft', 'issued', 'voided', 'cancelled', 'deleted')),
  CONSTRAINT output_invoices_currency_check CHECK (currency IN ('VND')),
  CONSTRAINT output_invoices_storage_provider_check CHECK (storage_provider IN ('local')),
  CONSTRAINT output_invoices_mime_check CHECK (mime_type = 'application/pdf'),
  CONSTRAINT output_invoices_file_size_positive CHECK (file_size > 0),
  CONSTRAINT output_invoices_checksum_check CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT output_invoices_amount_before_vat_nonnegative CHECK (amount_before_vat >= 0),
  CONSTRAINT output_invoices_vat_nonnegative CHECK (vat_amount >= 0),
  CONSTRAINT output_invoices_total_positive CHECK (total_amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_output_invoices_project_status
  ON output_invoices(project_id, status, invoice_date DESC, created_at DESC)
  WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_output_invoices_receivable_status
  ON output_invoices(receivable_id, status, invoice_date DESC, created_at DESC)
  WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_output_invoices_payment
  ON output_invoices(payment_id)
  WHERE payment_id IS NOT NULL AND status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_output_invoices_document
  ON output_invoices(document_id)
  WHERE document_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_output_invoices_client_number
  ON output_invoices(project_id, client_tax_code, invoice_number)
  WHERE client_tax_code IS NOT NULL
    AND client_tax_code <> ''
    AND status NOT IN ('deleted', 'cancelled', 'voided');
