ALTER TABLE tax_periods
  ADD COLUMN IF NOT EXISTS period_type TEXT NOT NULL DEFAULT 'month';

ALTER TABLE tax_periods
  DROP CONSTRAINT IF EXISTS tax_periods_book_period_unique,
  DROP CONSTRAINT IF EXISTS tax_periods_number_check;

ALTER TABLE tax_periods
  ADD CONSTRAINT tax_periods_type_check_v81
    CHECK (period_type IN ('month','quarter')),
  ADD CONSTRAINT tax_periods_number_check_v81
    CHECK (
      (period_type='month' AND period_no BETWEEN 1 AND 12)
      OR (period_type='quarter' AND period_no BETWEEN 1 AND 4)
    ),
  ADD CONSTRAINT tax_periods_book_period_unique_v81
    UNIQUE (book_id,fiscal_year,period_type,period_no);

INSERT INTO tax_periods (book_id,fiscal_year,period_type,period_no,date_from,date_to,status)
SELECT
  book.id,
  EXTRACT(YEAR FROM CURRENT_DATE)::integer,
  'quarter',
  quarter_no,
  make_date(EXTRACT(YEAR FROM CURRENT_DATE)::integer,(quarter_no-1)*3+1,1),
  (make_date(EXTRACT(YEAR FROM CURRENT_DATE)::integer,(quarter_no-1)*3+1,1) + INTERVAL '3 month - 1 day')::date,
  'open'
FROM accounting_books book
CROSS JOIN generate_series(1,4) AS quarter_no
WHERE book.active
ON CONFLICT (book_id,fiscal_year,period_type,period_no) DO NOTHING;

CREATE TABLE IF NOT EXISTS vat_return_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tax_period_id UUID NOT NULL REFERENCES tax_periods(id) ON DELETE RESTRICT,
  book_id UUID NOT NULL REFERENCES accounting_books(id) ON DELETE RESTRICT,
  amendment_no INTEGER NOT NULL DEFAULT 0,
  input_invoice_count INTEGER NOT NULL DEFAULT 0,
  output_invoice_count INTEGER NOT NULL DEFAULT 0,
  input_vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  deductible_input_vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  output_vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  input_adjustment_vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  output_adjustment_vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  adjustment_vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  credit_brought_forward NUMERIC(18,2) NOT NULL DEFAULT 0,
  vat_payable NUMERIC(18,2) NOT NULL DEFAULT 0,
  credit_carried_forward NUMERIC(18,2) NOT NULL DEFAULT 0,
  prepared_by UUID REFERENCES users(id),
  prepared_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT vat_return_snapshot_period_amendment_unique UNIQUE (tax_period_id,amendment_no),
  CONSTRAINT vat_return_snapshot_counts_check CHECK (input_invoice_count >= 0 AND output_invoice_count >= 0),
  CONSTRAINT vat_return_snapshot_amounts_check CHECK (
    input_vat_amount >= 0
    AND deductible_input_vat_amount >= 0
    AND output_vat_amount >= 0
    AND credit_brought_forward >= 0
    AND vat_payable >= 0
    AND credit_carried_forward >= 0
  ),
  CONSTRAINT vat_return_snapshot_amendment_check CHECK (amendment_no >= 0)
);

CREATE TABLE IF NOT EXISTS tax_period_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tax_period_id UUID NOT NULL REFERENCES tax_periods(id) ON DELETE RESTRICT,
  vat_return_snapshot_id UUID REFERENCES vat_return_snapshots(id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL,
  from_status TEXT NOT NULL,
  to_status TEXT NOT NULL,
  amendment_no INTEGER NOT NULL DEFAULT 0,
  filing_reference TEXT,
  reason TEXT,
  actor_id UUID REFERENCES users(id),
  actor_name TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT tax_period_events_type_check CHECK (event_type IN ('prepared','filed','locked','amendment_started')),
  CONSTRAINT tax_period_events_amendment_check CHECK (amendment_no >= 0),
  CONSTRAINT tax_period_events_reason_check CHECK (
    event_type NOT IN ('locked','amendment_started') OR NULLIF(btrim(reason),'') IS NOT NULL
  ),
  CONSTRAINT tax_period_events_filing_check CHECK (
    event_type <> 'filed' OR NULLIF(btrim(filing_reference),'') IS NOT NULL
  )
);

