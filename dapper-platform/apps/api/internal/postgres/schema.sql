-- Technical organization scopes are not an approved legal/franchise mapping.
CREATE TABLE organizations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 name text NOT NULL CHECK (length(trim(name)) BETWEEN 2 AND 120)
);
CREATE TABLE locations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL REFERENCES organizations(id),
 name text NOT NULL,
 UNIQUE (organization_id,id)
);
CREATE TABLE customers (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 name text NOT NULL CHECK (length(trim(name)) BETWEEN 2 AND 100),
 email text NOT NULL,
 phone text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (organization_id,location_id) REFERENCES locations(organization_id,id),
 UNIQUE (organization_id,location_id,id)
);
CREATE INDEX customers_scope_idx ON customers(organization_id,location_id,created_at,id);
CREATE TABLE vehicles (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 customer_id uuid NOT NULL,
 description text NOT NULL CHECK (length(trim(description)) BETWEEN 2 AND 120),
 category_description text NOT NULL DEFAULT '',
 identifier text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (organization_id,location_id,customer_id) REFERENCES customers(organization_id,location_id,id)
);
CREATE TABLE audit_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL,
 location_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 action text NOT NULL,
 entity_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (organization_id,location_id) REFERENCES locations(organization_id,id)
);
