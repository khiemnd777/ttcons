-- A singleton serializes challenges and their attempt limits across API instances.
CREATE TABLE developer_bootstrap_otp (
  singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
  code_mac BYTEA,
  expires_at TIMESTAMPTZ,
  sent_at TIMESTAMPTZ,
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  delivery_pending BOOLEAN NOT NULL DEFAULT false
);

INSERT INTO developer_bootstrap_otp (singleton) VALUES (true);
