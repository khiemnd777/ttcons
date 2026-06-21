ALTER TABLE boq_items
  ADD COLUMN IF NOT EXISTS expected_unit_price NUMERIC(18,2),
  ADD COLUMN IF NOT EXISTS norm_unit_price NUMERIC(18,2),
  ADD COLUMN IF NOT EXISTS final_unit_price NUMERIC(18,2),
  ADD COLUMN IF NOT EXISTS unit_price_source TEXT NOT NULL DEFAULT 'expected',
  ADD COLUMN IF NOT EXISTS norm_price_locked BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS norm_price_locked_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS norm_price_locked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS norm_price_comment TEXT,
  ADD COLUMN IF NOT EXISTS final_price_locked BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS final_price_locked_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS final_price_locked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS final_price_comment TEXT,
  ADD COLUMN IF NOT EXISTS price_revision_no INTEGER NOT NULL DEFAULT 0;

UPDATE boq_items
SET expected_unit_price = COALESCE(expected_unit_price, unit_price, 0),
    unit_price_source = CASE
      WHEN final_price_locked THEN 'final'
      WHEN norm_price_locked THEN 'norm'
      ELSE 'expected'
    END
WHERE expected_unit_price IS NULL;

ALTER TABLE boq_items
  ALTER COLUMN expected_unit_price SET DEFAULT 0,
  ALTER COLUMN expected_unit_price SET NOT NULL;

ALTER TABLE boq_items
  DROP CONSTRAINT IF EXISTS boq_items_expected_unit_price_nonnegative,
  ADD CONSTRAINT boq_items_expected_unit_price_nonnegative CHECK (expected_unit_price >= 0);

ALTER TABLE boq_items
  DROP CONSTRAINT IF EXISTS boq_items_norm_unit_price_nonnegative,
  ADD CONSTRAINT boq_items_norm_unit_price_nonnegative CHECK (norm_unit_price IS NULL OR norm_unit_price >= 0);

ALTER TABLE boq_items
  DROP CONSTRAINT IF EXISTS boq_items_final_unit_price_nonnegative,
  ADD CONSTRAINT boq_items_final_unit_price_nonnegative CHECK (final_unit_price IS NULL OR final_unit_price >= 0);

ALTER TABLE boq_items
  DROP CONSTRAINT IF EXISTS boq_items_unit_price_source_check,
  ADD CONSTRAINT boq_items_unit_price_source_check CHECK (unit_price_source IN ('expected', 'norm', 'final'));

ALTER TABLE boq_items
  DROP CONSTRAINT IF EXISTS boq_items_norm_lock_consistency,
  ADD CONSTRAINT boq_items_norm_lock_consistency CHECK (
    (NOT norm_price_locked AND norm_price_locked_by IS NULL AND norm_price_locked_at IS NULL)
    OR
    (norm_price_locked AND norm_unit_price IS NOT NULL AND norm_price_locked_at IS NOT NULL)
  );

ALTER TABLE boq_items
  DROP CONSTRAINT IF EXISTS boq_items_final_lock_consistency,
  ADD CONSTRAINT boq_items_final_lock_consistency CHECK (
    (NOT final_price_locked AND final_price_locked_by IS NULL AND final_price_locked_at IS NULL)
    OR
    (final_price_locked AND norm_price_locked AND final_unit_price IS NOT NULL AND final_price_locked_at IS NOT NULL)
  );

ALTER TABLE boq_items
  DROP CONSTRAINT IF EXISTS boq_items_price_revision_nonnegative,
  ADD CONSTRAINT boq_items_price_revision_nonnegative CHECK (price_revision_no >= 0);

CREATE TABLE IF NOT EXISTS boq_unit_price_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  boq_item_id UUID NOT NULL REFERENCES boq_items(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  price_source TEXT NOT NULL,
  from_unit_price NUMERIC(18,2),
  to_unit_price NUMERIC(18,2) NOT NULL,
  revision_no INTEGER NOT NULL,
  comment TEXT,
  snapshot_json JSONB NOT NULL DEFAULT '{}',
  actor_id UUID REFERENCES users(id),
  actor_name TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT boq_unit_price_events_type_check CHECK (event_type IN ('expected_updated', 'norm_locked', 'final_locked')),
  CONSTRAINT boq_unit_price_events_source_check CHECK (price_source IN ('expected', 'norm', 'final')),
  CONSTRAINT boq_unit_price_events_revision_positive CHECK (revision_no > 0),
  CONSTRAINT boq_unit_price_events_to_price_nonnegative CHECK (to_unit_price >= 0)
);

