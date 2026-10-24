-- Member profile (name, phone; email lives with the sign-in credential) and authenticator-app MFA.
-- TOTP secrets are stored encrypted (AES-256-GCM) with a server key; recovery codes are stored only as hashes.
CREATE TABLE member_profiles (
 principal_id uuid PRIMARY KEY REFERENCES principals(id),
 full_name text NOT NULL CHECK(length(btrim(full_name)) BETWEEN 2 AND 120),
 phone text NOT NULL CHECK(phone ~ '^\+[1-9][0-9]{6,14}$'),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE member_mfa (
 principal_id uuid PRIMARY KEY REFERENCES principals(id),
 secret_ciphertext bytea NOT NULL CHECK(octet_length(secret_ciphertext)>28),
 enabled_at timestamptz,
 last_used_step bigint NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE member_mfa_recovery_codes (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 principal_id uuid NOT NULL REFERENCES principals(id),
 code_hash bytea NOT NULL CHECK(octet_length(code_hash)=32),
 used_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(principal_id,code_hash)
);
CREATE TABLE member_security_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 principal_id uuid NOT NULL REFERENCES principals(id),
 action text NOT NULL CHECK(action IN ('profile_saved','mfa_setup_started','mfa_enabled','mfa_disabled','mfa_recovery_used')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
