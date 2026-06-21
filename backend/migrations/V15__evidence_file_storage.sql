ALTER TABLE evidence_records
  ADD COLUMN IF NOT EXISTS storage_provider TEXT NOT NULL DEFAULT 'external',
  ADD COLUMN IF NOT EXISTS storage_key TEXT,
  ADD COLUMN IF NOT EXISTS checksum_sha256 TEXT;

ALTER TABLE evidence_records
  DROP CONSTRAINT IF EXISTS evidence_records_storage_provider_check,
  ADD CONSTRAINT evidence_records_storage_provider_check CHECK (storage_provider IN ('external', 'local'));

ALTER TABLE evidence_records
  DROP CONSTRAINT IF EXISTS evidence_records_storage_key_provider_check,
  ADD CONSTRAINT evidence_records_storage_key_provider_check CHECK (
    storage_key IS NULL OR storage_provider = 'local'
  );

ALTER TABLE evidence_records
  DROP CONSTRAINT IF EXISTS evidence_records_checksum_sha256_check,
  ADD CONSTRAINT evidence_records_checksum_sha256_check CHECK (
    checksum_sha256 IS NULL OR checksum_sha256 ~ '^[0-9a-f]{64}$'
  );

CREATE UNIQUE INDEX IF NOT EXISTS ux_evidence_records_local_storage_key
  ON evidence_records(storage_provider, storage_key)
  WHERE storage_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_evidence_records_storage_key
  ON evidence_records(storage_key)
  WHERE storage_key IS NOT NULL;
