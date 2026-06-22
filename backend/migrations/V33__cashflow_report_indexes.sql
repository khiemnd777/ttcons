CREATE INDEX IF NOT EXISTS idx_payments_cashflow_project_date_v33
  ON payments(project_id, status, (COALESCE(paid_at, updated_at, created_at)), payment_type, target_type)
  WHERE status <> 'cancelled';
