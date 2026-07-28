ALTER TABLE accounting_budgets
  DROP CONSTRAINT IF EXISTS accounting_budgets_status_check_v73;

ALTER TABLE accounting_budgets
  ADD CONSTRAINT accounting_budgets_status_check_v77
  CHECK (status IN ('draft','submitted','approved','cancelled')) NOT VALID;

ALTER TABLE payables
  DROP CONSTRAINT IF EXISTS payables_status_check_v73;

ALTER TABLE payables
  ADD CONSTRAINT payables_status_check_v77
  CHECK (status IN ('draft','recorded','submitted','approved','partially_paid','paid','overdue','cancelled','deleted')) NOT VALID;

ALTER TABLE receivables
  DROP CONSTRAINT IF EXISTS receivables_status_check_v73;

ALTER TABLE receivables
  ADD CONSTRAINT receivables_status_check_v77
  CHECK (status IN ('draft','recorded','submitted','approved','partially_received','received','overdue','cancelled','deleted')) NOT VALID;

ALTER TABLE payments
  DROP CONSTRAINT IF EXISTS payments_status_check_v73;

-- Legacy vouchers used direction-specific terminal statuses. In the current
-- workflow, approval is the single posted/paid state and paid_at preserves the
-- actual cash date.
UPDATE payments
SET status='approved',
    paid_at=COALESCE(paid_at,updated_at,created_at)
WHERE status IN ('paid','received');

-- Older summary vouchers stored a target kind without a target record. Keep
-- the voucher and its payment_type, but clear the unusable partial reference.
UPDATE payments
SET target_type=NULL
WHERE target_type IS NOT NULL AND target_id IS NULL;

ALTER TABLE payments
  ADD CONSTRAINT payments_status_check_v77
  CHECK (status IN ('draft','submitted','pending','waiting_approval','approved','cancelled','deleted')) NOT VALID;

ALTER TABLE payments
  ADD CONSTRAINT payments_target_pair_check_v77
  CHECK ((target_type IS NULL) = (target_id IS NULL)) NOT VALID;

ALTER TABLE payments
  ADD CONSTRAINT payments_method_check_v77
  CHECK (payment_method IN ('bank_transfer','cash')) NOT VALID;

ALTER TABLE accounting_budgets VALIDATE CONSTRAINT accounting_budgets_amounts_check_v73;
ALTER TABLE accounting_budgets VALIDATE CONSTRAINT accounting_budgets_status_check_v77;
ALTER TABLE payables VALIDATE CONSTRAINT payables_amounts_check_v73;
ALTER TABLE payables VALIDATE CONSTRAINT payables_status_check_v77;
ALTER TABLE receivables VALIDATE CONSTRAINT receivables_amounts_check_v73;
ALTER TABLE receivables VALIDATE CONSTRAINT receivables_status_check_v77;
ALTER TABLE payments VALIDATE CONSTRAINT payments_amount_positive_v73;
ALTER TABLE payments VALIDATE CONSTRAINT payments_status_check_v77;
ALTER TABLE payments VALIDATE CONSTRAINT payments_type_check_v73;
ALTER TABLE payments VALIDATE CONSTRAINT payments_target_pair_check_v77;
ALTER TABLE payments VALIDATE CONSTRAINT payments_method_check_v77;
