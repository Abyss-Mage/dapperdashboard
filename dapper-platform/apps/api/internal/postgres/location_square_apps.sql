-- error404 #50 (owner, 2026-11-13): each franchise can use its own Square application (Application ID and secret, per test/live),
-- used for its Square sign-in, token renewal and card form instead of the shared one. No row: the shared keys.
CREATE TABLE location_square_apps (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 environment text NOT NULL CHECK (environment IN ('sandbox','production')),
 application_id text NOT NULL CHECK (length(btrim(application_id)) BETWEEN 6 AND 200),
 secret_sealed bytea NOT NULL CHECK (octet_length(secret_sealed) > 28),
 updated_by uuid NOT NULL REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id, location_id, environment),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id) ON DELETE CASCADE
);
