-- V159 is already applied. Keep historical method values but do not require one for new links.
ALTER TABLE account_recovery_requests
  DROP CONSTRAINT account_recovery_requests_check1;