CREATE INDEX IF NOT EXISTS idx_boq_unit_price_events_item_created
  ON boq_unit_price_events(boq_item_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_boq_unit_price_events_project_created
  ON boq_unit_price_events(project_id, created_at DESC);

ALTER TABLE procurement_request_items
  ADD COLUMN IF NOT EXISTS boq_price_source TEXT,
  ADD COLUMN IF NOT EXISTS boq_price_revision_no INTEGER,
  ADD COLUMN IF NOT EXISTS boq_price_event_id UUID REFERENCES boq_unit_price_events(id);

ALTER TABLE procurement_request_items
  DROP CONSTRAINT IF EXISTS procurement_request_items_boq_price_source_check,
  ADD CONSTRAINT procurement_request_items_boq_price_source_check CHECK (
    boq_price_source IS NULL OR boq_price_source IN ('expected', 'norm', 'final')
  );

ALTER TABLE procurement_request_items
  DROP CONSTRAINT IF EXISTS procurement_request_items_boq_price_revision_check,
  ADD CONSTRAINT procurement_request_items_boq_price_revision_check CHECK (
    boq_price_revision_no IS NULL OR boq_price_revision_no > 0
  );

CREATE OR REPLACE FUNCTION sync_boq_unit_price_lifecycle()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND OLD.final_price_locked THEN
    IF NEW.category IS DISTINCT FROM OLD.category
      OR NEW.work_item_name IS DISTINCT FROM OLD.work_item_name
      OR NEW.description IS DISTINCT FROM OLD.description
      OR NEW.unit IS DISTINCT FROM OLD.unit
      OR NEW.quantity IS DISTINCT FROM OLD.quantity
      OR NEW.material_id IS DISTINCT FROM OLD.material_id
      OR NEW.node_id IS DISTINCT FROM OLD.node_id
      OR NEW.expected_unit_price IS DISTINCT FROM OLD.expected_unit_price
      OR NEW.norm_unit_price IS DISTINCT FROM OLD.norm_unit_price
      OR NEW.final_unit_price IS DISTINCT FROM OLD.final_unit_price
      OR NEW.unit_price IS DISTINCT FROM OLD.unit_price
      OR NEW.norm_price_locked IS DISTINCT FROM OLD.norm_price_locked
      OR NEW.final_price_locked IS DISTINCT FROM OLD.final_price_locked THEN
      RAISE EXCEPTION 'boq item is final price locked';
    END IF;
  END IF;

  IF TG_OP = 'UPDATE' AND OLD.norm_price_locked AND NOT OLD.final_price_locked THEN
    IF NEW.expected_unit_price IS DISTINCT FROM OLD.expected_unit_price
      OR NEW.norm_unit_price IS DISTINCT FROM OLD.norm_unit_price
      OR NEW.norm_price_locked IS DISTINCT FROM OLD.norm_price_locked
      OR NEW.unit_price_source = 'expected' THEN
      RAISE EXCEPTION 'boq item norm price is locked';
    END IF;
  END IF;

  IF NEW.expected_unit_price IS NULL THEN
    NEW.expected_unit_price := COALESCE(NEW.unit_price, 0);
  END IF;

  IF TG_OP = 'UPDATE'
    AND NOT OLD.norm_price_locked
    AND NEW.expected_unit_price IS NOT DISTINCT FROM OLD.expected_unit_price
    AND NEW.unit_price IS DISTINCT FROM OLD.unit_price THEN
    NEW.expected_unit_price := COALESCE(NEW.unit_price, 0);
  END IF;

  IF COALESCE(NEW.final_price_locked, false) THEN
    IF NEW.final_unit_price IS NULL THEN
      RAISE EXCEPTION 'final unit price is required when final locked';
    END IF;
    NEW.unit_price := NEW.final_unit_price;
    NEW.unit_price_source := 'final';
  ELSIF COALESCE(NEW.norm_price_locked, false) THEN
    IF NEW.norm_unit_price IS NULL THEN
      RAISE EXCEPTION 'norm unit price is required when norm locked';
    END IF;
    NEW.unit_price := NEW.norm_unit_price;
    NEW.unit_price_source := 'norm';
  ELSE
    NEW.unit_price := NEW.expected_unit_price;
    NEW.unit_price_source := 'expected';
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_boq_items_unit_price_lifecycle ON boq_items;

CREATE TRIGGER trg_boq_items_unit_price_lifecycle
BEFORE INSERT OR UPDATE ON boq_items
FOR EACH ROW
EXECUTE FUNCTION sync_boq_unit_price_lifecycle();

CREATE OR REPLACE FUNCTION prevent_boq_unit_price_event_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'boq_unit_price_events are immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_boq_unit_price_events_immutable ON boq_unit_price_events;

CREATE TRIGGER trg_boq_unit_price_events_immutable
BEFORE UPDATE OR DELETE ON boq_unit_price_events
FOR EACH ROW
EXECUTE FUNCTION prevent_boq_unit_price_event_mutation();
