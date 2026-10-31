-- Emailed password-reset links. Only a SHA-256 digest of the token is stored; a token works once and expires.
CREATE TABLE password_reset_tokens (
 token_hash bytea PRIMARY KEY CHECK(octet_length(token_hash)=32),
 principal_id uuid NOT NULL REFERENCES principals(id),
 expires_at timestamptz NOT NULL,
 used_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX password_reset_tokens_principal_idx ON password_reset_tokens(principal_id,created_at DESC);

ALTER TABLE member_security_events DROP CONSTRAINT member_security_events_action_check;
ALTER TABLE member_security_events ADD CONSTRAINT member_security_events_action_check
 CHECK(action IN ('profile_saved','mfa_setup_started','mfa_enabled','mfa_disabled','mfa_recovery_used','password_changed','email_changed','password_reset_requested','password_reset_completed'));
