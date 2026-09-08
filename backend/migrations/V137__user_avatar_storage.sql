ALTER TABLE users
  ADD COLUMN avatar_storage_provider TEXT,
  ADD COLUMN avatar_storage_key TEXT,
  ADD COLUMN avatar_mime_type TEXT,
  ADD COLUMN avatar_updated_at TIMESTAMPTZ;

ALTER TABLE users
  ADD CONSTRAINT users_avatar_storage_complete_check
    CHECK (
      (
        avatar_storage_provider IS NULL
        AND avatar_storage_key IS NULL
        AND avatar_mime_type IS NULL
        AND avatar_updated_at IS NULL
      )
      OR (
        avatar_storage_provider IN ('local','s3')
        AND avatar_storage_key IS NOT NULL
        AND avatar_storage_key = btrim(avatar_storage_key)
        AND avatar_storage_key <> ''
        AND avatar_mime_type = 'image/png'
        AND avatar_updated_at IS NOT NULL
      )
    );
