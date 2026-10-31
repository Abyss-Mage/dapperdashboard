-- A franchise's own logo for its printed invoices. Raster images only (PNG, JPEG, WebP), small, stored with a digest.
CREATE TABLE location_logos (
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 content_type text NOT NULL CHECK (content_type IN ('image/png','image/jpeg','image/webp')),
 data bytea NOT NULL CHECK (octet_length(data) BETWEEN 100 AND 524288),
 sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
 updated_by uuid REFERENCES principals(id),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id, location_id),
 FOREIGN KEY (organization_id, location_id) REFERENCES locations(organization_id, id)
);
