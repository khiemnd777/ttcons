CREATE UNIQUE INDEX IF NOT EXISTS ux_output_invoices_id_receivable_project_v30
  ON output_invoices(id, receivable_id, project_id);

CREATE TABLE IF NOT EXISTS receivable_payment_schedules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL,
  receivable_id UUID NOT NULL,
  schedule_code TEXT NOT NULL,
  milestone_name TEXT NOT NULL,
  schedule_type TEXT NOT NULL DEFAULT 'progress',
  planned_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  received_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  remaining_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  due_date DATE,
  status TEXT NOT NULL DEFAULT 'draft',
  notes TEXT,
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT receivable_payment_schedules_receivable_project_fk
    FOREIGN KEY (receivable_id, project_id) REFERENCES receivables(id, project_id) ON DELETE CASCADE,
  CONSTRAINT receivable_payment_schedules_type_check
    CHECK (schedule_type IN ('advance', 'progress', 'final', 'retention', 'other')),
  CONSTRAINT receivable_payment_schedules_status_check
    CHECK (status IN ('draft', 'approved', 'partially_received', 'received', 'overdue', 'cancelled', 'deleted')),
  CONSTRAINT receivable_payment_schedules_amounts_check
    CHECK (
      planned_amount >= 0
      AND received_amount >= 0
      AND remaining_amount >= 0
      AND received_amount <= planned_amount
      AND remaining_amount <= planned_amount
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_receivable_payment_schedules_code
  ON receivable_payment_schedules(receivable_id, schedule_code)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE INDEX IF NOT EXISTS idx_receivable_payment_schedules_project_status
  ON receivable_payment_schedules(project_id, status, due_date, created_at DESC)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE INDEX IF NOT EXISTS idx_receivable_payment_schedules_receivable_status
  ON receivable_payment_schedules(receivable_id, status, due_date, created_at DESC)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE TABLE IF NOT EXISTS client_advance_receipts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL,
  receivable_id UUID NOT NULL,
  receipt_code TEXT NOT NULL,
  client_name TEXT,
  requested_by UUID REFERENCES users(id),
  amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  received_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  settled_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  reason TEXT,
  due_date DATE,
  payment_id UUID,
  received_at TIMESTAMPTZ,
  settled_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'draft',
  notes TEXT,
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT client_advance_receipts_receivable_project_fk
    FOREIGN KEY (receivable_id, project_id) REFERENCES receivables(id, project_id) ON DELETE CASCADE,
  CONSTRAINT client_advance_receipts_payment_project_fk
    FOREIGN KEY (payment_id, project_id) REFERENCES payments(id, project_id),
  CONSTRAINT client_advance_receipts_status_check
    CHECK (status IN ('draft', 'submitted', 'approved', 'partially_received', 'received', 'partially_settled', 'settled', 'cancelled', 'deleted')),
  CONSTRAINT client_advance_receipts_amounts_check
    CHECK (
      amount >= 0
      AND received_amount >= 0
      AND settled_amount >= 0
      AND received_amount <= amount
      AND settled_amount <= amount
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_client_advance_receipts_code
  ON client_advance_receipts(project_id, receipt_code)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE INDEX IF NOT EXISTS idx_client_advance_receipts_receivable_status
  ON client_advance_receipts(receivable_id, status, created_at DESC)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE TABLE IF NOT EXISTS receipt_allocations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL,
  payment_id UUID NOT NULL,
  receivable_id UUID NOT NULL,
  schedule_id UUID,
  advance_receipt_id UUID,
  output_invoice_id UUID,
  allocation_type TEXT NOT NULL DEFAULT 'receivable_receipt',
  amount NUMERIC(18,2) NOT NULL,
  notes TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT receipt_allocations_payment_project_fk
    FOREIGN KEY (payment_id, project_id) REFERENCES payments(id, project_id) ON DELETE CASCADE,
  CONSTRAINT receipt_allocations_receivable_project_fk
    FOREIGN KEY (receivable_id, project_id) REFERENCES receivables(id, project_id) ON DELETE CASCADE,
  CONSTRAINT receipt_allocations_schedule_fk
    FOREIGN KEY (schedule_id) REFERENCES receivable_payment_schedules(id),
  CONSTRAINT receipt_allocations_advance_fk
    FOREIGN KEY (advance_receipt_id) REFERENCES client_advance_receipts(id),
  CONSTRAINT receipt_allocations_invoice_receivable_project_fk
    FOREIGN KEY (output_invoice_id, receivable_id, project_id) REFERENCES output_invoices(id, receivable_id, project_id),
  CONSTRAINT receipt_allocations_type_check
    CHECK (allocation_type IN ('receivable_receipt', 'advance_receipt', 'advance_settlement')),
  CONSTRAINT receipt_allocations_amount_positive
    CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_receipt_allocations_receivable
  ON receipt_allocations(receivable_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_receipt_allocations_payment
  ON receipt_allocations(payment_id);

CREATE INDEX IF NOT EXISTS idx_receipt_allocations_schedule
  ON receipt_allocations(schedule_id)
  WHERE schedule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_receipt_allocations_advance
  ON receipt_allocations(advance_receipt_id)
  WHERE advance_receipt_id IS NOT NULL;
