CREATE TABLE IF NOT EXISTS payment_reversals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  payment_id UUID NOT NULL UNIQUE REFERENCES payments(id) ON DELETE RESTRICT,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
  original_journal_entry_id UUID NOT NULL REFERENCES journal_entries(id) ON DELETE RESTRICT,
  reversal_journal_entry_id UUID NOT NULL UNIQUE REFERENCES journal_entries(id) ON DELETE RESTRICT,
  reason TEXT NOT NULL,
  reversed_by UUID REFERENCES users(id),
  reversed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT payment_reversals_reason_nonblank CHECK (NULLIF(btrim(reason),'') IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_payment_reversals_project_time
  ON payment_reversals(project_id,reversed_at DESC);

CREATE OR REPLACE FUNCTION prevent_payment_reversal_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'chứng từ đảo phiếu thu/chi là bất biến';
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_payment_reversal_mutation ON payment_reversals;
CREATE TRIGGER trg_prevent_payment_reversal_mutation
BEFORE UPDATE OR DELETE ON payment_reversals
FOR EACH ROW EXECUTE FUNCTION prevent_payment_reversal_mutation();
