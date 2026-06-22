CREATE INDEX IF NOT EXISTS idx_output_invoices_profit_report_period
  ON output_invoices(project_id, status, invoice_date, receivable_id)
  WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_input_invoices_profit_report_period
  ON input_invoices(project_id, status, invoice_date, payable_id, procurement_request_id)
  WHERE status <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_payables_profit_report_classification
  ON payables(project_id, source_type, vendor_type, status)
  WHERE status <> 'cancelled';
