CREATE UNIQUE INDEX IF NOT EXISTS ux_payables_id_project_v29
  ON payables(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payments_id_project_v29
  ON payments(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_input_invoices_id_payable_project_v29
  ON input_invoices(id, payable_id, project_id);

ALTER TABLE advance_requests
  ADD COLUMN IF NOT EXISTS payable_id UUID,
  ADD COLUMN IF NOT EXISTS vendor_name TEXT,
  ADD COLUMN IF NOT EXISTS due_date DATE,
  ADD COLUMN IF NOT EXISTS payment_id UUID,
  ADD COLUMN IF NOT EXISTS paid_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS notes TEXT;

ALTER TABLE advance_requests
  ADD CONSTRAINT advance_requests_payable_project_fk
  FOREIGN KEY (payable_id, project_id) REFERENCES payables(id, project_id) ON DELETE CASCADE
  NOT VALID;

ALTER TABLE advance_requests
  ADD CONSTRAINT advance_requests_payment_project_fk
  FOREIGN KEY (payment_id, project_id) REFERENCES payments(id, project_id)
  NOT VALID;

ALTER TABLE advance_requests
  ADD CONSTRAINT advance_requests_amount_nonnegative
  CHECK (amount >= 0) NOT VALID;

ALTER TABLE advance_requests
  ADD CONSTRAINT advance_requests_settled_nonnegative
  CHECK (settled_amount >= 0 AND settled_amount <= amount) NOT VALID;

ALTER TABLE advance_requests
  ADD CONSTRAINT advance_requests_paid_nonnegative
  CHECK (paid_amount >= 0 AND paid_amount <= amount) NOT VALID;

CREATE INDEX IF NOT EXISTS idx_advance_requests_payable_status
  ON advance_requests(payable_id, status, created_at DESC)
  WHERE payable_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS payable_payment_schedules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL,
  payable_id UUID NOT NULL,
  schedule_code TEXT NOT NULL,
  milestone_name TEXT NOT NULL,
  schedule_type TEXT NOT NULL DEFAULT 'progress',
  planned_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  paid_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  remaining_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  due_date DATE,
  status TEXT NOT NULL DEFAULT 'draft',
  notes TEXT,
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT payable_payment_schedules_payable_project_fk
    FOREIGN KEY (payable_id, project_id) REFERENCES payables(id, project_id) ON DELETE CASCADE,
  CONSTRAINT payable_payment_schedules_type_check
    CHECK (schedule_type IN ('advance', 'progress', 'final', 'retention', 'other')),
  CONSTRAINT payable_payment_schedules_status_check
    CHECK (status IN ('draft', 'approved', 'partially_paid', 'paid', 'overdue', 'cancelled', 'deleted')),
  CONSTRAINT payable_payment_schedules_amounts_check
    CHECK (
      planned_amount >= 0
      AND paid_amount >= 0
      AND remaining_amount >= 0
      AND paid_amount <= planned_amount
      AND remaining_amount <= planned_amount
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_payable_payment_schedules_code
  ON payable_payment_schedules(payable_id, schedule_code)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE INDEX IF NOT EXISTS idx_payable_payment_schedules_project_status
  ON payable_payment_schedules(project_id, status, due_date, created_at DESC)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE INDEX IF NOT EXISTS idx_payable_payment_schedules_payable_status
  ON payable_payment_schedules(payable_id, status, due_date, created_at DESC)
  WHERE status NOT IN ('cancelled', 'deleted');

CREATE TABLE IF NOT EXISTS payment_allocations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL,
  payment_id UUID NOT NULL,
  payable_id UUID NOT NULL,
  schedule_id UUID,
  advance_request_id UUID,
  input_invoice_id UUID,
  allocation_type TEXT NOT NULL DEFAULT 'payable_payment',
  amount NUMERIC(18,2) NOT NULL,
  notes TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT payment_allocations_payment_project_fk
    FOREIGN KEY (payment_id, project_id) REFERENCES payments(id, project_id) ON DELETE CASCADE,
  CONSTRAINT payment_allocations_payable_project_fk
    FOREIGN KEY (payable_id, project_id) REFERENCES payables(id, project_id) ON DELETE CASCADE,
  CONSTRAINT payment_allocations_schedule_fk
    FOREIGN KEY (schedule_id) REFERENCES payable_payment_schedules(id),
  CONSTRAINT payment_allocations_advance_fk
    FOREIGN KEY (advance_request_id) REFERENCES advance_requests(id),
  CONSTRAINT payment_allocations_invoice_payable_project_fk
    FOREIGN KEY (input_invoice_id, payable_id, project_id) REFERENCES input_invoices(id, payable_id, project_id),
  CONSTRAINT payment_allocations_type_check
    CHECK (allocation_type IN ('payable_payment', 'advance_payment', 'advance_settlement')),
  CONSTRAINT payment_allocations_amount_positive
    CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_payment_allocations_payable
  ON payment_allocations(payable_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_payment_allocations_payment
  ON payment_allocations(payment_id);

CREATE INDEX IF NOT EXISTS idx_payment_allocations_schedule
  ON payment_allocations(schedule_id)
  WHERE schedule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payment_allocations_advance
  ON payment_allocations(advance_request_id)
  WHERE advance_request_id IS NOT NULL;
