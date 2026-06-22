CREATE INDEX IF NOT EXISTS idx_payables_project_due_open_v31
  ON payables(project_id, due_date, status)
  WHERE status <> 'cancelled'
    AND remaining_amount > 0;

CREATE INDEX IF NOT EXISTS idx_payment_allocations_payable_type_v31
  ON payment_allocations(payable_id, allocation_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_advance_requests_payable_due_open_v31
  ON advance_requests(payable_id, due_date, status)
  WHERE payable_id IS NOT NULL
    AND status NOT IN ('cancelled', 'deleted');
