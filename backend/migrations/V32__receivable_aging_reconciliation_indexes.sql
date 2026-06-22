CREATE INDEX IF NOT EXISTS idx_receivables_project_due_open_v32
  ON receivables(project_id, due_date, status)
  WHERE status <> 'cancelled'
    AND remaining_amount > 0;

CREATE INDEX IF NOT EXISTS idx_receipt_allocations_receivable_type_v32
  ON receipt_allocations(receivable_id, allocation_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_client_advance_receipts_receivable_due_open_v32
  ON client_advance_receipts(receivable_id, due_date, status)
  WHERE status NOT IN ('cancelled', 'deleted');
