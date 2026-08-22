CREATE TABLE IF NOT EXISTS bulk_deletion_batches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type TEXT NOT NULL,
  idempotency_key UUID NOT NULL UNIQUE,
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  preview_hash TEXT NOT NULL,
  selected_count INTEGER NOT NULL,
  deleted_count INTEGER NOT NULL DEFAULT 0,
  result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT bulk_deletion_batches_entity_type_check CHECK (entity_type IN ('partner','project')),
  CONSTRAINT bulk_deletion_batches_preview_hash_check CHECK (preview_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT bulk_deletion_batches_selected_count_check CHECK (selected_count BETWEEN 1 AND 100),
  CONSTRAINT bulk_deletion_batches_deleted_count_check CHECK (deleted_count BETWEEN 0 AND selected_count),
  CONSTRAINT bulk_deletion_batches_result_object_check CHECK (jsonb_typeof(result_json)='object')
);

CREATE INDEX IF NOT EXISTS idx_bulk_deletion_batches_actor_created
  ON bulk_deletion_batches(actor_id,entity_type,created_at DESC)
  WHERE actor_id IS NOT NULL;
