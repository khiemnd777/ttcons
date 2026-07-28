ALTER TABLE accounting_backfill_issues
  ADD COLUMN IF NOT EXISTS resolution_journal_entry_id UUID REFERENCES journal_entries(id) ON DELETE RESTRICT;

ALTER TABLE accounting_backfill_issues
  DROP CONSTRAINT IF EXISTS accounting_backfill_issues_resolution_check;

ALTER TABLE accounting_backfill_issues
  ADD CONSTRAINT accounting_backfill_issues_resolution_check CHECK (
    (status = 'open' AND resolved_by IS NULL AND resolved_at IS NULL AND resolution_note IS NULL AND resolution_journal_entry_id IS NULL)
    OR
    (status = 'resolved' AND resolved_by IS NOT NULL AND resolved_at IS NOT NULL AND NULLIF(BTRIM(resolution_note),'') IS NOT NULL AND resolution_journal_entry_id IS NOT NULL)
    OR
    (status = 'accepted' AND resolved_by IS NOT NULL AND resolved_at IS NOT NULL AND NULLIF(BTRIM(resolution_note),'') IS NOT NULL AND resolution_journal_entry_id IS NULL)
  ) NOT VALID;

ALTER TABLE accounting_backfill_issues
  VALIDATE CONSTRAINT accounting_backfill_issues_resolution_check;

CREATE INDEX IF NOT EXISTS idx_accounting_backfill_issues_resolution_journal
  ON accounting_backfill_issues(resolution_journal_entry_id)
  WHERE resolution_journal_entry_id IS NOT NULL;
