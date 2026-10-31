-- Private calendar subscription links (Apple Calendar, Google Calendar, Outlook). The link holds a random token; only its
-- SHA-256 digest is stored, so a database copy cannot reveal a working link. A link can be revoked at any time.
CREATE TABLE calendar_feeds (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 token_hash bytea NOT NULL UNIQUE CHECK(octet_length(token_hash)=32),
 kind text NOT NULL CHECK(kind IN ('franchise','employee')),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 label text NOT NULL DEFAULT '' CHECK(length(label) <= 80),
 created_at timestamptz NOT NULL DEFAULT now(),
 revoked_at timestamptz,
 FOREIGN KEY(organization_id, location_id) REFERENCES locations(organization_id, id)
);
CREATE INDEX calendar_feeds_owner_idx ON calendar_feeds(principal_id, kind) WHERE revoked_at IS NULL;
CREATE INDEX calendar_feeds_location_idx ON calendar_feeds(organization_id, location_id, kind) WHERE revoked_at IS NULL;
