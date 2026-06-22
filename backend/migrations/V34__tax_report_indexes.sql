CREATE INDEX IF NOT EXISTS idx_input_invoices_tax_report_period
  ON input_invoices(project_id, status, invoice_date, vendor_tax_code)
  WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_output_invoices_tax_report_period
  ON output_invoices(project_id, status, invoice_date, client_tax_code)
  WHERE status <> 'deleted';
