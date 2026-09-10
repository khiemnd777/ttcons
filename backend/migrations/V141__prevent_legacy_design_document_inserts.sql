-- New design files must enter the canonical Evidence ledger. Keep legacy
-- document rows readable for historical snapshots, but fail closed if an old
-- application instance or a future regression attempts the former write path.

CREATE OR REPLACE FUNCTION prevent_legacy_design_document_insert()
RETURNS trigger AS $$
BEGIN
  IF NEW.target_type='design_release' THEN
    RAISE EXCEPTION 'design revision files must be stored as evidence records';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prevent_legacy_design_document_insert ON documents;
CREATE TRIGGER trg_prevent_legacy_design_document_insert
BEFORE INSERT ON documents
FOR EACH ROW EXECUTE FUNCTION prevent_legacy_design_document_insert();
