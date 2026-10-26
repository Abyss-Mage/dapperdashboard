-- Employee photo posts: dealership key tags and customer-car pre/post inspections.
-- Photo bytes live in the server upload directory (content-addressed); this schema keeps the audit trail.
-- No retention period is approved yet, so nothing here expires automatically.
CREATE TABLE employee_upload_posts (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 dealership_id uuid NOT NULL,
 principal_id uuid NOT NULL REFERENCES principals(id),
 kind text NOT NULL CHECK(kind IN ('dealership_tag','customer_car')),
 customer_name text CHECK(customer_name IS NULL OR length(btrim(customer_name)) BETWEEN 2 AND 120),
 inspection_type text CHECK(inspection_type IS NULL OR inspection_type IN ('pre','post')),
 notes text NOT NULL DEFAULT '' CHECK(length(notes)<=2000),
 latitude double precision CHECK(latitude IS NULL OR latitude BETWEEN -90 AND 90),
 longitude double precision CHECK(longitude IS NULL OR longitude BETWEEN -180 AND 180),
 accuracy_meters real CHECK(accuracy_meters IS NULL OR accuracy_meters BETWEEN 0 AND 100000),
 captured_at timestamptz NOT NULL,
 request_key text NOT NULL CHECK(length(request_key) BETWEEN 16 AND 128),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(organization_id,location_id,principal_id,request_key),
 CHECK((latitude IS NULL)=(longitude IS NULL)),
 CHECK((kind='customer_car' AND customer_name IS NOT NULL AND inspection_type IS NOT NULL) OR (kind='dealership_tag' AND customer_name IS NULL AND inspection_type IS NULL)),
 FOREIGN KEY(organization_id,location_id,dealership_id) REFERENCES dealership_accounts(organization_id,location_id,id)
);
CREATE INDEX employee_upload_posts_owner_idx ON employee_upload_posts(principal_id,created_at DESC);
CREATE TABLE employee_upload_photos (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 post_id uuid NOT NULL REFERENCES employee_upload_posts(id) ON DELETE CASCADE,
 ordinal integer NOT NULL CHECK(ordinal BETWEEN 1 AND 10),
 storage_key text NOT NULL UNIQUE CHECK(storage_key ~ '^[0-9a-f]{64}\.jpg$'),
 sha256 bytea NOT NULL CHECK(octet_length(sha256)=32),
 size_bytes integer NOT NULL CHECK(size_bytes BETWEEN 1 AND 8388608),
 width integer NOT NULL CHECK(width BETWEEN 1 AND 12000),
 height integer NOT NULL CHECK(height BETWEEN 1 AND 12000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(post_id,ordinal)
);
