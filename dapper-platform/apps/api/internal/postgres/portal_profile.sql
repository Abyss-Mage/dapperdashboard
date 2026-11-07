-- A signed-in customer's saved addresses (several, one default) and profile photo. Both belong to the customer record and are reached only through
-- the customer's own account link.
CREATE TABLE customer_addresses (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL, location_id uuid NOT NULL, customer_id uuid NOT NULL,
 label text NOT NULL CHECK(length(btrim(label)) BETWEEN 1 AND 40), address text NOT NULL CHECK(length(btrim(address)) BETWEEN 5 AND 300),
 is_default boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX customer_addresses_one_default ON customer_addresses(customer_id) WHERE is_default;
CREATE INDEX customer_addresses_customer ON customer_addresses(organization_id,location_id,customer_id,created_at);
CREATE TABLE customer_avatars (
 organization_id uuid NOT NULL, location_id uuid NOT NULL, customer_id uuid NOT NULL,
 content_type text NOT NULL CHECK(content_type IN ('image/jpeg','image/png')),
 data bytea NOT NULL CHECK(octet_length(data) BETWEEN 1 AND 2097152), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,location_id,customer_id),
 FOREIGN KEY(organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id) ON DELETE CASCADE
);