CREATE INDEX IF NOT EXISTS idx_vat_return_snapshots_period_created
  ON vat_return_snapshots(tax_period_id,amendment_no DESC,created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tax_period_events_period_created
  ON tax_period_events(tax_period_id,created_at DESC);

CREATE OR REPLACE FUNCTION prevent_vat_return_snapshot_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'tờ khai thuế GTGT đã chốt số liệu là bất biến; hãy lập tờ khai bổ sung';
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_vat_return_snapshot_mutation ON vat_return_snapshots;
CREATE TRIGGER trg_prevent_vat_return_snapshot_mutation
BEFORE UPDATE OR DELETE ON vat_return_snapshots
FOR EACH ROW EXECUTE FUNCTION prevent_vat_return_snapshot_mutation();

CREATE OR REPLACE FUNCTION prevent_tax_period_event_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'lịch sử kỳ thuế là bất biến';
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_tax_period_event_mutation ON tax_period_events;
CREATE TRIGGER trg_prevent_tax_period_event_mutation
BEFORE UPDATE OR DELETE ON tax_period_events
FOR EACH ROW EXECUTE FUNCTION prevent_tax_period_event_mutation();

DROP TRIGGER IF EXISTS trg_prevent_tax_adjustment_mutation ON tax_adjustments;

ALTER TABLE tax_adjustments
  ADD COLUMN IF NOT EXISTS book_id UUID,
  ADD COLUMN IF NOT EXISTS direction TEXT,
  ADD COLUMN IF NOT EXISTS offset_account_id UUID,
  ADD COLUMN IF NOT EXISTS entry_date DATE;

UPDATE tax_adjustments adjustment
SET book_id=period.book_id,
    direction=COALESCE(adjustment.direction,'output'),
    entry_date=COALESCE(adjustment.entry_date,adjustment.created_at::date)
FROM tax_periods period
WHERE period.id=adjustment.tax_period_id
  AND (adjustment.book_id IS NULL OR adjustment.direction IS NULL OR adjustment.entry_date IS NULL);

UPDATE tax_adjustments adjustment
SET offset_account_id=account.id
FROM accounting_accounts account
WHERE account.book_id=adjustment.book_id
  AND account.account_code='642'
  AND adjustment.offset_account_id IS NULL;

ALTER TABLE tax_adjustments
  ALTER COLUMN book_id SET NOT NULL,
  ALTER COLUMN direction SET NOT NULL,
  ALTER COLUMN offset_account_id SET NOT NULL,
  ALTER COLUMN entry_date SET NOT NULL,
  ADD CONSTRAINT tax_adjustments_direction_check_v81 CHECK (direction IN ('input','output')),
  ADD CONSTRAINT tax_adjustments_book_fk_v81 FOREIGN KEY (book_id) REFERENCES accounting_books(id) ON DELETE RESTRICT,
  ADD CONSTRAINT tax_adjustments_offset_account_book_fk_v81
    FOREIGN KEY (offset_account_id,book_id) REFERENCES accounting_accounts(id,book_id) ON DELETE RESTRICT;

CREATE TRIGGER trg_prevent_tax_adjustment_mutation
BEFORE UPDATE OR DELETE ON tax_adjustments
FOR EACH ROW EXECUTE FUNCTION prevent_tax_adjustment_mutation();

ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS tax_period_id UUID REFERENCES tax_periods(id) ON DELETE RESTRICT;

ALTER TABLE payments
  DROP CONSTRAINT IF EXISTS payments_type_check_v73;

ALTER TABLE payments
  ADD CONSTRAINT payments_type_check_v81
    CHECK (payment_type IN (
      'supplier_payment','subcontractor_payment','client_receipt','outbound','inbound',
      'advance','advance_settlement','tax_payment','other'
    )) NOT VALID,
  ADD CONSTRAINT payments_tax_period_link_check_v81
    CHECK (
      (payment_type='tax_payment' AND tax_period_id IS NOT NULL AND target_type='tax_period' AND target_id=tax_period_id)
      OR (payment_type<>'tax_payment' AND tax_period_id IS NULL)
    ) NOT VALID;

ALTER TABLE payments VALIDATE CONSTRAINT payments_type_check_v81;
ALTER TABLE payments VALIDATE CONSTRAINT payments_tax_period_link_check_v81;

CREATE INDEX IF NOT EXISTS idx_payments_tax_period_status
  ON payments(tax_period_id,status,created_at DESC)
  WHERE tax_period_id IS NOT NULL;
