ALTER TABLE journal_entries
  ADD COLUMN IF NOT EXISTS source_document_no TEXT;

ALTER TABLE journal_entries
  DROP CONSTRAINT IF EXISTS journal_entries_source_document_nonblank;

ALTER TABLE journal_entries
  ADD CONSTRAINT journal_entries_source_document_nonblank
  CHECK (source_document_no IS NULL OR NULLIF(btrim(source_document_no),'') IS NOT NULL)
  NOT VALID;

ALTER TABLE journal_entries
  VALIDATE CONSTRAINT journal_entries_source_document_nonblank;
