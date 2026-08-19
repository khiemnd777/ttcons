CREATE INDEX idx_auth_email_tokens_retention
  ON auth_email_tokens(created_at)
  WHERE consumed_at IS NOT NULL OR invalidated_at IS NOT NULL;

CREATE INDEX idx_auth_email_tokens_expired_retention
  ON auth_email_tokens(created_at, expires_at)
  WHERE consumed_at IS NULL AND invalidated_at IS NULL;
