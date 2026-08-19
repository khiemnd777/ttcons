ALTER TABLE users
  ADD COLUMN IF NOT EXISTS recovery_email TEXT,
  ADD COLUMN IF NOT EXISTS recovery_email_verified_at TIMESTAMPTZ;

ALTER TABLE users
  ADD CONSTRAINT users_recovery_email_normalized_check
    CHECK (
      recovery_email IS NULL OR (
        recovery_email = lower(btrim(recovery_email))
        AND char_length(recovery_email) BETWEEN 3 AND 254
      )
    ),
  ADD CONSTRAINT users_recovery_email_verified_check
    CHECK (recovery_email IS NOT NULL OR recovery_email_verified_at IS NULL);

CREATE TABLE auth_email_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  target_email TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  invalidated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT auth_email_tokens_purpose_check
    CHECK (purpose IN ('verify_recovery_email', 'reset_password')),
  CONSTRAINT auth_email_tokens_hash_check
    CHECK (char_length(token_hash) = 64),
  CONSTRAINT auth_email_tokens_target_email_check
    CHECK (
      target_email = lower(btrim(target_email))
      AND char_length(target_email) BETWEEN 3 AND 254
    ),
  CONSTRAINT auth_email_tokens_terminal_state_check
    CHECK (consumed_at IS NULL OR invalidated_at IS NULL)
);

CREATE INDEX idx_auth_email_tokens_user_active
  ON auth_email_tokens(user_id, purpose, created_at DESC)
  WHERE consumed_at IS NULL AND invalidated_at IS NULL;

CREATE INDEX idx_auth_email_tokens_expiry
  ON auth_email_tokens(expires_at)
  WHERE consumed_at IS NULL AND invalidated_at IS NULL;
