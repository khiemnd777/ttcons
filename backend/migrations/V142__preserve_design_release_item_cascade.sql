-- Keep released package membership immutable for direct item mutations while
-- preserving foreign-key cascades when an owning release/project is removed.

CREATE OR REPLACE FUNCTION enforce_design_release_item_immutability()
RETURNS trigger AS $$
DECLARE
  release_status TEXT;
BEGIN
  IF TG_OP='DELETE' AND pg_trigger_depth() > 1 THEN
    RETURN OLD;
  END IF;

  SELECT status INTO release_status
  FROM design_releases
  WHERE id=COALESCE(OLD.design_release_id, NEW.design_release_id);

  IF release_status='draft' THEN
    IF TG_OP='DELETE' THEN
      RETURN OLD;
    END IF;
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'released design revision items are immutable';
END;
$$ LANGUAGE plpgsql;
