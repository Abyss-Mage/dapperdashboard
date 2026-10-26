CREATE TABLE auth_sessions (
 token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash)=32),
 principal_id uuid NOT NULL REFERENCES principals(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 revoked_at timestamptz,
 CHECK (expires_at > created_at)
);
CREATE INDEX auth_sessions_principal_idx ON auth_sessions(principal_id);
